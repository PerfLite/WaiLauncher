package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const modrinthAPI = "https://api.modrinth.com/v2"

// ModrinthHit represents a mod search result from Modrinth API.
type ModrinthHit struct {
	ProjectID    string   `json:"project_id"`
	Slug         string   `json:"slug"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	Categories   []string `json:"categories"`
	ClientSide   string   `json:"client_side"`
	ServerSide   string   `json:"server_side"`
	IconURL      string   `json:"icon_url"`
	Author       string   `json:"author"`
	Downloads    int      `json:"downloads"`
	Follows      int      `json:"follows"`
	Versions     []string `json:"versions"`
	Gallery      []string `json:"gallery"`
	DateModified string   `json:"date_modified"`
}

// ModrinthSearchResponse is the response structure of Modrinth search endpoint.
type ModrinthSearchResponse struct {
	Hits      []ModrinthHit `json:"hits"`
	Offset    int           `json:"offset"`
	Limit     int           `json:"limit"`
	TotalHits int           `json:"total_hits"`
}

// ModrinthVersionFile is one file inside a Modrinth version release.
type ModrinthVersionFile struct {
	URL      string            `json:"url"`
	Filename string            `json:"filename"`
	Primary  bool              `json:"primary"`
	Size     int64             `json:"size"`
	Hashes   map[string]string `json:"hashes"`
}

// ModrinthVersion represents a release version of a project on Modrinth.
type ModrinthVersion struct {
	ID            string                `json:"id"`
	ProjectID     string                `json:"project_id"`
	Name          string                `json:"name"`
	VersionNum    string                `json:"version_number"`
	GameVersions  []string              `json:"game_versions"`
	Loaders       []string              `json:"loaders"`
	Files         []ModrinthVersionFile `json:"files"`
	DatePublished string                `json:"date_published"`
}

// ModItem represents an installed mod in an instance's mods/ directory.
type ModItem struct {
	Filename string `json:"filename"`
	Name     string `json:"name"`
	Enabled  bool   `json:"enabled"`
	Size     int64  `json:"size"`
	ModTime  int64  `json:"modTime"`
}

// SearchModrinth queries the Modrinth v2 API for projects matching query, projectType, loader, and mcVersion.
func (l *Launcher) SearchModrinth(ctx context.Context, query, projectType, loader, mcVersion string, offset, limit int) (*ModrinthSearchResponse, error) {
	if limit <= 0 {
		limit = 50
	}
	if projectType == "" {
		projectType = "mod"
	}
	u, err := url.Parse(modrinthAPI + "/search")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	trimmedQuery := strings.TrimSpace(query)
	q.Set("query", trimmedQuery)
	q.Set("offset", fmt.Sprintf("%d", offset))
	q.Set("limit", fmt.Sprintf("%d", limit))

	if trimmedQuery == "" {
		q.Set("index", "downloads")
	} else {
		q.Set("index", "relevance")
	}

	var facets [][]string
	facets = append(facets, []string{"project_type:" + projectType})
	if projectType == "mod" && loader != "" && loader != "vanilla" {
		ld := strings.ToLower(loader)
		if ld == "neoforge" {
			facets = append(facets, []string{"categories:neoforge", "categories:forge"})
		} else if ld == "forge" {
			facets = append(facets, []string{"categories:forge", "categories:neoforge"})
		} else {
			facets = append(facets, []string{"categories:" + ld})
		}
	}
	if mcVersion != "" {
		facets = append(facets, []string{"versions:" + mcVersion})
	}

	facetsJSON, err := json.Marshal(facets)
	if err == nil {
		q.Set("facets", string(facetsJSON))
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "WaiLauncher/0.1.0 (contact@wailauncher.local)")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("modrinth search request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("modrinth search returned %d: %s", resp.StatusCode, string(b))
	}

	var res ModrinthSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("modrinth search json: %w", err)
	}
	return &res, nil
}

// GetModrinthProjectVersions fetches release versions of a project filtered by loader and game version.
func (l *Launcher) GetModrinthProjectVersions(ctx context.Context, projectIDOrSlug, loader, mcVersion string) ([]ModrinthVersion, error) {
	fetchForLoader := func(ld string) ([]ModrinthVersion, error) {
		u, err := url.Parse(fmt.Sprintf("%s/project/%s/version", modrinthAPI, url.PathEscape(projectIDOrSlug)))
		if err != nil {
			return nil, err
		}
		q := u.Query()
		if ld != "" && ld != "vanilla" {
			loadersJSON, _ := json.Marshal([]string{strings.ToLower(ld)})
			q.Set("loaders", string(loadersJSON))
		}
		if mcVersion != "" {
			gameVersionsJSON, _ := json.Marshal([]string{mcVersion})
			q.Set("game_versions", string(gameVersionsJSON))
		}
		u.RawQuery = q.Encode()

		req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "WaiLauncher/0.1.0 (contact@wailauncher.local)")

		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("modrinth versions request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("modrinth versions returned %d: %s", resp.StatusCode, string(b))
		}

		var versions []ModrinthVersion
		if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
			return nil, fmt.Errorf("modrinth versions json: %w", err)
		}
		return versions, nil
	}

	vers, err := fetchForLoader(loader)
	if err == nil && len(vers) > 0 {
		return vers, nil
	}
	if strings.ToLower(loader) == "neoforge" {
		if forgeVers, forgeErr := fetchForLoader("forge"); forgeErr == nil && len(forgeVers) > 0 {
			return forgeVers, nil
		}
	}
	return vers, err
}

// DownloadModFile downloads a file from URL to destPath.
func (l *Launcher) DownloadModFile(ctx context.Context, fileURL, destPath string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "GET", fileURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "WaiLauncher/0.1.0 (contact@wailauncher.local)")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download mod file returned %d", resp.StatusCode)
	}

	tmp := destPath + fmt.Sprintf(".tmp-%d", time.Now().UnixNano())
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, resp.Body)
	f.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return copyErr
	}
	return os.Rename(tmp, destPath)
}
