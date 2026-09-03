package launcher

import (
	"archive/zip"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const curseForgeAPI = "https://api.curseforge.com/v1"
const curseForgeKey = "$2a$10$wuAJuNZuted3NORVmpgUC.m8sI.pv1tOPKZyBgLFGjxFp/br0lZCC"
const ftbAPI = "https://api.feed-the-beast.com/v1/modpacks/public/modpack"

// ModpackItem is a unified representation of a modpack from Modrinth, CurseForge or FTB.
type ModpackItem struct {
	ID           string   `json:"id"`
	Source       string   `json:"source"` // "modrinth" | "curseforge" | "ftb"
	Title        string   `json:"title"`
	Slug         string   `json:"slug"`
	Author       string   `json:"author"`
	Description  string   `json:"description"`
	IconURL      string   `json:"icon_url"`
	BannerURL    string   `json:"banner_url"`
	Downloads    int64    `json:"downloads"`
	Follows      int64    `json:"follows"`
	Categories   []string `json:"categories"`
	GameVersions []string `json:"game_versions"`
	Loaders      []string `json:"loaders"`
	DateModified string   `json:"date_modified"`
}

// ModpackVersionItem represents a downloadable release version of a modpack.
type ModpackVersionItem struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	VersionNumber string   `json:"version_number"`
	GameVersions  []string `json:"game_versions"`
	Loaders       []string `json:"loaders"`
	DownloadURL   string   `json:"download_url"`
	FileName      string   `json:"file_name"`
	FileSize      int64    `json:"file_size"`
	DatePublished string   `json:"date_published"`
	Changelog     string   `json:"changelog"`
}

// ModpackDetails contains full info and versions for a modpack.
type ModpackDetails struct {
	Item     ModpackItem          `json:"item"`
	Versions []ModpackVersionItem `json:"versions"`
	Body     string               `json:"body"`
}

// ModpackProgress is sent during modpack installation.
type ModpackProgress struct {
	Stage   string  `json:"stage"` // "downloading" | "extracting" | "mods" | "done" | "error"
	Percent float64 `json:"percent"`
	Message string  `json:"message"`
	Current int     `json:"current"`
	Total   int     `json:"total"`
}

// SearchModpacks searches Modrinth, CurseForge or FTB for modpacks.
func (l *Launcher) SearchModpacks(ctx context.Context, source, query, mcVersion, loader string, offset, limit int) ([]ModpackItem, error) {
	if limit <= 0 {
		limit = 30
	}
	source = strings.ToLower(source)
	if source == "curseforge" {
		return l.searchCurseForgeModpacks(ctx, query, mcVersion, loader, offset, limit)
	}
	if source == "ftb" {
		return l.searchFTBModpacks(ctx, query, mcVersion, loader, offset, limit)
	}
	return l.searchModrinthModpacks(ctx, query, mcVersion, loader, offset, limit)
}

