package ai

import (
	"fmt"

	"gcloud-ai/config"
)

// GetGeminiAPIKey returns the Gemini API key using this priority:
//  1. GEMINI_API_KEY env var
//  2. gemini_api_key field in ~/.gai/config.yml
func GetGeminiAPIKey() (string, error) {
	key := config.GetGeminiAPIKey()
	if key == "" {
		return "", fmt.Errorf(`Gemini API key not found.

  Option 1 — Environment variable (recommended):
    export GEMINI_API_KEY=your_key

  Option 2 — Config file (~/.gai/config.yml):
    gemini_api_key: your_key

  Get a free API key at: https://aistudio.google.com`)
	}
	return key, nil
}
