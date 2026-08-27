package launcher

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// LaunchConfig carries everything needed to start one game session.
type LaunchConfig struct {
	Username    string
	UUID        string
	AccessToken string
	UserType    string
	XUID        string
	ClientID    string
	RAMMB       int
	JavaPath    string   // empty = auto-detect
	JVMPreset   string   // aikar | zgc | shenandoah | default | none
	ExtraJVM    []string // user-provided JVM arguments (per-instance overrides)
	GameDir     string   // empty = shared <root>/game (per-instance dirs otherwise)
	Width       int      // 0 = don't pass --width
	Height      int
	Fullscreen  bool
	Server      string // optional auto-connect server e.g. "mc.hypixel.net" or "host:port"
}

func classpathSep() string {
	if runtime.GOOS == "windows" {
		return ";"
	}
	return ":"
}

// isNewFMLStartup reports whether the version is launched through NeoForge
// 21.3+'s own startup entrypoint. That loader locates the patched Minecraft
// jar and the universal jar itself inside -DlibraryDirectory at runtime; if
// either is also on the app classpath, FML misclassifies the setup as a
// merged dev environment and refuses to start.
func isNewFMLStartup(mainClass string) bool {
	return mainClass == "net.neoforged.fml.startup.Client"
}

func isForgeLike(mainClass string) bool {
	return mainClass == "cpw.mods.bootstraplauncher.BootstrapLauncher" ||
		mainClass == "net.minecraftforge.bootstrap.ForgeBootstrap" ||
		mainClass == "net.neoforged.fml.startup.Client"
}

// classpath joins all applicable library jars plus the client jar.
func (l *Launcher) classpath(v *VersionJSON) string {
	newFML := isNewFMLStartup(v.MainClass)
	forgeLike := isForgeLike(v.MainClass)
	var parts []string
	for _, lib := range v.Libraries {
		if !rulesAllow(lib.Rules, nil) {
			continue
		}
		if strings.HasPrefix(lib.Name, "net.neoforged:minecraft-client-patched:") ||
			strings.HasPrefix(lib.Name, "net.neoforged:neoforge:") {
			// resolved by NeoForge/FML from the libraries directory instead
			continue
		}
		var rel string
		if a := lib.Downloads.Artifact; a != nil && a.Path != "" {
			rel = a.Path
		} else if p, err := MavenPath(lib.Name); err == nil {
			// modloader libraries resolved by name (fabric maven, patched jars)
			rel = p
		} else {
			continue
		}
		parts = append(parts, l.libraryPath(rel))
	}
	if !newFML && !forgeLike {
		parts = append(parts, filepath.Join(l.VersionDir(v.ID), v.ID+".jar"))
	}
	return strings.Join(parts, classpathSep())
}

