#!/bin/bash
# SSH key generation script for E2E testing
# Generates test-specific SSH keys for secure git operations

set -euo pipefail

# Configuration
KEY_DIR="/app/ssh-keys"
KEY_NAME="test_key"
KEY_TYPE="ed25519"
KEY_COMMENT="test-key-for-dsm-e2e-testing"

# Ensure SSH key directory exists
mkdir -p "$KEY_DIR"
chmod 700 "$KEY_DIR"

# Generate SSH key pair if it doesn't exist
if [ ! -f "$KEY_DIR/$KEY_NAME" ]; then
    echo "🔑 Generating test SSH key pair..."

    # Generate private key
    ssh-keygen -t "$KEY_TYPE" \
        -f "$KEY_DIR/$KEY_NAME" \
        -N "" \
        -C "$KEY_COMMENT"

    # Set appropriate permissions
    chmod 600 "$KEY_DIR/$KEY_NAME"
    chmod 644 "$KEY_DIR/$KEY_NAME.pub"

    echo "✅ SSH key pair generated successfully"
else
    echo "✅ SSH key pair already exists"
fi

# Display public key for debugging (in test mode)
if [ "${TEST_SHOW_SSH_KEY:-false}" = "true" ]; then
    echo "📋 Public key:"
    cat "$KEY_DIR/$KEY_NAME.pub"
fi

# Create SSH config for test environment
cat > "$KEY_DIR/config" << EOF
# SSH config for DSM E2E testing
Host git-remote
    HostName git-remote
    Port 22
    User git
    IdentityFile $KEY_DIR/$KEY_NAME
    StrictHostKeyChecking no
    UserKnownHostsFile /dev/null
    IdentitiesOnly yes
    PasswordAuthentication no

Host *
    StrictHostKeyChecking no
    UserKnownHostsFile /dev/null
EOF

chmod 600 "$KEY_DIR/config"

echo "🔐 SSH configuration created"