package gitmanager

import (
	"errors"
	"fmt"
	"path/filepath"
)

const (
	defaultRemoteName = "origin"
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
	if c.RemoteURL == "" {
		return errors.New("gitmanager: remote URL is required")
	}
	if c.RemoteName == "" {
		c.RemoteName = defaultRemoteName
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
