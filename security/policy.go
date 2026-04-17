package security

import (
	"embed"
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed policy.yml
var defaultPolicy embed.FS

type Policy struct {
	BlockedKeywords    []string `yaml:"blocked_keywords"`
	RestrictedPatterns []string `yaml:"restricted_patterns"`
}

func LoadPolicy() (*Policy, error) {
	// Always use the embedded policy — no user override allowed
	// This prevents users from bypassing security restrictions
	data, err := defaultPolicy.ReadFile("policy.yml")
	if err != nil {
		return nil, fmt.Errorf("could not load embedded policy: %v", err)
	}

	var p Policy
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, err
	}

	return &p, nil
}

func EnforcePolicy(cmd string) error {
	policy, err := LoadPolicy()
	if err != nil {
		return err
	}

	lowerCmd := strings.ToLower(cmd)

	// 🚫 Block keywords
	for _, keyword := range policy.BlockedKeywords {
		pattern := fmt.Sprintf(`\b%s\b`, keyword)
		matched, _ := regexp.MatchString(pattern, lowerCmd)
		if matched {
			return fmt.Errorf("command blocked due to keyword: %s", keyword)
		}
	}

	// ⚠️ Block risky flags
	for _, pattern := range policy.RestrictedPatterns {
		if strings.Contains(cmd, pattern) {
			return fmt.Errorf("restricted pattern detected: %s", pattern)
		}
	}

	return nil
}
