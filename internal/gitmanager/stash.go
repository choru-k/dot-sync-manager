package gitmanager

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type stashSnapshot struct {
	Path      string        `json:"-"`
	Metadata  stashMetadata `json:"metadata"`
	CreatedAt time.Time     `json:"created_at"`
}

type stashMetadata struct {
	Files []stashFile `json:"files"`
}

type stashFile struct {
	Path    string      `json:"path"`
	Mode    os.FileMode `json:"mode"`
	Deleted bool        `json:"deleted"`
	HadBase bool        `json:"had_base"`
	Symlink bool        `json:"symlink"`
}

// ConflictError indicates local changes could not be automatically re-applied
// because the same paths changed remotely.
type ConflictError struct {
	Files       []string
	ConflictDir string
	StashPath   string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("gitmanager: conflicts restoring local changes for %s", strings.Join(e.Files, ", "))
}

func (gm *GitManager) createStash(status git.Status) (*stashSnapshot, error) {
	stashDir := filepath.Join(os.TempDir(), "dsm-stash")
	if err := os.MkdirAll(stashDir, 0o755); err != nil {
		return nil, fmt.Errorf("gitmanager: create stash dir: %w", err)
	}

	timestamp := time.Now().UTC()
	repoHint := sanitizeName(filepath.Base(gm.cfg.RepoPath))
	if repoHint == "" {
		repoHint = "repo"
	}
	snapshotDir := filepath.Join(stashDir, fmt.Sprintf("%s-%s", repoHint, timestamp.Format("20060102T150405Z0700")))
	localRoot := filepath.Join(snapshotDir, "local")
	baseRoot := filepath.Join(snapshotDir, "base")
	if err := os.MkdirAll(localRoot, 0o755); err != nil {
		return nil, fmt.Errorf("gitmanager: create local stash dir: %w", err)
	}
	if err := os.MkdirAll(baseRoot, 0o755); err != nil {
		return nil, fmt.Errorf("gitmanager: create base stash dir: %w", err)
	}

	var headTree *object.Tree
	if headRef, err := gm.repo.Head(); err == nil {
		if commit, err := gm.repo.CommitObject(headRef.Hash()); err == nil {
			if tree, err := commit.Tree(); err == nil {
				headTree = tree
			}
		}
	}

	meta := stashMetadata{}

	for path, fileStatus := range status {
		if fileStatus.Worktree == git.Unmodified && fileStatus.Staging == git.Unmodified {
			continue
		}

		entry := stashFile{
			Path: path,
		}
		fullPath := filepath.Join(gm.cfg.RepoPath, path)

		if fileStatus.Worktree == git.Deleted || fileStatus.Staging == git.Deleted {
			entry.Deleted = true
		} else {
			info, err := os.Lstat(fullPath)
			if err != nil {
				if os.IsNotExist(err) {
					entry.Deleted = true
				} else {
					return nil, fmt.Errorf("gitmanager: stat for stash: %w", err)
				}
			} else {
				if info.IsDir() {
					continue
				}
				entry.Mode = info.Mode()
				entry.Symlink = info.Mode()&os.ModeSymlink != 0
				dest := filepath.Join(localRoot, path)
				if err := copyFilesystemFile(fullPath, dest, entry.Symlink); err != nil {
					return nil, fmt.Errorf("gitmanager: stash local copy: %w", err)
				}
			}
		}

		if headTree != nil {
			if treeFile, err := headTree.File(path); err == nil {
				entry.HadBase = true
				if entry.Mode == 0 {
					entry.Mode = os.FileMode(treeFile.Mode)
				}
				dest := filepath.Join(baseRoot, path)
				if err := copyTreeFile(treeFile, dest); err != nil {
					return nil, fmt.Errorf("gitmanager: stash base copy: %w", err)
				}
			}
		}

		meta.Files = append(meta.Files, entry)
	}

	metaFile := filepath.Join(snapshotDir, "metadata.json")
	fileHandle, err := os.Create(metaFile)
	if err != nil {
		return nil, fmt.Errorf("gitmanager: stash metadata create: %w", err)
	}
	if err := json.NewEncoder(fileHandle).Encode(meta); err != nil {
		if closeErr := fileHandle.Close(); closeErr != nil {
			return nil, fmt.Errorf("gitmanager: stash metadata encode: %w (close error: %w)", err, closeErr)
		}
		return nil, fmt.Errorf("gitmanager: stash metadata encode: %w", err)
	}
	if err := fileHandle.Close(); err != nil {
		return nil, fmt.Errorf("gitmanager: stash metadata close: %w", err)
	}

	return &stashSnapshot{
		Path:      snapshotDir,
		Metadata:  meta,
		CreatedAt: timestamp,
	}, nil
}

