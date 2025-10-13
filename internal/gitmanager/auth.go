package gitmanager

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/skeema/knownhosts"
	"golang.org/x/crypto/ssh"
)

func (c *Config) authMethod() (transport.AuthMethod, error) {
	switch c.AuthType {
	case AuthStrategyNone:
		return nil, nil
	case AuthStrategyHTTPS:
		return &githttp.BasicAuth{
			Username: c.Username,
			Password: c.Password,
		}, nil
	case AuthStrategySSH:
		username := c.Username
		if username == "" {
			username = "git"
		}
		publicKeys, err := gitssh.NewPublicKeysFromFile(username, c.SSHKeyPath, c.SSHKeyPassphrase)
		if err != nil {
			return nil, fmt.Errorf("gitmanager: ssh key: %w", err)
		}
		if c.KnownHostsPath != "" {
			callback, err := knownhosts.New(c.KnownHostsPath)
			if err != nil {
				return nil, fmt.Errorf("gitmanager: invalid known_hosts: %w", err)
			}
			publicKeys.HostKeyCallback = ssh.HostKeyCallback(callback)
		} else {
			if defaultPath := defaultKnownHostsPath(); defaultPath != "" {
				if _, err := os.Stat(defaultPath); err == nil {
					callback, err := knownhosts.New(defaultPath)
					if err == nil {
						publicKeys.HostKeyCallback = ssh.HostKeyCallback(callback)
					}
				}
			}
		}
		return publicKeys, nil
	default:
		return nil, fmt.Errorf("gitmanager: unsupported auth type %q", c.AuthType)
	}
}

func defaultKnownHostsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".ssh", "known_hosts")
}
