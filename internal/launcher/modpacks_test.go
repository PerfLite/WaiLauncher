package launcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestRetryFailedDownloads(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content := []byte("recovered-mod")
	sum := sha1Hex(content)

	// Good server: serves matching content.
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(content)
	}))
	t.Cleanup(good.Close)

	// Bad server: always fails (retries will exhaust).
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(bad.Close)

	goodDest := filepath.Join(dir, "good.jar")
	badDest := filepath.Join(dir, "bad.jar")

	items := []FailedDownload{
		{URL: good.URL, Dest: goodDest, SHA1: sum, Size: int64(len(content))},
		{URL: bad.URL, Dest: badDest, SHA1: sum},
	}

	still := RetryFailedDownloads(context.Background(), items)

	if len(still) != 1 {
		t.Fatalf("still-failing = %d, want 1 (%+v)", len(still), still)
	}
	if still[0].Dest != badDest {
		t.Errorf("reported failure Dest = %q, want %q", still[0].Dest, badDest)
	}
	// Good download must have landed on disk.
	if !fileValid(goodDest, sum, int64(len(content))) {
		t.Error("recovered mod was not written to disk")
	}
}

func TestSanitizeID(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"My Pack!!":    "my_pack",
		"fabric-api":   "fabric-api",
		"  spaced  ":   "spaced",
		"Trailing___":  "trailing",
		"UPPER_case-1": "upper_case-1",
	}
	for in, want := range cases {
		if got := sanitizeID(in); got != want {
			t.Errorf("sanitizeID(%q) = %q, want %q", in, got, want)
		}
	}
	// All-junk input falls back to a time-based id.
	if got := sanitizeID("!!!"); !strings.HasPrefix(got, "pack-") {
		t.Errorf("sanitizeID(junk) = %q, want prefix \"pack-\"", got)
	}
}

func TestDetermineCurseForgeFolder(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		id   int
		file string
		want string
	}{
		{"resource class", 12, "whatever.zip", "resourcepacks"},
		{"shader class", 6552, "whatever.zip", "shaderpacks"},
		{"world class", 17, "any", "saves"},
		{"mod jar", 6, "ExampleMod.jar", "mods"},
		{"mod zip shader-name", 6, "ComplementaryShaders.zip", "shaderpacks"},
		{"mod zip locale-name", 6, "ru_resources.zip", "resourcepacks"},
		{"mod zip datapack-name", 6, "my_datapack.zip", "datapacks"},
		{"mod zip generic", 6, "RandomMod.zip", "mods"},
		{"default jar", 999, "SomeMod.jar", "mods"},
		{"default shader", 999, "BSL.zip", "shaderpacks"},
		{"default resource", 999, "Faithful.zip", "resourcepacks"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := determineCurseForgeFolder(c.id, c.file); got != c.want {
				t.Errorf("determineCurseForgeFolder(%d, %q) = %q, want %q", c.id, c.file, got, c.want)
			}
		})
	}
}
