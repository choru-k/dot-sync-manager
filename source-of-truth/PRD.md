# **Complete Product Requirements Document (PRD)**
## **Dotfile Sync Manager**

---

## **1. Project Overview**

### **Product Name**
Dotfile Sync Manager (DSM)

### **Purpose**
Automatically synchronize dotfiles between work and personal laptops using Git, with support for exclude patterns, conflict detection, and a lightweight GUI for monitoring sync status.

### **Target Users**
Developers who work across multiple machines and want to maintain consistent development environments without manual file management.

### **Key Value Proposition**
- **Automatic syncing** - No manual git commands needed
- **Visual status** - System tray icon shows sync status at a glance
- **Safe conflict handling** - Never lose data, always backed up
- **Lightweight** - Small binary (~10MB), low memory usage
- **Cross-platform** - Works on macOS, Linux, and Windows

---

## **2. Goals & Success Metrics**

### **Primary Goals**
1. Eliminate manual dotfile syncing between machines
2. Provide real-time visual feedback on sync status
3. Prevent data loss through smart conflict detection
4. Maintain consistent development environments across machines

### **Success Metrics**
- Sync latency < 5 minutes between machines
- Zero data loss from conflicts
- 95% of syncs complete without user intervention
- < 60MB memory usage
- < 3 second app startup time

---

## **3. Technical Architecture**

### **Technology Stack**

| Component | Technology | Rationale |
|-----------|-----------|-----------|
| **Framework** | Wails v3 | Lightweight, Go-based, fast compilation |
| **Backend** | Go 1.21+ | Simple, fast, excellent file I/O |
| **Frontend** | Vanilla HTML/CSS/JS | Ultra-lightweight, no build complexity |
| **CSS** | Pico CSS (optional) | Beautiful defaults, ~10KB, classless |
| **File Watching** | `fsnotify` | Go standard library, reliable |
| **Git Operations** | `go-git` | Pure Go, no git binary dependency |
| **Notifications** | Wails runtime | Cross-platform native notifications |

### **Why This Stack?**
- ✅ **Fast development** - Go is simple, Wails has quick compile times (~10-15 seconds)
- ✅ **Lightweight binary** - ~8-10MB total
- ✅ **Low memory** - ~60-90MB runtime
- ✅ **Simple maintenance** - No complex build pipelines
- ✅ **Cross-platform** - Single codebase for all platforms

---

### **System Architecture**

```
┌─────────────────────────────────────────┐
│      System Tray Application            │
│   (Wails GUI - HTML/CSS/JS)             │
│   - Status icon & menu                   │
│   - Status window                        │
│   - Settings window                      │
│   - Conflict resolution UI               │
└────────────────┬────────────────────────┘
                 │
                 │ Go API Bindings
                 │
┌────────────────▼────────────────────────┐
│         Sync Service (Go)                │
│  ┌────────────────────────────────────┐ │
│  │  File Watcher (fsnotify)           │ │
│  └────────────────────────────────────┘ │
│  ┌────────────────────────────────────┐ │
│  │  Git Manager (go-git)              │ │
│  └────────────────────────────────────┘ │
│  ┌────────────────────────────────────┐ │
│  │  Conflict Detector                 │ │
│  └────────────────────────────────────┘ │
│  ┌────────────────────────────────────┐ │
│  │  Symlink Manager                   │ │
│  └────────────────────────────────────┘ │
└────────────────┬────────────────────────┘
                 │
                 │ Read/Write
                 │
┌────────────────▼────────────────────────┐
│     ~/dotfiles/ (Git Repository)        │
│  - Actual dotfiles                      │
│  - .syncignore                          │
│  - .sync-config.json                    │
│  - .git/                                │
└─────────────────────────────────────────┘
                 │
                 │ Symlinks
                 │
┌────────────────▼────────────────────────┐
│        Home Directory (~/)               │
│  .bashrc -> ~/dotfiles/bashrc           │
│  .zshrc -> ~/dotfiles/zshrc             │
│  .config/nvim -> ~/dotfiles/config/nvim │
└─────────────────────────────────────────┘
```

---

### **Repository Structure**

```
~/dotfiles/                      # Git repository
├── .git/                        # Git data
├── .syncignore                  # Exclude patterns
├── .sync-config.json            # App configuration
├── README.md                    # Documentation
├── bashrc                       # Dotfiles (no leading dot)
├── zshrc
├── gitconfig
├── vimrc
├── config/                      # Directory dotfiles
│   ├── nvim/
│   │   ├── init.vim
│   │   └── plugins.vim
│   ├── kitty/
│   │   └── kitty.conf
│   └── tmux/
│       └── tmux.conf
└── ssh/
    └── config
```

**Symlinks in home directory:**
```
~/
├── .bashrc -> ~/dotfiles/bashrc
├── .zshrc -> ~/dotfiles/zshrc
├── .gitconfig -> ~/dotfiles/gitconfig
├── .config/
│   ├── nvim -> ~/dotfiles/config/nvim
│   └── kitty -> ~/dotfiles/config/kitty
└── .ssh/
    └── config -> ~/dotfiles/ssh/config
```

---

## **4. Core Features**

### **Feature 1: Automatic File Watching & Sync**

**Description:** Monitor dotfiles for changes and automatically sync via Git.

**User Story:**
> "As a developer, when I edit my `.bashrc` on my work laptop, I want it to automatically sync to Git so it's available on my personal laptop within minutes, without any manual commands."

**Requirements:**

**File Watching:**
- Monitor all files/directories in `~/dotfiles/` for changes
- Detect: file modifications, creations, deletions, renames
- Respect `.syncignore` patterns
- Use debouncing: wait 30 seconds after last change before syncing
- Handle rapid successive changes efficiently

**Auto-Commit:**
- When changes detected (after debounce):
  - Stage all modified files
  - Create commit with timestamp: `"Auto-sync: 2025-10-13 14:30:22"`
  - Include list of changed files in commit message
  - Push to remote repository
