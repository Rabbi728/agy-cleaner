package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"agy-cleaner/internal/cleaner"
	"agy-cleaner/internal/ui"
)

func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "d") {
		daysStr := strings.TrimSuffix(s, "d")
		days, err := strconv.Atoi(daysStr)
		if err != nil {
			return 0, err
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

// reorderArgs moves all flags (starting with '-') to the front so flag.FlagSet can parse them even if passed after positional args
func reorderArgs(args []string) []string {
	var flags []string
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			flags = append(flags, arg)
			// Check if flag takes a separate value argument
			if !strings.Contains(arg, "=") && (arg == "-w" || arg == "--workspace" || arg == "--keep-workspace" ||
				arg == "--older-than" || arg == "--id" || arg == "--ids" || arg == "-s" || arg == "--search" ||
				arg == "-n" || arg == "--limit" || arg == "--data-dir") {
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
					flags = append(flags, args[i+1])
					i++
				}
			}
		} else {
			positional = append(positional, arg)
		}
	}
	return append(flags, positional...)
}

func main() {
	var (
		dataDirFlag = flag.String("data-dir", "", "Custom Antigravity data directory (default: ~/.gemini/antigravity-cli)")
		helpFlag    = flag.Bool("help", false, "Show help message")
		versionFlag = flag.Bool("version", false, "Show version")
	)

	flag.Usage = printHelp
	flag.Parse()

	if *helpFlag {
		printHelp()
		return
	}

	if *versionFlag {
		fmt.Println("agy-cleaner v1.0.0 (Go 1.27)")
		return
	}

	scanner, err := cleaner.NewScanner(*dataDirFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	c := cleaner.NewCleaner(scanner)
	args := flag.Args()

	if len(args) == 0 {
		// Launch interactive mode
		cli := ui.NewInteractiveCLI(c, scanner)
		cli.Run()
		return
	}

	command := args[0]
	cmdArgs := args[1:]

	switch command {
	case "list", "ls":
		handleList(scanner, cmdArgs)
	case "clean", "delete", "rm":
		handleClean(c, scanner, cmdArgs)
	case "backup":
		handleBackup(c.BackupManager, cmdArgs)
	case "interactive", "menu":
		cli := ui.NewInteractiveCLI(c, scanner)
		cli.Run()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printHelp()
		os.Exit(1)
	}
}

func handleList(scanner *cleaner.Scanner, args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	workspace := fs.String("workspace", "", "Filter by workspace path")
	fs.StringVar(workspace, "w", "", "Filter by workspace path (shorthand)")
	search := fs.String("search", "", "Search by title or conversation ID")
	fs.StringVar(search, "s", "", "Search (shorthand)")
	limit := fs.Int("limit", 50, "Limit number of conversations to display")
	fs.IntVar(limit, "n", 50, "Limit (shorthand)")
	showAll := fs.Bool("all", false, "Show detailed list of individual conversations")
	jsonOutput := fs.Bool("json", false, "Output results in JSON format")
	_ = fs.Parse(reorderArgs(args))

	// If extra argument like 'conversations' or 'convs' is given
	if fs.NArg() > 0 {
		first := strings.ToLower(fs.Arg(0))
		if first == "conversations" || first == "convs" || first == "all" {
			*showAll = true
		} else if *workspace == "" {
			*workspace = fs.Arg(0)
		}
	}

	convMap, stats, err := scanner.ScanAll()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if *jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(stats)
		return
	}

	ui.PrintSystemSummary(stats)

	if *showAll || *workspace != "" || *search != "" {
		var list []*cleaner.Conversation
		searchLower := strings.ToLower(*search)
		for _, conv := range convMap {
			wsMatch := *workspace == "" || strings.Contains(strings.ToLower(conv.PrimaryWorkspace), strings.ToLower(*workspace))
			searchMatch := *search == "" || strings.Contains(strings.ToLower(conv.Title), searchLower) || strings.Contains(strings.ToLower(conv.ID), searchLower) || strings.Contains(strings.ToLower(conv.PrimaryWorkspace), searchLower)
			if wsMatch && searchMatch {
				list = append(list, conv)
			}
		}

		// Sort newest first
		sort.Slice(list, func(i, j int) bool {
			return list[i].LastModified.After(list[j].LastModified)
		})

		ui.PrintConversationList(list, *limit)
	} else {
		ui.PrintWorkspaceTable(stats)
		fmt.Println("  Tip: Run `agy-cleaner list --all` to view individual conversations,")
		fmt.Println("       or `agy-cleaner list -w <name>` to filter conversations by workspace,")
		fmt.Println("       or `agy-cleaner list -s <search>` to search by title/ID.")
	}
}

