package executor

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

func splitCommands(cmd string) []string {
	parts := strings.Split(cmd, "&&")
	var cleaned []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			cleaned = append(cleaned, p)
		}
	}
	return cleaned
}

// Run executes the command, prints output to stdout, and returns the combined output string.
func Run(cmd string) (string, error) {
	commands := splitCommands(cmd)
	var fullOutput strings.Builder

	for i, c := range commands {
		fmt.Printf("\n🚀 Step %d: Executing: %s\n\n", i+1, c)

		command := exec.Command("bash", "-c", c)

		// Tee output — write to both terminal and our buffer
		var buf bytes.Buffer
		command.Stdout = io.MultiWriter(os.Stdout, &buf)
		command.Stderr = io.MultiWriter(os.Stderr, &buf)

		err := command.Run()
		fullOutput.WriteString(buf.String())

		if err != nil {
			return fullOutput.String(), fmt.Errorf("step %d failed: %v", i+1, err)
		}
	}

	return fullOutput.String(), nil
}
