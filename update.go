package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"strings"
	"time"

	"WaiLauncher/internal/launcher"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// GitHub repository that hosts WaiLauncher releases.
const (
	updateRepoOwner = "PerfLite"
	updateRepoName  = "WaiLauncher"
)

// UpdateInfo describes the result of an update check against GitHub Releases.
type UpdateInfo struct {
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion"`
	UpdateAvailable bool   `json:"updateAvailable"`
	ReleaseURL      string `json:"releaseUrl"`
	ReleaseNotes    string `json:"releaseNotes"`
	PublishedAt     string `json:"publishedAt"`
	Error           string `json:"error,omitempty"`
}

type ghReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

type ghRelease struct {
	TagName     string           `json:"tag_name"`
	Name        string           `json:"name"`
	Body        string           `json:"body"`
	HTMLURL     string           `json:"html_url"`
	PublishedAt string           `json:"published_at"`
	Assets      []ghReleaseAsset `json:"assets"`
}

func fetchLatestRelease(ctx context.Context) (*ghRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", updateRepoOwner, updateRepoName)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "WaiLauncher/"+launcherVersion)

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API HTTP %d", resp.StatusCode)
	}
	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

// parseSemver turns "1.2.3" / "v1.2.3" into a (major, minor, patch) triple.
func parseSemver(s string) (int, int, int, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	if i := strings.IndexAny(s, "-+ "); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return 0, 0, 0, false
	}
	nums := make([]int, 3)
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return 0, 0, 0, false
		}
		nums[i] = n
	}
	return nums[0], nums[1], nums[2], true
}

// isNewerVersion reports whether a is strictly newer than b (semver).
func isNewerVersion(a, b string) bool {
	aMaj, aMin, aPat, okA := parseSemver(a)
	bMaj, bMin, bPat, okB := parseSemver(b)
	if !okA || !okB {
		return a != b
	}
	if aMaj != bMaj {
		return aMaj > bMaj
	}
	if aMin != bMin {
		return aMin > bMin
	}
	return aPat > bPat
}

// CheckLauncherUpdate queries GitHub for the latest release and compares it
// with the running launcher version.
func (a *App) CheckLauncherUpdate() UpdateInfo {
	info := UpdateInfo{CurrentVersion: launcherVersion}
	rel, err := fetchLatestRelease(a.ctx)
	if err != nil {
		info.Error = err.Error()
		return info
	}
	latest := strings.TrimPrefix(rel.TagName, "v")
	info.LatestVersion = latest
	info.ReleaseURL = rel.HTMLURL
	info.ReleaseNotes = rel.Body
	info.PublishedAt = rel.PublishedAt
	info.UpdateAvailable = isNewerVersion(latest, launcherVersion)
	return info
}

