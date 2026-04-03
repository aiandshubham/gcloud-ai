package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"gcloud-ai/ai"
	"gcloud-ai/executor"
	"gcloud-ai/internal/version"
	"gcloud-ai/security"
	"gcloud-ai/updater"
)

func main() {

	if len(os.Args) < 2 {
		fmt.Println("Usage: gcloud-ai \"your query\"")
		fmt.Println("       gcloud-ai --version           # show current version")
		fmt.Println("       gcloud-ai --clear-session     # forget previous context")
		fmt.Println("       gcloud-ai --history           # show last 20 commands")
		fmt.Println("       gcloud-ai --history 50        # show last N commands")
		return
	}

	// --version
	if os.Args[1] == "--version" {
		version.Print()
		return
	}

	// --clear-session
	if os.Args[1] == "--clear-session" {
		ai.ClearSession()
		fmt.Println("✅ Session cleared")
		return
	}

	// --history [n]
	if os.Args[1] == "--history" {
		n := 20
		if len(os.Args) >= 3 {
			fmt.Sscanf(os.Args[2], "%d", &n)
		}
		ai.PrintHistory(n)
		return
	}

	// 🔄 Check for updates once per day (silent if up to date, prompts if newer)
	updater.CheckAndUpdate(version.String())

	prompt := strings.Join(os.Args[1:], " ")

	fmt.Println("🤖 Generating command...")

	tool, cmd, err := ai.GenerateCommand(prompt)
	if err != nil {
		fmt.Println("❌ AI Error:", err)
		ai.WriteAudit(ai.AuditEntry{
			Prompt:  prompt,
			Tool:    "",
			Command: "",
			Status:  "failed",
			Error:   err.Error(),
		})
		return
	}

	fmt.Println("🔍 Tool:", tool)

	fmt.Println("\n📋 Execution Plan:")
	steps := strings.Split(cmd, "&&")
	for i, s := range steps {
		fmt.Printf("  Step %d: %s\n", i+1, strings.TrimSpace(s))
	}

	// 🔒 Validation
	if err := security.ValidateCommand(cmd, tool); err != nil {
		fmt.Println("❌ Security check failed:", err)
		ai.WriteAudit(ai.AuditEntry{
			Prompt:  prompt,
			Tool:    tool,
			Command: cmd,
			Status:  "policy_violation",
			Error:   err.Error(),
		})
		return
	}

	// 🛡️ Policy
	if err := security.EnforcePolicy(cmd); err != nil {
		fmt.Println("❌ Policy violation:", err)
		ai.WriteAudit(ai.AuditEntry{
			Prompt:  prompt,
			Tool:    tool,
			Command: cmd,
			Status:  "policy_violation",
			Error:   err.Error(),
		})
		return
	}

	// ✅ Confirmation
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\nExecute this command? (y/n): ")
	input, _ := reader.ReadString('\n')

	if strings.TrimSpace(input) != "y" {
		fmt.Println("❌ Execution cancelled")
		ai.WriteAudit(ai.AuditEntry{
			Prompt:  prompt,
			Tool:    tool,
			Command: cmd,
			Status:  "cancelled",
		})
		ai.SaveSession(prompt, cmd, "")
		return
	}

	// 🚀 Execute
	output, err := executor.Run(cmd)
	if err != nil {
		fmt.Println("❌ Execution failed:", err)
		ai.WriteAudit(ai.AuditEntry{
			Prompt:  prompt,
			Tool:    tool,
			Command: cmd,
			Status:  "failed",
			Error:   err.Error(),
		})
		ai.SaveSession(prompt, cmd, output)
		return
	}

	ai.SaveSession(prompt, cmd, output)
	ai.WriteAudit(ai.AuditEntry{
		Prompt:  prompt,
		Tool:    tool,
		Command: cmd,
		Status:  "executed",
	})
}
