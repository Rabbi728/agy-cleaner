package cleaner

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Scanner struct {
	DataDir string
}

func NewScanner(customDataDir string) (*Scanner, error) {
	dataDir := customDataDir
	if dataDir == "" {
		dataDir = os.Getenv("ANTIGRAVITY_DATA_DIR")
	}
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home dir: %w", err)
		}
		dataDir = filepath.Join(home, ".gemini", "antigravity-cli")
	}

	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("antigravity data directory not found at: %s", dataDir)
	}

	return &Scanner{DataDir: dataDir}, nil
}

func (s *Scanner) DBPath() string {
	return filepath.Join(s.DataDir, "conversation_summaries.db")
}

func (s *Scanner) ConversationsDir() string {
	return filepath.Join(s.DataDir, "conversations")
}

func (s *Scanner) BrainDir() string {
	return filepath.Join(s.DataDir, "brain")
}

func (s *Scanner) PresenceDir() string {
	return filepath.Join(s.DataDir, "presence")
}

func (s *Scanner) AnnotationsDir() string {
	return filepath.Join(s.DataDir, "annotations")
}

func (s *Scanner) HistoryPath() string {
	return filepath.Join(s.DataDir, "history.jsonl")
}

func (s *Scanner) BackupsDir() string {
	return filepath.Join(s.DataDir, "backups")
}

// IsConversationActive checks if presence lock is actively held by an Antigravity process
func (s *Scanner) IsConversationActive(convID string) bool {
	lockPath := filepath.Join(s.PresenceDir(), convID+".lock")
	file, err := os.OpenFile(lockPath, os.O_RDWR, 0666)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		// If permission denied or other error, assume it might be in use
		return false
	}
	defer file.Close()

	return isFileLocked(file)
}

// DirSize returns total size in bytes of a directory recursively
func DirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// ScanAll scans both database records and physical disk files
func (s *Scanner) ScanAll() (map[string]*Conversation, *SystemStats, error) {
	convMap := make(map[string]*Conversation)

	// 1. Query SQLite DB
	dbPath := s.DBPath()
	if _, err := os.Stat(dbPath); err == nil {
		db, err := sql.Open("sqlite", dbPath+"?mode=ro&_busy_timeout=5000")
		if err == nil {
			defer db.Close()

			rows, err := db.Query(`
				SELECT conversation_id, title, workspace_uris, last_modified_time, step_count, status
				FROM conversation_summaries
			`)
			if err == nil {
				defer rows.Close()
				for rows.Next() {
					var id, title, rawURIs, rawTime, status string
					var steps int
					if err := rows.Scan(&id, &title, &rawURIs, &rawTime, &steps, &status); err == nil {
						uris := parseURIs(rawURIs)
						modTime, _ := time.Parse(time.RFC3339Nano, rawTime)
						if modTime.IsZero() {
							modTime, _ = time.Parse("2006-01-02 15:04:05.999999999-07:00", rawTime)
						}

						primaryWS := ""
						if len(uris) > 0 {
							primaryWS = uris[0]
						}

						convMap[id] = &Conversation{
							ID:               id,
							Title:            title,
							WorkspaceURIs:    uris,
							PrimaryWorkspace: primaryWS,
							LastModified:     modTime,
							StepCount:        steps,
							Status:           status,
						}
					}
				}
			}
		}
	}

	// 2. Scan physical files in conversations/
	convDir := s.ConversationsDir()
	if entries, err := os.ReadDir(convDir); err == nil {
		for _, e := range entries {
			name := e.Name()
			if strings.HasSuffix(name, ".db") {
				id := strings.TrimSuffix(name, ".db")
				c, ok := convMap[id]
				if !ok {
					c = &Conversation{
						ID:               id,
						Title:            "(No DB record - orphaned file)",
						PrimaryWorkspace: "(Unknown)",
					}
					convMap[id] = c
				}
				c.DBFileExists = true
				if info, err := e.Info(); err == nil {
					c.DBFileSize = info.Size()
					if c.LastModified.IsZero() {
						c.LastModified = info.ModTime()
					}
				}
			}
		}
	}

	// 3. Scan physical directories in brain/
	brainDir := s.BrainDir()
	if entries, err := os.ReadDir(brainDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				id := e.Name()
				c, ok := convMap[id]
				if !ok {
					c = &Conversation{
						ID:               id,
						Title:            "(No DB record - brain folder only)",
						PrimaryWorkspace: "(Unknown)",
					}
					convMap[id] = c
				}
				c.BrainDirExists = true
				size, _ := DirSize(filepath.Join(brainDir, id))
				c.BrainDirSize = size
				if c.LastModified.IsZero() {
					if info, err := e.Info(); err == nil {
						c.LastModified = info.ModTime()
					}
				}
			}
		}
	}

	// 4. Scan presence & annotations, check activity & orphans
	for id, c := range convMap {
		annotPath := filepath.Join(s.AnnotationsDir(), id+".pbtxt")
		if _, err := os.Stat(annotPath); err == nil {
			c.AnnotationExists = true
		}

		lockPath := filepath.Join(s.PresenceDir(), id+".lock")
		if _, err := os.Stat(lockPath); err == nil {
			c.PresenceExists = true
		}

		c.IsActive = s.IsConversationActive(id)

		// Check orphan status:
		// An orphan is: DB record exists but NO files (no DB file & no brain dir),
		// OR files exist on disk but NO DB record.
		hasDiskFiles := c.DBFileExists || c.BrainDirExists
		hasDBRecord := c.StepCount > 0 || c.Title != "(No DB record - orphaned file)" && c.Title != "(No DB record - brain folder only)"
		if (hasDBRecord && !hasDiskFiles) || (!hasDBRecord && hasDiskFiles) {
			c.IsOrphan = true
		}
	}

	// 5. Build stats & workspace groups
	stats := s.buildStats(convMap)

	return convMap, stats, nil
}

