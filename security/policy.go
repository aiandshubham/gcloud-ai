package security

import (
	"fmt"
	"os"
	"strings"
	"regexp"
	"gopkg.in/yaml.v3"
)

type Policy struct {
	BlockedKeywords    []string `yaml:"blocked_keywords"`
	RestrictedPatterns []string `yaml:"restricted_patterns"`
}

func LoadPolicy() (*Policy, error) {
	data, err := os.ReadFile("config/policy.yml")
	if err != nil {
		return nil, err
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