func (gm *GitManager) applyStash(snapshot *stashSnapshot) error {
	localRoot := filepath.Join(snapshot.Path, "local")
	baseRoot := filepath.Join(snapshot.Path, "base")

	var conflictDir string
	pathSet := make(map[string]struct{})

	for _, file := range snapshot.Metadata.Files {
		target := filepath.Join(gm.cfg.RepoPath, file.Path)
		localPath := filepath.Join(localRoot, file.Path)
		basePath := filepath.Join(baseRoot, file.Path)

		targetInfo, targetErr := os.Lstat(target)
		targetExists := targetErr == nil
		if targetErr != nil && !os.IsNotExist(targetErr) {
			return fmt.Errorf("gitmanager: apply stash stat target: %w", targetErr)
		}

		switch {
		case file.Deleted:
			if !targetExists {
				continue
			}

			remoteBytes, err := readFileMaybeSymlink(target, targetInfo)
			if err != nil {
				return fmt.Errorf("gitmanager: apply stash read remote: %w", err)
			}

			baseBytes, err := os.ReadFile(basePath)
			if err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("gitmanager: apply stash read base: %w", err)
			}

			remoteUnchanged := file.HadBase && bytes.Equal(remoteBytes, baseBytes)
			if remoteUnchanged || (!file.HadBase && len(remoteBytes) == 0) {
				if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
					return fmt.Errorf("gitmanager: apply stash delete: %w", err)
				}
				continue
			}

			dir, err := ensureConflictDir(&conflictDir, snapshot.CreatedAt, gm.cfg.RepoPath)
			if err != nil {
				return err
			}
			if err := persistConflictArtifacts(dir, file.Path, remoteBytes, baseBytes, nil, file.Mode, true); err != nil {
				return err
			}
			pathSet[file.Path] = struct{}{}

		default:
			localBytes, err := os.ReadFile(localPath)
			if err != nil {
				return fmt.Errorf("gitmanager: apply stash read local: %w", err)
			}

			var remoteBytes []byte
			if targetExists {
				remoteBytes, err = readFileMaybeSymlink(target, targetInfo)
				if err != nil {
					return fmt.Errorf("gitmanager: apply stash read remote: %w", err)
				}
			}

			if targetExists && bytes.Equal(remoteBytes, localBytes) {
				if file.Mode != 0 && targetInfo.Mode() != file.Mode {
					if err := os.Chmod(target, file.Mode); err != nil {
						return fmt.Errorf("gitmanager: apply stash chmod: %w", err)
					}
				}
				continue
			}

			baseBytes, err := os.ReadFile(basePath)
			if err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("gitmanager: apply stash read base: %w", err)
			}

			remoteChanged := targetExists
			if targetExists {
				if len(baseBytes) > 0 {
					remoteChanged = !bytes.Equal(remoteBytes, baseBytes)
				}
				if bytes.Equal(remoteBytes, localBytes) {
					remoteChanged = false
				}
			} else {
				remoteChanged = file.HadBase
			}

			if remoteChanged {
				dir, err := ensureConflictDir(&conflictDir, snapshot.CreatedAt, gm.cfg.RepoPath)
				if err != nil {
					return err
				}
				if err := persistConflictArtifacts(dir, file.Path, remoteBytes, baseBytes, localBytes, file.Mode, false); err != nil {
					return err
				}
				pathSet[file.Path] = struct{}{}
				continue
			}

			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("gitmanager: apply stash mkdir: %w", err)
			}
			mode := normalizedPerm(file.Mode)
			if err := os.WriteFile(target, localBytes, mode); err != nil {
				return fmt.Errorf("gitmanager: apply stash write: %w", err)
			}
		}
	}

	if len(pathSet) > 0 {
		conflicts := make([]string, 0, len(pathSet))
		for path := range pathSet {
			conflicts = append(conflicts, path)
		}
		sort.Strings(conflicts)
		return &ConflictError{
			Files:       conflicts,
			ConflictDir: conflictDir,
			StashPath:   snapshot.Path,
		}
	}

	return os.RemoveAll(snapshot.Path)
}

