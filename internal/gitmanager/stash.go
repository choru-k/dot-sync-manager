package gitmanager

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
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
	if err := os.MkdirAll(snapshotDir, 0o755); err != nil {
		return nil, fmt.Errorf("gitmanager: create snapshot dir: %w", err)
	}

	meta := stashMetadata{}

	for path, fileStatus := range status {
		if fileStatus.Worktree == git.Unmodified && fileStatus.Staging == git.Unmodified {
			continue
		}

		fullPath := filepath.Join(gm.cfg.RepoPath, path)
		entry := stashFile{
			Path: path,
		}

		if fileStatus.Worktree == git.Deleted || fileStatus.Staging == git.Deleted {
			entry.Deleted = true
			meta.Files = append(meta.Files, entry)
			continue
		}

		info, err := os.Stat(fullPath)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("gitmanager: stat for stash: %w", err)
		}

		if err == nil && info.IsDir() {
			// skip directories; individual files will be captured.
			continue
		}

		if err == nil {
			entry.Mode = info.Mode()
		}

		dest := filepath.Join(snapshotDir, path)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return nil, fmt.Errorf("gitmanager: stash mkdir: %w", err)
		}

		src, err := os.Open(fullPath)
		if err != nil {
			return nil, fmt.Errorf("gitmanager: stash open: %w", err)
		}

		destFile, err := os.Create(dest)
		if err != nil {
			src.Close()
			return nil, fmt.Errorf("gitmanager: stash create: %w", err)
		}

		if _, err := io.Copy(destFile, src); err != nil {
			src.Close()
			destFile.Close()
			return nil, fmt.Errorf("gitmanager: stash copy: %w", err)
		}

		src.Close()
		destFile.Close()

		meta.Files = append(meta.Files, entry)
	}

	metaFile := filepath.Join(snapshotDir, "metadata.json")
	handle, err := os.Create(metaFile)
	if err != nil {
		return nil, fmt.Errorf("gitmanager: stash metadata create: %w", err)
	}
	if err := json.NewEncoder(handle).Encode(meta); err != nil {
		handle.Close()
		return nil, fmt.Errorf("gitmanager: stash metadata encode: %w", err)
	}
	handle.Close()

	return &stashSnapshot{
		Path:      snapshotDir,
		Metadata:  meta,
		CreatedAt: timestamp,
	}, nil
}

func (gm *GitManager) applyStash(snapshot *stashSnapshot) error {
	for _, file := range snapshot.Metadata.Files {
		target := filepath.Join(gm.cfg.RepoPath, file.Path)
		if file.Deleted {
			if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("gitmanager: apply stash delete: %w", err)
			}
			continue
		}

		source := filepath.Join(snapshot.Path, file.Path)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("gitmanager: apply stash mkdir: %w", err)
		}

		src, err := os.Open(source)
		if err != nil {
			return fmt.Errorf("gitmanager: apply stash open: %w", err)
		}
		dst, err := os.Create(target)
		if err != nil {
			src.Close()
			return fmt.Errorf("gitmanager: apply stash create: %w", err)
		}

		if _, err := io.Copy(dst, src); err != nil {
			src.Close()
			dst.Close()
			return fmt.Errorf("gitmanager: apply stash copy: %w", err)
		}

		src.Close()
		if err := dst.Close(); err != nil {
			return fmt.Errorf("gitmanager: apply stash close: %w", err)
		}

		if file.Mode != 0 {
			if err := os.Chmod(target, file.Mode); err != nil {
				return fmt.Errorf("gitmanager: apply stash chmod: %w", err)
			}
		}
	}

	return os.RemoveAll(snapshot.Path)
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
