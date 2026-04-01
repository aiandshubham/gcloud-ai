package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"gcloud-ai/config"
)

type AIResponse struct {
	Tool    string `json:"tool"`
	Command string `json:"command"`
}

type GeminiRequest struct {
	Contents []struct {
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	} `json:"contents"`
}

type GeminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

func existingKubeContexts() []string {
	out, err := exec.Command("kubectl", "config", "get-contexts", "--no-headers", "-o", "name").Output()
	if err != nil {
		return nil
	}
	var contexts []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			contexts = append(contexts, line)
		}
	}
	return contexts
}

func GenerateCommand(prompt string) (string, string, error) {

	apiKey, err := GetGeminiAPIKey()
	if err != nil {
		return "", "", err
	}

	// Get model from config (defaults to gemini-2.5-pro)
	model := config.Load().GeminiModel

	// Build kubeconfig context rule
	existingContexts := existingKubeContexts()
	var k8sContextRule string
	if len(existingContexts) == 0 {
		k8sContextRule = `KUBERNETES / GKE RULES:
- No kubeconfig contexts exist yet on this machine
- If the user mentions a cluster, ALWAYS start with:
  gcloud container clusters get-credentials <cluster> --region=<region> --project=<project>
- Then chain the actual kubectl command with &&
- Example: gcloud container clusters get-credentials prod-gnjf --region=asia-northeast1 --project=my-project && kubectl get pods --all-namespaces`
	} else {
		k8sContextRule = fmt.Sprintf(`KUBERNETES / GKE RULES:
- The following kubeconfig contexts already exist on this machine:
%s

- GKE context names follow the pattern: gke_<project>_<region>_<cluster>
- If the cluster the user is asking about already has a context in the list above →
  do NOT run gcloud container clusters get-credentials, go directly to the kubectl command
- If the cluster is NOT in the list above →
  start with: gcloud container clusters get-credentials <cluster> --region=<region> --project=<project>
  then chain the kubectl command with &&
- NEVER add --context flag to kubectl; the correct context is already active after get-credentials`,
			"  - "+strings.Join(existingContexts, "\n  - "))
	}

	// Build default context from config
	cfg := config.Load()
	defaultContext := ""
	if cfg.DefaultProject != "" || cfg.DefaultRegion != "" {
		defaultContext = "USER DEFAULTS (use these when not specified in the prompt):\n"
		if cfg.DefaultProject != "" {
			defaultContext += fmt.Sprintf("  - project: %s\n", cfg.DefaultProject)
		}
		if cfg.DefaultRegion != "" {
			defaultContext += fmt.Sprintf("  - region: %s\n", cfg.DefaultRegion)
		}
		if cfg.DefaultCluster != "" {
			defaultContext += fmt.Sprintf("  - cluster: %s\n", cfg.DefaultCluster)
		}
	}

	// Build session context block
	sessionBlock := ""
	if session := loadSession(); session != nil {
		output := session.LastOutput
		if len(output) > 3000 {
			output = "...(truncated)...\n" + output[len(output)-3000:]
		}
		sessionBlock = fmt.Sprintf(`PREVIOUS CONTEXT:
- The user previously ran: %s
- Which executed: %s
- And produced this output:
%s

If the current instruction is a follow-up referring to the above output (e.g. "from those", "from above", "the listed"),
use the data above to construct the correct command.`, session.LastPrompt, session.LastCommand, output)
	}

	fullPrompt := fmt.Sprintf(`
Convert the user instruction into one or more CLI commands.

Return JSON ONLY in this format:
{
  "tool": "<gcloud|kubectl|bq|gsutil>",
  "command": "<command or commands joined with &&>"
}

CHAINING RULES:
- Join dependent steps with && only when step 2 requires step 1 to complete first
- Do NOT use pipes ( | ), redirects ( > < ) or subshells ( $() )

%s

%s

%s

GENERAL:
- No explanations, no markdown, return raw JSON only
- Command must be valid CLI syntax
- Use the primary/final tool for the "tool" field
- For GCP services use the appropriate tool (gcloud, bq, gsutil)
- Do NOT ignore user-provided project, cluster, region, or namespace

Instruction:
%s
`, k8sContextRule, defaultContext, sessionBlock, prompt)

	reqBody := GeminiRequest{
		Contents: []struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		}{
			{
				Parts: []struct {
					Text string `json:"text"`
				}{
					{Text: fullPrompt},
				},
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal request: %v", err)
	}

	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		model, apiKey,
	)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	if os.Getenv("GAI_DEBUG") != "" {
		fmt.Println("🔍 RAW RESPONSE:\n", string(bodyBytes))
	}

	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("Gemini API error: %s", string(bodyBytes))
	}

	var geminiResp GeminiResponse
	if err := json.Unmarshal(bodyBytes, &geminiResp); err != nil {
		return "", "", err
	}

	if len(geminiResp.Candidates) == 0 {
		return "", "", fmt.Errorf("no candidates in response: %s", string(bodyBytes))
	}

	rawText := geminiResp.Candidates[0].Content.Parts[0].Text
	rawText = strings.TrimSpace(rawText)
	rawText = strings.TrimPrefix(rawText, "```json")
	rawText = strings.TrimSuffix(rawText, "```")
	rawText = strings.TrimSpace(rawText)

	if os.Getenv("GAI_DEBUG") != "" {
		fmt.Println("🔍 CLEANED AI OUTPUT:\n", rawText)
	}

	var aiResp AIResponse
	if err = json.Unmarshal([]byte(rawText), &aiResp); err != nil {
		return "", "", fmt.Errorf("failed to parse AI JSON: %s", rawText)
	}

	return aiResp.Tool, aiResp.Command, nil
}
