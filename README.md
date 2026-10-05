# agy-cleaner

A fast, safe, and robust CLI tool written in Go to inspect, manage, and clean Google Antigravity CLI conversations and storage.

---

## Features

- **Interactive Terminal UI (TUI Menu)**: Run `agy-cleaner` with no arguments to get an interactive menu with full cleanup controls.
- **Storage Overview**: Inspect disk space occupied by Antigravity conversations (`conversations/*.db`), brains (`brain/*`), and summaries (`conversation_summaries.db`).
- **Grouped by Workspace**: See conversations grouped by project directory with conversation counts, disk space, and relative timestamps.
- **Orphan Detection & Removal**: Detect and clean ghost database records whose files have been removed, or leftover files not tracked by the database.
- **Active Session Protection**: Automatically detects running Antigravity CLI instances via kernel file locks (`flock`) and prevents accidental deletion of active sessions.
- **Automatic Pre-Cleanup Backups**: Automatically creates timestamped backups of `conversation_summaries.db` and `history.jsonl` before performing any destructive actions.
- **Dry Run Support**: Test any cleanup operation with `--dry-run` to preview exactly what will be deleted and how much space will be freed.
- **History Synchronization**: Strips deleted conversation IDs from `history.jsonl` to keep history cleanly in sync.
- **Database Vacuuming**: Automatically runs SQLite `VACUUM` and WAL checkpointing after cleanup to immediately reclaim disk space.

---

## Installation

### Prerequisites
- Go 1.22+
- GCC (for SQLite3 CGO bindings)

### Build & Install
```bash
cd /var/www/agy-cleaner
make install
```
This builds and installs the binary to `~/.local/bin/agy-cleaner`.

---

## Usage

### 1. Interactive Menu
Simply run:
```bash
agy-cleaner
```
You will be presented with a menu:
```
  Menu Options:

  1.  View Workspaces & Conversation Counts
  2.  List Detailed Conversations
  3.  Clean Conversations by Workspace
  4.  Clean Conversations Older Than X Days
  5.  Clean Orphaned / Ghost Records Only
  6.  Delete a Specific Conversation by ID
  7.  Clean ALL Conversations (Except Active)
  8.  Backups (List & Restore)
  0.  Exit
```

---

### 2. Command-Line Subcommands

#### Inspect Conversations
```bash
# View summary by workspace
agy-cleaner list

# View individual conversation records
agy-cleaner list --all

# Filter list by workspace
agy-cleaner list -w englishmoza-backend

# Output summary in JSON
agy-cleaner list --json
```

#### Clean Conversations

```bash
# Preview what would be deleted for a workspace (dry-run)
agy-cleaner clean -w Keya --dry-run

# Delete conversations for a specific workspace
agy-cleaner clean -w Keya

# Delete all conversations EXCEPT a specific workspace
agy-cleaner clean --keep-workspace englishmoza-backend

# Clean conversations older than 30 days (or 7d, 24h)
agy-cleaner clean --older-than 30d

# Clean only orphaned/ghost records
agy-cleaner clean --orphans -y

# Delete a specific conversation by ID
agy-cleaner clean --id <conversation-uuid>

# Delete ALL conversations (active sessions are always protected)
agy-cleaner clean --all
```

#### Manage Backups
```bash
# List available backups
agy-cleaner backup list

# Create a manual backup
agy-cleaner backup create my_backup_tag

# Restore from a backup directory
agy-cleaner backup restore ~/.gemini/antigravity-cli/backups/backup_20261005_121600
```

---

## Flags Reference

| Flag | Description |
|---|---|
| `-w, --workspace <str>` | Match conversations belonging to workspace path/substring |
| `--keep-workspace <str>` | Delete all conversations EXCEPT this workspace |
| `--older-than <duration>` | Delete conversations older than duration (`30d`, `7d`, `24h`) |
| `--id <uuid>` | Delete a specific conversation by UUID |
| `--orphans` | Clean only orphaned/ghost records and leftover files |
| `--all` | Delete all stored conversations |
| `--dry-run` | Preview what will be deleted without modifying anything |
| `-y, --yes` | Skip interactive `[y/N]` confirmation prompt |
| `--no-backup` | Skip automatic backup before deletion |
| `--force` | Force deletion even if session appears active |
| `--data-dir <path>` | Custom Antigravity data directory (default: `~/.gemini/antigravity-cli`) |

---

## Architecture & Storage Cleaned

When a conversation is deleted, `agy-cleaner` thoroughly purges all related artifacts:
1. **SQLite Database**: Removes matching rows from `conversation_summaries` in `conversation_summaries.db`.
2. **Session Storage**: Deletes `conversations/<id>.db`, `.db-wal`, and `.db-shm`.
3. **Brain & Artifacts**: Recursively removes `brain/<id>/` (transcripts, task states, scratch scripts).
4. **Presence & Locks**: Removes `presence/<id>.lock`.
5. **Annotations**: Removes `annotations/<id>.pbtxt`.
6. **Command History**: Cleans `history.jsonl` entries referencing the deleted conversation ID.
7. **Compaction**: Runs `PRAGMA wal_checkpoint(TRUNCATE)` and SQLite `VACUUM` to free physical disk space.