// DownloadLauncherUpdate downloads the latest release binary in the background,
// replaces the running executable and restarts the launcher.
// Progress is reported via "update-progress" events; completion via
// "update-done" and failures via "update-error".
func (a *App) DownloadLauncherUpdate() error {
	a.mu.Lock()
	if a.updating {
		a.mu.Unlock()
		return fmt.Errorf("update already in progress")
	}
	a.updating = true
	a.mu.Unlock()

	go func() {
		defer func() {
			a.mu.Lock()
			a.updating = false
			a.mu.Unlock()
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		rel, err := fetchLatestRelease(ctx)
		if err != nil {
			runtime.EventsEmit(a.ctx, "update-error", err.Error())
			return
		}

		downloadURL := pickUpdateAsset(rel)
		if downloadURL == "" {
			runtime.EventsEmit(a.ctx, "update-error", "no matching asset in release")
			return
		}

		progress := func(pct float64, msg string) {
			runtime.EventsEmit(a.ctx, "update-progress", map[string]any{
				"percent": pct,
				"message": msg,
			})
		}
		progress(0, "Downloading update…")

		tmpPath := filepath.Join(os.TempDir(), "wailauncher-update.exe")
		if goruntime.GOOS != "windows" {
			tmpPath = filepath.Join(os.TempDir(), "wailauncher-update")
		}
		tmpPath, err = downloadToFile(ctx, downloadURL, tmpPath, progress)
		if err != nil {
			runtime.EventsEmit(a.ctx, "update-error", err.Error())
			return
		}
		defer os.Remove(tmpPath)

		exePath, err := os.Executable()
		if err != nil {
			runtime.EventsEmit(a.ctx, "update-error", "executable path: "+err.Error())
			return
		}

		if err := replaceExecutable(tmpPath, exePath); err != nil {
			runtime.EventsEmit(a.ctx, "update-error", err.Error())
			return
		}

		runtime.EventsEmit(a.ctx, "update-done", map[string]any{"path": exePath})
		launcher.LogInfo("Launcher updated to %s, restarting", strings.TrimPrefix(rel.TagName, "v"))

		go func() {
			time.Sleep(400 * time.Millisecond)
			cmd := exec.Command(exePath)
			cmd.Dir = filepath.Dir(exePath)
			if err := cmd.Start(); err != nil {
				launcher.LogError("Restart after update failed: %v", err)
				return
			}
			runtime.Quit(a.ctx)
		}()
	}()
	return nil
}

// pickUpdateAsset picks the executable asset that matches the current platform.
func pickUpdateAsset(rel *ghRelease) string {
	var fallback string
	for _, asset := range rel.Assets {
		name := strings.ToLower(asset.Name)
		if goruntime.GOOS == "windows" {
			if !strings.HasSuffix(name, ".exe") {
				continue
			}
			if strings.Contains(name, "wailauncher") {
				return asset.BrowserDownloadURL
			}
			if fallback == "" {
				fallback = asset.BrowserDownloadURL
			}
			continue
		}
		lower := strings.ToLower(asset.Name)
		if strings.HasSuffix(lower, ".deb") || strings.HasSuffix(lower, ".rpm") ||
			strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".zip") {
			continue
		}
		if strings.Contains(name, "wailauncher") {
			return asset.BrowserDownloadURL
		}
		if fallback == "" {
			fallback = asset.BrowserDownloadURL
		}
	}
	return fallback
}

// downloadToFile streams url to dst, reporting progress via cb.
func downloadToFile(ctx context.Context, url, dst string, cb func(pct float64, msg string)) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "WaiLauncher/"+launcherVersion)

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(dst)
	if err != nil {
		return "", err
	}

	total := resp.ContentLength
	written := int64(0)
	buf := make([]byte, 64*1024)
	lastPct := -1.0
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				out.Close()
				os.Remove(dst)
				return "", werr
			}
			written += int64(n)
			if total > 0 {
				pct := float64(written) * 100 / float64(total)
				if pct-lastPct >= 1 || pct >= 100 {
					lastPct = pct
					cb(pct, fmt.Sprintf("%d%%", int(pct)))
				}
			}
		}
		if readErr != nil {
			if readErr != io.EOF {
				out.Close()
				os.Remove(dst)
				return "", readErr
			}
			break
		}
	}
	if err := out.Close(); err != nil {
		os.Remove(dst)
		return "", err
	}
	return dst, nil
}

// replaceExecutable moves the downloaded file into exePath's slot.
// On Windows the running .exe cannot be overwritten, but it can be renamed
// away: the old binary is kept as <name>.old and cleaned up on next start.
func replaceExecutable(src, exePath string) error {
	dir := filepath.Dir(exePath)
	staged := filepath.Join(dir, ".wailauncher-update-"+filepath.Base(exePath))
	if err := copyFile(src, staged); err != nil {
		return err
	}
	if goruntime.GOOS == "windows" {
		old := exePath + ".old"
		os.Remove(old)
		if err := os.Rename(exePath, old); err != nil {
			os.Remove(staged)
			return fmt.Errorf("replace current exe: %w", err)
		}
		if err := os.Rename(staged, exePath); err != nil {
			_ = os.Rename(old, exePath)
			return err
		}
		return nil
	}
	if err := os.Rename(staged, exePath); err != nil {
		os.Remove(staged)
		return err
	}
	return os.Chmod(exePath, 0o755)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(dst)
		return err
	}
	return out.Close()
}

// cleanupOldExecutable removes the leftover .old binary from a previous update.
func cleanupOldExecutable() {
	exePath, err := os.Executable()
	if err != nil {
		return
	}
	old := exePath + ".old"
	info, err := os.Stat(old)
	if err != nil {
		return
	}
	if cur, curErr := os.Stat(exePath); curErr == nil && info.ModTime().Before(cur.ModTime()) {
		_ = os.Remove(old)
	}
}
