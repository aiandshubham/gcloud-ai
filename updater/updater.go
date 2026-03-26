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
	repoOwner      = "Exabeam"
	repoName       = "gcloud-ai"
	checkIntervalH = 24
	lastCheckFile  = "/.gai/last_update_check"
)

type GithubRelease struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

type Asset struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func getToken() string {
	return os.Getenv("GITHUB_API_TOKEN")
}

// githubGet makes an authenticated GET request to the GitHub API
func githubGet(token, url string, v interface{}) error {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "token "+token)
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

// githubDownloadAsset downloads a release asset by ID using the API endpoint
// with Accept: application/octet-stream — required for private repo assets
func githubDownloadAsset(token string, assetID int64, dest *os.File) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/assets/%d",
		repoOwner, repoName, assetID)

	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/octet-stream")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("asset download returned %d: %s", resp.StatusCode, string(body))
	}

	_, err = io.Copy(dest, resp.Body)
	return err
}

// CheckAndUpdate checks GitHub for a newer release once per day.
func CheckAndUpdate(currentVersion string) {
	if !shouldCheck() {
		return
	}

	token := getToken()
	if token == "" {
		// Silent skip — don't block user if token not set
		return
	}

	saveLastCheck()

	release, err := fetchLatestRelease(token)
	if err != nil {
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

	if err := doUpdate(token, release); err != nil {
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

func fetchLatestRelease(token string) (*GithubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest",
		repoOwner, repoName)
	var release GithubRelease
	if err := githubGet(token, url, &release); err != nil {
		return nil, err
	}
	return &release, nil
}

func isNewer(latest, current string) bool {
	latest = strings.TrimPrefix(latest, "v")
	current = strings.TrimPrefix(current, "v")
	return latest != current && latest > current
}

func doUpdate(token string, release *GithubRelease) error {
	assetName := buildAssetName(release.TagName)

	// Find asset IDs for the binary and checksum file
	var binaryAssetID, checksumAssetID int64
	for _, a := range release.Assets {
		if a.Name == assetName {
			binaryAssetID = a.ID
		}
		if a.Name == "checksums.txt" {
			checksumAssetID = a.ID
		}
	}

	if binaryAssetID == 0 {
		return fmt.Errorf("no binary found for %s/%s in release %s",
			runtime.GOOS, runtime.GOARCH, release.TagName)
	}
	if checksumAssetID == 0 {
		return fmt.Errorf("checksums.txt not found in release %s", release.TagName)
	}

	// Download checksum file
	checksumTmp, err := os.CreateTemp("", "gcloud-ai-checksums-*")
	if err != nil {
		return err
	}
	defer os.Remove(checksumTmp.Name())

	if err := githubDownloadAsset(token, checksumAssetID, checksumTmp); err != nil {
		return fmt.Errorf("could not download checksums: %v", err)
	}
	checksumTmp.Close()

	expectedChecksum, err := parseChecksum(checksumTmp.Name(), assetName)
	if err != nil {
		return fmt.Errorf("could not parse checksum: %v", err)
	}

	// Download binary
	fmt.Printf("   Downloading %s...\n", assetName)
	binaryTmp, err := os.CreateTemp("", "gcloud-ai-update-*")
	if err != nil {
		return err
	}
	defer os.Remove(binaryTmp.Name())

	if err := githubDownloadAsset(token, binaryAssetID, binaryTmp); err != nil {
		return fmt.Errorf("could not download binary: %v", err)
	}
	binaryTmp.Close()

	// Verify checksum
	if err := verifyChecksum(binaryTmp.Name(), expectedChecksum); err != nil {
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

	if err := os.Chmod(binaryTmp.Name(), 0755); err != nil {
		return err
	}

	return os.Rename(binaryTmp.Name(), execPath)
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

func parseChecksum(checksumFile, assetName string) (string, error) {
	data, err := os.ReadFile(checksumFile)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[1] == assetName {
			return parts[0], nil
		}
	}
	return "", fmt.Errorf("checksum for %s not found", assetName)
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
