package ui

import (
	"fmt"
	"strings"

	"agy-cleaner/internal/cleaner"
)

// PrintSystemSummary displays an overview of Antigravity storage
func PrintSystemSummary(stats *cleaner.SystemStats) {
	fmt.Println(Colorize(Bold+White, "\n  Storage Overview"))
	PrintDivider()
	fmt.Printf("  Data Directory:          %s\n", Colorize(Cyan, stats.DataDir))
	fmt.Printf("  Database Records:        %s\n", Colorize(Yellow, fmt.Sprintf("%d", stats.TotalDBRecords)))
	fmt.Printf("  Total Disk Usage:        %s (Conversations: %s, Brain: %s)\n",
		Colorize(Green+Bold, FormatBytes(stats.TotalDiskUsage)),
		FormatBytes(stats.ConversationsDirSize),
		FormatBytes(stats.BrainDirSize),
	)
	if stats.ActiveSessions > 0 {
		fmt.Printf("  Active Running Sessions: %s\n", Colorize(Green+Bold, fmt.Sprintf("%d (Protected)", stats.ActiveSessions)))
	}
	if stats.OrphanDBRecords > 0 || stats.OrphanDiskFiles > 0 {
		fmt.Printf("  Orphaned Records/Files:  %s (%d in DB missing files, %d files missing DB)\n",
			Colorize(Red+Bold, fmt.Sprintf("%d total", stats.OrphanDBRecords+stats.OrphanDiskFiles)),
			stats.OrphanDBRecords,
			stats.OrphanDiskFiles,
		)
	}
	PrintDivider()
}

// PrintWorkspaceTable prints summary grouped by workspace
func PrintWorkspaceTable(stats *cleaner.SystemStats) {
	fmt.Println(Colorize(Bold+White, "\n  Conversations by Workspace Directory\n"))

	header := fmt.Sprintf("  %-4s %-42s %-8s %-10s %-12s %-10s",
		"#", "Workspace Path", "Count", "Disk Size", "Latest", "Status")
	fmt.Println(Colorize(Bold, header))
	fmt.Println("  " + Colorize(Gray, strings.Repeat("─", 88)))

	for i, g := range stats.WorkspaceGroups {
		wsDisp := TruncateString(g.WorkspacePath, 40)
		status := ""
		if g.ActiveCount > 0 {
			status += Colorize(Green, fmt.Sprintf("[Active: %d] ", g.ActiveCount))
		}
		if g.OrphanCount > 0 {
			status += Colorize(Yellow, fmt.Sprintf("[Orphans: %d]", g.OrphanCount))
		}
		if status == "" {
			status = Colorize(Gray, "Normal")
		}

		line := fmt.Sprintf("  %-4d %-42s %-8d %-10s %-12s %s",
			i+1,
			wsDisp,
			len(g.Conversations),
			FormatBytes(g.TotalSize),
			FormatRelativeTime(g.NewestTime),
			status,
		)
		fmt.Println(line)
	}
	fmt.Println()
}

// PrintConversationList displays a detailed list of conversations
func PrintConversationList(convs []*cleaner.Conversation, max int) {
	fmt.Println(Colorize(Bold+White, "\n  Conversation Details\n"))

	header := fmt.Sprintf("  %-38s %-28s %-26s %-10s %-10s %-8s",
		"Conversation ID", "Title", "Workspace", "Size", "Modified", "Status")
	fmt.Println(Colorize(Bold, header))
	fmt.Println("  " + Colorize(Gray, strings.Repeat("─", 126)))

	displayList := convs
	if max > 0 && len(convs) > max {
		displayList = convs[:max]
	}

	for _, c := range displayList {
		status := ""
		if c.IsActive {
			status = Colorize(Green+Bold, "ACTIVE")
		} else if c.IsOrphan {
			status = Colorize(Yellow, "ORPHAN")
		} else {
			status = Colorize(Gray, "Stored")
		}

		title := c.Title
		if title == "" {
			title = "(No Title)"
		}
		title = TruncateString(title, 26)

		ws := c.PrimaryWorkspace
		if ws == "" {
			ws = "-"
		} else {
			// Strip leading /var/www/ if present for cleaner display or truncate
			ws = strings.TrimPrefix(ws, "/var/www/")
			ws = TruncateString(ws, 24)
		}

		line := fmt.Sprintf("  %-38s %-28s %-26s %-10s %-10s %s",
			c.ID,
			title,
			ws,
			FormatBytes(c.TotalDiskSize()),
			FormatRelativeTime(c.LastModified),
			status,
		)
		fmt.Println(line)
	}

	if max > 0 && len(convs) > max {
		fmt.Printf("\n  %s\n", Colorize(Dim, fmt.Sprintf("... and %d more conversations (showing first %d)", len(convs)-max, max)))
	}
	fmt.Println()
}