- **Inactivity Guard:** The sync service runs commit/push inside a repository-wide debounce keyed by the repo path. Every fsnotify event resets the timer, guaranteeing at least 30 seconds of quiescence before committing. When the timer fires, it re-checks `git status`; skip the run if nothing changed so we never stage half-written files that are still being edited.

**Auto-Pull:**
- Pull from remote every 5 minutes (configurable)
- Before pulling, stash any uncommitted local changes
- After pulling, reapply stashed changes if any
- Detect conflicts (see Feature 3)

**Background Service:**
- Run as daemon/background service
- Start automatically on system boot (optional)
- Low CPU usage when idle (< 1%)
- Automatic reconnection on network loss

**Go Implementation Structure:**
```go
type SyncService struct {
    watcher    *fsnotify.Watcher
    gitManager *GitManager
    config     *Config
    debouncer  *Debouncer
}

func (s *SyncService) Start() {
    go s.watchFiles()
    go s.autoPull()
}
```

---

### **Feature 2: Exclude/Ignore Patterns**

**Description:** Flexible pattern-based file exclusion.

**User Story:**
> "As a developer, I want to exclude my SSH private keys and AWS credentials from syncing, so sensitive data never leaves my machine."

**Requirements:**

**`.syncignore` File:**
- Located at `~/dotfiles/.syncignore`
- Uses gitignore-style syntax
- Supports:
  - Exact paths: `ssh/id_rsa`
  - Wildcards: `*.log`, `*.tmp`, `*.key`
  - Directory patterns: `cache/`, `.cache/`
  - Negation: `!important.log`
  - Comments: `# This is a comment`
  - Blank lines (ignored)

**Default Exclude Patterns:**
```
# Sensitive authentication files
.ssh/id_rsa
.ssh/id_rsa.pub
.ssh/id_ed25519
.ssh/id_ed25519.pub
*.pem
*.key

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
```

**Pattern Matching:**
- Check patterns before watching files
- Check patterns before committing
- Fast matching algorithm (pre-compile patterns)
- Case-sensitive on Linux/Mac, insensitive on Windows

**UI Integration:**
- Settings window has "Edit Exclude Patterns" button
- Opens `.syncignore` in default text editor
- Warns if excluding all files
- Validates pattern syntax on save

---

### **Feature 3: Conflict Detection & Resolution**

**Description:** Detect Git merge conflicts and pause sync until manually resolved.

**User Story:**
> "As a developer, when I've edited `.bashrc` on both laptops and they conflict, I want the sync to pause and notify me, so I can manually choose which version to keep without losing either."

**Requirements:**

