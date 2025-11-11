package cmd

// Default .syncignore content
const defaultSyncIgnoreContent = `# Sensitive authentication files
.ssh/id_rsa
.ssh/id_rsa.pub
.ssh/id_ed25519
.ssh/id_ed25519.pub
*.pem
*.key

# DSM configuration (contains sensitive data)
.sync-config.json

# Cloud credentials
.aws/credentials
.aws/config
.gcp/credentials
.azure/credentials

# GPG keys
.gnupg/private-keys-v1.d/
.gnupg/*.key

# Other sensitive
.env
.env.local
secrets/

# Temporary files
*.tmp
*.log
*.swp
*~
.DS_Store
Thumbs.db

# Cache directories
.cache/
cache/
node_modules/
`