// PrintCleanPreview prints the list of candidates to be deleted
func PrintCleanPreview(candidates []*cleaner.Conversation, skippedActive []string, dryRun bool) {
	modeText := "CLEANUP PREVIEW"
	if dryRun {
		modeText = "DRY-RUN PREVIEW (No files or database records will be modified)"
	}
	fmt.Println(Colorize(Bold+Yellow, fmt.Sprintf("\n  === %s ===", modeText)))
	PrintDivider()

	var totalSize int64
	for _, c := range candidates {
		totalSize += c.TotalDiskSize()
	}

	fmt.Printf("  Target Conversations to Delete: %s\n", Colorize(Red+Bold, fmt.Sprintf("%d", len(candidates))))
	fmt.Printf("  Estimated Disk Space Freed:     %s\n", Colorize(Green+Bold, FormatBytes(totalSize)))

	if len(skippedActive) > 0 {
		fmt.Printf("  Skipped Active Sessions:        %s (Currently in use)\n",
			Colorize(Green+Bold, fmt.Sprintf("%d", len(skippedActive))))
	}
	PrintDivider()

	// Show sample of candidates
	maxShow := 10
	if len(candidates) > 0 {
		fmt.Println(Colorize(Bold, "\n  Sample of candidates to be removed:"))
		for i, c := range candidates {
			if i >= maxShow {
				fmt.Printf("  ... and %d more\n", len(candidates)-maxShow)
				break
			}
			title := c.Title
			if title == "" {
				title = "(No Title)"
			}
			fmt.Printf("   • [%s] %s (%s, %s)\n",
				c.ID[:8],
				TruncateString(title, 35),
				FormatBytes(c.TotalDiskSize()),
				TruncateString(c.PrimaryWorkspace, 30),
			)
		}
	}
	fmt.Println()
}

// PrintCleanResult prints the final result after cleaning
func PrintCleanResult(res *cleaner.CleanResult, dryRun bool) {
	PrintDivider()
	if dryRun {
		fmt.Println(Colorize(Cyan+Bold, "  DRY RUN COMPLETED - Nothing was deleted."))
		return
	}

	fmt.Println(Colorize(Green+Bold, "  Cleanup Completed Successfully!"))
	PrintDivider()
	if res.BackupDir != "" {
		fmt.Printf("  Automatic Backup:      %s\n", Colorize(Cyan, res.BackupDir))
	}
	fmt.Printf("  Database Records:      %s deleted\n", Colorize(White+Bold, fmt.Sprintf("%d", res.DeletedDBRecords)))
	fmt.Printf("  Conversation Files:    %s removed\n", Colorize(White+Bold, fmt.Sprintf("%d", res.DeletedDBFiles)))
	fmt.Printf("  Brain Directories:     %s removed\n", Colorize(White+Bold, fmt.Sprintf("%d", res.DeletedBrainDirs)))
	fmt.Printf("  Annotation Files:      %s removed\n", Colorize(White+Bold, fmt.Sprintf("%d", res.DeletedAnnotations)))
	fmt.Printf("  Presence Lock Files:   %s removed\n", Colorize(White+Bold, fmt.Sprintf("%d", res.DeletedLocks)))
	fmt.Printf("  History Entries:       %s removed\n", Colorize(White+Bold, fmt.Sprintf("%d", res.CleanedHistoryRows)))
	fmt.Printf("  Disk Space Reclaimed:  %s\n", Colorize(Green+Bold, FormatBytes(res.FreedDiskSpace)))

	if len(res.SkippedActiveIDs) > 0 {
		fmt.Printf("  Skipped Active:        %s active sessions protected\n", Colorize(Yellow, fmt.Sprintf("%d", len(res.SkippedActiveIDs))))
	}
	PrintDivider()
	fmt.Println()
}