// searchModrinthModpacks searches Modrinth modpacks.
func (l *Launcher) searchModrinthModpacks(ctx context.Context, query, mcVersion, loader string, offset, limit int) ([]ModpackItem, error) {
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
	facets = append(facets, []string{"project_type:modpack"})
	if loader != "" && loader != "all" {
		ld := strings.ToLower(loader)
		if ld == "neoforge" {
			facets = append(facets, []string{"categories:neoforge", "categories:forge"})
		} else {
			facets = append(facets, []string{"categories:" + ld})
		}
	}
	if mcVersion != "" && mcVersion != "all" {
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

	var items []ModpackItem
	for _, hit := range res.Hits {
		banner := ""
		if len(hit.Gallery) > 0 {
			banner = hit.Gallery[0]
		}
		items = append(items, ModpackItem{
			ID:           hit.ProjectID,
			Source:       "modrinth",
			Title:        hit.Title,
			Slug:         hit.Slug,
			Author:       hit.Author,
			Description:  hit.Description,
			IconURL:      hit.IconURL,
			BannerURL:    banner,
			Downloads:    int64(hit.Downloads),
			Follows:      int64(hit.Follows),
			Categories:   hit.Categories,
			GameVersions: hit.Versions,
			DateModified: hit.DateModified,
		})
	}
	return items, nil
}

// searchCurseForgeModpacks searches CurseForge for modpacks (classId=4471).
func (l *Launcher) searchCurseForgeModpacks(ctx context.Context, query, mcVersion, loader string, offset, limit int) ([]ModpackItem, error) {
	u, err := url.Parse(curseForgeAPI + "/mods/search")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("gameId", "432")     // Minecraft
	q.Set("classId", "4471")    // Modpacks
	q.Set("sortField", "2")     // Popularity / Total Downloads
	q.Set("sortOrder", "desc")
	q.Set("pageSize", fmt.Sprintf("%d", limit))
	q.Set("index", fmt.Sprintf("%d", offset))

	if strings.TrimSpace(query) != "" {
		q.Set("searchFilter", strings.TrimSpace(query))
	}
	if mcVersion != "" && mcVersion != "all" {
		q.Set("gameVersion", mcVersion)
	}
	if loader != "" && loader != "all" {
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

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", curseForgeKey)
	req.Header.Set("User-Agent", "WaiLauncher/0.1.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("curseforge search request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("curseforge returned %d: %s", resp.StatusCode, string(b))
	}

	var cfRes struct {
		Data []struct {
			ID          int    `json:"id"`
			Name        string `json:"name"`
			Slug        string `json:"slug"`
			Summary     string `json:"summary"`
			DownloadCount float64 `json:"downloadCount"`
			Logo        struct {
				URL          string `json:"url"`
				ThumbnailURL string `json:"thumbnailUrl"`
			} `json:"logo"`
			Authors []struct {
				Name string `json:"name"`
			} `json:"authors"`
			LatestFilesIndexes []struct {
				GameVersion   string `json:"gameVersion"`
				ModLoader     int    `json:"modLoader"`
			} `json:"latestFilesIndexes"`
			DateModified string `json:"dateModified"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&cfRes); err != nil {
		return nil, fmt.Errorf("curseforge decode json: %w", err)
	}

	var items []ModpackItem
	for _, m := range cfRes.Data {
		var author string
		if len(m.Authors) > 0 {
			author = m.Authors[0].Name
		}
		var gVers []string
		verMap := make(map[string]bool)
		for _, fi := range m.LatestFilesIndexes {
			if fi.GameVersion != "" && !verMap[fi.GameVersion] {
				verMap[fi.GameVersion] = true
				gVers = append(gVers, fi.GameVersion)
			}
		}

		icon := m.Logo.ThumbnailURL
		if icon == "" {
			icon = m.Logo.URL
		}

		items = append(items, ModpackItem{
			ID:           fmt.Sprintf("%d", m.ID),
			Source:       "curseforge",
			Title:        m.Name,
			Slug:         m.Slug,
			Author:       author,
			Description:  m.Summary,
			IconURL:      icon,
			BannerURL:    m.Logo.URL,
			Downloads:    int64(m.DownloadCount),
			GameVersions: gVers,
			DateModified: m.DateModified,
		})
	}
	return items, nil
}

// GetModpackDetails fetches full details and versions for a Modrinth, CurseForge or FTB modpack.
func (l *Launcher) GetModpackDetails(ctx context.Context, source, idOrSlug string) (*ModpackDetails, error) {
	source = strings.ToLower(source)
	if source == "curseforge" {
		return l.getCurseForgeModpackDetails(ctx, idOrSlug)
	}
	if source == "ftb" {
		return l.getFTBModpackDetails(ctx, idOrSlug)
	}
	return l.getModrinthModpackDetails(ctx, idOrSlug)
}

func (l *Launcher) getModrinthModpackDetails(ctx context.Context, idOrSlug string) (*ModpackDetails, error) {
	// 1. Fetch project info
	u := fmt.Sprintf("%s/project/%s", modrinthAPI, url.PathEscape(idOrSlug))
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "WaiLauncher/0.1.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("modrinth project status %d", resp.StatusCode)
	}

	var p struct {
		ID          string   `json:"id"`
		Slug        string   `json:"slug"`
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Body        string   `json:"body"`
		IconURL     string   `json:"icon_url"`
		Downloads   int64    `json:"downloads"`
		Followers   int64    `json:"followers"`
		Categories  []string `json:"categories"`
		GameVersions []string `json:"game_versions"`
		Loaders     []string `json:"loaders"`
		Updated     string   `json:"updated"`
		Gallery     []struct {
			URL string `json:"url"`
		} `json:"gallery"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return nil, err
	}

	banner := ""
	if len(p.Gallery) > 0 {
		banner = p.Gallery[0].URL
	}

	// 2. Fetch versions
	vUrl := fmt.Sprintf("%s/project/%s/version", modrinthAPI, url.PathEscape(p.ID))
	vReq, err := http.NewRequestWithContext(ctx, "GET", vUrl, nil)
	if err != nil {
		return nil, err
	}
	vReq.Header.Set("User-Agent", "WaiLauncher/0.1.0")

	vResp, err := httpClient.Do(vReq)
	if err != nil {
		return nil, err
	}
	defer vResp.Body.Close()

	var versions []ModrinthVersion
	if err := json.NewDecoder(vResp.Body).Decode(&versions); err != nil {
		return nil, err
	}

	var verItems []ModpackVersionItem
	for _, v := range versions {
		var dlURL, fName string
		var fSize int64
		for _, f := range v.Files {
			if f.Primary || dlURL == "" {
				dlURL = f.URL
				fName = f.Filename
				fSize = f.Size
			}
		}
		verItems = append(verItems, ModpackVersionItem{
			ID:            v.ID,
			Name:          v.Name,
			VersionNumber: v.VersionNum,
			GameVersions:  v.GameVersions,
			Loaders:       v.Loaders,
			DownloadURL:   dlURL,
			FileName:      fName,
			FileSize:      fSize,
			DatePublished: v.DatePublished,
		})
	}

	return &ModpackDetails{
		Item: ModpackItem{
			ID:           p.ID,
			Source:       "modrinth",
			Title:        p.Title,
			Slug:         p.Slug,
			Description:  p.Description,
			IconURL:      p.IconURL,
			BannerURL:    banner,
			Downloads:    p.Downloads,
			Follows:      p.Followers,
			Categories:   p.Categories,
			GameVersions: p.GameVersions,
			Loaders:      p.Loaders,
			DateModified: p.Updated,
		},
		Versions: verItems,
		Body:     p.Body,
	}, nil
}

func (l *Launcher) getCurseForgeModpackDetails(ctx context.Context, idStr string) (*ModpackDetails, error) {
	// 1. Fetch mod info
	u := fmt.Sprintf("%s/mods/%s", curseForgeAPI, url.PathEscape(idStr))
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", curseForgeKey)
	req.Header.Set("User-Agent", "WaiLauncher/0.1.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("curseforge mod status %d", resp.StatusCode)
	}

	var mRes struct {
		Data struct {
			ID          int    `json:"id"`
			Name        string `json:"name"`
			Slug        string `json:"slug"`
			Summary     string `json:"summary"`
			DownloadCount float64 `json:"downloadCount"`
			Logo        struct {
				URL          string `json:"url"`
				ThumbnailURL string `json:"thumbnailUrl"`
			} `json:"logo"`
			Authors []struct {
				Name string `json:"name"`
			} `json:"authors"`
			DateModified string `json:"dateModified"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&mRes); err != nil {
		return nil, err
	}

	// 2. Fetch files (versions)
	fUrl := fmt.Sprintf("%s/mods/%s/files?pageSize=50", curseForgeAPI, url.PathEscape(idStr))
	fReq, err := http.NewRequestWithContext(ctx, "GET", fUrl, nil)
	if err != nil {
		return nil, err
	}
	fReq.Header.Set("x-api-key", curseForgeKey)
	fReq.Header.Set("User-Agent", "WaiLauncher/0.1.0")

	fResp, err := httpClient.Do(fReq)
	if err != nil {
		return nil, err
	}
	defer fResp.Body.Close()

	var filesRes struct {
		Data []struct {
			ID           int      `json:"id"`
			DisplayName  string   `json:"displayName"`
			FileName     string   `json:"fileName"`
			FileDate     string   `json:"fileDate"`
			FileLength   int64    `json:"fileLength"`
			DownloadURL  string   `json:"downloadUrl"`
			GameVersions []string `json:"gameVersions"`
		} `json:"data"`
	}
	if err := json.NewDecoder(fResp.Body).Decode(&filesRes); err != nil {
		return nil, err
	}

	var verItems []ModpackVersionItem
	for _, f := range filesRes.Data {
		dl := f.DownloadURL
		if dl == "" {
			dl = fmt.Sprintf("https://edge.forgecdn.net/files/%d/%d/%s", f.ID/1000, f.ID%1000, url.PathEscape(f.FileName))
		}
		verItems = append(verItems, ModpackVersionItem{
			ID:            fmt.Sprintf("%d", f.ID),
			Name:          f.DisplayName,
			VersionNumber: f.DisplayName,
			GameVersions:  f.GameVersions,
			DownloadURL:   dl,
			FileName:      f.FileName,
			FileSize:      f.FileLength,
			DatePublished: f.FileDate,
		})
	}

	var author string
	if len(mRes.Data.Authors) > 0 {
		author = mRes.Data.Authors[0].Name
	}

	return &ModpackDetails{
		Item: ModpackItem{
			ID:           fmt.Sprintf("%d", mRes.Data.ID),
			Source:       "curseforge",
			Title:        mRes.Data.Name,
			Slug:         mRes.Data.Slug,
			Author:       author,
			Description:  mRes.Data.Summary,
			IconURL:      mRes.Data.Logo.ThumbnailURL,
			BannerURL:    mRes.Data.Logo.URL,
			Downloads:    int64(mRes.Data.DownloadCount),
			DateModified: mRes.Data.DateModified,
		},
		Versions: verItems,
	}, nil
}

// ModrinthIndexManifest defines the structure of modrinth.index.json inside .mrpack.
type ModrinthIndexManifest struct {
	FormatVersion int    `json:"formatVersion"`
	Game          string `json:"game"`
	VersionID     string `json:"versionId"`
	Name          string `json:"name"`
	Summary       string `json:"summary"`
	Files         []struct {
		Path      string            `json:"path"`
		Hashes    map[string]string `json:"hashes"`
		Env       map[string]string `json:"env"`
		Downloads []string          `json:"downloads"`
		FileSize  int64             `json:"fileSize"`
	} `json:"files"`
	Dependencies map[string]string `json:"dependencies"`
}

// CurseForgeManifest defines the manifest.json structure inside CurseForge modpack zip.
type CurseForgeManifest struct {
	Minecraft struct {
		Version    string `json:"version"`
		ModLoaders []struct {
			ID      string `json:"id"`
			Primary bool   `json:"primary"`
		} `json:"modLoaders"`
	} `json:"minecraft"`
	ManifestType    string `json:"manifestType"`
	ManifestVersion int    `json:"manifestVersion"`
	Name            string `json:"name"`
	Version         string `json:"version"`
	Author          string `json:"author"`
	Files           []struct {
		ProjectID int  `json:"projectID"`
		FileID    int  `json:"fileID"`
		Required  bool `json:"required"`
	} `json:"files"`
	Overrides string `json:"overrides"`
}

// InstalledPackInfo contains metadata of the created instance.
type InstalledPackInfo struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	VersionID     string `json:"versionId"`
	Loader        string `json:"loader"`
	LoaderVersion string `json:"loaderVersion"`
}

// InstallModpackFromURL downloads and installs a Modrinth (.mrpack), CurseForge or FTB modpack.
func (l *Launcher) InstallModpackFromURL(
	ctx context.Context,
	source, downloadURL, customName string,
	progressFn func(ModpackProgress),
) (*InstalledPackInfo, error) {
	if progressFn == nil {
		progressFn = func(p ModpackProgress) {}
	}

	if strings.ToLower(source) == "ftb" || strings.Contains(downloadURL, "api.feed-the-beast.com") {
		return l.installFTBPack(ctx, downloadURL, customName, progressFn)
	}

	progressFn(ModpackProgress{Stage: "downloading", Percent: 0.05, Message: "Скачивание архива сборки…"})

	// 1. Download pack to temp file
	tmpPack, err := os.CreateTemp("", "modpack-*.zip")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmpPack.Name())
	defer tmpPack.Close()

	req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return nil, err
	}
	if strings.ToLower(source) == "curseforge" {
		req.Header.Set("x-api-key", curseForgeKey)
	}
	req.Header.Set("User-Agent", "WaiLauncher/0.1.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download modpack: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download modpack failed with status %d", resp.StatusCode)
	}

	if _, err := io.Copy(tmpPack, resp.Body); err != nil {
		return nil, fmt.Errorf("write temp pack: %w", err)
	}
	tmpPack.Close()

	progressFn(ModpackProgress{Stage: "extracting", Percent: 0.20, Message: "Чтение манифеста сборки…"})

	zr, err := zip.OpenReader(tmpPack.Name())
	if err != nil {
		return nil, fmt.Errorf("open zip pack: %w", err)
	}
	defer zr.Close()

	// Check if Modrinth (.mrpack with modrinth.index.json) or CurseForge (manifest.json)
	var mrIndex *ModrinthIndexManifest
	var cfManifest *CurseForgeManifest

	for _, f := range zr.File {
		if f.Name == "modrinth.index.json" {
			rc, err := f.Open()
			if err == nil {
				var idx ModrinthIndexManifest
				if json.NewDecoder(rc).Decode(&idx) == nil {
					mrIndex = &idx
				}
				rc.Close()
			}
		} else if f.Name == "manifest.json" {
			rc, err := f.Open()
			if err == nil {
				var mf CurseForgeManifest
				if json.NewDecoder(rc).Decode(&mf) == nil {
					cfManifest = &mf
				}
				rc.Close()
			}
		}
	}

	if mrIndex == nil && cfManifest == nil {
		return nil, fmt.Errorf("не удалось распознать формат модпака (отсутствует modrinth.index.json или manifest.json)")
	}

	if mrIndex != nil {
		return l.installModrinthPack(ctx, zr, mrIndex, customName, progressFn)
	}
	return l.installCurseForgePack(ctx, zr, cfManifest, customName, progressFn)
}

// installModrinthPack handles Modrinth .mrpack installation.
func (l *Launcher) installModrinthPack(
	ctx context.Context,
	zr *zip.ReadCloser,
	mr *ModrinthIndexManifest,
	customName string,
	progressFn func(ModpackProgress),
) (*InstalledPackInfo, error) {
	packName := mr.Name
	if customName != "" {
		packName = customName
	}
	mcVer := mr.Dependencies["minecraft"]
	if mcVer == "" {
		mcVer = mr.VersionID
	}

	loader := "vanilla"
	loaderVer := ""
	if v, ok := mr.Dependencies["fabric-loader"]; ok && v != "" {
		loader = "fabric"
		loaderVer = v
	} else if v, ok := mr.Dependencies["neoforge"]; ok && v != "" {
		loader = "neoforge"
		loaderVer = v
	} else if v, ok := mr.Dependencies["forge"]; ok && v != "" {
		loader = "forge"
		loaderVer = v
	} else if v, ok := mr.Dependencies["quilt-loader"]; ok && v != "" {
		loader = "fabric"
		loaderVer = v
	}

	// Create Instance ID
	instID := sanitizeID(packName)
	instDir := l.InstanceDir(instID)
	if _, err := os.Stat(instDir); err == nil {
		instID = fmt.Sprintf("%s-%d", instID, time.Now().Unix()%10000)
		instDir = l.InstanceDir(instID)
	}

	if err := os.MkdirAll(instDir, 0755); err != nil {
		return nil, fmt.Errorf("create instance dir: %w", err)
	}
	os.MkdirAll(filepath.Join(instDir, "mods"), 0755)

	// Extract overrides
	progressFn(ModpackProgress{Stage: "extracting", Percent: 0.30, Message: "Распаковка конфигураций и файлов…"})
	for _, f := range zr.File {
		cleanName := filepath.Clean(f.Name)
		var relPath string
		if strings.HasPrefix(cleanName, "overrides/") || strings.HasPrefix(cleanName, "overrides\\") {
			relPath = cleanName[10:]
		} else if strings.HasPrefix(cleanName, "client-overrides/") || strings.HasPrefix(cleanName, "client-overrides\\") {
			relPath = cleanName[17:]
		}
		if relPath != "" {
			target := filepath.Join(instDir, relPath)
			if f.FileInfo().IsDir() {
				os.MkdirAll(target, 0755)
			} else {
				os.MkdirAll(filepath.Dir(target), 0755)
				rc, err := f.Open()
				if err == nil {
					out, err := os.Create(target)
					if err == nil {
						io.Copy(out, rc)
						out.Close()
					}
					rc.Close()
				}
			}
		} else if f.Name == "icon.png" || f.Name == "logo.png" {
			rc, err := f.Open()
			if err == nil {
				target := filepath.Join(instDir, "icon.png")
				out, err := os.Create(target)
				if err == nil {
					io.Copy(out, rc)
					out.Close()
				}
				rc.Close()
			}
		}
	}

	// Filter client files
	type downloadItem struct {
		url    string
		path   string
		sha1   string
	}
	var downloads []downloadItem
	for _, f := range mr.Files {
		if f.Env != nil && f.Env["client"] == "unsupported" {
			continue
		}
		if len(f.Downloads) == 0 {
			continue
		}
		h := ""
		if f.Hashes != nil {
			h = f.Hashes["sha1"]
		}
		downloads = append(downloads, downloadItem{
			url:  f.Downloads[0],
			path: filepath.Join(instDir, f.Path),
			sha1: h,
		})
	}

	total := len(downloads)
	var completed int32

	// Concurrently download mods with worker pool of 6
	progressFn(ModpackProgress{
		Stage:   "mods",
		Percent: 0.35,
		Message: fmt.Sprintf("Загрузка модов (0/%d)…", total),
		Current: 0,
		Total:   total,
	})

	workChan := make(chan downloadItem, len(downloads))
	for _, d := range downloads {
		workChan <- d
	}
	close(workChan)

	numWorkers := 6
	var wg sync.WaitGroup
	var downloadErr error
	var errOnce sync.Once

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range workChan {
				if ctx.Err() != nil {
					return
				}
				os.MkdirAll(filepath.Dir(item.path), 0755)
				err := downloadModWithSha1(ctx, item.url, item.path, item.sha1)
				if err != nil {
					errOnce.Do(func() {
						downloadErr = fmt.Errorf("failed download %s: %w", item.url, err)
					})
				}
				done := atomic.AddInt32(&completed, 1)
				pct := 0.35 + (float64(done)/float64(total))*0.60
				progressFn(ModpackProgress{
					Stage:   "mods",
					Percent: pct,
					Message: fmt.Sprintf("Загрузка модов (%d/%d)…", done, total),
					Current: int(done),
					Total:   total,
				})
			}
		}()
	}
	wg.Wait()

	if downloadErr != nil {
		return nil, downloadErr
	}

	// Finalize instance
	progressFn(ModpackProgress{Stage: "done", Percent: 1.0, Message: "Сборка успешно установлена!"})
	return &InstalledPackInfo{
		ID:            instID,
		Name:          packName,
		VersionID:     mcVer,
		Loader:        loader,
		LoaderVersion: loaderVer,
	}, nil
}

// installCurseForgePack handles CurseForge modpack zip installation.
func (l *Launcher) installCurseForgePack(
	ctx context.Context,
	zr *zip.ReadCloser,
	cf *CurseForgeManifest,
	customName string,
	progressFn func(ModpackProgress),
) (*InstalledPackInfo, error) {
	packName := cf.Name
	if customName != "" {
		packName = customName
	}
	mcVer := cf.Minecraft.Version

	loader := "vanilla"
	loaderVer := ""
	if len(cf.Minecraft.ModLoaders) > 0 {
		ldID := cf.Minecraft.ModLoaders[0].ID
		parts := strings.SplitN(ldID, "-", 2)
		if len(parts) == 2 {
			loader = strings.ToLower(parts[0])
			loaderVer = parts[1]
		}
	}

	instID := sanitizeID(packName)
	instDir := l.InstanceDir(instID)
	if _, err := os.Stat(instDir); err == nil {
		instID = fmt.Sprintf("%s-%d", instID, time.Now().Unix()%10000)
		instDir = l.InstanceDir(instID)
	}

	if err := os.MkdirAll(instDir, 0755); err != nil {
		return nil, fmt.Errorf("create instance dir: %w", err)
	}
	os.MkdirAll(filepath.Join(instDir, "mods"), 0755)

	// Extract overrides
	progressFn(ModpackProgress{Stage: "extracting", Percent: 0.30, Message: "Распаковка настроек и конфигураций…"})
	overridesDir := cf.Overrides
	if overridesDir == "" {
		overridesDir = "overrides"
	}
	overridesPrefix := overridesDir + "/"
	overridesPrefixBS := overridesDir + "\\"

	for _, f := range zr.File {
		cleanName := filepath.Clean(f.Name)
		var relPath string
		if strings.HasPrefix(cleanName, overridesPrefix) || strings.HasPrefix(cleanName, overridesPrefixBS) {
			relPath = cleanName[len(overridesDir)+1:]
		}
		if relPath != "" {
			target := filepath.Join(instDir, relPath)
			if f.FileInfo().IsDir() {
				os.MkdirAll(target, 0755)
			} else {
				os.MkdirAll(filepath.Dir(target), 0755)
				rc, err := f.Open()
				if err == nil {
					out, err := os.Create(target)
					if err == nil {
						io.Copy(out, rc)
						out.Close()
					}
					rc.Close()
				}
			}
		}
	}

	// Collect all project IDs to batch fetch class IDs (mods vs resource packs vs shaders)
	var projectIDs []int
	for _, f := range cf.Files {
		projectIDs = append(projectIDs, f.ProjectID)
	}
	classMap := l.fetchCurseForgeProjectClasses(ctx, projectIDs)

	// Fetch file URLs from CurseForge API and download
	total := len(cf.Files)
	var completed int32

	progressFn(ModpackProgress{
		Stage:   "mods",
		Percent: 0.35,
		Message: fmt.Sprintf("Загрузка модов CurseForge (0/%d)…", total),
		Current: 0,
		Total:   total,
	})

	type cfDownloadTask struct {
		projectID int
		fileID    int
	}
	workChan := make(chan cfDownloadTask, len(cf.Files))
	for _, f := range cf.Files {
		workChan <- cfDownloadTask{projectID: f.ProjectID, fileID: f.FileID}
	}
	close(workChan)

	numWorkers := 6
	var wg sync.WaitGroup

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range workChan {
				if ctx.Err() != nil {
					return
				}
				// Query file info
				fileInfoURL := fmt.Sprintf("%s/mods/%d/files/%d", curseForgeAPI, task.projectID, task.fileID)
				req, err := http.NewRequestWithContext(ctx, "GET", fileInfoURL, nil)
				if err == nil {
					req.Header.Set("x-api-key", curseForgeKey)
					req.Header.Set("User-Agent", "PrismLauncher/9.2")
					resp, err := httpClient.Do(req)
					if err == nil {
						var fileRes struct {
							Data struct {
								FileName    string `json:"fileName"`
								DownloadURL string `json:"downloadUrl"`
							} `json:"data"`
						}
						if json.NewDecoder(resp.Body).Decode(&fileRes) == nil {
							resp.Body.Close()
							fileName := fileRes.Data.FileName
							dlURL := fileRes.Data.DownloadURL
							if dlURL == "" && fileName != "" {
								dlURL = fmt.Sprintf("https://edge.forgecdn.net/files/%d/%d/%s", task.fileID/1000, task.fileID%1000, url.PathEscape(fileName))
							}
							if dlURL != "" && fileName != "" {
								folder := determineCurseForgeFolder(classMap[task.projectID], fileName)
								dest := filepath.Join(instDir, folder, fileName)
								_ = os.MkdirAll(filepath.Dir(dest), 0755)
								_ = downloadModWithSha1(ctx, dlURL, dest, "")
							}
						} else {
							resp.Body.Close()
						}
					}
				}
				done := atomic.AddInt32(&completed, 1)
				pct := 0.35 + (float64(done)/float64(total))*0.60
				progressFn(ModpackProgress{
					Stage:   "mods",
					Percent: pct,
					Message: fmt.Sprintf("Загрузка модов CurseForge (%d/%d)…", done, total),
					Current: int(done),
					Total:   total,
				})
			}
		}()
	}
	wg.Wait()

	// Organize any remaining misplaced zip files
	OrganizeInstanceFolders(instDir)

	progressFn(ModpackProgress{Stage: "done", Percent: 1.0, Message: "Сборка CurseForge успешно установлена!"})
	return &InstalledPackInfo{
		ID:            instID,
		Name:          packName,
		VersionID:     mcVer,
		Loader:        loader,
		LoaderVersion: loaderVer,
	}, nil
}

// fetchCurseForgeProjectClasses batch queries CurseForge API for project class IDs.
func (l *Launcher) fetchCurseForgeProjectClasses(ctx context.Context, projectIDs []int) map[int]int {
	classMap := make(map[int]int)
	if len(projectIDs) == 0 {
		return classMap
	}
	for i := 0; i < len(projectIDs); i += 50 {
		end := i + 50
		if end > len(projectIDs) {
			end = len(projectIDs)
		}
		chunk := projectIDs[i:end]
		reqBody, _ := json.Marshal(map[string][]int{"modIds": chunk})
		req, err := http.NewRequestWithContext(ctx, "POST", curseForgeAPI+"/mods", strings.NewReader(string(reqBody)))
		if err != nil {
			continue
		}
		req.Header.Set("x-api-key", curseForgeKey)
		req.Header.Set("User-Agent", "PrismLauncher/9.2")
		req.Header.Set("Content-Type", "application/json")
		resp, err := httpClient.Do(req)
		if err != nil {
			continue
		}
		var res struct {
			Data []struct {
				ID      int `json:"id"`
				ClassID int `json:"classId"`
			} `json:"data"`
		}
		if json.NewDecoder(resp.Body).Decode(&res) == nil {
			for _, m := range res.Data {
				classMap[m.ID] = m.ClassID
			}
		}
		resp.Body.Close()
	}
	return classMap
}

func determineCurseForgeFolder(classID int, fileName string) string {
	lower := strings.ToLower(fileName)
	switch classID {
	case 12:
		return "resourcepacks"
	case 6552, 6945, 4546:
		return "shaderpacks"
	case 17:
		return "saves"
	case 6:
		if strings.HasSuffix(lower, ".zip") {
			if strings.Contains(lower, "shader") || strings.Contains(lower, "complementary") || strings.Contains(lower, "bsl") || strings.Contains(lower, "euphoria") {
				return "shaderpacks"
			}
			if strings.Contains(lower, "resource") || strings.Contains(lower, "texture") || strings.Contains(lower, "translation") || strings.Contains(lower, "zh_") || strings.Contains(lower, "ru_") {
				return "resourcepacks"
			}
			if strings.Contains(lower, "datapack") || strings.Contains(lower, "data_") {
				return "datapacks"
			}
		}
		return "mods"
	default:
		if strings.HasSuffix(lower, ".jar") {
			return "mods"
		}
		if strings.Contains(lower, "shader") || strings.Contains(lower, "complementary") || strings.Contains(lower, "bsl") || strings.Contains(lower, "euphoria") {
			return "shaderpacks"
		}
		if strings.Contains(lower, "datapack") {
			return "datapacks"
		}
		return "resourcepacks"
	}
}

// OrganizeInstanceFolders inspects instDir/mods and moves resource packs, shader packs,
// and datapacks that were saved as .zip files to their respective subdirectories.
func OrganizeInstanceFolders(instDir string) {
	modsDir := filepath.Join(instDir, "mods")
	entries, err := os.ReadDir(modsDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		lower := strings.ToLower(name)
		if strings.HasSuffix(lower, ".zip") {
			var destFolder string
			if strings.Contains(lower, "shader") || strings.Contains(lower, "complementary") || strings.Contains(lower, "bsl") || strings.Contains(lower, "euphoria") || strings.Contains(lower, "unbound") || strings.Contains(lower, "reimagined") {
				destFolder = "shaderpacks"
			} else if strings.Contains(lower, "datapack") || strings.Contains(lower, "data_") {
				destFolder = "datapacks"
			} else {
				destFolder = "resourcepacks"
			}
			targetDir := filepath.Join(instDir, destFolder)
			_ = os.MkdirAll(targetDir, 0755)
			_ = os.Rename(filepath.Join(modsDir, name), filepath.Join(targetDir, name))
		}
	}
}

func downloadModWithSha1(ctx context.Context, dlURL, destPath, expectedSha1 string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", dlURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "WaiLauncher/0.1.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}

	tmpFile := destPath + ".tmp"
	f, err := os.Create(tmpFile)
	if err != nil {
		return err
	}

	hasher := sha1.New()
	w := io.MultiWriter(f, hasher)
	if _, err := io.Copy(w, resp.Body); err != nil {
		f.Close()
		os.Remove(tmpFile)
		return err
	}
	f.Close()

	if expectedSha1 != "" {
		actualSha1 := hex.EncodeToString(hasher.Sum(nil))
		if !strings.EqualFold(actualSha1, expectedSha1) {
			os.Remove(tmpFile)
			return fmt.Errorf("sha1 mismatch: expected %s got %s", expectedSha1, actualSha1)
		}
	}

	return os.Rename(tmpFile, destPath)
}

func sanitizeID(name string) string {
	reg := regexp.MustCompile(`[^a-zA-Z0-9_\-]+`)
	s := reg.ReplaceAllString(strings.TrimSpace(name), "_")
	s = strings.ToLower(strings.Trim(s, "_"))
	if s == "" {
		s = fmt.Sprintf("pack-%d", time.Now().Unix()%10000)
	}
	return s
}

// searchFTBModpacks searches the Feed The Beast public API.
func (l *Launcher) searchFTBModpacks(ctx context.Context, query, mcVersion, loader string, offset, limit int) ([]ModpackItem, error) {
	if limit <= 0 {
		limit = 25
	}
	trimmedQuery := strings.TrimSpace(query)
	var searchURL string
	if trimmedQuery == "" {
		searchURL = fmt.Sprintf("%s/popular/installs/%d", ftbAPI, limit)
	} else {
		searchURL = fmt.Sprintf("%s/search/%d?term=%s", ftbAPI, limit, url.QueryEscape(trimmedQuery))
	}

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "WaiLauncher/0.1.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("FTB search returned %d", resp.StatusCode)
	}

	var res struct {
		Packs []int `json:"packs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	if len(res.Packs) == 0 {
		return nil, nil
	}

	type ftbPackDetails struct {
		ID          int    `json:"id"`
		Name        string `json:"name"`
		Slug        string `json:"slug"`
		Synopsis    string `json:"synopsis"`
		Description string `json:"description"`
		Art         []struct {
			Type string `json:"type"`
			URL  string `json:"url"`
		} `json:"art"`
		Authors []struct {
			Name string `json:"name"`
		} `json:"authors"`
		Installs int64 `json:"installs"`
		Plays    int64 `json:"plays"`
		Targets  []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
			Type    string `json:"type"`
		} `json:"targets"`
		Updated int64 `json:"updated"`
	}

	items := make([]ModpackItem, len(res.Packs))
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i, pid := range res.Packs {
		wg.Add(1)
		go func(idx, packID int) {
			defer wg.Done()
			pReq, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/%d", ftbAPI, packID), nil)
			if err != nil {
				return
			}
			pReq.Header.Set("User-Agent", "WaiLauncher/0.1.0")
			pResp, err := httpClient.Do(pReq)
			if err != nil {
				return
			}
			defer pResp.Body.Close()
			if pResp.StatusCode != http.StatusOK {
				return
			}

			var pd ftbPackDetails
			if err := json.NewDecoder(pResp.Body).Decode(&pd); err != nil {
				return
			}

			var iconURL, bannerURL string
			for _, a := range pd.Art {
				if a.Type == "square" && iconURL == "" {
					iconURL = a.URL
				} else if a.Type == "logo" && iconURL == "" {
					iconURL = a.URL
				} else if a.Type == "splash" && bannerURL == "" {
					bannerURL = a.URL
				}
			}
			if iconURL == "" && len(pd.Art) > 0 {
				iconURL = pd.Art[0].URL
			}

			var author string
			if len(pd.Authors) > 0 {
				author = pd.Authors[0].Name
			}

			var gVers []string
			for _, t := range pd.Targets {
				if t.Name == "minecraft" && t.Version != "" {
					gVers = append(gVers, t.Version)
				}
			}

			downloads := pd.Installs
			if pd.Plays > downloads {
				downloads = pd.Plays
			}

			desc := pd.Synopsis
			if desc == "" {
				desc = pd.Description
				if len(desc) > 200 {
					desc = desc[:200] + "..."
				}
			}

			mu.Lock()
			items[idx] = ModpackItem{
				ID:           fmt.Sprintf("%d", pd.ID),
				Source:       "ftb",
				Title:        pd.Name,
				Slug:         pd.Slug,
				Author:       author,
				Description:  desc,
				IconURL:      iconURL,
				BannerURL:    bannerURL,
				Downloads:    downloads,
				GameVersions: gVers,
				DateModified: time.Unix(pd.Updated, 0).Format(time.RFC3339),
			}
			mu.Unlock()
		}(i, pid)
	}
	wg.Wait()

	var result []ModpackItem
	for _, it := range items {
		if it.ID != "" {
			if mcVersion != "" && mcVersion != "all" {
				match := false
				for _, v := range it.GameVersions {
					if strings.HasPrefix(v, mcVersion) {
						match = true
						break
					}
				}
				if !match && len(it.GameVersions) > 0 {
					continue
				}
			}
			result = append(result, it)
		}
	}
	return result, nil
}

// getFTBModpackDetails fetches full metadata and available versions for an FTB pack.
func (l *Launcher) getFTBModpackDetails(ctx context.Context, idOrSlug string) (*ModpackDetails, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/%s", ftbAPI, url.PathEscape(idOrSlug)), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "WaiLauncher/0.1.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("FTB modpack status %d", resp.StatusCode)
	}

	var p struct {
		ID          int    `json:"id"`
		Name        string `json:"name"`
		Slug        string `json:"slug"`
		Synopsis    string `json:"synopsis"`
		Description string `json:"description"`
		Art         []struct {
			Type string `json:"type"`
			URL  string `json:"url"`
		} `json:"art"`
		Authors []struct {
			Name string `json:"name"`
		} `json:"authors"`
		Installs int64 `json:"installs"`
		Plays    int64 `json:"plays"`
		Targets  []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"targets"`
		Versions []struct {
			ID      int    `json:"id"`
			Name    string `json:"name"`
			Type    string `json:"type"`
			Updated int64  `json:"updated"`
		} `json:"versions"`
		Updated int64 `json:"updated"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return nil, err
	}

	var iconURL, bannerURL string
	for _, a := range p.Art {
		if a.Type == "square" && iconURL == "" {
			iconURL = a.URL
		} else if a.Type == "logo" && iconURL == "" {
			iconURL = a.URL
		} else if a.Type == "splash" && bannerURL == "" {
			bannerURL = a.URL
		}
	}
	if iconURL == "" && len(p.Art) > 0 {
		iconURL = p.Art[0].URL
	}

	var author string
	if len(p.Authors) > 0 {
		author = p.Authors[0].Name
	}

	var gVers []string
	for _, t := range p.Targets {
		if t.Name == "minecraft" && t.Version != "" {
			gVers = append(gVers, t.Version)
		}
	}

	downloads := p.Installs
	if p.Plays > downloads {
		downloads = p.Plays
	}

	var verItems []ModpackVersionItem
	for _, v := range p.Versions {
		dlURL := fmt.Sprintf("%s/%d/%d", ftbAPI, p.ID, v.ID)
		verItems = append(verItems, ModpackVersionItem{
			ID:            fmt.Sprintf("%d", v.ID),
			Name:          v.Name,
			VersionNumber: v.Name,
			GameVersions:  gVers,
			DatePublished: time.Unix(v.Updated, 0).Format(time.RFC3339),
			DownloadURL:   dlURL,
			FileName:      fmt.Sprintf("%s-%s.json", p.Slug, v.Name),
		})
	}

	// FTB API returns versions in chronological ascending order (oldest first).
	// Reverse so that the newest release is first (index 0).
	for i, j := 0, len(verItems)-1; i < j; i, j = i+1, j-1 {
		verItems[i], verItems[j] = verItems[j], verItems[i]
	}

	return &ModpackDetails{
		Item: ModpackItem{
			ID:           fmt.Sprintf("%d", p.ID),
			Source:       "ftb",
			Title:        p.Name,
			Slug:         p.Slug,
			Author:       author,
			Description:  p.Synopsis,
			IconURL:      iconURL,
			BannerURL:    bannerURL,
			Downloads:    downloads,
			GameVersions: gVers,
			DateModified: time.Unix(p.Updated, 0).Format(time.RFC3339),
		},
		Versions: verItems,
		Body:     p.Description,
	}, nil
}

// installFTBPack downloads and installs an FTB modpack from its version manifest URL.
func (l *Launcher) installFTBPack(
	ctx context.Context,
	manifestURL, customName string,
	progressFn func(ModpackProgress),
) (*InstalledPackInfo, error) {
	progressFn(ModpackProgress{Stage: "downloading", Percent: 0.05, Message: "Загрузка манифеста FTB…"})

	req, err := http.NewRequestWithContext(ctx, "GET", manifestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "WaiLauncher/0.1.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch FTB manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("FTB manifest returned %d", resp.StatusCode)
	}

	var m struct {
		ID      int    `json:"id"`
		Name    string `json:"name"`
		Parent  int    `json:"parent"`
		Targets []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
			Type    string `json:"type"`
		} `json:"targets"`
		Files []struct {
			ID         int64  `json:"id"`
			Name       string `json:"name"`
			Path       string `json:"path"`
			URL        string `json:"url"`
			SHA1       string `json:"sha1"`
			Size       int64  `json:"size"`
			ServerOnly bool   `json:"serveronly"`
		} `json:"files"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, fmt.Errorf("decode FTB manifest: %w", err)
	}

	packName := m.Name
	if customName != "" {
		packName = customName
	}
	if packName == "" {
		packName = fmt.Sprintf("FTB Pack %d", m.Parent)
	}

	mcVer := "1.20.1"
	loader := "forge"
	loaderVer := ""

	for _, t := range m.Targets {
		switch strings.ToLower(t.Name) {
		case "minecraft":
			mcVer = t.Version
		case "forge":
			loader = "forge"
			loaderVer = t.Version
		case "neoforge":
			loader = "neoforge"
			loaderVer = t.Version
		case "fabric":
			loader = "fabric"
			loaderVer = t.Version
		}
	}

	instID := fmt.Sprintf("ftb_%d_%s", time.Now().UnixNano()%1000000, sanitizeID(packName))
	instDir := l.InstanceDir(instID)
	if err := os.MkdirAll(instDir, 0755); err != nil {
		return nil, err
	}

	var clientFiles []struct {
		ID         int64  `json:"id"`
		Name       string `json:"name"`
		Path       string `json:"path"`
		URL        string `json:"url"`
		SHA1       string `json:"sha1"`
		Size       int64  `json:"size"`
		ServerOnly bool   `json:"serveronly"`
	}
	for _, f := range m.Files {
		if !f.ServerOnly && f.URL != "" {
			clientFiles = append(clientFiles, f)
		}
	}

	total := len(clientFiles)
	var downloaded int64
	var mu sync.Mutex

	concurrency := 12
	if total < concurrency {
		concurrency = max(1, total)
	}
	jobs := make(chan int, total)
	for i := range clientFiles {
		jobs <- i
	}
	close(jobs)

	var wg sync.WaitGroup

	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				f := clientFiles[idx]
				relPath := filepath.Clean(strings.TrimPrefix(f.Path, "./"))
				destDir := filepath.Join(instDir, relPath)
				_ = os.MkdirAll(destDir, 0755)
				destFile := filepath.Join(destDir, f.Name)

				_ = downloadModWithSha1(ctx, f.URL, destFile, f.SHA1)

				mu.Lock()
				downloaded++
				cur := int(downloaded)
				pct := 0.15 + (float64(cur)/float64(max(1, total)))*0.80
				mu.Unlock()

				progressFn(ModpackProgress{
					Stage:   "mods",
					Percent: pct,
					Message: fmt.Sprintf("Загрузка файлов (%d/%d): %s", cur, total, f.Name),
					Current: cur,
					Total:   total,
				})
			}
		}()
	}
	wg.Wait()

	progressFn(ModpackProgress{Stage: "done", Percent: 1.0, Message: "Установка сборки FTB завершена!"})

	return &InstalledPackInfo{
		ID:            instID,
		Name:          packName,
		VersionID:     mcVer,
		Loader:        loader,
		LoaderVersion: loaderVer,
	}, nil
}