func handleClean(c *cleaner.Cleaner, scanner *cleaner.Scanner, args []string) {
	fs := flag.NewFlagSet("clean", flag.ExitOnError)
	workspace := fs.String("workspace", "", "Delete conversations matching workspace name/path")
	fs.StringVar(workspace, "w", "", "Workspace (shorthand)")
	keepWS := fs.String("keep-workspace", "", "Delete ALL conversations EXCEPT this workspace")
	olderThanStr := fs.String("older-than", "", "Delete conversations older than duration (e.g. 30d, 7d, 24h)")
	id := fs.String("id", "", "Delete specific conversation ID or multiple UUIDs (comma or space separated)")
	ids := fs.String("ids", "", "Delete multiple conversation IDs (comma or space separated)")
	orphansOnly := fs.Bool("orphans", false, "Clean only orphaned records and files")
	all := fs.Bool("all", false, "Delete all stored conversations")
	dryRun := fs.Bool("dry-run", false, "Preview deletions without modifying anything")
	yes := fs.Bool("yes", false, "Skip confirmation prompt")
	fs.BoolVar(yes, "y", false, "Skip confirmation (shorthand)")
	noBackup := fs.Bool("no-backup", false, "Skip automatic pre-cleanup backup")
	force := fs.Bool("force", false, "Allow deleting active running sessions")
	_ = fs.Parse(reorderArgs(args))

	var rawIDs []string
	if *id != "" {
		rawIDs = append(rawIDs, *id)
	}
	if *ids != "" {
		rawIDs = append(rawIDs, *ids)
	}
	// Also accept positional UUIDs e.g.: agy-cleaner clean uuid1 uuid2 uuid3
	rawIDs = append(rawIDs, fs.Args()...)
	convIDs := cleaner.ParseUUIDs(rawIDs...)

	if *workspace == "" && *keepWS == "" && *olderThanStr == "" && len(convIDs) == 0 && !*orphansOnly && !*all {
		fmt.Println("Error: No filter specified. Specify --workspace, --older-than, --orphans, --id/--ids, or --all.")
		fmt.Println("Run `agy-cleaner --help` for usage.")
		os.Exit(1)
	}

	var dur time.Duration
	if *olderThanStr != "" {
		var err error
		dur, err = parseDuration(*olderThanStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid duration '%s': %v\n", *olderThanStr, err)
			os.Exit(1)
		}
	}

	filter := cleaner.CleanFilter{
		Workspace:       *workspace,
		KeepWorkspace:   *keepWS,
		OlderThan:       dur,
		ConversationIDs: convIDs,
		OnlyOrphans:     *orphansOnly,
		All:             *all,
		Force:           *force,
		DryRun:          *dryRun,
		SkipBackup:      *noBackup,
		RunVacuum:       true,
	}

	candidates, skippedActive, err := c.FindCandidates(filter)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	ui.PrintCleanPreview(candidates, skippedActive, *dryRun)

	if len(candidates) == 0 {
		fmt.Println(ui.Colorize(ui.Green, "  No matching conversations found for deletion."))
		return
	}

	if *dryRun {
		res, _ := c.ExecuteClean(filter)
		ui.PrintCleanResult(res, true)
		return
	}

	if !*yes {
		fmt.Print("  Proceed with deletion? [y/N]: ")
		var reply string
		_, _ = fmt.Scanln(&reply)
		reply = strings.ToLower(strings.TrimSpace(reply))
		if reply != "y" && reply != "yes" {
			fmt.Println(ui.Colorize(ui.Yellow, "  Clean canceled."))
			return
		}
	}

	result, err := c.ExecuteClean(filter)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error during cleanup: %v\n", err)
		os.Exit(1)
	}

	ui.PrintCleanResult(result, false)
}

