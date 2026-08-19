package launcher

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Manual end-to-end check of the Forge/NeoForge installer pipeline against
// the real launcher data dir. Run explicitly:
//
//	go test ./internal/launcher -run TestManualForgeLike -v -timeout 20m
func TestManualForgeLike(t *testing.T) {
	loader := os.Getenv("WAI_LOADER")
	lv := os.Getenv("WAI_LOADER_VERSION")
	mc := os.Getenv("WAI_MC")
	if loader == "" || lv == "" || mc == "" {
		t.Skip("set WAI_LOADER, WAI_LOADER_VERSION, WAI_MC")
	}
	root := os.Getenv("WAI_ROOT")
	if root == "" {
		t.Skip("set WAI_ROOT")
	}
	l, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	emit := func(e ProgressEvent) {
		fmt.Printf("[stage] %s %.0f%% %s\n", e.Stage, e.Percent, e.Message)
	}
	onLog := func(s string) { fmt.Println(s) }
	v, err := l.ResolveLoaderVersion(ctx, loader, lv, mc, emit, onLog)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	fmt.Printf("merged id=%s mainClass=%s libs=%d\n", v.ID, v.MainClass, len(v.Libraries))

	if os.Getenv("WAI_LAUNCH") != "1" {
		return
	}
	if err := l.EnsureInstalled(ctx, v, emit); err != nil {
		t.Fatalf("install: %v", err)
	}
	gameDir := filepath.Join(root, "instances", os.Getenv("WAI_GAMEDIR"))
	_ = os.MkdirAll(gameDir, 0o755)
	ensureFMLEarlyWindowOff(v, gameDir)
	cfg := LaunchConfig{Username: "TestUser", UUID: "00000000-0000-0000-0000-000000000000", RAMMB: 4096, GameDir: gameDir, Width: 854, Height: 480}
	if jp, err := l.EnsureJavaFor(ctx, v, emit); err != nil {
		t.Fatalf("java: %v", err)
	} else if jp != "" {
		cfg.JavaPath = jp
	}
	java, args, err := l.BuildCommand(v, cfg)
	if err != nil {
		t.Fatalf("build cmd: %v", err)
	}
	fmt.Printf("java=%s args=%d\n", java, len(args))
	cmd := exec.Command(java, args...)
	cmd.Dir = gameDir
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	go func() {
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			fmt.Println("[game] " + sc.Text())
		}
	}()
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()
	select {
	case err := <-waited:
		fmt.Printf("game exited early: %v\n", err)
		if err != nil {
			t.Fatalf("game exited early: %v", err)
		}
	case <-time.After(120 * time.Second):
		fmt.Println("game still alive after 120s — SUCCESS, killing")
		_ = cmd.Process.Kill()
		<-waited
	}
}
