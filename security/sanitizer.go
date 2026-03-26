package security

import (
	"fmt"
	"strings"
)

var allowedTools = []string{
	"gcloud",
	"kubectl",
	"bq",
	"gsutil",
}

func ValidateCommand(cmd string, tool string) error {

	commands := strings.Split(cmd, "&&")

	for _, c := range commands {
		c = strings.TrimSpace(c)

		if !strings.HasPrefix(c, tool+" ") &&
			!strings.HasPrefix(c, "gcloud ") &&
			!strings.HasPrefix(c, "kubectl ") &&
			!strings.HasPrefix(c, "bq ") &&
			!strings.HasPrefix(c, "gsutil ") {
			return fmt.Errorf("invalid command: %s", c)
		}

		// check dangerous patterns
		dangerous := []string{
			";", "|", ">", "<", "`", "$(", "||",
		}

		for _, d := range dangerous {
			if strings.Contains(c, d) {
				return fmt.Errorf("dangerous pattern detected: %s", d)
			}
		}
	}

	return nil
}

func isAllowedTool(tool string) bool {
	for _, t := range allowedTools {
		if tool == t {
			return true
		}
	}
	return false
}
