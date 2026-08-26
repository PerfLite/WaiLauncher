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

// ModrinthDependency represents a dependency requirement of a version.
type ModrinthDependency struct {
	VersionID      *string `json:"version_id"`
	ProjectID      *string `json:"project_id"`
	FileName       *string `json:"file_name"`
	DependencyType string  `json:"dependency_type"` // "required", "optional", "incompatible", "embedded"
}

// ModrinthProject represents Modrinth project details.
type ModrinthProject struct {
	ID          string   `json:"id"`
	Slug        string   `json:"slug"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	IconURL     string   `json:"icon_url"`
	Categories  []string `json:"categories"`
}

// ResolvedDependency represents a resolved dependency ready for installation.
type ResolvedDependency struct {
	ProjectID        string `json:"projectId"`
	ProjectSlug      string `json:"projectSlug"`
	ProjectTitle     string `json:"projectTitle"`
	IconURL          string `json:"iconUrl"`
	DependencyType   string `json:"dependencyType"` // "required" | "optional"
	VersionID        string `json:"versionId"`
	VersionNumber    string `json:"versionNumber"`
	FileName         string `json:"fileName"`
	DownloadURL      string `json:"downloadUrl"`
	AlreadyInstalled bool   `json:"alreadyInstalled"`
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
	Dependencies  []ModrinthDependency  `json:"dependencies"`
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

// GetModrinthVersion fetches a single version by ID.
func (l *Launcher) GetModrinthVersion(ctx context.Context, versionID string) (*ModrinthVersion, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", modrinthAPI+"/version/"+url.PathEscape(versionID), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "WaiLauncher/1.0.0 (contact@wailauncher.local)")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("modrinth version request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("modrinth version returned %d", resp.StatusCode)
	}

	var v ModrinthVersion
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return nil, fmt.Errorf("decode modrinth version: %w", err)
	}
	return &v, nil
}

// GetModrinthProject fetches a single project by ID or slug.
func (l *Launcher) GetModrinthProject(ctx context.Context, projectIDOrSlug string) (*ModrinthProject, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", modrinthAPI+"/project/"+url.PathEscape(projectIDOrSlug), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "WaiLauncher/1.0.0 (contact@wailauncher.local)")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("modrinth project request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("modrinth project returned %d", resp.StatusCode)
	}

	var p ModrinthProject
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return nil, fmt.Errorf("decode modrinth project: %w", err)
	}
	return &p, nil
}

// ResolveModDependencies resolves all required and optional dependencies of a mod version.
func (l *Launcher) ResolveModDependencies(ctx context.Context, versionID, loader, mcVersion string, installedFilenames []string) ([]ResolvedDependency, error) {
	ver, err := l.GetModrinthVersion(ctx, versionID)
	if err != nil {
		return nil, err
	}
	if len(ver.Dependencies) == 0 {
		return nil, nil
	}

	isInstalled := func(slug, title string) bool {
		sLow := strings.ToLower(slug)
		tLow := strings.ToLower(strings.ReplaceAll(title, " ", "-"))
		for _, fn := range installedFilenames {
			fnLow := strings.ToLower(fn)
			if strings.Contains(fnLow, sLow) || strings.Contains(fnLow, tLow) {
				return true
			}
		}
		return false
	}

	var results []ResolvedDependency
	seenProjects := make(map[string]bool)

	for _, dep := range ver.Dependencies {
		if dep.DependencyType != "required" && dep.DependencyType != "optional" {
			continue
		}

		var projectID string
		var chosenVersion *ModrinthVersion
		var depProj *ModrinthProject

		if dep.VersionID != nil && *dep.VersionID != "" {
			if dv, err := l.GetModrinthVersion(ctx, *dep.VersionID); err == nil && dv != nil {
				chosenVersion = dv
				projectID = dv.ProjectID
			}
		} else if dep.ProjectID != nil && *dep.ProjectID != "" {
			projectID = *dep.ProjectID
		}

		if projectID == "" || seenProjects[projectID] {
			continue
		}
		seenProjects[projectID] = true

		if dp, err := l.GetModrinthProject(ctx, projectID); err == nil && dp != nil {
			depProj = dp
		} else {
			continue
		}

		if chosenVersion == nil {
			// Find latest compatible version
			vers, err := l.GetModrinthProjectVersions(ctx, projectID, loader, mcVersion)
			if err == nil && len(vers) > 0 {
				chosenVersion = &vers[0]
			}
		}

		if chosenVersion == nil || len(chosenVersion.Files) == 0 {
			continue
		}

		var chosenFile ModrinthVersionFile
		for _, f := range chosenVersion.Files {
			if f.Primary {
				chosenFile = f
				break
			}
		}
		if chosenFile.URL == "" {
			chosenFile = chosenVersion.Files[0]
		}

		alreadyInstalled := isInstalled(depProj.Slug, depProj.Title)

		results = append(results, ResolvedDependency{
			ProjectID:        depProj.ID,
			ProjectSlug:      depProj.Slug,
			ProjectTitle:     depProj.Title,
			IconURL:          depProj.IconURL,
			DependencyType:   dep.DependencyType,
			VersionID:        chosenVersion.ID,
			VersionNumber:    chosenVersion.VersionNum,
			FileName:         chosenFile.Filename,
			DownloadURL:      chosenFile.URL,
			AlreadyInstalled: alreadyInstalled,
		})
	}

	return results, nil
}

