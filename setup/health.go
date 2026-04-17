package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const pluginCheckInterval = 24 * time.Hour
const healthFile = "/.gai/health_check.json"

type HealthState struct {
	LastPluginCheck    time.Time `json:"last_plugin_check"`
	GKEPluginInstalled bool      `json:"gke_plugin_installed"`
}

// EnsureSetup runs on every gcloud-ai invocation.
// Auth checks run every time — they're fast and credentials can expire at midnight.
// Plugin check runs once per day — it never expires once installed.
func EnsureSetup() {
	anyRefreshed := false

	// ── Auth checks — always run, credentials expire at midnight ──────────

	if !isGcloudAuthenticated() {
		fmt.Println("🔧 gcloud credentials expired, refreshing...")
		cmd := exec.Command("gcloud", "auth", "login")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Println("❌ gcloud auth login failed:", err)
			os.Exit(1)
		}
		anyRefreshed = true
	}

	if !isADCAuthenticated() {
		fmt.Println("🔧 ADC credentials expired, refreshing...")
		cmd := exec.Command("gcloud", "auth", "application-default", "login")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Println("❌ gcloud auth application-default login failed:", err)
			os.Exit(1)
		}
		anyRefreshed = true
	}

	// ── Plugin check — once per day, installation never expires ───────────

	if shouldCheckPlugin() {
		if !isGKEPluginInstalled() {
			fmt.Println("🔧 Installing gke-gcloud-auth-plugin...")
			cmd := exec.Command("gcloud", "components", "install",
				"gke-gcloud-auth-plugin", "-q")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				fmt.Println("❌ Failed to install gke-gcloud-auth-plugin:", err)
				os.Exit(1)
			}
			anyRefreshed = true
		}
		saveHealthCheck()
	}

	// ── Always set env var for current session ────────────────────────────
	os.Setenv("USE_GKE_GCLOUD_AUTH_PLUGIN", "True")

	if anyRefreshed {
		fmt.Println("✅ Setup refreshed\n")
	}
}

// shouldCheckPlugin returns true if 24h have passed since last plugin check
func shouldCheckPlugin() bool {
	path := os.Getenv("HOME") + healthFile
	data, err := os.ReadFile(path)
	if err != nil {
		return true
	}
	var state HealthState
	if err := json.Unmarshal(data, &state); err != nil {
		return true
	}
	return time.Since(state.LastPluginCheck) > pluginCheckInterval
}

func saveHealthCheck() {
	path := os.Getenv("HOME") + healthFile
	os.MkdirAll(os.Getenv("HOME")+"/.gai", 0755)
	state := HealthState{
		LastPluginCheck:    time.Now(),
		GKEPluginInstalled: isGKEPluginInstalled(),
	}
	data, _ := json.Marshal(state)
	os.WriteFile(path, data, 0644)
}

func isGcloudAuthenticated() bool {
	out, err := exec.Command(
		"gcloud", "auth", "print-access-token",
	).Output()
	return err == nil && len(strings.TrimSpace(string(out))) > 0
}

func isADCAuthenticated() bool {
	out, err := exec.Command(
		"gcloud", "auth", "application-default", "print-access-token",
	).Output()
	return err == nil && len(strings.TrimSpace(string(out))) > 0
}

func isGKEPluginInstalled() bool {
	_, err := exec.LookPath("gke-gcloud-auth-plugin")
	return err == nil
}