func handleBackup(bm *cleaner.BackupManager, args []string) {
	if len(args) == 0 {
		args = []string{"list"}
	}

	sub := args[0]
	switch sub {
	case "create":
		tag := ""
		if len(args) > 1 {
			tag = args[1]
		}
		path, err := bm.CreateBackup(tag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Backup failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Backup created at: %s\n", path)

	case "list":
		backups, err := bm.ListBackups()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if len(backups) == 0 {
			fmt.Println("No backups found.")
			return
		}
		fmt.Println("\nAvailable Backups:")
		for i, b := range backups {
			fmt.Printf("  [%d] %-30s (%s, %s)\n",
				i+1,
				b.Name,
				ui.FormatBytes(b.SizeBytes),
				ui.FormatTime(b.CreatedAt),
			)
		}
		fmt.Println()

	case "restore":
		if len(args) < 2 {
			fmt.Println("Usage: agy-cleaner backup restore <backup_path_or_name>")
			os.Exit(1)
		}
		target := args[1]
		if err := bm.RestoreBackup(target); err != nil {
			fmt.Fprintf(os.Stderr, "Restore failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Backup restored successfully!")

	case "delete", "rm":
		if len(args) < 2 {
			fmt.Println("Usage: agy-cleaner backup delete <backup_path_or_name>")
			os.Exit(1)
		}
		target := args[1]
		if err := bm.DeleteBackup(target); err != nil {
			fmt.Fprintf(os.Stderr, "Error deleting backup: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Backup '%s' deleted successfully!\n", target)

	case "prune":
		keep := 5
		if len(args) > 1 {
			var err error
			keep, err = strconv.Atoi(args[1])
			if err != nil || keep < 0 {
				fmt.Println("Error: Invalid keep count. Example: agy-cleaner backup prune 5")
				os.Exit(1)
			}
		}
		deleted, err := bm.PruneBackups(keep)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error pruning backups: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Pruned %d old backup(s) successfully (kept newest %d).\n", deleted, keep)

	default:
		fmt.Println("Usage: agy-cleaner backup [create|list|restore|delete|prune]")
	}
}

func printHelp() {
	ui.PrintBanner()
	helpText := `
Usage:
  agy-cleaner                           Start interactive terminal UI
  agy-cleaner list [flags]              List stored conversations and workspaces
  agy-cleaner clean [flags]             Delete conversations matching filters
  agy-cleaner backup [subcommand]       Manage database backups

Commands:
  interactive, menu    Launch interactive terminal menu
  list, ls             Show conversations overview or details
  clean, delete, rm    Safely delete conversations from DB and disk
  backup               Backup operations (create, list, restore)

Clean Flags:
  -w, --workspace <str>       Delete conversations for workspace (e.g. -w Keya)
  --keep-workspace <str>      Delete all EXCEPT this workspace
  --older-than <duration>     Delete conversations older than duration (e.g. 30d, 7d)
  --id, --ids <uuid,...>      Delete one or multiple conversation IDs (comma/space separated or positional)
  --orphans                   Clean only ghost/orphaned records and files
  --all                       Delete ALL conversations (except active sessions)
  --dry-run                   Preview what will be deleted without modifying anything
  -y, --yes                   Skip confirmation prompt
  --no-backup                 Skip automatic pre-cleanup backup
  --force                     Force deletion even if session appears active

List Flags:
  -w, --workspace <str>       Filter list by workspace
  -s, --search <str>          Search by title, ID, or workspace keyword
  -n, --limit <int>           Limit number of conversations to display (default: 50)
  --all                       List individual conversation records
  --json                      Output raw statistics as JSON

Examples:
  # Start interactive menu
  agy-cleaner

  # See overview of workspaces & disk usage
  agy-cleaner list

  # Delete multiple conversations by UUIDs
  agy-cleaner clean --ids "uuid1, uuid2, uuid3"
  agy-cleaner clean uuid1 uuid2 uuid3

  # Dry run: preview what would be deleted for workspace 'Keya'
  agy-cleaner clean -w Keya --dry-run

  # Delete conversations for 'Keya' with automatic backup & confirmation
  agy-cleaner clean -w Keya

  # Delete all conversations except English-Moja backend
  agy-cleaner clean --keep-workspace englishmoza-backend

  # Clean conversations older than 30 days
  agy-cleaner clean --older-than 30d

  # Clean orphaned records (DB entries missing disk files or vice versa)
  agy-cleaner clean --orphans -y
`
	fmt.Println(helpText)
}
