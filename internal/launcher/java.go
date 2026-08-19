package launcher

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Java runtime management: every Minecraft version declares the Java major
// it needs (javaVersion.majorVersion). Running an old version on a too-new
// JVM breaks LWJGL (native JNI crashes), so we prefer an exact major match
// among installed JDKs and auto-download a Temurin build when none exists —
// the same behavior Modrinth/Prism launchers provide.

// JavaInfo is one discovered or installed JDK.
type JavaInfo struct {
	Path  string // path to the javaw/java executable
	Major int    // major version, e.g. 17
}

var releaseVerRe = regexp.MustCompile(`JAVA_VERSION="([^"]+)"`)

// javaHomeMajor reads the JDK's "release" file to get its major version.
func javaHomeMajor(home string) int {
	data, err := os.ReadFile(filepath.Join(home, "release"))
	if err != nil {
		return 0
	}
	m := releaseVerRe.FindSubmatch(data)
	if m == nil {
		return 0
	}
	return parseJavaMajor(string(m[1]))
}

// parseJavaMajor turns "17.0.2", "25" or "1.8.0_362" into 17, 25, 8.
func parseJavaMajor(v string) int {
	parts := strings.SplitN(v, ".", 3)
	n, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0
	}
	if n == 1 && len(parts) > 1 { // old 1.x scheme
		if n2, err := strconv.Atoi(parts[1]); err == nil {
			return n2
		}
	}
	return n
}

func javaExeName() string {
	if runtime.GOOS == "windows" {
		return "javaw.exe"
	}
	return "java"
}

// javaHomeCandidates lists JDK home directories from well-known locations.
func javaHomeCandidates() []string {
	var homes []string
	if home := os.Getenv("JAVA_HOME"); home != "" {
		homes = append(homes, home)
	}
	// derive home from java(w) binaries on PATH
	for _, name := range []string{"javaw.exe", "java.exe", "java"} {
		if p, err := execLookPath(name); err == nil {
			homes = append(homes, filepath.Dir(filepath.Dir(p)))
		}
	}
	if runtime.GOOS == "windows" {
		vendors := []string{"Java", "Eclipse Adoptium", "BellSoft", "Microsoft", "Zulu", "Semeru"}
		for _, v := range vendors {
			matches, _ := filepath.Glob(filepath.Join(`C:\Program Files`, v, "*"))
			homes = append(homes, matches...)
			matches, _ = filepath.Glob(filepath.Join(`C:\Program Files (x86)`, v, "*"))
			homes = append(homes, matches...)
		}
		if up := os.Getenv("USERPROFILE"); up != "" {
			matches, _ := filepath.Glob(filepath.Join(up, ".jdks", "*"))
			homes = append(homes, matches...)
			matches, _ = filepath.Glob(filepath.Join(up, `AppData\Local\Programs\Eclipse Adoptium`, "*"))
			homes = append(homes, matches...)
		}
	} else {
		matches, _ := filepath.Glob("/usr/lib/jvm/*")
		homes = append(homes, matches...)
		matches, _ = filepath.Glob("/Library/Java/JavaVirtualMachines/*/Contents/Home")
		homes = append(homes, matches...)
	}
	return homes
}

// execLookPath is exec.LookPath split out for clarity.
func execLookPath(name string) (string, error) {
	return exec.LookPath(name)
}

// enumerateJavas returns all JDKs found on the system, newest first.
func enumerateJavas() []JavaInfo {
	seen := make(map[string]bool)
	var out []JavaInfo
	for _, home := range javaHomeCandidates() {
		exe := filepath.Join(home, "bin", javaExeName())
		if runtime.GOOS != "windows" {
			exe = filepath.Join(home, "bin", "java")
		}
		if st, err := os.Stat(exe); err != nil || st.IsDir() {
			continue
		}
		abs, _ := filepath.Abs(exe)
		if seen[strings.ToLower(abs)] {
			continue
		}
		seen[strings.ToLower(abs)] = true
		out = append(out, JavaInfo{Path: exe, Major: javaHomeMajor(home)})
	}
	return out
}

// managedJavaDir is where auto-downloaded JDKs live: <root>/java/temurin-N.
func (l *Launcher) managedJavaDir(major int) string {
	return filepath.Join(l.Root, "java", fmt.Sprintf("temurin-%d", major))
}

