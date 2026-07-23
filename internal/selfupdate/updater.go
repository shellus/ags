package selfupdate

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
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

const defaultRepository = "shellus/ags"

type Updater struct {
	Client     *http.Client
	Repository string
}

type Release struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func (u Updater) client() *http.Client {
	if u.Client != nil {
		return u.Client
	}
	return &http.Client{Timeout: 60 * time.Second}
}

func (u Updater) repository() string {
	if strings.TrimSpace(u.Repository) == "" {
		return defaultRepository
	}
	return u.Repository
}

func (u Updater) Latest(ctx context.Context) (Release, error) {
	url := "https://api.github.com/repos/" + u.repository() + "/releases/latest"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Release{}, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "ags-self-update")
	response, err := u.client().Do(request)
	if err != nil {
		return Release{}, fmt.Errorf("query latest AGS release: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("query latest AGS release: HTTP %s", response.Status)
	}
	var release Release
	if err := json.NewDecoder(response.Body).Decode(&release); err != nil {
		return Release{}, fmt.Errorf("parse latest AGS release: %w", err)
	}
	if release.TagName == "" {
		return Release{}, fmt.Errorf("latest AGS release has no tag")
	}
	return release, nil
}

func (u Updater) Update(ctx context.Context, executable string) (string, error) {
	release, err := u.Latest(ctx)
	if err != nil {
		return "", err
	}
	assetName, err := platformAssetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", err
	}
	binaryURL := assetURL(release, assetName)
	checksumURL := assetURL(release, "checksums.txt")
	if binaryURL == "" || checksumURL == "" {
		return "", fmt.Errorf("release %s lacks %s or checksums.txt", release.TagName, assetName)
	}
	binary, err := u.download(ctx, binaryURL)
	if err != nil {
		return "", err
	}
	checksums, err := u.download(ctx, checksumURL)
	if err != nil {
		return "", err
	}
	expected, err := findChecksum(checksums, assetName)
	if err != nil {
		return "", err
	}
	actual := sha256.Sum256(binary)
	if hex.EncodeToString(actual[:]) != expected {
		return "", fmt.Errorf("checksum mismatch for %s", assetName)
	}
	if err := replaceExecutable(executable, binary); err != nil {
		return "", err
	}
	return release.TagName, nil
}

func (u Updater) download(ctx context.Context, url string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "ags-self-update")
	response, err := u.client().Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: HTTP %s", url, response.Status)
	}
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func platformAssetName(goos, goarch string) (string, error) {
	if goarch != "amd64" {
		return "", fmt.Errorf("unsupported architecture %s", goarch)
	}
	switch goos {
	case "linux":
		return "ags-linux-amd64", nil
	case "windows":
		return "ags-windows-amd64.exe", nil
	default:
		return "", fmt.Errorf("unsupported operating system %s", goos)
	}
}

func assetURL(release Release, name string) string {
	for _, asset := range release.Assets {
		if asset.Name == name {
			return asset.BrowserDownloadURL
		}
	}
	return ""
}

func findChecksum(data []byte, name string) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && strings.TrimPrefix(fields[len(fields)-1], "*") == name {
			if len(fields[0]) != 64 {
				return "", fmt.Errorf("invalid checksum for %s", name)
			}
			return strings.ToLower(fields[0]), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("checksums.txt lacks %s", name)
}

func replaceExecutable(executable string, content []byte) error {
	executable, err := filepath.Abs(executable)
	if err != nil {
		return err
	}
	dir := filepath.Dir(executable)
	tmp, err := os.CreateTemp(dir, ".ags-update-*")
	if err != nil {
		return fmt.Errorf("create update file beside %s: %w", executable, err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o755); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		backup := executable + ".old"
		_ = os.Remove(backup)
		if err := os.Rename(executable, backup); err != nil {
			return fmt.Errorf("move current AGS executable: %w", err)
		}
		if err := os.Rename(tmpPath, executable); err != nil {
			_ = os.Rename(backup, executable)
			return fmt.Errorf("install updated AGS executable: %w", err)
		}
		return nil
	}
	if err := os.Rename(tmpPath, executable); err != nil {
		return fmt.Errorf("replace AGS executable: %w", err)
	}
	return nil
}
