package launcher

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ProgressEvent describes one step of the install/launch pipeline.
type ProgressEvent struct {
	Stage   string  `json:"stage"`   // manifest | client | libraries | natives | assets | start
	Percent float64 `json:"percent"` // 0..100 within the stage
	Message string  `json:"message"`
}

// EnsureInstalled downloads everything required to run v: client jar,
// libraries, natives and assets. Idempotent: once fully installed, a marker
// file skips the expensive library/asset verification on later launches.
func (l *Launcher) EnsureInstalled(ctx context.Context, v *VersionJSON, emit func(ProgressEvent)) error {
	if err := l.installClient(ctx, v, emit); err != nil {
		return err
	}
	if l.installMarkerValid(v) {
		// Everything was verified on a previous launch — only re-extract
		// natives (cheap) and go straight to starting the game.
		return l.extractAllNatives(v)
	}
	if err := l.installLibraries(ctx, v, emit); err != nil {
		return err
	}
	if err := l.installAssets(ctx, v, emit); err != nil {
		return err
	}
	l.writeInstallMarker(v)
	return l.extractAllNatives(v)
}

// LoadVersion loads the VersionJSON for a vanilla or local modloader version ID.
func (l *Launcher) LoadVersion(ctx context.Context, versionID string) (*VersionJSON, error) {
	m, err := l.GetManifest(ctx, false)
	if err == nil {
		if ref := l.FindVersion(m, versionID); ref != nil {
			v, err := l.GetVersionJSON(ctx, *ref)
			if err == nil {
				return v, nil
			}
		}
	}
	// Fallback to local version
	if lv, lerr := l.loadLocalVersion(versionID); lerr == nil {
		return lv, nil
	}
	if err != nil {
		return nil, fmt.Errorf(l.T("err.manifest"), err)
	}
	return nil, fmt.Errorf(l.T("err.not_found"), versionID)
}

// VerifyAndRepair forcibly checks SHA1 and size of client jar, libraries, and assets for version v,
// downloading any missing or corrupted files, and re-extracts natives.
func (l *Launcher) VerifyAndRepair(ctx context.Context, v *VersionJSON, emit func(ProgressEvent)) (int, int, error) {
	// Remove install marker to force full verification
	_ = os.Remove(l.installMarkerPath(v.ID))

	if err := l.installClient(ctx, v, emit); err != nil {
		return 0, 0, err
	}
	if err := l.installLibraries(ctx, v, emit); err != nil {
		return 0, 0, err
	}
	if err := l.installAssets(ctx, v, emit); err != nil {
		return 0, 0, err
	}
	l.writeInstallMarker(v)
	if err := l.extractAllNatives(v); err != nil {
		return 0, 0, err
	}
	return len(v.Libraries), 0, nil
}

// installMarkerValid reports whether version v was fully installed with the
// same asset index before.
func (l *Launcher) installMarkerValid(v *VersionJSON) bool {
	data, err := os.ReadFile(l.installMarkerPath(v.ID))
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(data)) == installMarkerValue(v)
}

func (l *Launcher) writeInstallMarker(v *VersionJSON) {
	_ = os.WriteFile(l.installMarkerPath(v.ID), []byte(installMarkerValue(v)), 0o644)
}

func (l *Launcher) installMarkerPath(id string) string {
	return filepath.Join(l.VersionDir(id), ".installed")
}

// installMarkerValue ties the marker to the version and its asset index, so
// an asset-index change forces re-verification.
func installMarkerValue(v *VersionJSON) string {
	if v.AssetIndex != nil {
		return v.ID + "|" + v.AssetIndex.SHA1
	}
	return v.ID
}

func (l *Launcher) installClient(ctx context.Context, v *VersionJSON, emit func(ProgressEvent)) error {
	client := v.Downloads.Client
	dest := filepath.Join(l.VersionDir(v.ID), v.ID+".jar")
	if client.URL == "" {
		// Loader builds ship a pre-generated game jar (patched client).
		if st, err := os.Stat(dest); err == nil && st.Size() > 0 {
			return nil
		}
		return fmt.Errorf(l.T("err.no_client"), v.ID)
	}
	emit(ProgressEvent{Stage: "client", Message: v.ID})
	err := downloadOne(ctx, dlTask{
		url: client.URL, dest: dest, sha1: client.SHA1, size: client.Size,
		prog: func(w int64) {
			if client.Size > 0 {
				emit(ProgressEvent{Stage: "client", Percent: float64(w) / float64(client.Size) * 100})
			}
		},
	})
	if err != nil {
		return fmt.Errorf("client: %w", err)
	}
	return nil
}

func (l *Launcher) installLibraries(ctx context.Context, v *VersionJSON, emit func(ProgressEvent)) error {
	emit(ProgressEvent{Stage: "libraries"})
	var tasks []dlTask
	for _, lib := range v.Libraries {
		if !rulesAllow(lib.Rules, nil) {
			continue
		}
		switch a := lib.Downloads.Artifact; {
		case a != nil && a.Path != "" && a.URL != "":
			tasks = append(tasks, dlTask{url: a.URL, dest: l.libraryPath(a.Path), sha1: a.SHA1, size: a.Size})
		default:
			// Modloader libraries without explicit downloads are resolved
			// from their maven base url by coordinate.
			rel, err := MavenPath(lib.Name)
			if err != nil {
				break
			}
			url := ""
			if a != nil && a.URL != "" {
				url = a.URL
			} else if lib.URL != "" {
				url = lib.URL
			}
			if url == "" {
				break // produced locally (e.g. patched client jar)
			}
			if !strings.HasSuffix(url, "/") {
				url += "/"
			}
			dest := l.libraryPath(rel)
			if _, err := os.Stat(dest); err != nil {
				tasks = append(tasks, dlTask{url: url + filepath.ToSlash(rel), dest: dest})
			}
		}
		if cls, ok := nativeClassifier(lib); ok {
			if d, ok := lib.Downloads.Classifiers[cls]; ok {
				tasks = append(tasks, dlTask{url: d.URL, dest: l.libraryPath(d.Path), sha1: d.SHA1, size: d.Size})
			}
		}
	}
	if err := downloadAll(ctx, tasks, 8, func(done, total int) {
		emit(ProgressEvent{Stage: "libraries", Percent: float64(done) / float64(total) * 100, Message: fmt.Sprintf("%d/%d", done, total)})
	}); err != nil {
		return fmt.Errorf("libraries: %w", err)
	}
	return nil
}

