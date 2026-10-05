package cleaner

import (
	"time"
)

// Conversation represents an Antigravity conversation record
type Conversation struct {
	ID               string    `json:"id"`
	Title            string    `json:"title"`
	WorkspaceURIs    []string  `json:"workspace_uris"`
	PrimaryWorkspace string    `json:"primary_workspace"`
	LastModified     time.Time `json:"last_modified"`
	StepCount        int       `json:"step_count"`
	Status           string    `json:"status"`

	// Disk usage & status
	DBFileExists      bool  `json:"db_file_exists"`
	DBFileSize        int64 `json:"db_file_size"`
	BrainDirExists    bool  `json:"brain_dir_exists"`
	BrainDirSize      int64 `json:"brain_dir_size"`
	AnnotationExists  bool  `json:"annotation_exists"`
	PresenceExists    bool  `json:"presence_exists"`
	IsActive          bool  `json:"is_active"`
	IsOrphan          bool  `json:"is_orphan"` // DB row exists but no files, or vice versa
	HistoryEntryCount int   `json:"history_entry_count"`
}

// TotalDiskSize returns total disk space used by this conversation
func (c *Conversation) TotalDiskSize() int64 {
	return c.DBFileSize + c.BrainDirSize
}

// WorkspaceGroup groups conversations by their workspace path
type WorkspaceGroup struct {
	WorkspacePath string          `json:"workspace_path"`
	Conversations []*Conversation `json:"conversations"`
	TotalSize     int64           `json:"total_size"`
	ActiveCount   int             `json:"active_count"`
	OrphanCount   int             `json:"orphan_count"`
	OldestTime    time.Time       `json:"oldest_time"`
	NewestTime    time.Time       `json:"newest_time"`
}

// SystemStats contains general statistics about Antigravity storage
type SystemStats struct {
	DataDir             string            `json:"data_dir"`
	TotalDBRecords      int               `json:"total_db_records"`
	TotalDiskFiles      int               `json:"total_disk_files"`
	TotalDiskUsage      int64             `json:"total_disk_usage"`
	ConversationsDirSize int64            `json:"conversations_dir_size"`
	BrainDirSize        int64             `json:"brain_dir_size"`
	OrphanDBRecords     int               `json:"orphan_db_records"`
	OrphanDiskFiles     int               `json:"orphan_disk_files"`
	ActiveSessions      int               `json:"active_sessions"`
	WorkspaceGroups     []*WorkspaceGroup `json:"workspace_groups"`
}

// CleanFilter defines criteria for cleaning conversations
type CleanFilter struct {
	Workspace        string        // Workspace substring or exact match
	KeepWorkspace    string        // Delete all EXCEPT this workspace
	OlderThan        time.Duration // Delete conversations older than this duration
	ConversationIDs  []string      // Specific conversation IDs
	OnlyOrphans      bool          // Clean only orphaned records/files
	All              bool          // Clean everything (except active)
	Force            bool          // Allow deleting active sessions
	DryRun           bool          // Do not delete anything, only preview
	SkipBackup       bool          // Skip backup before deleting
	RunVacuum        bool          // Vacuum DB after deleting
}

// CleanResult reports what was removed
type CleanResult struct {
	DeletedDBRecords   int      `json:"deleted_db_records"`
	DeletedDBFiles     int      `json:"deleted_db_files"`
	DeletedBrainDirs   int      `json:"deleted_brain_dirs"`
	DeletedAnnotations int      `json:"deleted_annotations"`
	DeletedLocks       int      `json:"deleted_locks"`
	CleanedHistoryRows int      `json:"cleaned_history_rows"`
	FreedDiskSpace     int64    `json:"freed_disk_space"`
	BackupDir          string   `json:"backup_dir,omitempty"`
	SkippedActiveIDs   []string `json:"skipped_active_ids,omitempty"`
	AffectedIDs        []string `json:"affected_ids"`
}
