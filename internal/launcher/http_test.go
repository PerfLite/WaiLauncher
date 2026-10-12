package launcher

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func sha1Hex(b []byte) string {
	h := sha1.Sum(b)
	return hex.EncodeToString(h[:])
}

// staticServer serves a fixed body at the given path and counts requests.
func staticServer(t *testing.T, body []byte, handler func(w http.ResponseWriter, r *http.Request)) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if handler != nil {
			handler(w, r)
			return
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func TestFileValid(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "f")

	// missing file
	if fileValid(path, "", 0) {
		t.Fatal("missing file should be invalid")
	}

	content := []byte("hello world")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha1Hex(content)

	if !fileValid(path, sum, int64(len(content))) {
		t.Error("matching sha1+size should be valid")
	}
	if fileValid(path, "0000000000000000000000000000000000000000", 0) {
		t.Error("wrong sha1 should be invalid")
	}
	if fileValid(path, "", int64(len(content)+1)) {
		t.Error("wrong size should be invalid")
	}
	if fileValid(path, "", 0) != true {
		t.Error("non-empty file with no expectations should be valid")
	}

	// empty file is always invalid
	empty := filepath.Join(dir, "empty")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if fileValid(empty, "", 0) {
		t.Error("0-byte file should be invalid")
	}
}

func TestDownloadAlwaysSuccessAtomic(t *testing.T) {
	t.Parallel()
	body := []byte("payload-12345")
	srv, _ := staticServer(t, body, nil)
	dest := filepath.Join(t.TempDir(), "out", "file.jar")

	// downloadAlways creates parent dirs; downloadAttempt does not.
	err := downloadAlways(context.Background(), dlTask{url: srv.URL, dest: dest})
	if err != nil {
		t.Fatalf("downloadAttempt: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(body) {
		t.Errorf("content = %q, want %q", got, body)
	}
	if _, err := os.Stat(dest + ".part"); !os.IsNotExist(err) {
		t.Error(".part temp file should be removed after rename")
	}
}

func TestDownloadAttemptSHA1Mismatch(t *testing.T) {
	t.Parallel()
	body := []byte("actual-bytes")
	srv, _ := staticServer(t, body, nil)
	dest := filepath.Join(t.TempDir(), "file.bin")

	wrong := sha1Hex([]byte("different"))
	err := downloadAlways(context.Background(), dlTask{url: srv.URL, dest: dest, sha1: wrong})
	if err == nil {
		t.Fatal("expected sha1 mismatch error")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Error("destination must not exist after verification failure")
	}
	if _, err := os.Stat(dest + ".part"); !os.IsNotExist(err) {
		t.Error(".part must be cleaned up after failure")
	}
}

func TestDownloadAttemptSizeMismatch(t *testing.T) {
	t.Parallel()
	body := []byte("short")
	srv, _ := staticServer(t, body, nil)
	dest := filepath.Join(t.TempDir(), "file.bin")

	err := downloadAttempt(context.Background(), dlTask{url: srv.URL, dest: dest, size: int64(len(body) + 99)})
	if err == nil {
		t.Fatal("expected size mismatch error")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Error("destination must not exist after size failure")
	}
}

func TestDownloadAttemptEmptyBody(t *testing.T) {
	t.Parallel()
	srv, _ := staticServer(t, nil, nil) // 0-byte response
	dest := filepath.Join(t.TempDir(), "file.bin")

	err := downloadAttempt(context.Background(), dlTask{url: srv.URL, dest: dest})
	if err == nil {
		t.Fatal("expected empty-file error")
	}
}

func TestDownloadAttemptSendsUserAgent(t *testing.T) {
	t.Parallel()
	var gotUA atomic.Value
	srv, _ := staticServer(t, []byte("x"), func(w http.ResponseWriter, r *http.Request) {
		gotUA.Store(r.Header.Get("User-Agent"))
		_, _ = w.Write([]byte("x"))
	})
	dest := filepath.Join(t.TempDir(), "file.bin")

	if err := downloadAttempt(context.Background(), dlTask{url: srv.URL, dest: dest, userAgent: "WaiLauncher/0.1.0"}); err != nil {
		t.Fatal(err)
	}
	if ua, _ := gotUA.Load().(string); ua != "WaiLauncher/0.1.0" {
		t.Errorf("server saw UA %q, want WaiLauncher/0.1.0", ua)
	}
}

func TestDownloadAlwaysRetriesThenSucceeds(t *testing.T) {
	t.Parallel()
	var n atomic.Int32
	srv, hits := staticServer(t, []byte("ok"), func(w http.ResponseWriter, r *http.Request) {
		if n.Add(1) < 2 { // first attempt fails
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte("ok"))
	})
	dest := filepath.Join(t.TempDir(), "file.bin")

	if err := downloadAlways(context.Background(), dlTask{url: srv.URL, dest: dest}); err != nil {
		t.Fatalf("retry did not recover: %v", err)
	}
	if hits.Load() != 2 {
		t.Errorf("server hits = %d, want 2 (one fail + one success)", hits.Load())
	}
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("dest missing after retry: %v", err)
	}
}

