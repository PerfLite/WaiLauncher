package launcher

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var httpClient = &http.Client{Timeout: 0} // cancellation via context only

func fetchJSON(ctx context.Context, url string, out any) error {
	data, err := fetchBytes(ctx, url)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

// fetchBytes GETs a URL and returns the raw body.
func fetchBytes(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "WaiLauncher/0.1")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func fileSHA1(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// fileValid reports whether path exists and matches size/sha1 (when given).
func fileValid(path, sha1sum string, size int64) bool {
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		return false
	}
	// Always reject empty 0-byte files
	if st.Size() == 0 {
		return false
	}
	if size > 0 && st.Size() != size {
		return false
	}
	if sha1sum == "" {
		return true
	}
	sum, err := fileSHA1(path)
	return err == nil && strings.EqualFold(sum, sha1sum)
}

type dlTask struct {
	url, dest, sha1 string
	size            int64
	prog            func(written int64) // optional byte-level progress
}

type countingWriter struct {
	w     io.Writer
	total int64
	prog  func(written int64)
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.total += int64(n)
	if c.prog != nil {
		c.prog(c.total)
	}
	return n, err
}

func downloadOne(ctx context.Context, t dlTask) error {
	if fileValid(t.dest, t.sha1, t.size) {
		if t.prog != nil && t.size > 0 {
			t.prog(t.size)
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(t.dest), 0o755); err != nil {
		return err
	}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if err := downloadAttempt(ctx, t); err != nil {
			lastErr = err
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt+1) * 700 * time.Millisecond):
			}
			continue
		}
		return nil
	}
	return lastErr
}

func downloadAttempt(ctx context.Context, t dlTask) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.url, nil)
	if err != nil {
		return err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", t.url, resp.Status)
	}
	tmp := t.dest + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	var dst io.Writer = f
	if t.prog != nil {
		dst = &countingWriter{w: f, prog: t.prog}
	}
	_, copyErr := io.Copy(dst, resp.Body)
	f.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return copyErr
	}
	st, err := os.Stat(tmp)
	if err != nil || st.Size() == 0 {
		_ = os.Remove(tmp)
		return fmt.Errorf("downloaded empty file for %s", t.url)
	}
	if t.size > 0 && st.Size() != t.size {
		_ = os.Remove(tmp)
		return fmt.Errorf("size mismatch for %s (expected %d, got %d)", t.url, t.size, st.Size())
	}
	if t.sha1 != "" {
		sum, err := fileSHA1(tmp)
		if err != nil || !strings.EqualFold(sum, t.sha1) {
			_ = os.Remove(tmp)
			return fmt.Errorf("sha1 mismatch for %s (expected %s, got %s)", t.url, t.sha1, sum)
		}
	}
	_ = os.Remove(t.dest)
	return os.Rename(tmp, t.dest)
}

// downloadAll runs tasks through a worker pool and reports completed counts.
// Remaining tasks keep downloading after individual failures; the first error
// is returned at the end.
func downloadAll(ctx context.Context, tasks []dlTask, workers int, onProgress func(done, total int)) error {
	total := len(tasks)
	if total == 0 {
		return nil
	}
	if workers < 1 {
		workers = 1
	}
	var done atomic.Int64
	var errOnce sync.Once
	var firstErr error

	ch := make(chan dlTask)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range ch {
				if err := downloadOne(ctx, t); err != nil {
					errOnce.Do(func() { firstErr = err })
				}
				if onProgress != nil {
					onProgress(int(done.Add(1)), total)
				}
			}
		}()
	}
feed:
	for _, t := range tasks {
		select {
		case <-ctx.Done():
			break feed
		case ch <- t:
		}
	}
	close(ch)
	wg.Wait()
	if firstErr != nil {
		return firstErr
	}
	return ctx.Err()
}
