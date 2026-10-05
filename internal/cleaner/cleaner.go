package cleaner

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Cleaner struct {
	Scanner       *Scanner
	BackupManager *BackupManager
}

func NewCleaner(scanner *Scanner) *Cleaner {
	return &Cleaner{
		Scanner:       scanner,
		BackupManager: NewBackupManager(scanner),
	}
}

// ParseUUIDs extracts clean, unique UUIDs/IDs from strings separated by commas, spaces, or newlines
func ParseUUIDs(inputs ...string) []string {
	var result []string
	seen := make(map[string]bool)
	for _, in := range inputs {
		replaced := strings.ReplaceAll(in, ",", " ")
		fields := strings.Fields(replaced)
		for _, f := range fields {
			f = strings.TrimSpace(f)
			if f != "" && !seen[f] {
				seen[f] = true
				result = append(result, f)
			}
		}
	}
	return result
}

// FindCandidates selects conversations that match the given CleanFilter criteria
func (c *Cleaner) FindCandidates(filter CleanFilter) ([]*Conversation, []string, error) {
	convMap, _, err := c.Scanner.ScanAll()
	if err != nil {
		return nil, nil, err
	}

	var candidates []*Conversation
	var skippedActive []string
	now := time.Now()

	for _, conv := range convMap {
		matched := false

		// Check criteria
		if len(filter.ConversationIDs) > 0 {
			for _, targetID := range filter.ConversationIDs {
				if conv.ID == targetID || strings.HasPrefix(conv.ID, targetID) {
					matched = true
					break
				}
			}
		} else if filter.OnlyOrphans {
			if conv.IsOrphan {
				matched = true
			}
		} else if filter.KeepWorkspace != "" {
			// Delete all except specified workspace
			isTargetWS := false
			for _, uri := range conv.WorkspaceURIs {
				if strings.Contains(strings.ToLower(uri), strings.ToLower(filter.KeepWorkspace)) {
					isTargetWS = true
					break
				}
			}
			if !isTargetWS {
				matched = true
			}
		} else if filter.Workspace != "" {
			wsFilter := strings.ToLower(filter.Workspace)
			for _, uri := range conv.WorkspaceURIs {
				if strings.Contains(strings.ToLower(uri), wsFilter) {
					matched = true
					break
				}
			}
			if !matched && strings.Contains(strings.ToLower(conv.PrimaryWorkspace), wsFilter) {
				matched = true
			}
		} else if filter.OlderThan > 0 {
			if !conv.LastModified.IsZero() && now.Sub(conv.LastModified) > filter.OlderThan {
				matched = true
			}
		} else if filter.All {
			matched = true
		}

		if matched {
			if conv.IsActive && !filter.Force {
				skippedActive = append(skippedActive, conv.ID)
			} else {
				candidates = append(candidates, conv)
			}
		}
	}

	return candidates, skippedActive, nil
}

// ExecuteClean performs the cleanup operations based on filter
func (c *Cleaner) ExecuteClean(filter CleanFilter) (*CleanResult, error) {
	candidates, skippedActive, err := c.FindCandidates(filter)
	if err != nil {
		return nil, err
	}

	result := &CleanResult{
		SkippedActiveIDs: skippedActive,
	}

	if len(candidates) == 0 {
		return result, nil
	}

	deleteIDsMap := make(map[string]bool)
	var candidateIDs []string
	for _, conv := range candidates {
		deleteIDsMap[conv.ID] = true
		candidateIDs = append(candidateIDs, conv.ID)
		result.AffectedIDs = append(result.AffectedIDs, conv.ID)
		result.FreedDiskSpace += conv.TotalDiskSize()
	}

	if filter.DryRun {
		// Just simulate and count
		for _, conv := range candidates {
			if conv.StepCount > 0 || (conv.Title != "(No DB record - orphaned file)" && conv.Title != "(No DB record - brain folder only)") {
				result.DeletedDBRecords++
			}
			if conv.DBFileExists {
				result.DeletedDBFiles++
			}
			if conv.BrainDirExists {
				result.DeletedBrainDirs++
			}
			if conv.AnnotationExists {
				result.DeletedAnnotations++
			}
			if conv.PresenceExists {
				result.DeletedLocks++
			}
		}
		return result, nil
	}

	// 1. Create backup unless skipped
	if !filter.SkipBackup {
		backupPath, err := c.BackupManager.CreateBackup("pre_clean")
		if err != nil {
			return nil, fmt.Errorf("backup failed: %w (aborting clean to ensure safety)", err)
		}
		result.BackupDir = backupPath
	}

	// 2. Delete database records in conversation_summaries.db
	dbPath := c.Scanner.DBPath()
	if _, err := os.Stat(dbPath); err == nil {
		db, err := sql.Open("sqlite3", dbPath+"?_busy_timeout=10000")
		if err == nil {
			defer db.Close()

			// Batch delete in chunks of 50
			chunkSize := 50
			for i := 0; i < len(candidateIDs); i += chunkSize {
				end := i + chunkSize
				if end > len(candidateIDs) {
					end = len(candidateIDs)
				}
				chunk := candidateIDs[i:end]

				placeholders := make([]string, len(chunk))
				args := make([]interface{}, len(chunk))
				for j, id := range chunk {
					placeholders[j] = "?"
					args[j] = id
				}

				query := fmt.Sprintf("DELETE FROM conversation_summaries WHERE conversation_id IN (%s)", strings.Join(placeholders, ","))
				res, err := db.Exec(query, args...)
				if err == nil {
					rowsAffected, _ := res.RowsAffected()
					result.DeletedDBRecords += int(rowsAffected)
				}
			}

			// Vacuum & checkpoint if requested
			if filter.RunVacuum {
				_, _ = db.Exec("PRAGMA wal_checkpoint(TRUNCATE);")
				_, _ = db.Exec("VACUUM;")
			}
		}
	}

	// 3. Delete physical conversation database files (.db, -wal, -shm)
	convDir := c.Scanner.ConversationsDir()
	for id := range deleteIDsMap {
		baseFile := filepath.Join(convDir, id+".db")
		if _, err := os.Stat(baseFile); err == nil {
			_ = os.Remove(baseFile)
			result.DeletedDBFiles++
		}
		_ = os.Remove(baseFile + "-wal")
		_ = os.Remove(baseFile + "-shm")
	}

	// 4. Delete brain directories
	brainDir := c.Scanner.BrainDir()
	for id := range deleteIDsMap {
		dir := filepath.Join(brainDir, id)
		if _, err := os.Stat(dir); err == nil {
			_ = os.RemoveAll(dir)
			result.DeletedBrainDirs++
		}
	}

	// 5. Delete annotation files
	annotDir := c.Scanner.AnnotationsDir()
	for id := range deleteIDsMap {
		file := filepath.Join(annotDir, id+".pbtxt")
		if _, err := os.Stat(file); err == nil {
			_ = os.Remove(file)
			result.DeletedAnnotations++
		}
	}

	// 6. Delete presence lock files (only if not active)
	presDir := c.Scanner.PresenceDir()
	for id := range deleteIDsMap {
		file := filepath.Join(presDir, id+".lock")
		if _, err := os.Stat(file); err == nil {
			_ = os.Remove(file)
			result.DeletedLocks++
		}
	}

	// 7. Clean history.jsonl
	cleanedHistory, err := CleanHistory(c.Scanner.HistoryPath(), deleteIDsMap)
	if err == nil {
		result.CleanedHistoryRows = cleanedHistory
	}

	return result, nil
}
