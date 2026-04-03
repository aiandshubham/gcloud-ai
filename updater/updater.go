package updater

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	repoOwner      = "aiandshubham"
	repoName       = "gcloud-ai"
	checkIntervalH = 24
	lastCheckFile  = "/.gai/last_update_check"
)

type GithubRelease struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

type Asset struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// publicGet makes an unauthenticated GET to GitHub API — works for public repos
func publicGet(url string, v interface{}) error {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("github API returned %d: %s", resp.StatusCode, string(body))
	}

	return json.NewDecoder(resp.Body).Decode(v)
}

// CheckAndUpdate checks GitHub for a newer release once per day.
func CheckAndUpdate(currentVersion string) {
	if !shouldCheck() {
		return
	}

	saveLastCheck()

	release, err := fetchLatestRelease()
	if err != nil {
		// Silent fail — don't block the user for an update check
		return
	}

	if !isNewer(release.TagName, currentVersion) {
		return
	}

	fmt.Printf("\n🆕 New version available: %s (you have %s)\n", release.TagName, currentVersion)
	fmt.Print("   Update now? (y/n): ")

	var input string
	fmt.Scanln(&input)
	if strings.TrimSpace(strings.ToLower(input)) != "y" {
		fmt.Println("   Skipping update.")
		return
	}

	if err := doUpdate(release); err != nil {
		fmt.Println("❌ Update failed:", err)
		return
	}

	fmt.Println("✅ Updated successfully. Please re-run your command.")
	os.Exit(0)
}

func shouldCheck() bool {
	path := os.Getenv("HOME") + lastCheckFile
	data, err := os.ReadFile(path)
	if err != nil {
		return true
	}
	var lastCheck time.Time
	if err := json.Unmarshal(data, &lastCheck); err != nil {
		return true
	}
	return time.Since(lastCheck) > checkIntervalH*time.Hour
}

func saveLastCheck() {
	path := os.Getenv("HOME") + lastCheckFile
	os.MkdirAll(filepath.Dir(path), 0755)
	data, _ := json.Marshal(time.Now())
	os.WriteFile(path, data, 0644)
}

func fetchLatestRelease() (*GithubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest",
		repoOwner, repoName)
	var release GithubRelease
	if err := publicGet(url, &release); err != nil {
		return nil, err
	}
	return &release, nil
}

func isNewer(latest, current string) bool {
	latest = strings.TrimPrefix(latest, "v")
	current = strings.TrimPrefix(current, "v")
	return latest != current && latest > current
}

func doUpdate(release *GithubRelease) error {
	assetName := buildAssetName(release.TagName)

	// Find binary and checksum assets
	var binaryURL, checksumURL string
	for _, a := range release.Assets {
		if a.Name == assetName {
			binaryURL = a.BrowserDownloadURL
		}
		if a.Name == "checksums.txt" {
			checksumURL = a.BrowserDownloadURL
		}
	}

	if binaryURL == "" {
		return fmt.Errorf("no binary found for %s/%s in release %s",
			runtime.GOOS, runtime.GOARCH, release.TagName)
	}
	if checksumURL == "" {
		return fmt.Errorf("checksums.txt not found in release %s", release.TagName)
	}

	// Download checksum file
	expectedChecksum, err := fetchExpectedChecksum(checksumURL, assetName)
	if err != nil {
		return fmt.Errorf("could not fetch checksum: %v", err)
	}

	// Download binary
	fmt.Printf("   Downloading %s...\n", assetName)
	tmpFile, err := os.CreateTemp("", "gcloud-ai-update-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())

	if err := downloadFile(binaryURL, tmpFile); err != nil {
		return fmt.Errorf("could not download binary: %v", err)
	}
	tmpFile.Close()

	// Verify checksum
	if err := verifyChecksum(tmpFile.Name(), expectedChecksum); err != nil {
		return fmt.Errorf("checksum mismatch — aborting update: %v", err)
	}

	// Replace current binary atomically
	execPath, err := os.Executable()
	if err != nil {
		return err
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return err
	}

	if err := os.Chmod(tmpFile.Name(), 0755); err != nil {
		return err
	}

	return os.Rename(tmpFile.Name(), execPath)
}

func buildAssetName(version string) string {
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("gcloud-ai_%s_%s_%s%s",
		strings.TrimPrefix(version, "v"), goos, goarch, ext)
}

func fetchExpectedChecksum(checksumURL, assetName string) (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(checksumURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	for _, line := range strings.Split(string(body), "\n") {
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[1] == assetName {
			return parts[0], nil
		}
	}

	return "", fmt.Errorf("checksum for %s not found", assetName)
}

func downloadFile(url string, dest *os.File) error {
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.Copy(dest, resp.Body)
	return err
}

func verifyChecksum(filePath, expected string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}

	actual := fmt.Sprintf("%x", h.Sum(nil))
	if actual != expected {
		return fmt.Errorf("expected %s, got %s", expected, actual)
	}

	return nil
}
