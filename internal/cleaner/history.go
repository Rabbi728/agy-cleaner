package cleaner

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type HistoryEntry struct {
	ConversationID string `json:"conversationId"`
}

// CleanHistory removes lines from history.jsonl matching any of the given conversation IDs.
// Returns the number of removed lines.
func CleanHistory(historyPath string, deleteIDs map[string]bool) (int, error) {
	if _, err := os.Stat(historyPath); os.IsNotExist(err) {
		return 0, nil
	}

	inFile, err := os.Open(historyPath)
	if err != nil {
		return 0, fmt.Errorf("failed to open history file: %w", err)
	}
	defer inFile.Close()

	tempPath := filepath.Join(filepath.Dir(historyPath), "history.jsonl.tmp")
	outFile, err := os.OpenFile(tempPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return 0, fmt.Errorf("failed to create temporary history file: %w", err)
	}

	reader := bufio.NewReader(inFile)
	writer := bufio.NewWriter(outFile)

	removedCount := 0
	keptCount := 0

	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			var entry HistoryEntry
			_ = json.Unmarshal(line, &entry)

			if entry.ConversationID != "" && deleteIDs[entry.ConversationID] {
				removedCount++
			} else {
				_, _ = writer.Write(line)
				keptCount++
			}
		}

		if err != nil {
			if err == io.EOF {
				break
			}
			_ = outFile.Close()
			_ = os.Remove(tempPath)
			return 0, fmt.Errorf("error reading history file: %w", err)
		}
	}

	if err := writer.Flush(); err != nil {
		_ = outFile.Close()
		_ = os.Remove(tempPath)
		return 0, fmt.Errorf("error flushing history file: %w", err)
	}
	if err := outFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return 0, fmt.Errorf("error closing history temp file: %w", err)
	}

	// Atomically replace history.jsonl
	if err := os.Rename(tempPath, historyPath); err != nil {
		return 0, fmt.Errorf("failed to replace history file: %w", err)
	}

	return removedCount, nil
}