// BuildCommand resolves the java binary and the full argument list for v.
func (l *Launcher) BuildCommand(v *VersionJSON, cfg LaunchConfig) (string, []string, error) {
	java := cfg.JavaPath
	if java == "" {
		java = findJava()
	}
	if java == "" {
		return "", nil, fmt.Errorf("%s", l.T("err.no_java"))
	}
	if cfg.RAMMB <= 0 {
		cfg.RAMMB = 4096
	}
	if cfg.GameDir == "" {
		cfg.GameDir = l.GameDir()
	}

	userType := cfg.UserType
	if userType == "" {
		userType = "legacy"
	}
	uuid := cfg.UUID
	if uuid == "" {
		uuid = "0"
	}
	token := cfg.AccessToken
	if token == "" {
		token = "0"
	}

	replace := func(s string) string {
		s = strings.ReplaceAll(s, "${auth_player_name}", cfg.Username)
		s = strings.ReplaceAll(s, "${version_name}", v.ID)
		s = strings.ReplaceAll(s, "${game_directory}", cfg.GameDir)
		s = strings.ReplaceAll(s, "${assets_root}", l.AssetsDir())
		s = strings.ReplaceAll(s, "${assets_index_name}", v.Assets)
		s = strings.ReplaceAll(s, "${auth_uuid}", uuid)
		s = strings.ReplaceAll(s, "${auth_access_token}", token)
		s = strings.ReplaceAll(s, "${user_type}", userType)
		s = strings.ReplaceAll(s, "${version_type}", v.Type)
		s = strings.ReplaceAll(s, "${natives_directory}/java", filepath.Join(l.NativesDir(v.ID), "java"))
		s = strings.ReplaceAll(s, "${natives_directory}/jna", filepath.Join(l.NativesDir(v.ID), "jna"))
		s = strings.ReplaceAll(s, "${natives_directory}/lwjgl", filepath.Join(l.NativesDir(v.ID), "lwjgl"))
		s = strings.ReplaceAll(s, "${natives_directory}/netty", filepath.Join(l.NativesDir(v.ID), "netty"))
		s = strings.ReplaceAll(s, "${natives_directory}", l.NativesDir(v.ID))
		s = strings.ReplaceAll(s, "${launcher_name}", "WaiLauncher")
		s = strings.ReplaceAll(s, "${launcher_version}", "0.1.0")
		s = strings.ReplaceAll(s, "${classpath}", l.classpath(v))
		s = strings.ReplaceAll(s, "${classpath_separator}", classpathSep())
		s = strings.ReplaceAll(s, "${library_directory}", l.LibrariesDir())
		s = strings.ReplaceAll(s, "${library_separator}", string(os.PathListSeparator))
		s = strings.ReplaceAll(s, "${auth_xuid}", cfg.XUID)
		s = strings.ReplaceAll(s, "${clientid}", cfg.ClientID)
		s = strings.ReplaceAll(s, "${resolution_width}", fmt.Sprint(cfg.Width))
		s = strings.ReplaceAll(s, "${resolution_height}", fmt.Sprint(cfg.Height))
		return s
	}

	features := map[string]bool{
		"has_custom_resolution": cfg.Width > 0 && cfg.Height > 0,
	}

	var jvm []string
	var game []string

	userHasHeap := false
	for _, extra := range cfg.ExtraJVM {
		if strings.HasPrefix(extra, "-Xmx") {
			userHasHeap = true
		}
	}
	if !userHasHeap {
		jvm = append(jvm, fmt.Sprintf("-Xmx%dM", cfg.RAMMB))
	}
	presetFlags := GetJVMPresetFlags(cfg.JVMPreset)
	if len(presetFlags) > 0 {
		jvm = append(jvm, presetFlags...)
	}
	jvm = append(jvm, cfg.ExtraJVM...)

	hasGameResRule := false
	switch {
	case len(v.Arguments.JVM) > 0 || len(v.Arguments.Game) > 0:
		for _, a := range v.Arguments.JVM {
			if rulesAllow(a.Rules, features) {
				for _, val := range a.Values {
					if strings.Contains(val, "MojangTricksIntelDriversForPerformance") {
						continue // causes crash in legacy AMD driver atio6axx.dll
					}
					if strings.Contains(val, "--sun-misc-unsafe-memory-access") {
						continue // only supported in Java 24+
					}
					jvm = append(jvm, replace(val))
				}
			}
		}
		for _, a := range v.Arguments.Game {
			if rulesAllow(a.Rules, features) {
				for _, val := range a.Values {
					game = append(game, replace(val))
				}
			}
			for _, r := range a.Rules {
				if r.Features != nil && r.Features["has_custom_resolution"] {
					hasGameResRule = true
				}
			}
		}
	case v.MinecraftArguments != "":
		jvm = append(jvm, "-Djava.library.path="+l.NativesDir(v.ID), "-cp", l.classpath(v))
		for _, tok := range strings.Fields(v.MinecraftArguments) {
			game = append(game, replace(tok))
		}
	default:
		return "", nil, fmt.Errorf(l.T("err.no_args"), v.ID)
	}

	if cfg.Fullscreen {
		game = append(game, "--fullscreen")
	} else if (!hasGameResRule || v.MinecraftArguments != "") && cfg.Width > 0 && cfg.Height > 0 {
		game = append(game, "--width", fmt.Sprint(cfg.Width), "--height", fmt.Sprint(cfg.Height))
	}

	if srv := strings.TrimSpace(cfg.Server); srv != "" {
		host := srv
		port := "25565"
		if strings.Contains(srv, ":") {
			parts := strings.SplitN(srv, ":", 2)
			host = parts[0]
			port = parts[1]
		}
		if host != "" {
			game = append(game, "--server", host, "--port", port)
		}
	}

	mainClass := v.MainClass
	if mainClass == "" {
		mainClass = "net.minecraft.client.main.Main"
	}
	args := append(jvm, mainClass)
	args = append(args, game...)
	return java, args, nil
}

