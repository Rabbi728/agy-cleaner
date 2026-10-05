package ui

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"agy-cleaner/internal/cleaner"
)

type InteractiveCLI struct {
	Cleaner *cleaner.Cleaner
	Scanner *cleaner.Scanner
	reader  *bufio.Reader
}

func NewInteractiveCLI(c *cleaner.Cleaner, s *cleaner.Scanner) *InteractiveCLI {
	return &InteractiveCLI{
		Cleaner: c,
		Scanner: s,
		reader:  bufio.NewReader(os.Stdin),
	}
}

func (cli *InteractiveCLI) readLine() string {
	text, _ := cli.reader.ReadString('\n')
	return strings.TrimSpace(text)
}

func (cli *InteractiveCLI) askConfirm(prompt string) bool {
	fmt.Printf("  %s %s: ", prompt, Colorize(Yellow, "[y/N]"))
	ans := strings.ToLower(cli.readLine())
	return ans == "y" || ans == "yes"
}

func (cli *InteractiveCLI) Run() {
	PrintBanner()

	for {
		_, stats, err := cli.Scanner.ScanAll()
		if err != nil {
			fmt.Printf("Error scanning: %v\n", err)
			return
		}

		PrintSystemSummary(stats)

		fmt.Println(Colorize(Bold+White, "\n  Menu Options:\n"))
		fmt.Println("  1.  View Workspaces & Conversation Counts")
		fmt.Println("  2.  List Detailed Conversations")
		fmt.Println("  3.  Clean Conversations by Workspace")
		fmt.Println("  4.  Clean Conversations Older Than X Days")
		fmt.Println("  5.  Clean Orphaned / Ghost Records Only")
		fmt.Println("  6.  Delete a Specific Conversation by ID")
		fmt.Println("  7.  Clean ALL Conversations (Except Active)")
		fmt.Println("  8.  Backups (List & Restore)")
		fmt.Println("  0.  Exit")
		fmt.Print("\n  Select an option [0-8]: ")

		choice := cli.readLine()
		fmt.Println()

		switch choice {
		case "1":
			PrintWorkspaceTable(stats)

		case "2":
			cli.handleListDetailed()

		case "3":
			cli.handleCleanByWorkspace(stats)

		case "4":
			cli.handleCleanOlderThan()

		case "5":
			cli.handleCleanOrphans()

		case "6":
			cli.handleCleanByID()

		case "7":
			cli.handleCleanAll()

		case "8":
			cli.handleBackups()

		case "0", "exit", "quit", "q":
			fmt.Println(Colorize(Green, "  Goodbye!"))
			return

		default:
			fmt.Println(Colorize(Red, "  Invalid option. Please choose between 0 and 8."))
		}

		fmt.Println("\n  Press Enter to continue...")
		_ = cli.readLine()
	}
}

func (cli *InteractiveCLI) handleListDetailed() {
	fmt.Print("  Enter workspace filter (or press Enter for all): ")
	filter := cli.readLine()

	convMap, _, err := cli.Scanner.ScanAll()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	var list []*cleaner.Conversation
	for _, c := range convMap {
		if filter == "" || strings.Contains(strings.ToLower(c.PrimaryWorkspace), strings.ToLower(filter)) {
			list = append(list, c)
		}
	}

	PrintConversationList(list, 50)
}

func (cli *InteractiveCLI) handleCleanByWorkspace(stats *cleaner.SystemStats) {
	if len(stats.WorkspaceGroups) == 0 {
		fmt.Println(Colorize(Yellow, "  No workspaces found."))
		return
	}

	PrintWorkspaceTable(stats)
	fmt.Print("  Enter the workspace number (#) to delete: ")
	input := cli.readLine()
	idx, err := strconv.Atoi(input)
	if err != nil || idx < 1 || idx > len(stats.WorkspaceGroups) {
		fmt.Println(Colorize(Red, "  Invalid selection."))
		return
	}

	targetGroup := stats.WorkspaceGroups[idx-1]
	wsPath := targetGroup.WorkspacePath
	fmt.Printf("\n  Selected: %s (%d conversations, %s)\n",
		Colorize(Cyan+Bold, wsPath),
		len(targetGroup.Conversations),
		FormatBytes(targetGroup.TotalSize),
	)

	filter := cleaner.CleanFilter{
		Workspace:  wsPath,
		RunVacuum:  true,
		SkipBackup: false,
	}

	candidates, skippedActive, err := cli.Cleaner.FindCandidates(filter)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	PrintCleanPreview(candidates, skippedActive, false)

	if len(candidates) == 0 {
		fmt.Println(Colorize(Yellow, "  No matching conversations eligible for deletion."))
		return
	}

	if !cli.askConfirm("Are you sure you want to delete these conversations?") {
		fmt.Println(Colorize(Yellow, "  Operation canceled."))
		return
	}

	result, err := cli.Cleaner.ExecuteClean(filter)
	if err != nil {
		fmt.Printf("Error during clean: %v\n", err)
		return
	}

	PrintCleanResult(result, false)
}

