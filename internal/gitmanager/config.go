package gitmanager

import (
	"errors"
	"fmt"
	"path/filepath"
)

const (
	// DefaultRemoteName is the default name for git remotes
	DefaultRemoteName = "origin"
	
	defaultRemoteName = DefaultRemoteName // Internal alias for backward compatibility
)

// AuthStrategy identifies the authentication mode for remote git operations.
type AuthStrategy string

const (
	// AuthStrategyNone performs unauthenticated operations (public repos/local remotes).
	AuthStrategyNone AuthStrategy = "none"
	// AuthStrategySSH uses an SSH private key for authentication.
	AuthStrategySSH AuthStrategy = "ssh"
	// AuthStrategyHTTPS uses username/password (or personal access token) over HTTPS.
	AuthStrategyHTTPS AuthStrategy = "https"
)

// Config captures the wiring for the GitManager.
type Config struct {
	// RepoPath is the absolute path of the working repository (e.g., ~/dotfiles).
	RepoPath string
	// RemoteURL is the URL used for cloning/pushing (ssh:// or https://).
	RemoteURL string
	// RemoteName defaults to origin when empty.
	RemoteName string
	// AuthorName is used for commit signature.
	AuthorName string
	// AuthorEmail is used for commit signature.
	AuthorEmail string

	// AuthType selects auth provider.
	AuthType AuthStrategy
	// Username required for HTTPS auth or SSH user override.
	Username string
	// Password (or access token) for HTTPS auth.
	Password string
	// SSHKeyPath to private key file for SSH auth.
	SSHKeyPath string
	// SSHKeyPassphrase for encrypted private keys (optional).
	SSHKeyPassphrase string
	// KnownHostsPath overrides the default known_hosts location (optional).
	KnownHostsPath string
}

// normalize applies default values to optional config fields.
// This should be called before validate().
func (c *Config) normalize() {
	// Set default remote name if remote URL is provided but name is empty
	if c.RemoteURL != "" && c.RemoteName == "" {
		c.RemoteName = defaultRemoteName
	}
}

func (c *Config) validate() error {
	if c == nil {
		return errors.New("gitmanager: config is nil")
	}
	if c.RepoPath == "" {
		return errors.New("gitmanager: repo path is required")
	}
	if !filepath.IsAbs(c.RepoPath) {
		return fmt.Errorf("gitmanager: repo path must be absolute (%s)", c.RepoPath)
	}
	// RemoteURL is optional - local-only repos don't need a remote
	// If remote URL is set, remote name must be set (normalized before validation)
	if c.RemoteURL != "" && c.RemoteName == "" {
		return errors.New("gitmanager: remote name is required when remote URL is set")
	}
	if c.AuthorName == "" || c.AuthorEmail == "" {
		return errors.New("gitmanager: author name and email are required")
	}

	switch c.AuthType {
	case AuthStrategyNone:
		// nothing to validate
	case AuthStrategyHTTPS:
		if c.Username == "" || c.Password == "" {
			return errors.New("gitmanager: https auth requires username and password/token")
		}
	case AuthStrategySSH:
		if c.SSHKeyPath == "" {
			return errors.New("gitmanager: ssh auth requires private key path")
		}
	default:
		return fmt.Errorf("gitmanager: unknown auth type %q", c.AuthType)
	}

	return nil
}