// findJava looks for javaw/java in JAVA_HOME, PATH, and standard system install paths.
func findJava() string {
	var candidates []string

	// 1. JAVA_HOME
	if home := os.Getenv("JAVA_HOME"); home != "" {
		candidates = append(candidates,
			filepath.Join(home, "bin", "javaw.exe"),
			filepath.Join(home, "bin", "java.exe"),
			filepath.Join(home, "bin", "java"),
		)
	}

	// 2. PATH
	for _, name := range []string{"javaw.exe", "java.exe", "java"} {
		if p, err := exec.LookPath(name); err == nil {
			candidates = append(candidates, p)
		}
	}

	// 3. Standard Windows install locations
	if runtime.GOOS == "windows" {
		patterns := []string{
			`C:\Program Files\Java\*\bin\javaw.exe`,
			`C:\Program Files\Eclipse Adoptium\*\bin\javaw.exe`,
			`C:\Program Files\BellSoft\*\bin\javaw.exe`,
			`C:\Program Files\Microsoft\*\bin\javaw.exe`,
			`C:\Program Files\Zulu\*\bin\javaw.exe`,
			`C:\Program Files\Semeru\*\bin\javaw.exe`,
			`C:\Program Files (x86)\Java\*\bin\javaw.exe`,
		}
		if userProfile := os.Getenv("USERPROFILE"); userProfile != "" {
			patterns = append(patterns,
				filepath.Join(userProfile, `.jdks\*\bin\javaw.exe`),
				filepath.Join(userProfile, `AppData\Local\Programs\Eclipse Adoptium\*\bin\javaw.exe`),
			)
		}
		for _, pat := range patterns {
			if matches, err := filepath.Glob(pat); err == nil {
				candidates = append(candidates, matches...)
			}
		}
	} else {
		// Linux / macOS locations
		unixPatterns := []string{
			"/usr/lib/jvm/*/bin/java",
			"/Library/Java/JavaVirtualMachines/*/Contents/Home/bin/java",
		}
		for _, pat := range unixPatterns {
			if matches, err := filepath.Glob(pat); err == nil {
				candidates = append(candidates, matches...)
			}
		}
	}

	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c
		}
	}
	return ""
}

// GetJVMPresetFlags returns optimized JVM garbage collection and performance flags.
func GetJVMPresetFlags(preset string) []string {
	switch strings.ToLower(strings.TrimSpace(preset)) {
	case "zgc":
		return []string{
			"-XX:+UnlockExperimentalVMOptions",
			"-XX:+UseZGC",
			"-XX:+AlwaysPreTouch",
			"-XX:+DisableExplicitGC",
		}
	case "shenandoah":
		return []string{
			"-XX:+UnlockExperimentalVMOptions",
			"-XX:+UseShenandoahGC",
			"-XX:ShenandoahGCMode=iu",
			"-XX:+AlwaysPreTouch",
			"-XX:+DisableExplicitGC",
		}
	case "default":
		return []string{
			"-XX:+UnlockExperimentalVMOptions",
			"-XX:+UseG1GC",
			"-XX:MaxGCPauseMillis=50",
		}
	case "none":
		return nil
	case "aikar", "":
		fallthrough
	default:
		// Aikar's optimized flags for Minecraft G1GC
		return []string{
			"-XX:+UnlockExperimentalVMOptions",
			"-XX:+UseG1GC",
			"-XX:G1NewSizePercent=20",
			"-XX:G1ReservePercent=20",
			"-XX:MaxGCPauseMillis=50",
			"-XX:G1HeapRegionSize=32M",
			"-XX:+ParallelRefProcEnabled",
			"-XX:+AlwaysPreTouch",
			"-XX:+DisableExplicitGC",
		}
	}
}
