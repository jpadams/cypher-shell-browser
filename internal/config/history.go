package config

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	historyFile      = ".cypher-shell-browser_history"
	maxHistorySize   = 500
	entrySeparator   = "\x00---ENTRY---\x00"
)

func HistoryPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return historyFile
	}
	return filepath.Join(home, historyFile)
}

func LoadHistory() []string {
	data, err := os.ReadFile(HistoryPath())
	if err != nil {
		return nil
	}
	raw := strings.TrimRight(string(data), "\n")
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, entrySeparator)
	var result []string
	for _, part := range parts {
		entry := strings.TrimSpace(part)
		if entry != "" {
			result = append(result, entry)
		}
	}
	return result
}

func SaveHistory(entries []string) {
	if len(entries) > maxHistorySize {
		entries = entries[len(entries)-maxHistorySize:]
	}
	data := strings.Join(entries, entrySeparator) + "\n"
	os.WriteFile(HistoryPath(), []byte(data), 0600)
}
