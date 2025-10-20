package cmd

// File permission constants for CLI operations
const (
	// defaultFilePerms allows owner write, group/others read (standard files)
	defaultFilePerms = 0644 // -rw-r--r--

	// defaultLogLines is the default number of log lines to show
	defaultLogLines = 50

	// daemonSleepTime is the time to wait between daemon operations
	daemonSleepTime = 1 // TODO: Use time.Duration when needed

	// Editor fallbacks by platform
	editorNotepad  = "notepad"
	editorTextEdit = "open -a TextEdit"
	editorNano     = "nano"
	editorVSCode   = "code"
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