func ensureConflictDir(current *string, createdAt time.Time, repoPath string) (string, error) {
	if *current != "" {
		return *current, nil
	}
	dir := filepath.Join(repoPath, ".dsm", "conflicts", createdAt.Format("20060102T150405Z0700"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("gitmanager: create conflict dir: %w", err)
	}
	*current = dir
	return dir, nil
}

func persistConflictArtifacts(baseDir, relPath string, remote, base, local []byte, localMode os.FileMode, localDeleted bool) error {
	conflictPath := filepath.Join(baseDir, relPath)
	if err := os.MkdirAll(filepath.Dir(conflictPath), 0o755); err != nil {
		return fmt.Errorf("gitmanager: conflict mkdir: %w", err)
	}

	if len(remote) > 0 {
		if err := os.WriteFile(conflictPath+".remote", remote, 0o644); err != nil {
			return fmt.Errorf("gitmanager: conflict remote copy: %w", err)
		}
	} else {
		if err := os.WriteFile(conflictPath+".remote", []byte("<<remote deleted>>\n"), 0o644); err != nil {
			return fmt.Errorf("gitmanager: conflict remote tombstone: %w", err)
		}
	}

	if len(base) > 0 {
		if err := os.WriteFile(conflictPath+".base", base, 0o644); err != nil {
			return fmt.Errorf("gitmanager: conflict base copy: %w", err)
		}
	}

	switch {
	case localDeleted:
		if err := os.WriteFile(conflictPath+".local", []byte("<<local deleted file>>\n"), 0o644); err != nil {
			return fmt.Errorf("gitmanager: conflict local tombstone: %w", err)
		}
	case local != nil:
		if err := os.WriteFile(conflictPath+".local", local, normalizedPerm(localMode)); err != nil {
			return fmt.Errorf("gitmanager: conflict local copy: %w", err)
		}
	default:
		if err := os.WriteFile(conflictPath+".local", []byte("<<local data missing>>\n"), 0o644); err != nil {
			return fmt.Errorf("gitmanager: conflict local missing note: %w", err)
		}
	}

	return nil
}

func copyFilesystemFile(source, destination string, symlink bool) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	if symlink {
		target, err := os.Readlink(source)
		if err != nil {
			return err
		}
		return os.Symlink(target, destination)
	}
	src, err := os.Open(source)
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()

	dst, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer func() { _ = dst.Close() }()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}
	return nil
}

func copyTreeFile(file *object.File, destination string) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	reader, err := file.Reader()
	if err != nil {
		return err
	}
	defer func() { _ = reader.Close() }()

	dst, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer func() { _ = dst.Close() }()

	if _, err := io.Copy(dst, reader); err != nil {
		return err
	}
	return nil
}

func readFileMaybeSymlink(path string, info os.FileInfo) ([]byte, error) {
	if info != nil && info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		if err != nil {
			return nil, err
		}
		return []byte(target), nil
	}
	return os.ReadFile(path)
}

func normalizedPerm(mode os.FileMode) os.FileMode {
	if mode == 0 {
		return 0o644
	}
	return mode
}

func sanitizeName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		string(os.PathSeparator), "_",
		" ", "_",
		":", "_",
		"\\", "_",
		"/", "_",
	)
	name = replacer.Replace(name)
	return name
}
