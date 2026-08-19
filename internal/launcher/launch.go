package launcher

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// GameHandle wraps the running game process.
type GameHandle struct {
	Cmd *exec.Cmd
}

// Launch resolves a version by id (Mojang manifest, falling back to locally
// stored modloader builds), installs it if needed and starts the game.
func (l *Launcher) Launch(ctx context.Context, versionID string, cfg LaunchConfig, emit func(ProgressEvent), onLog func(line string)) (*GameHandle, error) {
	emit(ProgressEvent{Stage: "manifest", Message: versionID})
	var v *VersionJSON
	m, err := l.GetManifest(ctx, false)
	if err == nil {
		if ref := l.FindVersion(m, versionID); ref != nil {
			v, err = l.GetVersionJSON(ctx, *ref)
			if err != nil {
				return nil, fmt.Errorf(l.T("err.meta"), err)
			}
		}
	}
	if v == nil {
		// Not a vanilla version — maybe a generated modloader build.
		if lv, lerr := l.loadLocalVersion(versionID); lerr == nil {
			v = lv
		} else if err != nil {
			return nil, fmt.Errorf(l.T("err.manifest"), err)
		} else {
			return nil, fmt.Errorf(l.T("err.not_found"), versionID)
		}
	}
	return l.LaunchVersion(ctx, v, cfg, emit, onLog)
}

// LaunchVersion installs the given version json (if needed) and starts the
// game process. It returns once the process has started; wait on the handle
// for exit. Cancelling ctx aborts downloads; once started it also kills the game.
func (l *Launcher) LaunchVersion(ctx context.Context, v *VersionJSON, cfg LaunchConfig, emit func(ProgressEvent), onLog func(line string)) (*GameHandle, error) {
	if err := l.EnsureInstalled(ctx, v, emit); err != nil {
		return nil, err
	}
	// Pick (or auto-install) the Java major this version actually needs —
	// a too-new JVM crashes LWJGL on older Minecraft builds.
	if jp, err := l.EnsureJavaFor(ctx, v, emit); err != nil {
		return nil, err
	} else if jp != "" {
		cfg.JavaPath = jp
	}

	emit(ProgressEvent{Stage: "start", Message: "JVM"})
	gameDir := cfg.GameDir
	if gameDir == "" {
		gameDir = l.GameDir()
	}
	if err := os.MkdirAll(gameDir, 0o755); err != nil {
		return nil, err
	}
	ensureFMLEarlyWindowOff(v, gameDir)
	l.SyncOptions(cfg)
	java, args, err := l.BuildCommand(v, cfg)
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, java, args...)
	cmd.Dir = gameDir
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf(l.T("err.java_start"), err)
	}
	CenterProcessWindow(cmd.Process.Pid, cfg.Fullscreen)
	if onLog != nil {
		go func() {
			sc := bufio.NewScanner(stdout)
			sc.Buffer(make([]byte, 64*1024), 1024*1024)
			for sc.Scan() {
				onLog(sc.Text())
			}
		}()
	}
	return &GameHandle{Cmd: cmd}, nil
}

// ensureFMLEarlyWindowOff writes flat earlyWindowControl = false into config/fml.toml
// so FML skips the early splash window (fmlearlywindow), preventing AMD GPU driver crashes.
func ensureFMLEarlyWindowOff(v *VersionJSON, gameDir string) {
	if v == nil || !isFMLMainClass(v.MainClass) {
		return
	}
	cfgDir := filepath.Join(gameDir, "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		return
	}
	fmlPath := filepath.Join(cfgDir, "fml.toml")
	data, err := os.ReadFile(fmlPath)
	if err != nil {
		content := "earlyWindowControl = false\nearlyWindowProvider = \"none\"\n"
		_ = os.WriteFile(fmlPath, []byte(content), 0o644)
		return
	}
	s := string(data)
	if strings.Contains(s, "earlyWindowControl") {
		s = regexp.MustCompile(`(?m)^\s*earlyWindowControl\s*=.*$`).ReplaceAllString(s, "earlyWindowControl = false")
	} else {
		s = "earlyWindowControl = false\nearlyWindowProvider = \"none\"\n" + s
	}
	// Remove any invalid section headers
	s = strings.ReplaceAll(s, "[earlyDisplay]\n", "")
	s = strings.ReplaceAll(s, "[earlyDisplay]\r\n", "")
	_ = os.WriteFile(fmlPath, []byte(s), 0o644)
}

// isFMLMainClass reports whether the version is launched through Forge /
// NeoForge's FML bootstrap (both use the same early-window machinery).
// NeoForge 21.3+ moved from bootstraplauncher to its own startup class.
func isFMLMainClass(mainClass string) bool {
	switch mainClass {
	case "cpw.mods.bootstraplauncher.BootstrapLauncher", "net.neoforged.fml.startup.Client":
		return true
	}
	return false
}

// SyncOptions writes resolution and fullscreen settings to the game dir's
// options.txt so Minecraft sizes the window as requested on launch.
func (l *Launcher) SyncOptions(cfg LaunchConfig) {
	gameDir := cfg.GameDir
	if gameDir == "" {
		gameDir = l.GameDir()
	}
	optPath := filepath.Join(gameDir, "options.txt")
	var keptLines []string
	if data, err := os.ReadFile(optPath); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimRight(line, "\r")
			if strings.HasPrefix(line, "overrideWidth:") ||
				strings.HasPrefix(line, "overrideHeight:") ||
				strings.HasPrefix(line, "fullscreen:") {
				continue
			}
			if line != "" {
				keptLines = append(keptLines, line)
			}
		}
	}

	if cfg.Fullscreen {
		keptLines = append(keptLines, "fullscreen:true")
	} else {
		keptLines = append(keptLines, "fullscreen:false")
		w := cfg.Width
		h := cfg.Height
		if w <= 0 {
			w = 854
		}
		if h <= 0 {
			h = 480
		}
		keptLines = append(keptLines, fmt.Sprintf("overrideWidth:%d", w))
		keptLines = append(keptLines, fmt.Sprintf("overrideHeight:%d", h))
	}
	_ = os.WriteFile(optPath, []byte(strings.Join(keptLines, "\r\n")+"\r\n"), 0o644)
}

// Wait blocks until the game process exits.
func (h *GameHandle) Wait() error {
	return h.Cmd.Wait()
}