// findLocalJava looks for an installed JDK with exactly the given major,
// checking launcher-managed installs first.
func (l *Launcher) findLocalJava(major int) string {
	if major <= 0 {
		return ""
	}
	dir := l.managedJavaDir(major)
	if exe := findJavaInHome(dir); exe != "" {
		return exe
	}
	// the zip root folder carries the full version (jdk-17.0.13+11)
	if matches, _ := filepath.Glob(filepath.Join(dir, "*", "bin", javaExeName())); len(matches) > 0 {
		return matches[0]
	}
	if runtime.GOOS != "windows" {
		if matches, _ := filepath.Glob(filepath.Join(dir, "*", "bin", "java")); len(matches) > 0 {
			return matches[0]
		}
	}
	for _, j := range enumerateJavas() {
		if j.Major == major {
			return j.Path
		}
	}
	return ""
}

// FindJavaForMajor returns path to an installed JDK for the major version.
func (l *Launcher) FindJavaForMajor(major int) string {
	return l.findLocalJava(major)
}

// DownloadTemurinDirect downloads and extracts Temurin OpenJDK for the major version.
func (l *Launcher) DownloadTemurinDirect(ctx context.Context, major int, emit func(ProgressEvent)) error {
	return l.downloadTemurin(ctx, major, emit)
}

// DeleteManagedJava removes the launcher-downloaded JDK directory for the major version.
func (l *Launcher) DeleteManagedJava(major int) error {
	dir := l.managedJavaDir(major)
	return os.RemoveAll(dir)
}

func findJavaInHome(home string) string {
	exe := filepath.Join(home, "bin", javaExeName())
	if st, err := os.Stat(exe); err == nil && !st.IsDir() {
		return exe
	}
	return ""
}

// EnsureJavaFor returns the java executable to launch v with. When the
// version declares a javaVersion and no matching JDK is installed, a Temurin
// build is downloaded and extracted into <root>/java. An empty return means
// "use the default detection" (cfg.JavaPath / findJava).
func (l *Launcher) EnsureJavaFor(ctx context.Context, v *VersionJSON, emit func(ProgressEvent)) (string, error) {
	if v.JavaVersion == nil || v.JavaVersion.MajorVersion <= 0 {
		return "", nil
	}
	major := v.JavaVersion.MajorVersion
	if exe := l.findLocalJava(major); exe != "" {
		return exe, nil
	}
	if emit != nil {
		emit(ProgressEvent{Stage: "java", Percent: 0, Message: fmt.Sprintf("Temurin %d", major)})
	}
	if err := l.downloadTemurin(ctx, major, emit); err != nil {
		return "", fmt.Errorf(l.T("err.java_dl"), major, err)
	}
	if exe := l.findLocalJava(major); exe != "" {
		return exe, nil
	}
	return "", fmt.Errorf(l.T("err.java_dl"), major, fmt.Errorf("jdk not found after install"))
}

// downloadTemurin fetches the latest GA Temurin JDK for the major version
// from the Adoptium API and extracts it into the managed java dir.
func (l *Launcher) downloadTemurin(ctx context.Context, major int, emit func(ProgressEvent)) error {
	aos, aarch := adoptiumPlatform()
	if aos == "" {
		return fmt.Errorf("unsupported platform")
	}
	url := fmt.Sprintf("https://api.adoptium.net/v3/binary/latest/%d/ga/%s/%s/jdk/hotspot/normal/eclipse", major, aos, aarch)
	zipPath := filepath.Join(l.CacheDir(), fmt.Sprintf("temurin-%d.zip", major))

	// find out the real size for a meaningful progress bar (HEAD follows redirects)
	var size int64
	if req, err := newHeadRequest(ctx, url); err == nil {
		if resp, err := httpClient.Do(req); err == nil {
			size = resp.ContentLength
			resp.Body.Close()
		}
	}

	err := downloadOne(ctx, dlTask{
		url:  url,
		dest: zipPath,
		size: size,
		prog: func(written int64) {
			if emit == nil || size <= 0 {
				return
			}
			pct := float64(written) / float64(size) * 95
			if pct > 95 {
				pct = 95
			}
			emit(ProgressEvent{Stage: "java", Percent: pct, Message: fmt.Sprintf("Temurin %d", major)})
		},
	})
	if err != nil {
		return err
	}

	dest := l.managedJavaDir(major)
	if err := os.RemoveAll(dest); err != nil {
		return err
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	if emit != nil {
		emit(ProgressEvent{Stage: "java", Percent: 97, Message: fmt.Sprintf("Temurin %d", major)})
	}
	if err := unzipAll(zipPath, dest); err != nil {
		return err
	}
	os.Remove(zipPath)
	if emit != nil {
		emit(ProgressEvent{Stage: "java", Percent: 100, Message: fmt.Sprintf("Temurin %d", major)})
	}
	return nil
}

// newHeadRequest builds a HEAD request used to probe the download size.
func newHeadRequest(ctx context.Context, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "WaiLauncher/0.1")
	return req, nil
}

