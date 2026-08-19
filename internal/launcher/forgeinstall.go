package launcher

import (
	"archive/zip"
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// forgeSideValue carries the per-side value of an install_profile data entry.
type forgeSideValue struct {
	Client string `json:"client"`
	Server string `json:"server"`
}

// forgeProcessor is one step of the Forge/NeoForge install pipeline.
type forgeProcessor struct {
	Sides     []string `json:"sides"`
	Jar       string   `json:"jar"`
	Classpath []string `json:"classpath"`
	Args      []string `json:"args"`
}

// forgeInstallProfile is the install_profile.json inside installer jars.
type forgeInstallProfile struct {
	Spec      int    `json:"spec"`
	Version   string `json:"version"`
	Minecraft string `json:"minecraft"`
	Install   struct {
		ProfileName string `json:"profileName"`
		Target      string `json:"target"`
		Path        string `json:"path"`     // universal maven coordinate (legacy)
		FilePath    string `json:"filePath"` // universal jar path inside the zip (legacy)
	} `json:"install"`
	Data        map[string]forgeSideValue `json:"data"`
	Processors  []forgeProcessor          `json:"processors"`
	Libraries   []Library                 `json:"libraries"`
	VersionInfo *VersionJSON              `json:"versionInfo"` // legacy (<=1.12.2)
}

func forgeInstallerURL(loader, mcVersion, loaderVersion string) string {
	if loader == "neoforge" {
		return neoMavenURL + "net/neoforged/neoforge/" + loaderVersion + "/neoforge-" + loaderVersion + "-installer.jar"
	}
	full := mcVersion + "-" + loaderVersion
	return forgeMavenURL + "net/minecraftforge/forge/" + full + "/forge-" + full + "-installer.jar"
}

// installForgeLike runs the Forge/NeoForge installer flow: download the
// installer jar, download its libraries, execute the client-side processors
// and produce a merged version json.
func (l *Launcher) installForgeLike(ctx context.Context, loader, mcVersion, loaderVersion string, emit func(ProgressEvent), onLog func(string)) (*VersionJSON, error) {
	emit(ProgressEvent{Stage: "loader", Message: loader + " " + loaderVersion})

	installerURL := forgeInstallerURL(loader, mcVersion, loaderVersion)
	installerPath := filepath.Join(l.CacheDir(), "installers", filepath.Base(installerURL))
	if err := downloadOne(ctx, dlTask{url: installerURL, dest: installerPath}); err != nil {
		return nil, fmt.Errorf(l.T("err.installer"), filepath.Base(installerURL), err)
	}

	zr, err := zip.OpenReader(installerPath)
	if err != nil {
		return nil, fmt.Errorf(l.T("err.installer"), filepath.Base(installerPath), err)
	}
	defer zr.Close()

	var profile forgeInstallProfile
	if err := readZipJSON(&zr.Reader, "install_profile.json", &profile); err != nil {
		return nil, fmt.Errorf(l.T("err.installer"), "install_profile.json", err)
	}

	// Legacy flow (1.12.2 and older): embedded universal jar, no processors.
	if profile.VersionInfo != nil {
		return l.installLegacyForge(ctx, &zr.Reader, &profile, mcVersion, emit)
	}

	var loaderVJ VersionJSON
	if err := readZipJSON(&zr.Reader, "version.json", &loaderVJ); err != nil {
		return nil, fmt.Errorf(l.T("err.installer"), "version.json", err)
	}
	mergedID := loaderVJ.ID
	if mergedID == "" {
		mergedID = profile.Version
	}

	// Already installed on a previous launch?
	if v, err := l.loadLocalVersion(mergedID); err == nil {
		return v, nil
	}

	base, err := l.baseVersionJSON(ctx, mcVersion)
	if err != nil {
		return nil, err
	}

	// The processors need the vanilla client jar on disk.
	mcJar := filepath.Join(l.VersionDir(mcVersion), mcVersion+".jar")
	if base.Downloads.Client.URL != "" {
		emit(ProgressEvent{Stage: "client", Message: mcVersion})
		err := downloadOne(ctx, dlTask{
			url: base.Downloads.Client.URL, dest: mcJar,
			sha1: base.Downloads.Client.SHA1, size: base.Downloads.Client.Size,
		})
		if err != nil {
			return nil, fmt.Errorf("client: %w", err)
		}
	}

	// All installer libraries (processor tools + loader runtime jars).
	emit(ProgressEvent{Stage: "libraries"})
	var tasks []dlTask
	for _, lib := range profile.Libraries {
		if a := lib.Downloads.Artifact; a != nil && a.Path != "" && a.URL != "" {
			tasks = append(tasks, dlTask{url: a.URL, dest: l.libraryPath(a.Path), sha1: a.SHA1, size: a.Size})
		}
	}
	if err := downloadAll(ctx, tasks, 8, func(done, total int) {
		emit(ProgressEvent{Stage: "libraries", Percent: float64(done) / float64(total) * 100, Message: fmt.Sprintf("%d/%d", done, total)})
	}); err != nil {
		return nil, fmt.Errorf("libraries: %w", err)
	}

	// Scratch dir for entries extracted out of the installer zip.
	tmpDir := filepath.Join(l.CacheDir(), "installer-tmp", mergedID)
	_ = os.MkdirAll(tmpDir, 0o755)

	env := map[string]string{
		"SIDE":               "client",
		"MINECRAFT_JAR":      mcJar,
		"MINECRAFT_VERSION":  mcVersion,
		"ROOT":               l.Root,
		"INSTALLER":          installerPath,
		"LIBRARY_DIR":        l.LibrariesDir(),
		"JAVA_HOME":          filepath.Dir(consoleJava()),
		"MAPPINGS_VERSION":   mcVersion,
		"PATCHED_MINECRAFT":  "",
		"OUTPUT_DIR":         l.LibrariesDir(),
		"MINECRAFT_VERSIONS": l.VersionsDir(),
	}
	resolve := func(tok string) (string, error) {
		return l.resolveForgeToken(tok, env, profile.Data, &zr.Reader, tmpDir)
	}

	// Run client-side processors sequentially.
	var total, done int
	for _, p := range profile.Processors {
		if processorForClient(p.Sides) {
			total++
		}
	}
	for _, p := range profile.Processors {
		if !processorForClient(p.Sides) {
			continue
		}
		done++
		emit(ProgressEvent{Stage: "loader", Percent: float64(done) / float64(total) * 100, Message: fmt.Sprintf("%s %d/%d", loader, done, total)})
		if onLog != nil {
			onLog(fmt.Sprintf("[%s processor %d/%d] %s", loader, done, total, p.Jar))
		}
		if err := l.runProcessor(ctx, p, resolve, onLog); err != nil {
			return nil, fmt.Errorf(l.T("err.processor"), p.Jar, err)
		}
	}

	// The patched jar becomes the game jar of the merged version.
	patchedCoord, patchedPath, err := l.patchedJarInfo(profile.Data, env, &zr.Reader, tmpDir)
	if err != nil {
		return nil, err
	}
	gameJar := filepath.Join(l.VersionDir(mergedID), mergedID+".jar")
	if err := os.MkdirAll(filepath.Dir(gameJar), 0o755); err != nil {
		return nil, err
	}
	if err := copyFile(patchedPath, gameJar); err != nil {
		return nil, err
	}

	// Merge: loader version json over vanilla, plus patched + universal jars.
	var gameArgs, jvmArgs []Arg
	if loaderVJ.Arguments != nil {
		gameArgs = loaderVJ.Arguments.Game
		jvmArgs = loaderVJ.Arguments.JVM
	}
	merged := mergeLoaderProfile(base, mergedID, loaderVJ.MainClass, gameArgs, jvmArgs, loaderVJ.Libraries)
	merged.Libraries = append([]Library{{Name: patchedCoord}}, merged.Libraries...)
	for _, lib := range profile.Libraries {
		if strings.HasSuffix(strings.Split(lib.Name, "@")[0], ":universal") {
			merged.Libraries = append([]Library{lib}, merged.Libraries...)
			break
		}
	}
	// The game jar is already in place — prevent installClient from
	// overwriting it with the vanilla client.
	merged.Downloads.Client = Download{}
	if err := l.saveLocalVersion(merged); err != nil {
		return nil, err
	}
	_ = os.RemoveAll(tmpDir)
	return merged, nil
}

// patchedJarInfo extracts the PATCHED data entry: its maven coordinate and
// the resolved file path produced by the binarypatcher processor.
func (l *Launcher) patchedJarInfo(data map[string]forgeSideValue, env map[string]string, zr *zip.Reader, tmpDir string) (string, string, error) {
	sv, ok := data["PATCHED"]
	if !ok || sv.Client == "" {
		return "", "", fmt.Errorf("install profile has no PATCHED entry")
	}
	raw := strings.TrimSpace(sv.Client)
	if !strings.HasPrefix(raw, "[") || !strings.HasSuffix(raw, "]") {
		return "", "", fmt.Errorf("unexpected PATCHED value %q", raw)
	}
	coord := raw[1 : len(raw)-1]
	rel, err := MavenPath(coord)
	if err != nil {
		return "", "", err
	}
	return coord, l.libraryPath(rel), nil
}

// processorForClient reports whether a processor runs on the client side.
func processorForClient(sides []string) bool {
	if len(sides) == 0 {
		return true
	}
	for _, s := range sides {
		if s == "client" {
			return true
		}
	}
	return false
}

// resolveForgeToken turns one processor argument into a concrete value:
// {ENV} tokens, {DATA} references, [maven] coordinates or plain literals.
func (l *Launcher) resolveForgeToken(tok string, env map[string]string, data map[string]forgeSideValue, zr *zip.Reader, tmpDir string) (string, error) {
	tok = strings.TrimSpace(tok)
	if strings.HasPrefix(tok, "{") && strings.HasSuffix(tok, "}") {
		key := tok[1 : len(tok)-1]
		if v, ok := env[key]; ok {
			return v, nil
		}
		sv, ok := data[key]
		if !ok {
			return "", fmt.Errorf("unknown token %s", tok)
		}
		return l.resolveForgeDataValue(sv.Client, zr, tmpDir)
	}
	if strings.HasPrefix(tok, "[") && strings.HasSuffix(tok, "]") {
		rel, err := MavenPath(tok[1 : len(tok)-1])
		if err != nil {
			return "", err
		}
		return l.libraryPath(rel), nil
	}
	return tok, nil
}

// resolveForgeDataValue interprets one install_profile data value:
// [maven] -> library path, /path -> extracted from the installer, 'x' -> literal.
func (l *Launcher) resolveForgeDataValue(raw string, zr *zip.Reader, tmpDir string) (string, error) {
	raw = strings.TrimSpace(raw)
	switch {
	case strings.HasPrefix(raw, "[") && strings.HasSuffix(raw, "]"):
		rel, err := MavenPath(raw[1 : len(raw)-1])
		if err != nil {
			return "", err
		}
		return l.libraryPath(rel), nil
	case strings.HasPrefix(raw, "/"):
		entry := strings.TrimPrefix(raw, "/")
		dest := filepath.Join(tmpDir, filepath.Base(filepath.FromSlash(entry)))
		if err := extractZipEntry(zr, entry, dest); err != nil {
			return "", fmt.Errorf("extract %s: %w", entry, err)
		}
		return dest, nil
	case strings.HasPrefix(raw, "'") && strings.HasSuffix(raw, "'") && len(raw) >= 2:
		return raw[1 : len(raw)-1], nil
	}
	return raw, nil
}

// runProcessor executes one installer processor as `java -cp ... Main args`.
func (l *Launcher) runProcessor(ctx context.Context, p forgeProcessor, resolve func(string) (string, error), onLog func(string)) error {
	rel, err := MavenPath(p.Jar)
	if err != nil {
		return err
	}
	jarPath := l.libraryPath(rel)
	mainClass, err := jarMainClass(jarPath)
	if err != nil {
		return fmt.Errorf("main class of %s: %w", filepath.Base(jarPath), err)
	}

	cp := []string{jarPath}
	for _, c := range p.Classpath {
		crel, err := MavenPath(c)
		if err != nil {
			return err
		}
		cp = append(cp, l.libraryPath(crel))
	}

	args := []string{"-cp", strings.Join(cp, classpathSep()), mainClass}
	for _, a := range p.Args {
		v, err := resolve(a)
		if err != nil {
			return fmt.Errorf("arg %s: %w", a, err)
		}
		args = append(args, v)
	}

	java := consoleJava()
	if java == "" {
		return fmt.Errorf("%s", l.T("err.no_java"))
	}
	cmd := exec.CommandContext(ctx, java, args...)
	cmd.SysProcAttr = hideWindowAttr()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return err
	}
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		if onLog != nil {
			onLog("  " + sc.Text())
		}
	}
	if err := cmd.Wait(); err != nil {
		return err
	}
	return nil
}

