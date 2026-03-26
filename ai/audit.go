package ai

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type AuditEntry struct {
	Timestamp string `json:"timestamp"`
	Prompt    string `json:"prompt"`
	Tool      string `json:"tool"`
	Command   string `json:"command"`
	Status    string `json:"status"` // executed | cancelled | failed | policy_violation
	Error     string `json:"error,omitempty"`
}

var auditLog = os.Getenv("HOME") + "/.gai/history.log"

func WriteAudit(entry AuditEntry) {
	os.MkdirAll(os.Getenv("HOME")+"/.gai", 0755)

	entry.Timestamp = time.Now().Format(time.RFC3339)

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	f, err := os.OpenFile(auditLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	f.Write(append(data, '\n'))
}

func PrintHistory(n int) {
	data, err := os.ReadFile(auditLog)
	if err != nil {
		fmt.Println("No history found.")
		return
	}

	lines := splitLines(string(data))

	// Take last n entries
	start := 0
	if len(lines) > n {
		start = len(lines) - n
	}

	for _, line := range lines[start:] {
		if line == "" {
			continue
		}
		var entry AuditEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		statusIcon := "✅"
		switch entry.Status {
		case "cancelled":
			statusIcon = "⏭️"
		case "failed":
			statusIcon = "❌"
		case "policy_violation":
			statusIcon = "🚫"
		}

		fmt.Printf("%s [%s] %s\n", statusIcon, entry.Timestamp, entry.Prompt)
		fmt.Printf("   └─ %s\n", entry.Command)
		if entry.Error != "" {
			fmt.Printf("   └─ error: %s\n", entry.Error)
		}
		fmt.Println()
	}
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