// copyWithLimit copies r to w; archives are bounded to a sane maximum.
func copyWithLimit(w io.Writer, r io.Reader) (int64, error) {
	return io.Copy(w, r)
}

func adoptiumPlatform() (osName, arch string) {
	switch runtime.GOOS {
	case "windows":
		osName = "windows"
	case "linux":
		osName = "linux"
	case "darwin":
		osName = "mac"
	default:
		return "", ""
	}
	switch runtime.GOARCH {
	case "amd64":
		return osName, "x64"
	case "arm64":
		return osName, "aarch64"
	case "386":
		return osName, "x32"
	}
	return "", ""
}

// JavaUpdateInfo describes a pending Temurin patch update.
type JavaUpdateInfo struct {
	Major           int    `json:"major"`
	InstalledVersion string `json:"installedVersion"` // e.g. "17.0.13+11"
	LatestVersion   string `json:"latestVersion"`
	UpdateAvailable bool   `json:"updateAvailable"`
}

// CheckTemurinUpdate compares installed managed JDKs against the latest GA
// builds published by Adoptium. Returns one entry per installed major.
func (l *Launcher) CheckTemurinUpdate(ctx context.Context, majors []int) []JavaUpdateInfo {
	var results []JavaUpdateInfo
	for _, major := range majors {
		exe := l.findLocalJava(major)
		if exe == "" || !strings.Contains(strings.ToLower(exe), strings.ToLower(l.Root)) {
			continue // only launcher-managed installs are updatable here
		}
		installed := readReleaseVersion(exe)
		if installed == "" {
			continue
		}
		latest, err := latestTemurinVersion(ctx, major)
		if err != nil || latest == "" {
			continue
		}
		results = append(results, JavaUpdateInfo{
			Major:            major,
			InstalledVersion: installed,
			LatestVersion:    latest,
			UpdateAvailable:  latest != installed,
		})
	}
	return results
}

// readReleaseVersion extracts the full version string (e.g. "17.0.13+11")
// from the "release" file next to a JDK binary.
func readReleaseVersion(exe string) string {
	home := filepath.Dir(filepath.Dir(exe))
	data, err := os.ReadFile(filepath.Join(home, "release"))
	if err != nil {
		return ""
	}
	m := releaseVerRe.FindSubmatch(data)
	if m == nil {
		return ""
	}
	return string(m[1])
}

// latestTemurinVersion queries Adoptium for the newest GA build version.
func latestTemurinVersion(ctx context.Context, major int) (string, error) {
	aos, aarch := adoptiumPlatform()
	if aos == "" {
		return "", fmt.Errorf("unsupported platform")
	}
	url := fmt.Sprintf("https://api.adoptium.net/v3/assets/latest/%d/hotspot?architecture=%s&image_type=jdk&os=%s&project=jdk", major, aarch, aos)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "WaiLauncher/0.1")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var entries []struct {
		Version struct {
			Semver string `json:"semver"`
		} `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return "", nil
	}
	return entries[0].Version.Semver, nil
}

// unzipAll extracts a zip archive validating every entry path (zip-slip).
func unzipAll(zipPath, dest string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()
	destAbs, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	for _, e := range zr.File {
		name := filepath.Clean(filepath.FromSlash(e.Name))
		if strings.HasPrefix(name, "..") || filepath.IsAbs(name) {
			return fmt.Errorf("unsafe path in zip: %s", e.Name)
		}
		out := filepath.Join(destAbs, name)
		if !strings.HasPrefix(out, destAbs+string(os.PathSeparator)) && out != destAbs {
			return fmt.Errorf("unsafe path in zip: %s", e.Name)
		}
		if e.FileInfo().IsDir() {
			if err := os.MkdirAll(out, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		rc, err := e.Open()
		if err != nil {
			return err
		}
		f, err := os.OpenFile(out, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			rc.Close()
			return err
		}
		_, cerr := copyWithLimit(f, rc)
		rc.Close()
		f.Close()
		if cerr != nil {
			return cerr
		}
	}
	return nil
}