**Conflict Detection:**
- Before pulling, check if merge would cause conflicts
- Check using `git merge --no-commit --no-ff` (dry run)
- If conflicts detected:
  - Stop auto-sync immediately
  - Create timestamped backup: `~/dotfiles/.backup-YYYYMMDD-HHMMSS/`
  - Keep local version (don't overwrite)
  - Send notification
  - Update tray icon to red
  - Log to `~/.dotfile-sync.log`

**Notification:**
```
Title: ⚠️ Dotfile Sync Conflict
Body: Conflict detected in 2 files:
  - .bashrc
  - config/nvim/init.vim

Sync paused. Click to resolve.
```

**Conflict Resolution UI:**
```
┌─────────────────────────────────────────┐
│  ⚠️  Sync Conflict Detected              │
├─────────────────────────────────────────┤
│                                          │
│  The following files have conflicts:     │
│                                          │
│  ☑ .bashrc                               │
│  ☑ config/nvim/init.vim                  │
│                                          │
│  A backup has been created at:           │
│  ~/dotfiles/.backup-20251013-143022/     │
│                                          │
│  Auto-sync is paused.                    │
│  Please resolve conflicts manually,      │
│  then click "Resume Sync".               │
│                                          │
│  [ Open Dotfiles Folder ]                │
│  [ Open Backup Folder ]                  │
│  [ View Diff ]                           │
│                                          │
│  After resolving conflicts:              │
│  [ ✓ Mark as Resolved & Resume ]         │
│                                          │
└─────────────────────────────────────────┘
```

**Resolution Process:**
1. User opens dotfiles folder
2. User manually edits conflicted files
3. User commits resolved version
4. User clicks "Mark as Resolved & Resume"
5. App validates conflicts are resolved
6. Resume auto-sync
7. Change tray icon back to green

**Conflict Log:**
```
[2025-10-13 14:30:22] CONFLICT DETECTED
Machine: work-laptop
Files in conflict:
  - bashrc (local: 2025-10-13 14:25:10, remote: 2025-10-13 14:28:00)
  - config/nvim/init.vim
Backup: ~/dotfiles/.backup-20251013-143022/
Status: Paused

[2025-10-13 14:45:33] CONFLICT RESOLVED
User manually resolved conflicts
Commit: abc123def "Resolved merge conflicts"
Status: Resumed
```

---

### **Feature 4: Symlink Management**

**Description:** Automatically create and manage symlinks between `~/dotfiles/` and home directory.

**User Story:**
> "As a developer setting up a new machine, I want to run one command that automatically creates all my dotfile symlinks, so I don't have to remember and type 20+ `ln -s` commands."

**Requirements:**

**Initial Setup Command: `dsm init`**

When user runs initialization:
1. Check if `~/dotfiles/` exists and is a Git repo
2. If not, prompt for Git repository URL or create new
3. Scan all files in `~/dotfiles/`
4. For each file/directory (respecting `.syncignore`):
   - Determine target location using mapping rules
   - Check if target exists:
     - **If symlink already exists:** Skip
     - **If file/directory exists:** Back up to `~/dotfiles/.backup-original/FILENAME`
     - **If doesn't exist:** Create parent directories if needed
   - Create symlink
5. Report results: X symlinks created, Y files backed up

**Mapping Rules:**

Default mapping (add leading dot):
```
~/dotfiles/bashrc        -> ~/.bashrc
~/dotfiles/zshrc         -> ~/.zshrc
~/dotfiles/gitconfig     -> ~/.gitconfig
~/dotfiles/vimrc         -> ~/.vimrc
```

Custom mappings in `.sync-config.json`:
```json
{
  "mappings": {
    "bashrc": "~/.bashrc",
    "zshrc": "~/.zshrc",
    "config/nvim": "~/.config/nvim",
    "config/kitty": "~/.config/kitty",
    "ssh/config": "~/.ssh/config"
  }
}
```

**Add File Command: `dsm add <filepath>`**

Example: `dsm add ~/.vimrc`

Process:
1. Resolve absolute path
2. Check if file exists
3. Determine relative path in dotfiles repo
4. Move file from home to `~/dotfiles/`
5. Create symlink
6. Git add the file
7. Auto-commit: `"Add .vimrc to dotfiles"`
8. Show success message

**Remove File Command: `dsm remove <filepath>`**

Example: `dsm remove ~/.vimrc`

Process:
1. Check if it's a symlink to dotfiles repo
2. Remove symlink
3. Optionally:
   - Keep file in `~/dotfiles/` (default)
   - Or delete from repo completely
4. Show success message

**List Command: `dsm list`**

Shows all currently managed files:
```
Tracked Dotfiles (23 files, 5 directories):

Files:
  .bashrc -> ~/dotfiles/bashrc
  .zshrc -> ~/dotfiles/zshrc
  .gitconfig -> ~/dotfiles/gitconfig
  .vimrc -> ~/dotfiles/vimrc

Directories:
  .config/nvim -> ~/dotfiles/config/nvim
  .config/kitty -> ~/dotfiles/config/kitty
  .ssh/config -> ~/dotfiles/ssh/config
```

**Validation:**
- Before creating symlinks, validate:
  - Source file exists in `~/dotfiles/`
  - Target location is writable
  - Parent directories can be created
  - No circular symlinks
- After creating, verify symlink is valid

---

### **Feature 5: System Tray GUI**

**Description:** Lightweight system tray application for monitoring and controlling sync.

**User Story:**
> "As a developer, I want a system tray icon that shows me at a glance whether my dotfiles are synced, and lets me quickly access sync controls without opening a terminal."

**Requirements:**

#### **A. System Tray Icon**

**Icon States:**
- 🟢 **Green:** Everything synced, working normally
- 🟡 **Yellow:** Sync in progress
- 🔴 **Red:** Conflict detected or error
- ⚪ **Gray:** Daemon stopped/disabled

**Tooltip on Hover:**
```
Dotfile Sync Manager
Status: ✓ Synced
Last update: 2 minutes ago
```

#### **B. Tray Menu (Right-click)**

```
┌─────────────────────────────────────┐
│ Dotfile Sync Manager                │
├─────────────────────────────────────┤
│ Status: ✓ Synced (2 min ago)        │
│ Files tracked: 23                    │
├─────────────────────────────────────┤
│ 📊 Show Status                       │
│ 🔄 Sync Now                          │
│ ⚙️  Settings                         │
│ 📝 View Log                          │
│ 📂 Open Dotfiles Folder              │
├─────────────────────────────────────┤
│ ⏸️  Pause Auto-Sync                  │
│ 🚪 Quit                              │
└─────────────────────────────────────┘
```

#### **C. Status Window**

When clicking "Show Status" or double-clicking tray icon:

```html
┌──────────────────────────────────────────┐
│  Dotfile Sync Manager              [X]   │
├──────────────────────────────────────────┤
│                                           │
│  Machine: Work Laptop                     │
│  Status: ● Synced                         │
│  Repository: ~/dotfiles                   │
│                                           │
│  ┌─────────────────────────────────────┐ │
│  │ Last Activity                       │ │
│  ├─────────────────────────────────────┤ │
│  │ Push: 2 minutes ago                 │ │
│  │ Pull: 5 minutes ago                  │ │
│  │ Files Changed: 2                     │ │
│  └─────────────────────────────────────┘ │
│                                           │
│  ┌─────────────────────────────────────┐ │
│  │ Recent Changes                      │ │
│  ├─────────────────────────────────────┤ │
│  │ • bashrc (2 min ago)                │ │
│  │ • config/nvim/init.vim (15 min ago) │ │
│  │ • gitconfig (1 hour ago)            │ │
│  └─────────────────────────────────────┘ │
│                                           │
│  ┌─────────────────────────────────────┐ │
│  │ Statistics                          │ │
│  ├─────────────────────────────────────┤ │
│  │ Tracked Files: 18 files              │ │
│  │ Tracked Dirs: 5 directories          │ │
│  │ Total Commits: 147                   │ │
│  │ Uptime: 3 days, 5 hours              │ │
│  └─────────────────────────────────────┘ │
│                                           │
│  [ Open Folder ]  [ Sync Now ]  [ Log ]  │
│                                           │
└──────────────────────────────────────────┘
```

#### **D. Settings Window**

When clicking "Settings":

```html
┌──────────────────────────────────────────┐
│  Settings                          [X]   │
├──────────────────────────────────────────┤
│                                           │
│  General                                  │
│  ☑ Start at system startup                │
│  ☑ Show notifications                     │
│  ☑ Auto-sync enabled                      │
│                                           │
│  Sync Intervals                           │
│  Pull every: [5] minutes                  │
│  Push delay: [30] seconds                 │
│                                           │
│  Machine Configuration                    │
│  Machine name: [Work Laptop    ]          │
│  Git email: [user@work.com     ]          │
│                                           │
│  Repository                               │
│  Path: [~/dotfiles          ] [Browse]    │
│  Remote: [origin                    ]     │
│  Branch: [main                      ]     │
│                                           │
│  Files & Patterns                         │
│  [ Edit .syncignore ]                     │
│  [ Edit Mappings ]                        │
│  [ Add File to Sync ]                     │
│                                           │
│  Advanced                                 │
│  ☐ Enable debug logging                   │
│  ☐ Keep conflict backups for [7] days     │
│                                           │
│  [ Save ]  [ Cancel ]  [ Reset Defaults ] │
│                                           │
└──────────────────────────────────────────┘
```

#### **E. Frontend Implementation**

**File Structure:**
```
frontend/
├── index.html          # Main window
├── settings.html       # Settings window
├── conflict.html       # Conflict resolution window
├── css/
│   ├── pico.min.css   # Optional CSS framework
│   └── styles.css     # Custom styles
└── js/
    ├── app.js         # Main app logic
    ├── settings.js    # Settings logic
    └── conflict.js    # Conflict handling
```

**index.html (Simplified):**
```html
<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <title>Dotfile Sync Manager</title>
  <link rel="stylesheet" href="css/pico.min.css">
  <link rel="stylesheet" href="css/styles.css">
</head>
<body>
  <main class="container">
    <h1>Dotfile Sync Manager</h1>

    <article>
      <header>
        <span id="status-icon">🟢</span>
        <strong id="status-text">Synced</strong>
      </header>
      <p>Last push: <span id="last-push">2 minutes ago</span></p>
      <p>Last pull: <span id="last-pull">5 minutes ago</span></p>
      <p>Files changed: <span id="files-changed">2</span></p>
    </article>

    <article>
      <header>Recent Changes</header>
      <ul id="recent-changes">
        <li>bashrc (2 min ago)</li>
        <li>config/nvim/init.vim (15 min ago)</li>
      </ul>
    </article>

    <div class="grid">
      <button onclick="openFolder()">Open Folder</button>
      <button onclick="syncNow()">Sync Now</button>
      <button onclick="viewLog()">View Log</button>
    </div>
  </main>

  <script src="wailsjs/runtime/runtime.js"></script>
  <script src="js/app.js"></script>
</body>
</html>
```

**app.js (Simplified):**
```javascript
// Wails provides window.go namespace for Go function calls

async function syncNow() {
  const btn = event.target;
  btn.disabled = true;
  btn.textContent = 'Syncing...';

  try {
    await window.go.main.App.ManualSync();
    updateStatus('✅ Synced successfully');
  } catch (error) {
    updateStatus('❌ Sync failed: ' + error);
  } finally {
    btn.disabled = false;
    btn.textContent = 'Sync Now';
  }
}

async function openFolder() {
  await window.go.main.App.OpenDotfilesFolder();
}

async function viewLog() {
  await window.go.main.App.OpenLog();
}

function updateStatus(message) {
  document.getElementById('status-text').textContent = message;
}

// Called by Go backend to update UI
function onSyncStatusChanged(status) {
  document.getElementById('status-text').textContent = status.message;
  document.getElementById('status-icon').textContent = status.icon;
  document.getElementById('last-push').textContent = status.lastPush;
  document.getElementById('last-pull').textContent = status.lastPull;
}

// Periodic status updates
setInterval(async () => {
  const status = await window.go.main.App.GetStatus();
  onSyncStatusChanged(status);
}, 5000);
```

**CSS (styles.css):**
```css
:root {
  --status-green: #10b981;
  --status-yellow: #f59e0b;
  --status-red: #ef4444;
  --status-gray: #6b7280;
}

#status-icon {
  font-size: 1.5em;
  margin-right: 0.5em;
}

.status-synced { color: var(--status-green); }
.status-syncing { color: var(--status-yellow); }
.status-error { color: var(--status-red); }
.status-stopped { color: var(--status-gray); }

#recent-changes {
  max-height: 200px;
  overflow-y: auto;
}

article {
  margin-bottom: 1rem;
}

.grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 0.5rem;
}
```

---

### **Feature 6: Notifications**

**Description:** Desktop notifications for important events.

**User Story:**
> "As a developer, I want to be notified when my dotfiles sync successfully or when conflicts occur, so I stay informed without constantly checking the app."

**Requirements:**

**Notification Types:**

**1. Sync Success (Optional, can be disabled):**
```
Title: ✅ Dotfiles Synced
Body: 2 files synced successfully
Auto-dismiss: 3 seconds
```

**2. Sync Failure:**
```
Title: ❌ Sync Failed
Body: Failed to push changes. Check network connection.
Action: Click to view details
Persistent: Until dismissed
```

**3. Conflict Detected:**
```
Title: ⚠️ Conflict Detected
Body: Conflicts in .bashrc and 1 other file
Action: Click to resolve
Persistent: Until dismissed
```

**4. Pull Received:**
```
Title: 📥 New Changes Pulled
Body: 3 files updated from remote
Auto-dismiss: 5 seconds
```

**Settings:**
- ☑ Enable notifications
- ☑ Show success notifications
- ☑ Show pull notifications
- ☑ Play sound on conflict (optional)

---

## **5. Configuration**

### **`.sync-config.json`**

Located at `~/dotfiles/.sync-config.json`:

```json
{
  "version": "1.0",
  "machine": "work-laptop",

  "git": {
    "remote": "origin",
    "branch": "main",
    "user_name": "John Doe",
    "user_email": "john@work.com"
  },

  "sync": {
    "auto_sync_enabled": true,
    "pull_interval_seconds": 300,
    "debounce_seconds": 30,
    "auto_commit": true,
    "auto_push": true,
    "auto_pull": true
  },

  "notifications": {
    "enabled": true,
    "show_success": false,
    "show_pulls": true,
    "play_sound_on_conflict": false
  },

  "conflict_resolution": {
    "strategy": "manual",
    "backup_dir": "~/dotfiles/.backup",
    "keep_backups_days": 7
  },

  "mappings": {
    "bashrc": "~/.bashrc",
    "zshrc": "~/.zshrc",
    "gitconfig": "~/.gitconfig",
    "vimrc": "~/.vimrc",
    "config/nvim": "~/.config/nvim",
    "config/kitty": "~/.config/kitty",
    "config/tmux": "~/.config/tmux",
    "ssh/config": "~/.ssh/config"
  },

  "ui": {
    "start_at_boot": true,
    "minimize_to_tray": true,
    "theme": "auto"
  },

  "advanced": {
    "debug_logging": false,
    "log_file": "~/.dotfile-sync.log",
    "max_log_size_mb": 10
  }
}
```

---

## **6. CLI Commands**

All commands use the `dsm` binary (Dotfile Sync Manager):

### **Setup Commands**

```bash
# Initialize dotfiles repository
dsm init [git-url]

# Initialize with new repo
dsm init
# Prompts: Where should dotfiles be stored? [~/dotfiles]
# Prompts: Create new Git repository? [Y/n]

# Initialize from existing repo
dsm init https://github.com/user/dotfiles.git
```

### **File Management**

```bash
# Add file to dotfiles
dsm add ~/.vimrc
dsm add ~/.config/nvim

# Remove file from tracking
dsm remove ~/.vimrc

# List all tracked files
dsm list

# Check sync status
dsm status
```

### **Sync Control**

```bash
# Start daemon (auto-sync)
dsm start

# Stop daemon
dsm stop

# Restart daemon
dsm restart

# Check if daemon is running
dsm status

# Manual sync (one-time)
dsm sync

# Manual push only
dsm push

# Manual pull only
dsm pull
```

### **Conflict Management**

```bash
# Check for conflicts
dsm check

# View conflict details
dsm conflicts

# Mark conflicts as resolved and resume
dsm resolve
```

### **Configuration**

```bash
# Edit configuration file
dsm config

# Edit ignore patterns
dsm ignore

# Set configuration value
dsm config set sync.pull_interval_seconds 600

# Get configuration value
dsm config get sync.pull_interval_seconds
```

### **Utility Commands**

```bash
# View sync log
dsm log

# View last N log entries
dsm log -n 20

# Open dotfiles folder
dsm open

# Show version
dsm version

# Show help
dsm help
```

---

## **7. Installation & Setup**

### **Installation**

**macOS:**
```bash
# Using Homebrew
brew install dsm

# Or download DMG
# Download: DotfileSyncManager.dmg
# Drag to Applications folder
```

**Linux:**
```bash
# Using package manager (Ubuntu/Debian)
sudo apt install dsm

# Or download AppImage
wget https://github.com/user/dsm/releases/latest/dsm.AppImage
chmod +x dsm.AppImage
./dsm.AppImage
```

**Windows:**
```bash
# Using Scoop
scoop install dsm

# Or download installer
# Download: DotfileSyncManager-Setup.exe
# Run installer
```

### **First-Time Setup Flow**

**Step 1: Launch App**
```bash
dsm
# GUI opens with setup wizard
```

**Step 2: Setup Wizard**

```
┌──────────────────────────────────────────┐
│  Welcome to Dotfile Sync Manager         │
├──────────────────────────────────────────┤
│                                           │
│  Let's set up your dotfiles!              │
│                                           │
│  Do you already have a dotfiles repo?     │
│                                           │
│  ( ) Yes, I have a Git repository         │
│  ( ) No, create a new one                 │
│                                           │
│  [ Next ]                                 │
└──────────────────────────────────────────┘
```

**Step 3a: Existing Repository**
```
┌──────────────────────────────────────────┐
│  Clone Existing Repository                │
├──────────────────────────────────────────┤
│                                           │
│  Git Repository URL:                      │
│  [https://github.com/user/dotfiles.git]   │
│                                           │
│  Clone to:                                │
│  [~/dotfiles              ] [Browse]      │
│                                           │
│  Authentication (if private):             │
│  ( ) HTTPS (username/password)            │
│  (•) SSH (use SSH key)                    │
│                                           │
│  [ Back ]  [ Clone & Continue ]           │
└──────────────────────────────────────────┘
```

**Step 3b: New Repository**
```
┌──────────────────────────────────────────┐
│  Create New Repository                    │
├──────────────────────────────────────────┤
│                                           │
│  Create dotfiles folder at:               │
│  [~/dotfiles              ] [Browse]      │
│                                           │
│  Initialize Git repository? [Yes]         │
│                                           │
│  Optional: Add remote repository          │
│  Git URL (leave empty for local only):    │
│  [                                    ]   │
│                                           │
│  [ Back ]  [ Create ]                     │
└──────────────────────────────────────────┘
```

**Step 4: Select Files**
```
┌──────────────────────────────────────────┐
│  Select Files to Sync                     │
├──────────────────────────────────────────┤
│                                           │
│  Found these dotfiles in your home:       │
│                                           │
│  ☑ .bashrc                                │
│  ☑ .zshrc                                 │
│  ☑ .gitconfig                             │
│  ☑ .vimrc                                 │
│  ☑ .config/nvim/                          │
│  ☐ .config/Code/ (large, not recommended) │
│  ☑ .ssh/config                            │
│  ☐ .ssh/id_rsa (private key, skip)       │
│                                           │
│  [ Select All ]  [ Deselect All ]         │
│                                           │
│  [ Back ]  [ Continue ]                   │
└──────────────────────────────────────────┘
```

**Step 5: Machine Configuration**
```
┌──────────────────────────────────────────┐
│  Machine Configuration                    │
├──────────────────────────────────────────┤
│                                           │
│  Give this machine a name:                │
│  [Work Laptop            ]                │
│                                           │
│  Git configuration:                       │
│  Name: [John Doe                    ]     │
│  Email: [john@work.com              ]     │
│                                           │
│  Sync settings:                           │
│  Pull every: [5] minutes                  │
│  ☑ Start sync daemon at boot              │
│  ☑ Enable notifications                   │
│                                           │
│  [ Back ]  [ Finish Setup ]               │
└──────────────────────────────────────────┘
```

**Step 6: Complete**
```
┌──────────────────────────────────────────┐
│  Setup Complete! 🎉                       │
├──────────────────────────────────────────┤
│                                           │
│  Your dotfiles are now being synced!      │
│                                           │
│  ✓ 8 files added to sync                  │
│  ✓ Symlinks created                       │
│  ✓ Initial commit pushed                  │
│  ✓ Auto-sync started                      │
│                                           │
│  The app will run in your system tray.    │
│  Look for the 🟢 icon.                    │
│                                           │
│  [ Open Status Window ]  [ Close ]        │
└──────────────────────────────────────────┘
```

### **Setup on Second Machine**

**Process:**
1. Install DSM app
2. Launch and select "Yes, I have a repository"
3. Enter Git URL (same as first machine)
4. Clone repository
5. App automatically creates symlinks
6. Start syncing

**No need to manually select files** - they're already in the repo!

---

## **8. Project File Structure**

```
dotfile-sync-manager/
├── main.go                     # Wails app entry point
├── go.mod
├── go.sum
├── wails.json                  # Wails configuration
├── build/                      # Build artifacts
│   ├── bin/
│   ├── darwin/
│   ├── linux/
│   └── windows/
├── app/
│   ├── app.go                 # Main app struct & Wails bindings
│   ├── sync_service.go        # Sync logic & file watching
│   ├── git_manager.go         # Git operations (go-git)
│   ├── conflict_detector.go   # Conflict detection
│   ├── symlink_manager.go     # Symlink creation/management
│   ├── config.go              # Configuration management
│   └── notifier.go            # Desktop notifications
├── frontend/
│   ├── index.html             # Main status window
│   ├── settings.html          # Settings window
│   ├── conflict.html          # Conflict resolution window
│   ├── css/
│   │   ├── pico.min.css       # CSS framework (optional)
│   │   └── styles.css         # Custom styles
│   └── js/
│       ├── app.js             # Main UI logic
│       ├── settings.js        # Settings UI
│       └── conflict.js        # Conflict UI
├── pkg/
│   ├── ignore/                # .syncignore parsing
│   ├── debouncer/             # File change debouncing
│   └── logger/                # Logging utilities
├── cmd/
│   └── dsm/
│       └── main.go            # CLI entry point
├── docs/
│   ├── README.md
│   ├── SETUP.md
│   └── TROUBLESHOOTING.md
└── scripts/
    ├── build.sh               # Build scripts
    └── package.sh             # Packaging scripts
```

---

## **9. Go Code Structure**

### **main.go**

```go
package main

import (
    "embed"
    "log"

    "github.com/wailsapp/wails/v2"
    "github.com/wailsapp/wails/v2/pkg/options"
    "github.com/wailsapp/wails/v2/pkg/options/assetserver"
    "github.com/wailsapp/wails/v2/pkg/options/mac"
    "github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
    // Create app instance
    app := NewApp()

    // Create Wails application
    err := wails.Run(&options.App{
        Title:  "Dotfile Sync Manager",
        Width:  800,
        Height: 600,
        AssetServer: &assetserver.Options{
            Assets: assets,
        },
        BackgroundColour: &options.RGBA{R: 255, G: 255, B: 255, A: 1},
        OnStartup:        app.startup,
        OnShutdown:       app.shutdown,
        Bind: []interface{}{
            app,
        },
        Mac: &mac.Options{
            TitleBar: mac.TitleBarHiddenInset(),
        },
        Windows: &windows.Options{
            WebviewIsTransparent: false,
            WindowIsTranslucent:  false,
        },
    })

    if err != nil {
        log.Fatal(err)
    }
}
```

### **app/app.go**

```go
package app

import (
    "context"
    "fmt"

    "github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
    ctx           context.Context
    syncService   *SyncService
    config        *Config
    gitManager    *GitManager
    symlinkMgr    *SymlinkManager
}

func NewApp() *App {
    return &App{}
}

func (a *App) startup(ctx context.Context) {
    a.ctx = ctx

    // Load configuration
    config, err := LoadConfig()
    if err != nil {
        runtime.LogError(ctx, "Failed to load config: "+err.Error())
        return
    }
    a.config = config

    // Initialize components
    a.gitManager = NewGitManager(config.Git.RepoPath)
    a.symlinkMgr = NewSymlinkManager(config)
    a.syncService = NewSyncService(a.gitManager, config, a.onSyncEvent)

    // Start sync service if enabled
    if config.Sync.AutoSyncEnabled {
        if err := a.syncService.Start(); err != nil {
            runtime.LogError(ctx, "Failed to start sync: "+err.Error())
        }
    }
}

func (a *App) shutdown(ctx context.Context) {
    if a.syncService != nil {
        a.syncService.Stop()
    }
}

// Exported methods callable from frontend

func (a *App) GetStatus() map[string]interface{} {
    return map[string]interface{}{
        "synced":      a.syncService.IsSynced(),
        "lastPush":    a.syncService.LastPush(),
        "lastPull":    a.syncService.LastPull(),
        "filesChanged": a.syncService.ChangedFilesCount(),
        "message":     a.syncService.StatusMessage(),
        "icon":        a.syncService.StatusIcon(),
    }
}

func (a *App) ManualSync() error {
    return a.syncService.ManualSync()
}

func (a *App) OpenDotfilesFolder() error {
    return runtime.BrowserOpenURL(a.ctx, a.config.Git.RepoPath)
}

func (a *App) OpenLog() error {
    return runtime.BrowserOpenURL(a.ctx, a.config.Advanced.LogFile)
}

// Event callback from sync service
func (a *App) onSyncEvent(event SyncEvent) {
    runtime.EventsEmit(a.ctx, "sync-event", event)

    if event.Type == EventConflict {
        // Show notification
        runtime.EventsEmit(a.ctx, "show-notification", map[string]string{
            "title": "Conflict Detected",
            "body":  fmt.Sprintf("Conflicts in %d files", len(event.ConflictFiles)),
        })
    }
}
```

### **app/sync_service.go**

```go
package app

import (
    "time"
    "github.com/fsnotify/fsnotify"
)

type SyncService struct {
    watcher     *fsnotify.Watcher
    gitManager  *GitManager
    config      *Config
    debouncer   *Debouncer
    running     bool
    onEvent     func(SyncEvent)
}

type SyncEvent struct {
    Type          string
    Message       string
    ConflictFiles []string
}

func NewSyncService(git *GitManager, config *Config, onEvent func(SyncEvent)) *SyncService {
    return &SyncService{
        gitManager: git,
        config:     config,
        debouncer:  NewDebouncer(time.Duration(config.Sync.DebounceSeconds) * time.Second),
        onEvent:    onEvent,
    }
}

func (s *SyncService) Start() error {
    watcher, err := fsnotify.NewWatcher()
    if err != nil {
        return err
    }
    s.watcher = watcher
    s.running = true

    // Watch dotfiles directory
    err = watcher.Add(s.config.Git.RepoPath)
    if err != nil {
        return err
    }

    // Start watching goroutine
    go s.watchLoop()

    // Start auto-pull goroutine
    go s.autoPullLoop()

    return nil
}

func (s *SyncService) watchLoop() {
    for s.running {
        select {
        case event := <-s.watcher.Events:
            // Debounce changes
            s.debouncer.Add(event.Name, func() {
                s.handleFileChange(event)
            })
        case err := <-s.watcher.Errors:
            s.onEvent(SyncEvent{Type: "error", Message: err.Error()})
        }
    }
}

func (s *SyncService) handleFileChange(event fsnotify.Event) {
    // Auto-commit and push
    if err := s.gitManager.CommitAndPush("Auto-sync: " + time.Now().Format("2006-01-02 15:04:05")); err != nil {
        s.onEvent(SyncEvent{Type: "error", Message: "Push failed: " + err.Error()})
    } else {
        s.onEvent(SyncEvent{Type: "synced", Message: "Changes synced"})
    }
}

func (s *SyncService) autoPullLoop() {
    ticker := time.NewTicker(time.Duration(s.config.Sync.PullIntervalSeconds) * time.Second)
    defer ticker.Stop()

    for s.running {
        <-ticker.C
        if err := s.gitManager.Pull(); err != nil {
            // Check if it's a conflict
            if s.gitManager.HasConflicts() {
                conflicts := s.gitManager.GetConflictFiles()
                s.onEvent(SyncEvent{
                    Type:          "conflict",
                    Message:       "Merge conflicts detected",
                    ConflictFiles: conflicts,
                })
                s.Stop() // Pause auto-sync
            } else {
                s.onEvent(SyncEvent{Type: "error", Message: "Pull failed: " + err.Error()})
            }
        }
    }
}

func (s *SyncService) Stop() {
    s.running = false
    if s.watcher != nil {
        s.watcher.Close()
    }
}
```

---

## **10. Non-Functional Requirements**

### **Performance**
- Startup time < 3 seconds
- Memory usage < 60MB idle, < 90MB active
- CPU usage < 1% idle, < 5% during sync
- File watcher latency < 100ms
- Sync push latency < 5 seconds after debounce
- Pull check every 5 minutes (configurable)

### **Reliability**
- Handle network interruptions gracefully (retry with exponential backoff)
- Never corrupt existing dotfiles
- Always create backups before destructive operations
- Atomic file operations (write to temp, then move)
- Crash recovery: resume state on restart

### **Security**
- Never sync files matching sensitive patterns (default .syncignore)
- Use SSH keys for Git authentication (recommended)
- Validate symlink targets (no directory traversal)
- Sanitize file paths
- Log security events

### **Cross-Platform**
- Support macOS 10.15+
- Support Linux (Ubuntu 20.04+, Fedora, Arch)
- Support Windows 10+ (including WSL)
- Handle path differences (~ expansion, separators)
- Respect platform conventions (e.g., .config on Linux vs Library on Mac)

### **Usability**
- Setup wizard completes in < 5 minutes
- No command line required (GUI for everything)
- Clear error messages with suggested fixes
- Comprehensive documentation
- In-app help and tooltips

---

## **11. Error Handling**

### **Common Errors & Solutions**

| Error | Cause | Solution Shown to User |
|-------|-------|----------------------|
| Git push failed | No network / auth failure | "Push failed. Check network and Git credentials. Retry?" |
| Merge conflict | Same file edited on both machines | "Conflict detected. Open folder to resolve manually." |
| Symlink failed | Target already exists | "File exists at ~/.bashrc. Backed up to ~/dotfiles/.backup/" |
| File watcher failed | Too many files | "File watching failed. Using polling mode instead." |
| Disk full | No space for backup | "Disk full. Free up space and try again." |
| Git not configured | Missing user.name/email | "Git not configured. Open settings to add your name and email." |

### **Logging**

**Log file location:** `~/.dotfile-sync.log`

**Log format:**
```
[2025-10-13 14:30:22] [INFO] Sync service started
[2025-10-13 14:30:25] [INFO] File changed: bashrc
[2025-10-13 14:30:55] [INFO] Auto-commit: Auto-sync: 2025-10-13 14:30:55
[2025-10-13 14:30:56] [INFO] Pushed to origin/main
[2025-10-13 14:35:22] [INFO] Auto-pull complete: 2 files updated
[2025-10-13 15:10:15] [ERROR] Merge conflict detected in bashrc
[2025-10-13 15:10:15] [INFO] Backup created: ~/dotfiles/.backup-20251013-151015/
[2025-10-13 15:10:15] [WARN] Auto-sync paused due to conflict
```

**Log rotation:**
- Max size: 10MB (configurable)
- Keep last 3 rotated logs
- Compress old logs

---

## **12. Testing Strategy**

### **Unit Tests**
- Git operations (commit, push, pull, conflict detection)
- File watcher (debouncing, pattern matching)
- Symlink creation (various scenarios)
- Configuration loading/saving
- Ignore pattern matching

### **Integration Tests**
- Full sync cycle (change file -> commit -> push -> pull on other machine)
- Conflict detection and resolution
- Multi-machine sync (3+ machines)
- Network failure recovery
- Backup creation and restoration

### **Manual Testing**
- Setup wizard on fresh machine
- All GUI interactions (buttons, menus, windows)
- System tray behavior
- Notifications on different platforms
- Cross-platform compatibility

### **Performance Tests**
- Sync 100+ files
- Large file handling (>10MB)
- Deep directory structures (10+ levels)
- Rapid consecutive changes
- Memory leak testing (run for 24+ hours)

---

## **13. Development Timeline**

### **Phase 1: Core Sync (Weeks 1-2)**
- ✅ Git operations (commit, push, pull)
- ✅ File watching with fsnotify
- ✅ Debouncing logic
- ✅ Basic configuration
- ✅ CLI commands (init, add, start, stop)

### **Phase 2: Symlink Management (Week 3)**
- ✅ Symlink creation and validation
- ✅ File mapping configuration
- ✅ Backup existing files
- ✅ Add/remove file commands

### **Phase 3: Conflict Handling (Week 4)**
- ✅ Conflict detection
- ✅ Backup creation
- ✅ Basic notification
- ✅ Manual resolution flow

### **Phase 4: GUI Development (Weeks 5-6)**
- ✅ System tray icon and menu
- ✅ Status window (HTML/CSS/JS)
- ✅ Settings window
- ✅ Conflict resolution UI
- ✅ Wails bindings (Go <-> JS)

### **Phase 5: Polish & Package (Week 7)**
- ✅ Setup wizard
- ✅ Cross-platform testing
- ✅ Build installers (DMG, AppImage, .exe)
- ✅ Documentation
- ✅ Icon design

### **Phase 6: Beta Testing (Week 8)**
- ✅ Internal testing
- ✅ Bug fixes
- ✅ Performance optimization
- ✅ User feedback incorporation

**Total: ~8 weeks to MVP**

---

## **14. Future Enhancements (V2+)**

### **V2.0 Features**
- **Encrypted storage** for sensitive files (using age or GPG)
- **Web dashboard** to view sync status across all machines
- **Mobile app** (view-only) to monitor sync status
- **Multiple profiles** (work, personal, gaming)
- **Scheduled backups** to cloud storage (S3, Dropbox)

### **V2.1 Features**
- **Conflict resolution UI** with visual diff
- **Template variables** for machine-specific configs
- **Plugin system** for custom sync logic
- **Integration with dotfile managers** (chezmoi, yadm)

### **V3.0 Features**
- **Team sharing** (share dotfiles within organization)
- **Cloud sync** option (alternative to Git)
- **AI-powered conflict resolution** suggestions
- **Dotfile marketplace** (share and discover dotfiles)

---

## **15. Risks & Mitigations**

| Risk | Impact | Probability | Mitigation |
|------|--------|-------------|------------|
| Data loss from conflicts | **High** | Medium | Always create backups, manual resolution required |
| Sensitive data leaked | **High** | Low | Strong default .syncignore, warn on sensitive patterns |
| Git merge fails silently | Medium | Low | Validate merge status, alert on failure |
| Network interruptions | Medium | High | Queue changes, retry with exponential backoff |
| Symlink breaks system | **High** | Low | Validate before creating, backup existing files |
| High memory usage | Low | Medium | Efficient file watching, limit tracked files |
| Cross-platform bugs | Medium | Medium | Extensive testing on all platforms |
| Git authentication issues | Medium | High | Clear error messages, guide to setup SSH keys |

---

## **16. Success Criteria**

### **MVP Launch Ready:**
- ✅ All core features working (Features 1-6)
- ✅ GUI functional on macOS, Linux, Windows
- ✅ < 10 known bugs
- ✅ Setup wizard completes successfully
- ✅ Installers for all platforms
- ✅ Basic documentation

### **Production Ready:**
- ✅ All MVP criteria
- ✅ < 3 critical bugs
- ✅ 20+ beta testers
- ✅ Performance within requirements
- ✅ Comprehensive documentation
- ✅ Video tutorials
- ✅ GitHub repository with CI/CD

### **Long-term Success:**
- 1000+ active users
- < 5% crash rate
- 4+ star rating
- Active community contributions
- Regular updates (monthly)

---

## **17. Open Questions**

**To be decided during development:**

1. Should we support multiple Git branches per machine?
2. Should we include a built-in text editor for conflict resolution?
3. Should we support cloud storage (Dropbox, Google Drive) as an alternative to Git?
4. Should we add a "dry run" mode to preview changes before syncing?
5. Should we support scheduling (e.g., only sync during work hours)?
6. Should we integrate with existing dotfile managers or remain standalone?
7. Should we add telemetry (opt-in) to improve the product?

---

## **18. Resources**

### **Documentation**
- User Guide: How to set up and use DSM
- Developer Guide: How to build and contribute
- API Reference: Go package documentation
- Troubleshooting Guide: Common issues and solutions

### **External Resources**
- Wails Documentation: https://wails.io/docs/
- go-git Documentation: https://pkg.go.dev/github.com/go-git/go-git/v5
- fsnotify Documentation: https://pkg.go.dev/github.com/fsnotify/fsnotify
- Pico CSS Documentation: https://picocss.com/docs

---

## **Appendix A: File Formats**

### **`.syncignore` Format**

```gitignore
# Comments start with #
# Blank lines are ignored

# Exact file paths
.ssh/id_rsa
.ssh/id_ed25519

# Wildcards
*.log
*.tmp
*.swp
*~

# Directory patterns (trailing slash)
.cache/
node_modules/
__pycache__/

# Negation (include despite previous exclusion)
!important.log

# Multiple wildcards
**/.git/
**/node_modules/
```

### **Symlink Mapping Format**

In `.sync-config.json`:
```json
{
  "mappings": {
    "source_in_dotfiles": "target_in_home",
    "bashrc": "~/.bashrc",
    "config/nvim": "~/.config/nvim"
  }
}
```

---

## **Appendix B: Git Commit Message Format**

**Auto-commits:**
```
Auto-sync: 2025-10-13 14:30:22

Changed files:
- bashrc
- config/nvim/init.vim
```

**Manual commits (via UI):**
```
Manual sync: User description

Changed files:
- gitconfig
```

**Initial commit:**
```
Initial dotfiles setup

Added files:
- bashrc
- zshrc
- gitconfig
- vimrc
- config/nvim/
- config/kitty/
```

---

## **Appendix C: Platform-Specific Notes**

### **macOS**
- Use `~/Library/Application Support/` for app config
- System tray called "menu bar"
- `.DS_Store` auto-ignored
- Use native notifications (NSUserNotification)

### **Linux**
- Use `~/.config/` for app config
- Different desktop environments (GNOME, KDE, etc.)
- System tray may not be available on all DEs
- Use libnotify for notifications

### **Windows**
- Use `%APPDATA%` for app config
- Path separator is `\` not `/`
- System tray called "notification area"
- Use Windows Toast notifications
- Consider WSL users (hybrid setup)

---

**End of PRD**