func parseURIs(raw string) []string {
	var list []string
	if err := json.Unmarshal([]byte(raw), &list); err == nil {
		for i, uri := range list {
			list[i] = strings.TrimPrefix(uri, "file://")
		}
		return list
	}
	// Fallback simple parsing if raw is not valid json array
	trimmed := strings.Trim(raw, "[]\"' ")
	if trimmed != "" {
		return []string{strings.TrimPrefix(trimmed, "file://")}
	}
	return nil
}

func (s *Scanner) buildStats(convMap map[string]*Conversation) *SystemStats {
	stats := &SystemStats{
		DataDir: s.DataDir,
	}

	wsGroups := make(map[string]*WorkspaceGroup)

	for _, c := range convMap {
		// Total disk files / size
		if c.DBFileExists {
			stats.TotalDiskFiles++
			stats.ConversationsDirSize += c.DBFileSize
		}
		if c.BrainDirExists {
			stats.BrainDirSize += c.BrainDirSize
		}
		stats.TotalDiskUsage += c.TotalDiskSize()

		if c.StepCount > 0 || (c.Title != "(No DB record - orphaned file)" && c.Title != "(No DB record - brain folder only)") {
			stats.TotalDBRecords++
			if !c.DBFileExists && !c.BrainDirExists {
				stats.OrphanDBRecords++
			}
		} else {
			stats.OrphanDiskFiles++
		}

		if c.IsActive {
			stats.ActiveSessions++
		}

		// Group by workspace
		ws := c.PrimaryWorkspace
		if ws == "" {
			ws = "(No Workspace / Global)"
		}
		group, ok := wsGroups[ws]
		if !ok {
			group = &WorkspaceGroup{
				WorkspacePath: ws,
				OldestTime:    c.LastModified,
				NewestTime:    c.LastModified,
			}
			wsGroups[ws] = group
		}
		group.Conversations = append(group.Conversations, c)
		group.TotalSize += c.TotalDiskSize()
		if c.IsActive {
			group.ActiveCount++
		}
		if c.IsOrphan {
			group.OrphanCount++
		}
		if !c.LastModified.IsZero() {
			if group.OldestTime.IsZero() || c.LastModified.Before(group.OldestTime) {
				group.OldestTime = c.LastModified
			}
			if c.LastModified.After(group.NewestTime) {
				group.NewestTime = c.LastModified
			}
		}
	}

	for _, g := range wsGroups {
		// Sort conversations in group newest first
		sort.Slice(g.Conversations, func(i, j int) bool {
			return g.Conversations[i].LastModified.After(g.Conversations[j].LastModified)
		})
		stats.WorkspaceGroups = append(stats.WorkspaceGroups, g)
	}

	// Sort groups by total count descending, then total size
	sort.Slice(stats.WorkspaceGroups, func(i, j int) bool {
		if len(stats.WorkspaceGroups[i].Conversations) != len(stats.WorkspaceGroups[j].Conversations) {
			return len(stats.WorkspaceGroups[i].Conversations) > len(stats.WorkspaceGroups[j].Conversations)
		}
		return stats.WorkspaceGroups[i].TotalSize > stats.WorkspaceGroups[j].TotalSize
	})

	return stats
}
