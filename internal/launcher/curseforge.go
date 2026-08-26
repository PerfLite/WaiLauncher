package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// CurseForgeMod represents a mod on CurseForge.
type CurseForgeMod struct {
	ID            int      `json:"id"`
	GameID        int      `json:"gameId"`
	Name          string   `json:"name"`
	Slug          string   `json:"slug"`
	Summary       string   `json:"summary"`
	DownloadCount float64  `json:"downloadCount"`
	ThumbsUpCount int      `json:"thumbsUpCount"`
	Logo          struct {
		ID           int    `json:"id"`
		ThumbnailURL string `json:"thumbnailUrl"`
		URL          string `json:"url"`
	} `json:"logo"`
	Authors []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"authors"`
	Categories []struct {
		ID      int    `json:"id"`
		Name    string `json:"name"`
		Slug    string `json:"slug"`
		IconURL string `json:"iconUrl"`
	} `json:"categories"`
	Screenshots []struct {
		ID  int    `json:"id"`
		URL string `json:"url"`
	} `json:"screenshots"`
	LatestFilesIndexes []struct {
		GameVersion string `json:"gameVersion"`
		FileID      int    `json:"fileId"`
		Filename    string `json:"filename"`
		ReleaseType int    `json:"releaseType"`
		ModLoader   int    `json:"modLoader"`
	} `json:"latestFilesIndexes"`
	DateModified string `json:"dateModified"`
}

// CurseForgeFile represents a downloadable release file on CurseForge.
type CurseForgeFile struct {
	ID           int      `json:"id"`
	GameID       int      `json:"gameId"`
	ModID        int      `json:"modId"`
	IsAvailable  bool     `json:"isAvailable"`
	DisplayName  string   `json:"displayName"`
	FileName     string   `json:"fileName"`
	ReleaseType  int      `json:"releaseType"`
	FileLength   int64    `json:"fileLength"`
	DownloadURL  string   `json:"downloadUrl"`
	GameVersions []string `json:"gameVersions"`
	Dependencies []struct {
		ModID        int `json:"modId"`
		RelationType int `json:"relationType"` // 1: optional, 2: embedded, 3: required
	} `json:"dependencies"`
}

// SearchCurseForge searches CurseForge for mods, resourcepacks, or shaders and formats results as ModrinthHit.
func (l *Launcher) SearchCurseForge(ctx context.Context, query, projectType, loader, mcVersion string, offset, limit int) (*ModrinthSearchResponse, error) {
	if limit <= 0 {
		limit = 40
	}

	classID := 6 // Default: Mods
	switch strings.ToLower(projectType) {
	case "resourcepack":
		classID = 12
	case "shader":
		classID = 6552
	case "modpack":
		classID = 4471
	}

	u, err := url.Parse(curseForgeAPI + "/mods/search")
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("gameId", "432")
	q.Set("classId", fmt.Sprintf("%d", classID))
	q.Set("pageSize", fmt.Sprintf("%d", limit))
	q.Set("index", fmt.Sprintf("%d", offset))
	q.Set("sortOrder", "desc")

	trimmedQuery := strings.TrimSpace(query)
	if trimmedQuery != "" {
		q.Set("searchFilter", trimmedQuery)
		q.Set("sortField", "0") // Relevance
	} else {
		q.Set("sortField", "2") // Popularity (downloads)
	}

	if mcVersion != "" && mcVersion != "all" {
		q.Set("gameVersion", mcVersion)
	}

	if classID == 6 && loader != "" && loader != "all" && loader != "vanilla" {
		switch strings.ToLower(loader) {
		case "forge":
			q.Set("modLoaderType", "1")
		case "fabric":
			q.Set("modLoaderType", "4")
		case "quilt":
			q.Set("modLoaderType", "5")
		case "neoforge":
			q.Set("modLoaderType", "6")
		}
	}

	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", curseForgeKey)
	req.Header.Set("User-Agent", "WaiLauncher/1.0.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("curseforge search request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("curseforge search returned %d: %s", resp.StatusCode, string(b))
	}

	var cfRes struct {
		Data       []CurseForgeMod `json:"data"`
		Pagination struct {
			Index      int `json:"index"`
			PageSize   int `json:"pageSize"`
			TotalCount int `json:"totalCount"`
		} `json:"pagination"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&cfRes); err != nil {
		return nil, fmt.Errorf("curseforge decode json: %w", err)
	}

	var hits []ModrinthHit
	for _, m := range cfRes.Data {
		author := ""
		if len(m.Authors) > 0 {
			author = m.Authors[0].Name
		}
		icon := m.Logo.ThumbnailURL
		if icon == "" {
			icon = m.Logo.URL
		}

		var cats []string
		for _, c := range m.Categories {
			cats = append(cats, c.Name)
		}

		var vers []string
		seenVers := make(map[string]bool)
		for _, fi := range m.LatestFilesIndexes {
			if fi.GameVersion != "" && !seenVers[fi.GameVersion] {
				seenVers[fi.GameVersion] = true
				vers = append(vers, fi.GameVersion)
			}
		}

		var gallery []string
		for _, s := range m.Screenshots {
			if s.URL != "" {
				gallery = append(gallery, s.URL)
			}
		}

		hits = append(hits, ModrinthHit{
			ProjectID:    fmt.Sprintf("%d", m.ID),
			Slug:         m.Slug,
			Title:        m.Name,
			Description:  m.Summary,
			Categories:   cats,
			ClientSide:   "optional",
			ServerSide:   "optional",
			IconURL:      icon,
			Author:       author,
			Downloads:    int(m.DownloadCount),
			Follows:      m.ThumbsUpCount,
			Versions:     vers,
			Gallery:      gallery,
			DateModified: m.DateModified,
		})
	}

	return &ModrinthSearchResponse{
		Hits:      hits,
		Offset:    cfRes.Pagination.Index,
		Limit:     cfRes.Pagination.PageSize,
		TotalHits: cfRes.Pagination.TotalCount,
	}, nil
}