func (cli *InteractiveCLI) handleCleanOlderThan() {
	fmt.Print("  Delete conversations older than how many days? (e.g. 7, 30): ")
	input := cli.readLine()
	days, err := strconv.Atoi(input)
	if err != nil || days <= 0 {
		fmt.Println(Colorize(Red, "  Please enter a valid positive number of days."))
		return
	}

	dur := time.Duration(days) * 24 * time.Hour
	filter := cleaner.CleanFilter{
		OlderThan:  dur,
		RunVacuum:  true,
		SkipBackup: false,
	}

	candidates, skippedActive, err := cli.Cleaner.FindCandidates(filter)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	PrintCleanPreview(candidates, skippedActive, false)

	if len(candidates) == 0 {
		fmt.Println(Colorize(Yellow, "  No conversations older than that found."))
		return
	}

	if !cli.askConfirm(fmt.Sprintf("Are you sure you want to delete conversations older than %d days?", days)) {
		fmt.Println(Colorize(Yellow, "  Operation canceled."))
		return
	}

	result, err := cli.Cleaner.ExecuteClean(filter)
	if err != nil {
		fmt.Printf("Error during clean: %v\n", err)
		return
	}

	PrintCleanResult(result, false)
}

func (cli *InteractiveCLI) handleCleanOrphans() {
	filter := cleaner.CleanFilter{
		OnlyOrphans: true,
		RunVacuum:   true,
		SkipBackup:  false,
	}

	candidates, skippedActive, err := cli.Cleaner.FindCandidates(filter)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	PrintCleanPreview(candidates, skippedActive, false)

	if len(candidates) == 0 {
		fmt.Println(Colorize(Green, "  No orphaned records or files found! Everything is clean."))
		return
	}

	if !cli.askConfirm("Are you sure you want to delete all orphaned records and files?") {
		fmt.Println(Colorize(Yellow, "  Operation canceled."))
		return
	}

	result, err := cli.Cleaner.ExecuteClean(filter)
	if err != nil {
		fmt.Printf("Error during clean: %v\n", err)
		return
	}

	PrintCleanResult(result, false)
}

func (cli *InteractiveCLI) handleCleanByID() {
	fmt.Print("  Enter full or partial Conversation ID (UUID): ")
	id := cli.readLine()
	if id == "" {
		fmt.Println(Colorize(Red, "  ID cannot be empty."))
		return
	}

	filter := cleaner.CleanFilter{
		ConversationIDs: []string{id},
		RunVacuum:       true,
		SkipBackup:      false,
	}

	candidates, skippedActive, err := cli.Cleaner.FindCandidates(filter)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	PrintCleanPreview(candidates, skippedActive, false)

	if len(candidates) == 0 {
		fmt.Println(Colorize(Yellow, "  No matching conversation found."))
		return
	}

	if !cli.askConfirm("Are you sure you want to delete this conversation?") {
		fmt.Println(Colorize(Yellow, "  Operation canceled."))
		return
	}

	result, err := cli.Cleaner.ExecuteClean(filter)
	if err != nil {
		fmt.Printf("Error during clean: %v\n", err)
		return
	}

	PrintCleanResult(result, false)
}

func (cli *InteractiveCLI) handleCleanAll() {
	fmt.Println(Colorize(Red+Bold, "\n  WARNING: This will delete ALL stored conversations except active sessions!"))
	filter := cleaner.CleanFilter{
		All:        true,
		RunVacuum:  true,
		SkipBackup: false,
	}

	candidates, skippedActive, err := cli.Cleaner.FindCandidates(filter)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	PrintCleanPreview(candidates, skippedActive, false)

	if len(candidates) == 0 {
		fmt.Println(Colorize(Yellow, "  No conversations found to delete."))
		return
	}

	if !cli.askConfirm("ARE YOU ABSOLUTELY SURE you want to delete ALL conversations?") {
		fmt.Println(Colorize(Yellow, "  Operation canceled."))
		return
	}

	result, err := cli.Cleaner.ExecuteClean(filter)
	if err != nil {
		fmt.Printf("Error during clean: %v\n", err)
		return
	}

	PrintCleanResult(result, false)
}

func (cli *InteractiveCLI) handleBackups() {
	bm := cli.Cleaner.BackupManager
	backups, err := bm.ListBackups()
	if err != nil {
		fmt.Printf("Error listing backups: %v\n", err)
		return
	}

	fmt.Println(Colorize(Bold+White, "\n  Available Backups:\n"))
	if len(backups) == 0 {
		fmt.Println(Colorize(Yellow, "  No backups found."))
	} else {
		for i, b := range backups {
			fmt.Printf("  %d. %-30s (%s, %s)\n",
				i+1,
				b.Name,
				FormatBytes(b.SizeBytes),
				FormatTime(b.CreatedAt),
			)
		}
	}

	fmt.Println("\n  1. Create new backup now")
	if len(backups) > 0 {
		fmt.Println("  2. Restore from a backup")
	}
	fmt.Println("  0. Back")
	fmt.Print("\n  Select an option: ")

	opt := cli.readLine()
	switch opt {
	case "1":
		fmt.Print("  Enter optional backup tag (or press Enter): ")
		tag := cli.readLine()
		targetDir, err := bm.CreateBackup(tag)
		if err != nil {
			fmt.Printf("Error creating backup: %v\n", err)
		} else {
			fmt.Println(Colorize(Green+Bold, fmt.Sprintf("  Backup created successfully at: %s", targetDir)))
		}
	case "2":
		if len(backups) == 0 {
			return
		}
		fmt.Print("  Enter backup number to restore: ")
		numStr := cli.readLine()
		n, err := strconv.Atoi(numStr)
		if err != nil || n < 1 || n > len(backups) {
			fmt.Println(Colorize(Red, "  Invalid selection."))
			return
		}
		selected := backups[n-1]
		if cli.askConfirm(fmt.Sprintf("Restore backup '%s'? This will overwrite current DB and history!", selected.Name)) {
			if err := bm.RestoreBackup(selected.Path); err != nil {
				fmt.Printf("Error restoring: %v\n", err)
			} else {
				fmt.Println(Colorize(Green+Bold, "  Backup restored successfully!"))
			}
		}
	}
}