func TestDownloadAlwaysGivesUpAfterThreeAttempts(t *testing.T) {
	t.Parallel()
	srv, hits := staticServer(t, nil, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	dest := filepath.Join(t.TempDir(), "file.bin")

	start := time.Now()
	err := downloadAlways(context.Background(), dlTask{url: srv.URL, dest: dest})
	if err == nil {
		t.Fatal("expected persistent-failure error")
	}
	if hits.Load() != 3 {
		t.Errorf("server hits = %d, want 3 attempts", hits.Load())
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Error("dest must not exist when all attempts fail")
	}
	_ = start
}

func TestDownloadAlwaysContextCancel(t *testing.T) {
	t.Parallel()
	srv, _ := staticServer(t, nil, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled
	dest := filepath.Join(t.TempDir(), "file.bin")

	err := downloadAlways(ctx, dlTask{url: srv.URL, dest: dest})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context canceled", err)
	}
}

func TestDownloadOneSkipsWhenValidExists(t *testing.T) {
	t.Parallel()
	body := []byte("already here")
	sum := sha1Hex(body)
	dest := filepath.Join(t.TempDir(), "file.bin")
	if err := os.WriteFile(dest, body, 0o644); err != nil {
		t.Fatal(err)
	}

	// Server would 500, but downloadOne must not even call it (cache hit).
	srv, hits := staticServer(t, nil, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	if err := downloadOne(context.Background(), dlTask{url: srv.URL, dest: dest, sha1: sum, size: int64(len(body))}); err != nil {
		t.Fatalf("downloadOne: %v", err)
	}
	if hits.Load() != 0 {
		t.Errorf("server was hit %d times despite a valid cached file", hits.Load())
	}
}

func TestDownloadOneUsesProg(t *testing.T) {
	t.Parallel()
	srv, _ := staticServer(t, []byte("abc"), nil)
	dest := filepath.Join(t.TempDir(), "file.bin")

	var last int64
	err := downloadOne(context.Background(), dlTask{
		url:  srv.URL,
		dest: dest,
		prog: func(written int64) { last = written },
	})
	if err != nil {
		t.Fatal(err)
	}
	if last == 0 {
		t.Error("progress callback never reported bytes")
	}
}

func TestDownloadAllContinuesPastFailure(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	goodSrv, _ := staticServer(t, []byte("good-content"), nil)
	badSrv, _ := staticServer(t, []byte("oops"), func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	good := filepath.Join(dir, "good.bin")
	bad := filepath.Join(dir, "bad.bin")
	tasks := []dlTask{
		{url: goodSrv.URL, dest: good},
		{url: badSrv.URL, dest: bad},
	}

	err := downloadAll(context.Background(), tasks, 1, nil)
	if err == nil {
		t.Fatal("downloadAll should report the failure")
	}
	if _, err := os.Stat(good); err != nil {
		t.Error("good task should still have downloaded")
	}
	if _, err := os.Stat(bad); !os.IsNotExist(err) {
		t.Error("failed task should not produce a file")
	}
}
