package ai

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	secretProject = "ops-dist-mgmt"
	secretName    = "dev-gemini-key"
)

// GetGeminiAPIKey fetches the Gemini API key with the following priority:
//  1. GEMINI_API_KEY env var — developer override, useful for testing
//  2. GCP Secret Manager via Application Default Credentials
func GetGeminiAPIKey() (string, error) {

	// 1. Env var override
	if key := os.Getenv("GEMINI_API_KEY"); key != "" {
		return key, nil
	}

	// 2. GCP Secret Manager
	token, err := getADCToken()
	if err != nil {
		return "", fmt.Errorf(
			"could not get GCP credentials: %v\n"+
				"  Run: gcloud auth application-default login",
			err,
		)
	}

	key, err := fetchSecret(token, secretProject, secretName)
	if err != nil {
		return "", fmt.Errorf(
			"could not fetch API key from Secret Manager: %v\n"+
				"  Make sure you have access to secret '%s' in project '%s'",
			err, secretName, secretProject,
		)
	}

	return key, nil
}

// getADCToken returns a short-lived access token from the user's
// existing gcloud Application Default Credentials.
func getADCToken() (string, error) {
	out, err := exec.Command(
		"gcloud", "auth", "application-default", "print-access-token",
	).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// fetchSecret calls the Secret Manager REST API.
// No extra SDK dependency — plain HTTP with the ADC bearer token.
func fetchSecret(token, project, secret string) (string, error) {
	url := fmt.Sprintf(
		"https://secretmanager.googleapis.com/v1/projects/%s/secrets/%s/versions/latest:access",
		project, secret,
	)

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("secret manager returned %d: %s", resp.StatusCode, string(body))
	}

	// Secret Manager returns: {"payload": {"data": "<base64-encoded-value>"}}
	// Go's json.Unmarshal automatically base64-decodes into []byte
	var result struct {
		Payload struct {
			Data []byte `json:"data"`
		} `json:"payload"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse secret response: %v", err)
	}

	if len(result.Payload.Data) == 0 {
		return "", fmt.Errorf("empty secret value returned from Secret Manager")
	}

	return strings.TrimSpace(string(result.Payload.Data)), nil
}
