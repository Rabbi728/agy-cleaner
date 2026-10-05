package cleaner

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type BackupInfo struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	CreatedAt time.Time `json:"created_at"`
	SizeBytes int64     `json:"size_bytes"`
	Files     []string  `json:"files"`
}

type BackupManager struct {
	Scanner *Scanner
}

func NewBackupManager(scanner *Scanner) *BackupManager {
	return &BackupManager{Scanner: scanner}
}

// CreateBackup backs up conversation_summaries.db and history.jsonl
func (bm *BackupManager) CreateBackup(tag string) (string, error) {
	backupsDir := bm.Scanner.BackupsDir()
	if err := os.MkdirAll(backupsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create backups directory: %w", err)
	}

	timestamp := time.Now().Format("20060102_150405")
	backupName := fmt.Sprintf("backup_%s", timestamp)
	if tag != "" {
		backupName = fmt.Sprintf("backup_%s_%s", timestamp, tag)
	}
	targetDir := filepath.Join(backupsDir, backupName)

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create target backup dir: %w", err)
	}

	// Files to back up
	filesToBackup := []string{
		"conversation_summaries.db",
		"conversation_summaries.db-wal",
		"conversation_summaries.db-shm",
		"history.jsonl",
		"jetbox_summaries_proto.pb",
	}

	copied := 0
	for _, f := range filesToBackup {
		src := filepath.Join(bm.Scanner.DataDir, f)
		if _, err := os.Stat(src); err == nil {
			dst := filepath.Join(targetDir, f)
			if err := copyFile(src, dst); err != nil {
				return "", fmt.Errorf("failed to copy %s to backup: %w", f, err)
			}
			copied++
		}
	}

	if copied == 0 {
		_ = os.RemoveAll(targetDir)
		return "", fmt.Errorf("no database or history files found to back up")
	}

	return targetDir, nil
}

// ListBackups lists existing backups in descending order of creation time
func (bm *BackupManager) ListBackups() ([]*BackupInfo, error) {
	searchDirs := []string{bm.Scanner.BackupsDir(), bm.Scanner.DataDir}
	var backups []*BackupInfo
	seen := make(map[string]bool)

	for _, dir := range searchDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, e := range entries {
			if e.IsDir() && strings.HasPrefix(e.Name(), "backup_") {
				fullPath := filepath.Join(dir, e.Name())
				if seen[fullPath] {
					continue
				}
				seen[fullPath] = true

				info, _ := e.Info()
				size, _ := DirSize(fullPath)

				var files []string
				if subEntries, err := os.ReadDir(fullPath); err == nil {
					for _, sub := range subEntries {
						files = append(files, sub.Name())
					}
				}

				modTime := time.Now()
				if info != nil {
					modTime = info.ModTime()
				}

				backups = append(backups, &BackupInfo{
					Name:      e.Name(),
					Path:      fullPath,
					CreatedAt: modTime,
					SizeBytes: size,
					Files:     files,
				})
			}
		}
	}

	sort.Slice(backups, func(i, j int) bool {
		return backups[i].CreatedAt.After(backups[j].CreatedAt)
	})

	return backups, nil
}

// RestoreBackup restores DB and history files from a specific backup directory
func (bm *BackupManager) RestoreBackup(backupDir string) error {
	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		return fmt.Errorf("backup directory not found: %s", backupDir)
	}

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return fmt.Errorf("failed to read backup directory: %w", err)
	}

	for _, e := range entries {
		if !e.IsDir() {
			src := filepath.Join(backupDir, e.Name())
			dst := filepath.Join(bm.Scanner.DataDir, e.Name())

			// Copy file to restore
			if err := copyFile(src, dst); err != nil {
				return fmt.Errorf("failed to restore file %s: %w", e.Name(), err)
			}
		}
	}

	return nil
}

// DeleteBackup removes a specific backup folder by name or path
func (bm *BackupManager) DeleteBackup(target string) error {
	backups, err := bm.ListBackups()
	if err != nil {
		return err
	}

	var targetPath string
	for _, b := range backups {
		if b.Name == target || b.Path == target || filepath.Base(b.Path) == target {
			targetPath = b.Path
			break
		}
	}

	if targetPath == "" {
		if _, err := os.Stat(target); err == nil {
			targetPath = target
		} else {
			return fmt.Errorf("backup not found: %s", target)
		}
	}

	return os.RemoveAll(targetPath)
}

// PruneBackups keeps only the newest keepCount backups and deletes older ones
func (bm *BackupManager) PruneBackups(keepCount int) (int, error) {
	if keepCount < 0 {
		keepCount = 0
	}
	backups, err := bm.ListBackups()
	if err != nil {
		return 0, err
	}

	if len(backups) <= keepCount {
		return 0, nil
	}

	deleted := 0
	for i := keepCount; i < len(backups); i++ {
		if err := os.RemoveAll(backups[i].Path); err == nil {
			deleted++
		}
	}
	return deleted, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
