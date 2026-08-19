package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const versionManifestURL = "https://piston-meta.mojang.com/mc/game/version_manifest_v2.json"
const resourceBaseURL = "https://resources.download.minecraft.net"

func (l *Launcher) manifestCachePath() string {
	return filepath.Join(l.CacheDir(), "version_manifest.json")
}

// GetManifest returns the version manifest, preferring a cache younger than 24h.
// On network failure it falls back to any stale cache.
func (l *Launcher) GetManifest(ctx context.Context, refresh bool) (*VersionManifest, error) {
	cache := l.manifestCachePath()
	if !refresh {
		if st, err := os.Stat(cache); err == nil && time.Since(st.ModTime()) < 24*time.Hour {
			if m := readManifestCache(cache); m != nil {
				return m, nil
			}
		}
	}
	var m VersionManifest
	if err := fetchJSON(ctx, versionManifestURL, &m); err != nil {
		if stale := readManifestCache(cache); stale != nil {
			return stale, nil
		}
		return nil, err
	}
	if data, err := json.Marshal(&m); err == nil {
		os.WriteFile(cache, data, 0o644)
	}
	return &m, nil
}

func readManifestCache(path string) *VersionManifest {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var m VersionManifest
	if json.Unmarshal(data, &m) != nil || len(m.Versions) == 0 {
		return nil
	}
	return &m
}

// GetVersionJSON returns the version metadata, cached at versions/<id>/<id>.json.
func (l *Launcher) GetVersionJSON(ctx context.Context, ref VersionRef) (*VersionJSON, error) {
	path := filepath.Join(l.VersionDir(ref.ID), ref.ID+".json")
	if data, err := os.ReadFile(path); err == nil {
		var v VersionJSON
		if json.Unmarshal(data, &v) == nil && v.ID != "" {
			return &v, nil
		}
	}
	var v VersionJSON
	if err := fetchJSON(ctx, ref.URL, &v); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err == nil {
		if data, err := json.Marshal(&v); err == nil {
			os.WriteFile(path, data, 0o644)
		}
	}
	return &v, nil
}

// FindVersion locates a manifest entry by id.
func (l *Launcher) FindVersion(m *VersionManifest, id string) *VersionRef {
	for i := range m.Versions {
		if m.Versions[i].ID == id {
			return &m.Versions[i]
		}
	}
	return nil
}

// IsInstalled reports whether the client jar for id exists locally.
func (l *Launcher) IsInstalled(id string) bool {
	st, err := os.Stat(filepath.Join(l.VersionDir(id), id+".jar"))
	return err == nil && !st.IsDir() && st.Size() > 0
}

// loadLocalVersion reads a locally stored version json (vanilla cache or a
// generated modloader build).
func (l *Launcher) loadLocalVersion(id string) (*VersionJSON, error) {
	data, err := os.ReadFile(filepath.Join(l.VersionDir(id), id+".json"))
	if err != nil {
		return nil, err
	}
	var v VersionJSON
	if json.Unmarshal(data, &v) != nil || v.ID == "" {
		return nil, fmt.Errorf("invalid local version json for %s", id)
	}
	return &v, nil
}

// saveLocalVersion persists a generated version json next to its folder.
func (l *Launcher) saveLocalVersion(v *VersionJSON) error {
	dir := l.VersionDir(v.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, v.ID+".json"), data, 0o644)
}
