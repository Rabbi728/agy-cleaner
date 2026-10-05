package ui

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// ANSI color codes
const (
	Reset   = "\033[0m"
	Bold    = "\033[1m"
	Dim     = "\033[2m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	White   = "\033[37m"
	Gray    = "\033[90m"
)

func Colorize(color, text string) string {
	if os.Getenv("NO_COLOR") != "" {
		return text
	}
	return color + text + Reset
}

func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func FormatTime(t time.Time) string {
	if t.IsZero() {
		return "N/A"
	}
	return t.Format("2006-01-02 15:04")
}

func FormatRelativeTime(t time.Time) string {
	if t.IsZero() {
		return "N/A"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return fmt.Sprintf("%dmo ago", int(d.Hours()/(24*30)))
	}
}

func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func PrintBanner() {
	banner := "" +
		"   ___            ______ _                              \n" +
		"  / _ | ___ ___ _/ ___/( )__ ____ _ ___  ___ ____       \n" +
		" / __ |/ _ '/ // / /__ |// _ '/ _ '/ _ \\/ -_) __/       \n" +
		"/_/ |_/\\_, /\\_, /\\___/   \\_,_/\\_, /_//_/\\__/_/          \n" +
		"      /___//___/             /___/                      \n" +
		"      Antigravity Conversation Cleaner & Manager\n"
	fmt.Println(Colorize(Cyan+Bold, banner))
}

func PrintDivider() {
	fmt.Println(Colorize(Gray, strings.Repeat("─", 80)))
}
