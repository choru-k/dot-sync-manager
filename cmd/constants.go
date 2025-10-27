package cmd

import "time"

// File permission constants for CLI operations
const (
	// defaultFilePerms allows owner write, group/others read (standard files)
	defaultFilePerms = 0644 // -rw-r--r--

	// defaultLogLines is the default number of log lines to show
	defaultLogLines = 50

	
	// dirPerms grants owner-level write access while keeping directories traversable by other users, matching standard 0755 expectations.
	dirPerms = 0755 // rwxr-xr-x so users can traverse synced directories

	// Editor fallbacks by platform
	editorNotepad  = "notepad"
	editorTextEdit = "open -a TextEdit"
	editorNano     = "nano"
	editorVSCode   = "code"
)

// Time function variable for testability
var timeNow = time.Now


// Configuration file name constants
const (
	// ConfigFileName is the PRD-standardized config filename
	ConfigFileName = ".sync-config.json"

	// LegacyConfigFileName is the historical config filename used before PRD standardization
	LegacyConfigFileName = ".dotfile-sync.json"
)

// Default .syncignore template with common exclusion patterns
const defaultIgnoreContent = `# .syncignore - Files and directories to exclude from sync
# Uses gitignore-style syntax
# Lines starting with # are comments

# Common files to exclude
*.log
*.tmp
*.swp
*.swo
*~

# Temporary directories
temp/
tmp/
.cache/

# Editor files
.vscode/settings.json
.idea/workspace.xml
*.sublime-*

# OS generated files
.DS_Store
.DS_Store?
._*
.Spotlight-V100
.Trashes
ehthumbs.db
Thumbs.db

# Build artifacts
build/
dist/
target/
bin/

# Node modules
node_modules/

# Python cache
__pycache__/
*.pyc
*.pyo
*.pyd

# Add your own patterns below
`
