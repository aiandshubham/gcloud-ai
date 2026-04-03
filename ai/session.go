package ai

import (
	"encoding/json"
	"os"
)

// Session stores the last command and its output for follow-up context
type Session struct {
	LastPrompt  string `json:"last_prompt"`
	LastCommand string `json:"last_command"`
	LastOutput  string `json:"last_output"`
}

var sessionFile = os.Getenv("HOME") + "/.gai/session.json"

func loadSession() *Session {
	data, err := os.ReadFile(sessionFile)
	if err != nil {
		return nil
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil
	}
	return &s
}

func SaveSession(prompt, command, output string) {
	os.MkdirAll(os.Getenv("HOME")+"/.gai", 0755)
	s := Session{
		LastPrompt:  prompt,
		LastCommand: command,
		LastOutput:  output,
	}
	data, _ := json.Marshal(s)
	os.WriteFile(sessionFile, data, 0644)
}

func ClearSession() {
	os.Remove(sessionFile)
}
