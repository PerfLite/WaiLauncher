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

// Manual launch of a vanilla version against the real data dir, used to
// isolate GPU-driver crashes from modloader issues. Run explicitly:
//
//	go test ./internal/launcher -run TestManualVanillaLaunch -v -timeout 20m
func TestManualVanillaLaunch(t *testing.T) {
	mc := os.Getenv("WAI_MC")
	root := os.Getenv("WAI_ROOT")
	if mc == "" || root == "" {
		t.Skip("set WAI_MC and WAI_ROOT")
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
	m, err := l.GetManifest(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	ref := l.FindVersion(m, mc)
	if ref == nil {
		t.Fatalf("version %s not found", mc)
	}
	v, err := l.GetVersionJSON(ctx, *ref)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.EnsureInstalled(ctx, v, emit); err != nil {
		t.Fatal(err)
	}
	cfg := LaunchConfig{Username: "TestUser", RAMMB: 4096, Width: 854, Height: 480}
	cfg.GameDir = filepath.Join(root, "instances", "vanilla-test-"+mc)
	_ = os.MkdirAll(cfg.GameDir, 0o755)
	if jp, err := l.EnsureJavaFor(ctx, v, emit); err != nil {
		t.Fatalf("java: %v", err)
	} else if jp != "" {
		cfg.JavaPath = jp
	}
	java, args, err := l.BuildCommand(v, cfg)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("java=%s\n", java)
	cmd := exec.Command(java, args...)
	cmd.Dir = cfg.GameDir
	stdout, _ := cmd.StdoutPipe()
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
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
	case <-time.After(90 * time.Second):
		fmt.Println("game still alive after 90s — SUCCESS, killing")
		_ = cmd.Process.Kill()
		<-waited
	}
}
