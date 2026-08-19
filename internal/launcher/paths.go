package launcher

import (
	"os"
	"path/filepath"
)

// Launcher owns the on-disk layout and provides the install/launch services.
type Launcher struct {
	Root string
	Lang string // UI language for backend messages: "ru" (default) or "en"
}

// T localizes a backend message using the launcher's current language.
func (l *Launcher) T(key string, args ...any) string {
	return T(l.Lang, key, args...)
}

// New creates the Launcher and ensures the directory tree exists.
func New(root string) (*Launcher, error) {
	l := &Launcher{Root: root}
	dirs := []string{
		l.VersionsDir(),
		l.LibrariesDir(),
		filepath.Join(l.AssetsDir(), "indexes"),
		filepath.Join(l.AssetsDir(), "objects"),
		l.InstancesDir(),
		l.CacheDir(),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, err
		}
	}
	return l, nil
}

// DefaultRoot returns %APPDATA%/WaiLauncher on Windows, ~/.config/WaiLauncher elsewhere.
func DefaultRoot() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "WaiLauncher"), nil
}

func (l *Launcher) VersionsDir() string         { return filepath.Join(l.Root, "versions") }
func (l *Launcher) VersionDir(id string) string { return filepath.Join(l.VersionsDir(), id) }
func (l *Launcher) LibrariesDir() string        { return filepath.Join(l.Root, "libraries") }
func (l *Launcher) AssetsDir() string           { return filepath.Join(l.Root, "assets") }
func (l *Launcher) GameDir() string             { return filepath.Join(l.Root, "game") }
func (l *Launcher) CacheDir() string            { return filepath.Join(l.Root, "cache") }
func (l *Launcher) InstancesDir() string        { return filepath.Join(l.Root, "instances") }
func (l *Launcher) InstanceDir(id string) string {
	return filepath.Join(l.InstancesDir(), filepath.Base(id))
}
func (l *Launcher) NativesDir(id string) string {
	return filepath.Join(l.VersionDir(id), "natives")
}