// installLegacyForge handles installers with a versionInfo section
// (Minecraft 1.12.2 and older): extract the embedded universal jar and
// use versionInfo as the launch profile.
func (l *Launcher) installLegacyForge(ctx context.Context, zr *zip.Reader, profile *forgeInstallProfile, mcVersion string, emit func(ProgressEvent)) (*VersionJSON, error) {
	vi := profile.VersionInfo
	id := profile.Install.Target
	if id == "" {
		id = vi.ID
	}
	if v, err := l.loadLocalVersion(id); err == nil {
		return v, nil
	}

	if profile.Install.FilePath != "" && profile.Install.Path != "" {
		rel, err := MavenPath(profile.Install.Path)
		if err != nil {
			return nil, err
		}
		if err := extractZipEntry(zr, profile.Install.FilePath, l.libraryPath(rel)); err != nil {
			return nil, fmt.Errorf(l.T("err.installer"), profile.Install.FilePath, err)
		}
	}

	base, err := l.baseVersionJSON(ctx, mcVersion)
	if err != nil {
		return nil, err
	}
	merged := *vi
	merged.ID = id
	if merged.Downloads.Client.URL == "" {
		merged.Downloads.Client = base.Downloads.Client
	}
	if merged.Assets == "" {
		merged.Assets = base.Assets
	}
	if merged.AssetIndex == nil {
		merged.AssetIndex = base.AssetIndex
	}
	if profile.Install.Path != "" {
		universal := Library{Name: profile.Install.Path, URL: forgeMavenURL}
		merged.Libraries = append([]Library{universal}, merged.Libraries...)
	}
	if err := l.saveLocalVersion(&merged); err != nil {
		return nil, err
	}
	return &merged, nil
}

