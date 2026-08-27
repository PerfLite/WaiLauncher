package launcher

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// CacheInfo contains statistics about cached files.
type CacheInfo struct {
	SizeBytes int64  `json:"sizeBytes"`
	Formatted string `json:"formatted"`
	FileCount int    `json:"fileCount"`
}

// GetCacheInfo calculates the total size and file count of the cache directory.
func (l *Launcher) GetCacheInfo() (*CacheInfo, error) {
	cacheDir := l.CacheDir()
	var totalSize int64
	var count int

	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		return &CacheInfo{SizeBytes: 0, Formatted: "0 Б", FileCount: 0}, nil
	}

	err := filepath.WalkDir(cacheDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		if !d.IsDir() {
			if info, err := d.Info(); err == nil {
				totalSize += info.Size()
				count++
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk cache dir: %w", err)
	}

	return &CacheInfo{
		SizeBytes: totalSize,
		Formatted: FormatBytes(totalSize),
		FileCount: count,
	}, nil
}

// CleanOldCache deletes files in CacheDir that have not been modified for olderThan duration.
func (l *Launcher) CleanOldCache(olderThan time.Duration) (int64, int, error) {
	cacheDir := l.CacheDir()
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		return 0, 0, nil
	}

	cutoff := time.Now().Add(-olderThan)
	var freedBytes int64
	var deletedCount int

	err := filepath.WalkDir(cacheDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		if !d.IsDir() {
			if info, err := d.Info(); err == nil {
				if info.ModTime().Before(cutoff) {
					size := info.Size()
					if removeErr := os.Remove(path); removeErr == nil {
						freedBytes += size
						deletedCount++
					}
				}
			}
		}
		return nil
	})

	return freedBytes, deletedCount, err
}

// ClearCache completely purges the cache directory and recreates it.
func (l *Launcher) ClearCache() (int64, error) {
	info, _ := l.GetCacheInfo()
	var freedBytes int64
	if info != nil {
		freedBytes = info.SizeBytes
	}

	cacheDir := l.CacheDir()
	_ = os.RemoveAll(cacheDir)
	_ = os.MkdirAll(cacheDir, 0o755)

	return freedBytes, nil
}

// FormatBytes formats a byte count into a human-readable string (e.g. "12.4 МБ").
func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d Б", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"КБ", "МБ", "ГБ", "ТБ"}
	if exp >= len(units) {
		exp = len(units) - 1
	}
	return fmt.Sprintf("%.1f %s", float64(b)/float64(div), units[exp])
}
