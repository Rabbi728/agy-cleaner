# agy-cleaner

A fast, safe, and powerful CLI tool written in **Go** to inspect, search, manage, and clean **Google Antigravity CLI** conversations and disk storage.

---

## Table of Contents

- [Overview](#overview)
- [Key Features](#key-features)
- [Installation](#installation)
- [Understanding Conversation Statuses](#understanding-conversation-statuses)
- [Conversation Steps & Details](#conversation-steps--details)
- [Safety & Backup System](#safety--backup-system)
- [Interactive Menu](#interactive-menu)
- [CLI Command Reference & Examples](#cli-command-reference--examples)
  - [Listing & Searching](#1-listing--searching)
  - [Cleaning & Deleting](#2-cleaning--deleting)
  - [Managing Backups](#3-managing-backups)
- [Technical Architecture](#technical-architecture)
- [License](#license)

---

## Overview

When working with Google Antigravity CLI, conversations, brain logs, transcripts, artifacts, and SQLite databases accumulate over time under `~/.gemini/antigravity-cli`, easily consuming gigabytes of disk space. Deleting these manually via a file manager is dangerous because:
1. File names are random UUIDs (`46683cb5-...`).
2. Directory-to-conversation mapping is maintained inside an SQLite database (`conversation_summaries.db`).
3. Manually deleting files leaves broken ghost/orphaned records in the database.

**`agy-cleaner`** solves this by providing a unified, safe, and fast tool that understands Antigravity's internal storage structure.

---

## Key Features

- **Interactive Terminal UI (TUI Menu)**: Launch without flags (`agy-cleaner`) to navigate an interactive menu.
- **Grouped by Workspace**: View disk usage and conversation counts categorized by project directory (e.g., backend, frontend, mobile).
- **Step Counts & Details**: Inspect individual conversations with ID, Title, Workspace, Step Count, Disk Size, and Last Modified time.
- **Search & Filter**: Search conversations by title keyword, workspace name, or UUID.
- **Ghost / Orphan Cleanup**: Detect and eliminate orphaned database rows whose physical files are already gone, as well as unindexed files on disk.
- **Active Session Protection**: Automatically detects live, running Antigravity CLI sessions via kernel file locks (`flock`) and prevents accidental deletion.
- **Automatic Pre-Clean Backups**: Creates timestamped backups of `conversation_summaries.db` and `history.jsonl` before executing any deletion.
- **One-Click Restore**: Easily restore databases and history from any previous backup point.
- **Dry-Run Mode**: Preview exactly what will be deleted and how much space will be freed using `--dry-run`.
- **History Synchronization**: Strips deleted conversation IDs from `history.jsonl` to keep prompt history clean.
- **Database Vacuuming**: Automatically runs WAL checkpoints and SQLite `VACUUM` to physically reclaim disk space.

---

## Installation

### Prerequisites
- **Go 1.22+**
- **GCC** (for SQLite3 CGO bindings)

### Build & Install

Clone or navigate to the repository:
```bash
cd /var/www/agy-cleaner
make install
```

This compiles the binary with optimization flags and installs it into `~/.local/bin/agy-cleaner`.

Verify installation:
```bash
agy-cleaner --version
# Output: agy-cleaner v1.0.0 (Go 1.27)
```

*(Note: Ensure `~/.local/bin` is in your `$PATH`.)*

---

## Understanding Conversation Statuses

When listing conversations, each entry displays a status badge:

| Status | SQLite DB Entry? | Disk Files Present? | Process Running? | Description & Action |
|:---:|:---:|:---:|:---:|---|
| <span style="color:green">**`ACTIVE`**</span> | Yes | Yes | **Yes** | A live Antigravity CLI session is **currently running** and actively using this conversation. `agy-cleaner` protects it from deletion. |
| <span style="color:gray">**`Stored`**</span> | Yes | Yes | **No** | A complete, healthy past conversation. Both database metadata and physical disk files exist. Safe to resume or delete. |
| <span style="color:goldenrod">**`ORPHAN`**</span> | Yes | **No** *(or vice-versa)* | **No** | A broken "ghost" record. Most commonly, metadata exists in the database, but files on disk were already deleted (shows `0 B` size). Safe to purge. |

To purge all ghost/orphan records in one step:
```bash
agy-cleaner clean --orphans -y
```

---

## Conversation Steps & Details

Each conversation record in Antigravity tracks its interaction steps. `agy-cleaner` displays these step counts directly in the list view:

```text
  Conversation ID                        Title                      Workspace                Steps   Size       Modified   Status  
  ──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────
  4060c725-d42a-4153-b80f-1fcf1331a12c   Golang Kore Antigravi...   agy-cleaner              159     3.6 MB     just now   ACTIVE
  d6918757-9234-4b11-83ae-a102270a3b57   Android Push Notifica...   English-Moja/englis...   296     5.4 MB     56m ago    ACTIVE
  d14bada2-a98f-4029-9ce3-5a66e02cd995   Implement Course Paym...   English-Moja/englis...   1300    19.1 MB    20h ago    Stored
```

- **Steps**: Shows how many user and model interaction steps took place in the session.
- **Size**: Total physical disk space consumed by `conversations/<id>.db` plus `brain/<id>/`.
- **Modified**: Relative time since the conversation was last active.

---

## Safety & Backup System

### What Does a Backup Contain?
Every backup captures:
1. `conversation_summaries.db` (and `-wal`, `-shm` files)
2. `history.jsonl` (CLI command & prompt history)
3. `jetbox_summaries_proto.pb` (cached protobuf summaries)

Backups are stored under:
```
~/.gemini/antigravity-cli/backups/backup_YYYYMMDD_HHMMSS_<tag>/
```

### Automatic Backups
Before `agy-cleaner` performs any deletion, it automatically creates a snapshot with the tag `pre_clean`. If you ever delete something by mistake, your database can be restored immediately.

### Listing & Restoring Backups
```bash
# List all available backups
agy-cleaner backup list

# Restore a specific backup
agy-cleaner backup restore ~/.gemini/antigravity-cli/backups/backup_before_cleanup

# Create a manual backup point
agy-cleaner backup create my_safe_point
```

---

## Interactive Menu

Simply run:
```bash
agy-cleaner
```

You will see the interactive dashboard:

```text
   ___            ______ _                              
  / _ | ___ ___ _/ ___/( )__ ____ _ ___  ___ ____       
 / __ |/ _ '/ // / /__ |// _ '/ _ '/ _ \/ -_) __/       
/_/ |_/\_, /\_, /\___/   \_,_/\_, /_//_/\__/_/          
      /___//___/             /___/                      
      Antigravity Conversation Cleaner & Manager

  Storage Overview
────────────────────────────────────────────────────────────────────────────────
  Data Directory:          /home/rabbi/.gemini/antigravity-cli
  Database Records:        137
  Total Disk Usage:        215.4 MB (Conversations: 170.7 MB, Brain: 44.8 MB)
  Active Running Sessions: 2 (Protected)
  Orphaned Records/Files:  46 total (46 in DB missing files, 0 files missing DB)
────────────────────────────────────────────────────────────────────────────────

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

  Select an option [0-8]:
```

---

## CLI Command Reference & Examples

### 1. Listing & Searching

#### Storage Overview & Workspace Summary
```bash
agy-cleaner list
```

#### List Individual Conversations (with Steps, Size, and Workspace)
```bash
# List all conversations
agy-cleaner list conversations

# Limit output to 15 conversations
agy-cleaner list conversations -n 15
```

#### Filter by Workspace
```bash
# Filter by workspace substring
agy-cleaner list -w englishmoza-backend

# Filter frontend conversations and limit to 10
agy-cleaner list -w frontend -n 10
```

#### Search by Keyword or UUID
```bash
# Search by title keyword
agy-cleaner list -s "notification"

# Search by conversation ID
agy-cleaner list -s "4060c725"
```

#### Output as JSON
```bash
agy-cleaner list --json
```

---

### 2. Cleaning & Deleting

#### Preview First (Dry-Run Mode)
Add `--dry-run` to any clean command to preview without touching files:
```bash
agy-cleaner clean -w Keya --dry-run
```

#### Delete by Workspace
```bash
# Delete conversations belonging to 'Keya'
agy-cleaner clean -w Keya

# Skip confirmation prompt (-y)
agy-cleaner clean -w Keya -y
```

#### Delete All EXCEPT a Specific Workspace
```bash
# Keep 'englishmoza-backend' and delete all others
agy-cleaner clean --keep-workspace englishmoza-backend
```

#### Delete Older Than Duration
```bash
# Delete conversations older than 30 days
agy-cleaner clean --older-than 30d

# Delete conversations older than 7 days
agy-cleaner clean --older-than 7d
```

#### Clean Orphaned / Ghost Records
```bash
# Remove all ghost DB rows and untracked files
agy-cleaner clean --orphans -y
```

#### Delete a Specific Conversation by ID
```bash
agy-cleaner clean --id 3469a52f-5fad-4de1-83b5-cd9fe5021549 -y
```

#### Delete All Conversations (Active Sessions Protected)
```bash
agy-cleaner clean --all
```

---

### 3. Managing Backups

```bash
# List backups
agy-cleaner backup list

# Create a backup
agy-cleaner backup create my_checkpoint

# Restore a backup
agy-cleaner backup restore ~/.gemini/antigravity-cli/backups/backup_20261005_121600
```

---

## Technical Architecture

### Storage Locations Cleaned
When a conversation is deleted, `agy-cleaner` thoroughly purges:

1. **`conversation_summaries.db`**: Deletes the matching row from the `conversation_summaries` SQLite table.
2. **`conversations/<id>.db*`**: Removes the conversation SQLite database and any `-wal` / `-shm` companion files.
3. **`brain/<id>/`**: Recursively deletes task logs, transcripts, scratch scripts, and artifacts.
4. **`annotations/<id>.pbtxt`**: Removes protobuf annotations.
5. **`presence/<id>.lock`**: Removes session lock files.
6. **`history.jsonl`**: Filters out matching `"conversationId"` entries while preserving non-conversation history.
7. **Compaction**: Executes `PRAGMA wal_checkpoint(TRUNCATE)` and SQLite `VACUUM` to free physical disk space back to the filesystem.

### Active Session Detection
Antigravity CLI holds an exclusive lock on `presence/<id>.lock` when running. `agy-cleaner` uses non-blocking kernel locks (`syscall.Flock` with `LOCK_EX | LOCK_NB`). If `EWOULDBLOCK` or `EAGAIN` is returned, `agy-cleaner` identifies the session as `ACTIVE` and skips deletion unless explicitly forced with `--force`.

---

## License

MIT License. Free to use, modify, and distribute.