// ---- helpers ----

func readZipJSON(zr *zip.Reader, name string, out any) error {
	f, err := zr.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

// extractZipEntry copies one entry of the zip to dest (mkdir included).
// The dest path is validated to stay inside its parent directory.
func extractZipEntry(zr *zip.Reader, name, dest string) error {
	name = filepath.ToSlash(filepath.Clean(filepath.FromSlash(name)))
	if strings.HasPrefix(name, "/") || strings.Contains(name, "..") {
		return fmt.Errorf("unsafe archive path %q", name)
	}
	dest = filepath.Clean(dest)
	base := filepath.Dir(dest)
	if !strings.HasPrefix(dest, base+string(os.PathSeparator)) {
		return fmt.Errorf("unsafe extraction target %q", dest)
	}
	f, err := zr.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := os.MkdirAll(base, 0o755); err != nil {
		return err
	}
	w, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer w.Close()
	_, err = io.Copy(w, f)
	return err
}

// jarMainClass reads the Main-Class attribute from a jar's manifest.
func jarMainClass(jarPath string) (string, error) {
	zr, err := zip.OpenReader(jarPath)
	if err != nil {
		return "", err
	}
	defer zr.Close()
	f, err := zr.Open("META-INF/MANIFEST.MF")
	if err != nil {
		return "", err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "Main-Class:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Main-Class:")), nil
		}
	}
	return "", fmt.Errorf("no Main-Class in %s", filepath.Base(jarPath))
}

// consoleJava returns a java binary that writes to stdout (not javaw).
func consoleJava() string {
	j := findJava()
	if strings.HasSuffix(j, "javaw.exe") {
		return strings.TrimSuffix(j, "javaw.exe") + "java.exe"
	}
	return j
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