// GetCurseForgeModFiles fetches files for a specific CurseForge mod, optionally filtered.
func (l *Launcher) GetCurseForgeModFiles(ctx context.Context, modID int, loader, mcVersion string) ([]CurseForgeFile, error) {
	u, err := url.Parse(fmt.Sprintf("%s/mods/%d/files", curseForgeAPI, modID))
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("pageSize", "50")
	if mcVersion != "" && mcVersion != "all" {
		q.Set("gameVersion", mcVersion)
	}
	if loader != "" && loader != "all" && loader != "vanilla" {
		switch strings.ToLower(loader) {
		case "forge":
			q.Set("modLoaderType", "1")
		case "fabric":
			q.Set("modLoaderType", "4")
		case "quilt":
			q.Set("modLoaderType", "5")
		case "neoforge":
			q.Set("modLoaderType", "6")
		}
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", curseForgeKey)
	req.Header.Set("User-Agent", "WaiLauncher/1.0.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("curseforge files request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("curseforge files returned %d: %s", resp.StatusCode, string(b))
	}

	var fileRes struct {
		Data []CurseForgeFile `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&fileRes); err != nil {
		return nil, fmt.Errorf("curseforge decode files: %w", err)
	}

	// Ensure download URLs are populated
	for i := range fileRes.Data {
		f := &fileRes.Data[i]
		if f.DownloadURL == "" && f.ID > 0 && f.FileName != "" {
			f.DownloadURL = fmt.Sprintf("https://edge.forgecdn.net/files/%d/%d/%s", f.ID/1000, f.ID%1000, url.PathEscape(f.FileName))
		}
	}

	return fileRes.Data, nil
}

// GetCurseForgeModDetails fetches details of a CurseForge mod by ID.
func (l *Launcher) GetCurseForgeModDetails(ctx context.Context, modID int) (*CurseForgeMod, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/mods/%d", curseForgeAPI, modID), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", curseForgeKey)
	req.Header.Set("User-Agent", "WaiLauncher/1.0.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("curseforge mod details request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("curseforge mod details returned %d", resp.StatusCode)
	}

	var res struct {
		Data CurseForgeMod `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("curseforge decode mod details: %w", err)
	}

	return &res.Data, nil
}

// ResolveCurseForgeDependencies resolves required and optional dependencies for a CurseForge file.
func (l *Launcher) ResolveCurseForgeDependencies(ctx context.Context, modID int, file *CurseForgeFile, loader, mcVersion string, installedFilenames []string) ([]ResolvedDependency, error) {
	if file == nil || len(file.Dependencies) == 0 {
		return nil, nil
	}

	isInstalled := func(slug, title string) bool {
		sLow := strings.ToLower(slug)
		tLow := strings.ToLower(strings.ReplaceAll(title, " ", "-"))
		for _, fn := range installedFilenames {
			fnLow := strings.ToLower(fn)
			if (sLow != "" && strings.Contains(fnLow, sLow)) || (tLow != "" && strings.Contains(fnLow, tLow)) {
				return true
			}
		}
		return false
	}

	var results []ResolvedDependency
	seen := make(map[int]bool)

	for _, dep := range file.Dependencies {
		// relationType: 3 = Required, 1 = Optional
		if dep.RelationType != 3 && dep.RelationType != 1 {
			continue
		}
		if dep.ModID == 0 || seen[dep.ModID] {
			continue
		}
		seen[dep.ModID] = true

		depMod, err := l.GetCurseForgeModDetails(ctx, dep.ModID)
		if err != nil || depMod == nil {
			continue
		}

		files, err := l.GetCurseForgeModFiles(ctx, dep.ModID, loader, mcVersion)
		if err != nil || len(files) == 0 {
			continue
		}

		chosenFile := files[0]
		depType := "required"
		if dep.RelationType == 1 {
			depType = "optional"
		}

		icon := depMod.Logo.ThumbnailURL
		if icon == "" {
			icon = depMod.Logo.URL
		}

		results = append(results, ResolvedDependency{
			ProjectID:        fmt.Sprintf("%d", depMod.ID),
			ProjectSlug:      depMod.Slug,
			ProjectTitle:     depMod.Name,
			IconURL:          icon,
			DependencyType:   depType,
			VersionID:        fmt.Sprintf("%d", chosenFile.ID),
			VersionNumber:    chosenFile.DisplayName,
			FileName:         chosenFile.FileName,
			DownloadURL:      chosenFile.DownloadURL,
			AlreadyInstalled: isInstalled(depMod.Slug, depMod.Name),
		})
	}

	return results, nil
}

// ParseCurseForgeID parses a string as an integer mod ID.
func ParseCurseForgeID(str string) (int, error) {
	return strconv.Atoi(strings.TrimSpace(str))
}
