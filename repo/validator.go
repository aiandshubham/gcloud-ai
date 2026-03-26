package repo

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	AllowedProjects []string `yaml:"allowed_projects"`
}

func ValidateProject() error {

	currentProject, err := getCurrentProject()
	if err != nil {
		return err
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	for _, p := range cfg.AllowedProjects {
		if p == currentProject {
			return nil
		}
	}

	return fmt.Errorf("project %s is not allowed", currentProject)
}

func getCurrentProject() (string, error) {
	out, err := exec.Command("gcloud", "config", "get-value", "project").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func loadConfig() (*Config, error) {
	data, err := os.ReadFile("config/repo-identification.yml")
	if err != nil {
		return nil, err
	}

	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	return &cfg, err
}