// libraryPath joins a maven-style relative path onto the libraries dir,
// rejecting anything that would escape it.
func (l *Launcher) libraryPath(rel string) string {
	clean := filepath.Clean(filepath.FromSlash(rel))
	base := l.LibrariesDir()
	p := filepath.Join(base, clean)
	if p != base && !strings.HasPrefix(p, base+string(os.PathSeparator)) {
		return filepath.Join(base, filepath.Base(clean))
	}
	return p
}

func nativeClassifier(lib Library) (string, bool) {
	if len(lib.Natives) == 0 {
		return "", false
	}
	tpl, ok := lib.Natives[osName()]
	if !ok {
		return "", false
	}
	arch := "64"
	if osArch() == "x86" {
		arch = "32"
	}
	return strings.ReplaceAll(tpl, "${arch}", arch), true
}

func isNativeLibrary(lib Library) (string, bool) {
	if cls, ok := nativeClassifier(lib); ok {
		if d, ok := lib.Downloads.Classifiers[cls]; ok && d.Path != "" {
			return d.Path, true
		}
	}
	target := ":natives-" + osName()
	if osName() == "windows" {
		target = ":natives-windows"
	}
	if strings.Contains(lib.Name, target) {
		if a := lib.Downloads.Artifact; a != nil && a.Path != "" {
			return a.Path, true
		}
		if p, err := MavenPath(lib.Name); err == nil {
			return p, true
		}
	}
	return "", false
}

func (l *Launcher) extractAllNatives(v *VersionJSON) error {
	nativesDir := l.NativesDir(v.ID)
	subdirs := []string{"", "java", "lwjgl", "netty", "jna"}
	for _, sub := range subdirs {
		if err := os.MkdirAll(filepath.Join(nativesDir, sub), 0o755); err != nil {
			return err
		}
	}
	for _, lib := range v.Libraries {
		if !rulesAllow(lib.Rules, nil) {
			continue
		}
		relPath, ok := isNativeLibrary(lib)
		if !ok || relPath == "" {
			continue
		}
		jarPath := l.libraryPath(relPath)
		for _, sub := range subdirs {
			if err := extractJar(jarPath, filepath.Join(nativesDir, sub)); err != nil {
				return fmt.Errorf("natives %s: %w", lib.Name, err)
			}
		}
	}
	return nil
}

// extractJar unpacks a natives jar into destDir, skipping META-INF and
// guarding against zip-slip entries.
func extractJar(jarPath, destDir string) error {
	r, err := zip.OpenReader(jarPath)
	if err != nil {
		return err
	}
	defer r.Close()
	base := filepath.Clean(destDir) + string(os.PathSeparator)
	for _, f := range r.File {
		if f.FileInfo().IsDir() || strings.HasPrefix(strings.ToUpper(f.Name), "META-INF/") {
			continue
		}
		dest := filepath.Join(destDir, filepath.FromSlash(f.Name))
		if !strings.HasPrefix(dest, base) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		w, err := os.Create(dest)
		if err != nil {
			rc.Close()
			return err
		}
		_, copyErr := io.Copy(w, rc)
		w.Close()
		rc.Close()
		if copyErr != nil {
			return copyErr
		}
	}
	return nil
}

func (l *Launcher) installAssets(ctx context.Context, v *VersionJSON, emit func(ProgressEvent)) error {
	if v.AssetIndex == nil {
		return nil
	}
	idxPath := filepath.Join(l.AssetsDir(), "indexes", v.Assets+".json")
	if !fileValid(idxPath, v.AssetIndex.SHA1, v.AssetIndex.Size) {
		if err := downloadOne(ctx, dlTask{url: v.AssetIndex.URL, dest: idxPath, sha1: v.AssetIndex.SHA1, size: v.AssetIndex.Size}); err != nil {
			return fmt.Errorf("asset index: %w", err)
		}
	}
	data, err := os.ReadFile(idxPath)
	if err != nil {
		return err
	}
	var idx AssetIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return fmt.Errorf("asset index parse: %w", err)
	}
	tasks := make([]dlTask, 0, len(idx.Objects))
	for _, obj := range idx.Objects {
		if len(obj.Hash) < 2 {
			continue
		}
		sub := obj.Hash[:2]
		tasks = append(tasks, dlTask{
			url:  resourceBaseURL + "/" + sub + "/" + obj.Hash,
			dest: filepath.Join(l.AssetsDir(), "objects", sub, obj.Hash),
			sha1: obj.Hash,
			size: obj.Size,
		})
	}
	emit(ProgressEvent{Stage: "assets"})
	if err := downloadAll(ctx, tasks, 12, func(done, total int) {
		emit(ProgressEvent{Stage: "assets", Percent: float64(done) / float64(total) * 100, Message: fmt.Sprintf("%d/%d", done, total)})
	}); err != nil {
		return fmt.Errorf("assets: %w", err)
	}
	return nil
}
