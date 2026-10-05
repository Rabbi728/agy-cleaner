# agy-cleaner — Complete Reference & User Guide

A fast, safe, and powerful CLI tool written in **Go** to inspect, search, manage, and clean **Google Antigravity CLI** conversations and disk storage.

---

## Table of Contents

1. [Overview & Why This Tool Exists](#1-overview--why-this-tool-exists)
2. [Glossary: What Does Each Term Mean?](#2-glossary-what-does-each-term-mean)
   - [Conversation ID](#conversation-id)
   - [Title](#title)
   - [Workspace](#workspace)
   - [Steps](#steps)
   - [Size](#size)
   - [Modified](#modified)
   - [Status Badges: ACTIVE, Stored, ORPHAN](#status-badges-active-stored-orphan)
   - [Antigravity Storage Folders Explained](#antigravity-storage-folders-explained)
3. [Interactive Menu: What Each Option Does](#3-interactive-menu-what-each-option-does)
4. [CLI Commands: What Each Command Does](#4-cli-commands-what-each-command-does)
   - [List Commands](#listing-commands)
   - [Clean Commands](#cleaning-commands)
   - [Backup Commands](#backup-commands)
5. [Complete Flags Reference](#5-complete-flags-reference)
6. [Scenarios: What Should I Do When...?](#6-scenarios-what-should-i-do-when)
7. [Installation & Build](#7-installation--build)
8. [Technical Architecture & Safety Mechanisms](#8-technical-architecture--safety-mechanisms)

---

## 1. Overview & Why This Tool Exists

When you interact with Google Antigravity CLI, every session writes data to disk inside `~/.gemini/antigravity-cli`:
- SQLite databases for conversation states
- Brain directories containing logs, task states, thinking steps, and file artifacts
- Global summary databases and history files

Over time, this directory can grow into **hundreds of megabytes or gigabytes**, cluttering your system and terminal lists.

### Why You Cannot Use a Normal File Manager
- **Random UUID File Names**: Files are named `0ff4dc59-26f6-4618-af5b-5a97fa36e6ce.db`. You cannot tell which project or date they belong to.
- **SQLite Database Indexing**: Antigravity keeps an internal database index (`conversation_summaries.db`). If you delete files by hand, the database retains broken ghost records.
- **Multi-Location Spread**: A single conversation has files in up to 5 different folders (`conversations/`, `brain/`, `annotations/`, `presence/`, `history.jsonl`).

**`agy-cleaner`** understands these relationships, ties everything together by workspace and title, and allows you to view and clean records safely.

---

## 2. Glossary: What Does Each Term Mean?

### `Conversation ID`
* **What it means:** The unique 36-character UUID (e.g. `4060c725-d42a-4153-b80f-1fcf1331a12c`) generated for each conversation session.
* **What it does:** Identifies the specific session across all database tables, log files, and lock files. You can use this ID with `--id <uuid>` to delete or search for an exact session.

### `Title`
* **What it means:** The human-readable summary of the conversation (e.g., `"Android Push Notification Implementation"`).
* **What it does:** Generated automatically by the agent from your prompts to help you recognize what was done in that conversation.

### `Workspace`
* **What it means:** The directory/folder path where you opened or ran the Antigravity CLI (e.g., `/var/www/English-Moja/englishmoza-backend`).
* **What it does:** Allows grouping and filtering of conversations by project, so you can clean up an old project without touching other projects.

### `Steps`
* **What it means:** The total count of turns/steps executed in that conversation.
* **What it does:** Each user prompt, agent response, and tool call increments this number. A session with `1300` steps was a long, complex task; a session with `2` steps was a quick lookup.

### `Size`
* **What it means:** The combined physical disk space consumed by this conversation.
* **What it calculates:** `Size = conversations/<id>.db + brain/<id>/ (logs, artifacts, task states)`.
* **Note on 0 B:** If a conversation shows `0 B`, its files have already been deleted from disk, but its metadata still remains in the SQLite database (an Orphan).

### `Modified`
* **What it means:** The relative time when this conversation was last edited, updated, or active (e.g., `just now`, `56m ago`, `2d ago`, `3mo ago`).
* **What it does:** Helps you identify old, stale conversations that you haven't opened in weeks or months.

---

### Status Badges: ACTIVE, Stored, ORPHAN

| Status Badge | What It Means | SQLite Record? | Disk Files Present? | Process Running? | What It Does / Action |
|:---:|---|:---:|:---:|:---:|---|
| <span style="color:green">**`ACTIVE`**</span> | **Currently Running Session** | Yes | Yes | **Yes** | An Antigravity CLI process is actively running this conversation right now. `agy-cleaner` detects this via file lock and **protects it from deletion**. |
| <span style="color:gray">**`Stored`**</span> | **Healthy Past Session** | Yes | Yes | **No** | A finished conversation stored on disk. Both the SQLite entry and the physical disk files are intact. Safe to resume or delete. |
| <span style="color:goldenrod">**`ORPHAN`**</span> | **Ghost / Broken Record** | Yes | **No** *(or vice versa)* | **No** | The database entry exists, but its physical files were deleted (or disk files exist with no DB record). Takes up clutter with `0 B` size. **Safe to purge anytime.** |

---

### Antigravity Storage Folders Explained

Everything is stored under `~/.gemini/antigravity-cli/`:

| Folder / File | What It Stores | How `agy-cleaner` Handles It |
|---|---|---|
| `conversation_summaries.db` | Master SQLite DB indexing all conversation titles, timestamps, workspaces, and step counts | Removes deleted rows; compacts with `VACUUM` |
| `conversations/` | `<id>.db` files storing message trajectories and full SQLite state | Deletes matching `.db`, `.db-wal`, `.db-shm` |
| `brain/` | `<id>/` folders containing transcripts, task logs, and artifacts | Recursively deletes the entire folder |
| `presence/` | `<id>.lock` files used for process concurrency locking | Checks lock state (`flock`) to detect active sessions; removes lock files on delete |
| `annotations/` | `<id>.pbtxt` containing protobuf session metadata | Deletes matching files |
| `history.jsonl` | Append-only log of prompts and slash commands | Filters out lines referencing deleted conversation IDs |
| `backups/` | Timestamped backup archives created before cleanup | Created automatically before any delete |

---

## 3. Interactive Menu: What Each Option Does

Run without arguments:
```bash
agy-cleaner
```

```text
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

### Detailed Breakdown of Each Option:

#### Option `1`: View Workspaces & Conversation Counts
* **What it does:** Displays a summary table of all projects/workspaces.
* **What you see:** Workspace path, number of conversations, total disk space consumed, latest activity date, and status flags (e.g. `[Active: 1]`, `[Orphans: 10]`).

#### Option `2`: List Detailed Conversations
* **What it does:** Shows individual conversations.
* **What you can do:** Press Enter to view all, or type a workspace name (e.g., `frontend`) to see only conversations belonging to that project.
* **What you see:** ID, Title, Workspace, Step Count, Disk Size, Last Modified, and Status.

#### Option `3`: Clean Conversations by Workspace
* **What it does:** Lets you select a project workspace by number (e.g. `3` for Keya-Distribution).
* **Safety:** Previews matching conversations, shows estimated disk space freed, asks for confirmation `[y/N]`, and creates an automatic backup before deleting.

#### Option `4`: Clean Conversations Older Than X Days
* **What it does:** Prompts you for a number of days (e.g. `30` or `14`). Deletes all conversations that have had no activity for that duration.
* **Safety:** Active sessions are automatically protected.

#### Option `5`: Clean Orphaned / Ghost Records Only
* **What it does:** Scans for all broken records where the database entry exists but physical files are missing (or leftover files without a database entry).
* **Benefit:** Cleans up clutter without touching any active or healthy stored conversations.

#### Option `6`: Delete a Specific Conversation by ID
* **What it does:** Prompts you for a conversation UUID (or partial UUID). Deletes only that single conversation from DB, disk, and history.

#### Option `7`: Clean ALL Conversations (Except Active)
* **What it does:** Cleans all stored past conversations across all workspaces.
* **Safety:** Any currently active CLI session is automatically protected and skipped. Requires explicit confirmation.

#### Option `8`: Backups (List & Restore)
* **What it does:** 
  1. Creates a manual snapshot of your database and history.
  2. Lists all existing backups with timestamps and sizes.
  3. Restores your database to an earlier backup state if you ever want to undo a deletion.

#### Option `0`: Exit
* **What it does:** Exits the program cleanly.

---

## 4. CLI Commands: What Each Command Does

If you prefer using commands or automating via shell scripts, use subcommands:

### Listing Commands

#### Overview Table
```bash
agy-cleaner list
```
* **Function:** Prints global storage stats and the workspace breakdown table.

#### List Individual Conversations
```bash
agy-cleaner list conversations
# or
agy-cleaner list --all
```
* **Function:** Prints conversations sorted newest first, showing ID, Title, Workspace, Steps, Size, and Status.

#### Limit the Number of Rows
```bash
agy-cleaner list conversations -n 15
```
* **Function:** Shows only the 15 most recent conversations.

#### Search by Keyword
```bash
agy-cleaner list -s "notification"
agy-cleaner list -s "exam"
```
* **Function:** Filters conversations whose Title, Workspace, or ID contains the keyword.

#### Filter by Workspace
```bash
agy-cleaner list -w englishmoza-backend
agy-cleaner list -w frontend -n 10
```
* **Function:** Shows only conversations that belong to workspaces matching the filter string.

#### JSON Output (for Scripts & Tooling)
```bash
agy-cleaner list --json
```
* **Function:** Outputs complete structured statistics and workspace groups in JSON format.

---

### Cleaning Commands

#### Dry Run (Preview Mode)
```bash
agy-cleaner clean -w Keya --dry-run
```
* **Function:** Shows exactly what would be deleted and how much space would be freed **without modifying or deleting anything**.

#### Clean by Workspace
```bash
agy-cleaner clean -w Keya
```
* **Function:** Deletes conversations matching "Keya". Asks for confirmation `[y/N]` and creates an automatic backup.
* **Skip prompt:** Add `-y` or `--yes`:
  ```bash
  agy-cleaner clean -w Keya -y
  ```

#### Keep One Workspace, Delete Everything Else
```bash
agy-cleaner clean --keep-workspace englishmoza-backend
```
* **Function:** Keeps all conversations for `englishmoza-backend` intact, but deletes all conversations from other workspaces.

#### Clean Older Than X Days / Hours
```bash
# Delete older than 30 days
agy-cleaner clean --older-than 30d

# Delete older than 7 days
agy-cleaner clean --older-than 7d

# Delete older than 24 hours
agy-cleaner clean --older-than 24h
```

#### Clean Orphan / Ghost Records
```bash
agy-cleaner clean --orphans -y
```
* **Function:** Purges all broken `0 B` records whose disk files are missing.

#### Delete a Single Conversation by UUID
```bash
agy-cleaner clean --id 3469a52f-5fad-4de1-83b5-cd9fe5021549 -y
```

#### Delete ALL Conversations
```bash
agy-cleaner clean --all
```
* **Function:** Purges all conversations except currently active sessions.

---

### Backup Commands

#### List Backups
```bash
agy-cleaner backup list
```
* **Function:** Shows all snapshots available under `~/.gemini/antigravity-cli/backups/`.

#### Create a Manual Backup
```bash
agy-cleaner backup create before_update
```
* **Function:** Creates an instant snapshot folder named `backup_YYYYMMDD_HHMMSS_before_update`.

#### Restore from Backup
```bash
agy-cleaner backup restore ~/.gemini/antigravity-cli/backups/backup_20261005_121600
```
* **Function:** Replaces the current `conversation_summaries.db` and `history.jsonl` with the files from that backup.

---

## 5. Complete Flags Reference

| Flag | Short | Command | What It Means & What It Does |
|---|:---:|:---:|---|
| `--workspace <str>` | `-w` | `list`, `clean` | Matches conversations whose workspace path contains this string. |
| `--search <str>` | `-s` | `list` | Filters conversation list by matching title, ID, or workspace. |
| `--limit <int>` | `-n` | `list` | Maximum number of conversations to display (default: 50). |
| `--all` | | `list`, `clean` | In `list`: shows individual conversations. In `clean`: targets all stored conversations. |
| `--keep-workspace <str>` | | `clean` | Deletes all conversations EXCEPT those matching this workspace. |
| `--older-than <dur>` | | `clean` | Deletes conversations older than specified duration (e.g. `30d`, `7d`, `24h`). |
| `--id <uuid>` | | `clean` | Deletes only the conversation matching this UUID. |
| `--orphans` | | `clean` | Targets only orphaned records (DB entries missing files or vice versa). |
| `--dry-run` | | `clean` | Previews actions without deleting any files or records. |
| `--yes` | `-y` | `clean` | Skips interactive `[y/N]` confirmation prompts. |
| `--no-backup` | | `clean` | Skips automatic backup creation before deletion (not recommended). |
| `--force` | | `clean` | Allows deleting active running sessions (use with extreme caution). |
| `--json` | | `list` | Outputs raw statistics as formatted JSON. |
| `--data-dir <path>` | | *All* | Overrides the default Antigravity data directory (`~/.gemini/antigravity-cli`). |
| `--help` | `-h` | *All* | Shows usage help. |
| `--version` | `-v` | *All* | Prints version information. |

---

## 6. Scenarios: What Should I Do When...?

### Scenario 1: "My disk space is getting low"
Run:
```bash
agy-cleaner clean --older-than 30d
```
*Cleans all conversations older than a month while protecting your recent and active work.*

### Scenario 2: "I worked on an old client project that I don't need anymore"
Run:
```bash
agy-cleaner clean -w old-project-name
```
*Removes all traces of that project from Antigravity.*

### Scenario 3: "I have dozens of 0 B ghost records cluttering my list"
Run:
```bash
agy-cleaner clean --orphans -y
```
*Removes all orphaned entries instantly.*

### Scenario 4: "I only want to keep my backend project and wipe everything else"
Run:
```bash
agy-cleaner clean --keep-workspace englishmoza-backend
```
*Safely preserves your backend conversations and cleans everything else.*

### Scenario 5: "I accidentally deleted something I needed"
Run:
```bash
agy-cleaner backup list
# Find the backup taken right before your cleanup, then:
agy-cleaner backup restore ~/.gemini/antigravity-cli/backups/<backup-name>
```
*Restores your conversation metadata and history.*

---

## 7. Installation & Build

### Fast Build via Makefile
```bash
cd /var/www/agy-cleaner

# Build binary
make build

# Install to ~/.local/bin/agy-cleaner
make install

# Clean local build artifacts
make clean
```

### Direct Go Build
```bash
go build -ldflags="-s -w" -o agy-cleaner .
```

---

## 8. Technical Architecture & Safety Mechanisms

### 7-Layer Purge Process
When a conversation is deleted, `agy-cleaner` performs all 7 steps:
1. **Database Indexing**: Removes matching row in `conversation_summaries.db`.
2. **Session Database**: Deletes `conversations/<id>.db`, `.db-wal`, and `.db-shm`.
3. **Brain & Artifacts**: Recursively deletes `brain/<id>/` (task logs, scripts, transcripts).
4. **Presence Lock**: Deletes `presence/<id>.lock`.
5. **Annotations**: Deletes `annotations/<id>.pbtxt`.
6. **Command History**: Rewrites `history.jsonl` omitting lines containing the deleted `"conversationId"`.
7. **Storage Compaction**: Executes `PRAGMA wal_checkpoint(TRUNCATE)` and SQLite `VACUUM` to release allocated blocks back to Linux filesystem.

### Kernel Concurrency Protection
Antigravity CLI holds an exclusive lock on `presence/<id>.lock` when a session is open. `agy-cleaner` tests this lock using non-blocking kernel locks:
```go
syscall.Flock(fd, syscall.LOCK_EX | syscall.LOCK_NB)
```
If the lock fails with `EWOULDBLOCK` or `EAGAIN`, the session is marked `ACTIVE` and cannot be deleted unless `--force` is explicitly provided.

---

## License

MIT License. Free to use, modify, and distribute.
