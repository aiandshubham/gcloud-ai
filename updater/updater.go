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
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func getToken() string {
	return os.Getenv("GITHUB_API_TOKEN")
}

// CheckAndUpdate checks GitHub for a newer release once per day.
func CheckAndUpdate(currentVersion string) {
	if !shouldCheck() {
		return
	}

	token := getToken()
	if token == "" {
		// Silent skip — don't block the user just because token isn't set
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
	url := fmt.Sprintf("https://%s@api.github.com/repos/%s/%s/releases/latest",
		token, repoOwner, repoName)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var release GithubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
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
	checksumName := "checksums.txt"

	var assetURL, checksumURL string
	for _, a := range release.Assets {
		if a.Name == assetName {
			assetURL = a.BrowserDownloadURL
		}
		if a.Name == checksumName {
			checksumURL = a.BrowserDownloadURL
		}
	}

	if assetURL == "" {
		return fmt.Errorf("no binary found for %s/%s in release %s",
			runtime.GOOS, runtime.GOARCH, release.TagName)
	}

	// Inject token into download URLs
	assetURL = injectToken(token, assetURL)
	checksumURL = injectToken(token, checksumURL)

	expectedChecksum, err := fetchExpectedChecksum(checksumURL, assetName)
	if err != nil {
		return fmt.Errorf("could not fetch checksum: %v", err)
	}

	fmt.Printf("   Downloading %s...\n", assetName)
	tmpFile, err := os.CreateTemp("", "gcloud-ai-update-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())

	if err := downloadFile(assetURL, tmpFile); err != nil {
		return err
	}
	tmpFile.Close()

	if err := verifyChecksum(tmpFile.Name(), expectedChecksum); err != nil {
		return fmt.Errorf("checksum mismatch — aborting update: %v", err)
	}

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

// injectToken converts https://github.com/... to https://TOKEN@github.com/...
func injectToken(token, rawURL string) string {
	return strings.Replace(rawURL, "https://", fmt.Sprintf("https://%s@", token), 1)
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
