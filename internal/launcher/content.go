package launcher

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ContentItem represents an installed mod, resourcepack, shaderpack, or datapack.
type ContentItem struct {
	Filename     string `json:"filename"`
	Name         string `json:"name"`
	Version      string `json:"version"`
	Type         string `json:"type"` // "mod" | "resourcepack" | "shaderpack" | "datapack"
	Enabled      bool   `json:"enabled"`
	Size         int64  `json:"size"`
	ModTime      int64  `json:"modTime"`
	Author       string `json:"author"`
	AuthorAvatar string `json:"authorAvatar"`
	IconURL      string `json:"iconUrl"`
	Sha1         string `json:"sha1"`
	Source       string `json:"source,omitempty"`
	ProjectID    string `json:"projectId,omitempty"`
	HasUpdate    bool   `json:"hasUpdate"`
	UpdateVer    string `json:"updateVer"`
	UpdateURL    string `json:"updateUrl"`
	UpdateFile   string `json:"updateFile"`
}

// ModVersionOption represents a version choice for switching versions of an installed mod.
type ModVersionOption struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	VersionNumber string   `json:"versionNumber"`
	GameVersions  []string `json:"gameVersions"`
	Loaders       []string `json:"loaders"`
	DownloadURL   string   `json:"downloadUrl"`
	Filename      string   `json:"filename"`
	FileSize      int64    `json:"fileSize"`
	DatePublished string   `json:"datePublished"`
	IsCurrent     bool     `json:"isCurrent"`
	VersionType   string   `json:"versionType"` // "release" | "beta" | "alpha"
	Changelog     string   `json:"changelog"`
	IsCompatible  bool     `json:"isCompatible"`
}

// ModVersionChangeInfo contains info and candidate versions for changing a mod's version.
type ModVersionChangeInfo struct {
	Filename       string             `json:"filename"`
	ModName        string             `json:"modName"`
	Source         string             `json:"source"` // "modrinth" | "curseforge"
	CurrentVersion string             `json:"currentVersion"`
	CurrentSha1    string             `json:"currentSha1"`
	ProjectID      string             `json:"projectId"`
	Versions       []ModVersionOption `json:"versions"`
}

// WorldItem represents a Minecraft world save in saves/.
type WorldItem struct {
	Name       string `json:"name"`
	FolderName string `json:"folderName"`
	Size       int64  `json:"size"`
	LastPlayed int64  `json:"lastPlayed"`
	IconBase64 string `json:"iconBase64"`
}

// CachedModMeta stores lightweight metadata on disk without heavy base64 strings.
type CachedModMeta struct {
	Sha1         string `json:"sha1"`
	Title        string `json:"title"`
	IconURL      string `json:"iconUrl,omitempty"` // Web URL or empty if stored in local icon file
	Author       string `json:"author"`
	AuthorAvatar string `json:"authorAvatar,omitempty"`
	Version      string `json:"version"`
	HasLocalIcon bool   `json:"hasLocalIcon,omitempty"`
	Source       string `json:"source,omitempty"`
	ProjectID    string `json:"projectId,omitempty"`
}

var (
	metaMemCache = make(map[string]CachedModMeta)
	metaCacheMu  sync.RWMutex
)

// GetInstanceAllContent scans mods/, resourcepacks/, shaderpacks/, and datapacks/.
func (l *Launcher) GetInstanceAllContent(instDir string) []ContentItem {
	OrganizeInstanceFolders(instDir)

	var items []ContentItem

	// 1. Scan folders
	scanFolder := func(subDir, itemType string, validExts []string) {
		dirPath := filepath.Join(instDir, subDir)
		_ = os.MkdirAll(dirPath, 0755)
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			return
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			lower := strings.ToLower(name)

			isValid := false
			for _, ext := range validExts {
				if strings.HasSuffix(lower, ext) || strings.HasSuffix(lower, ext+".disabled") {
					isValid = true
					break
				}
			}
			if !isValid {
				continue
			}

			info, err := e.Info()
			if err != nil {
				continue
			}

			enabled := !strings.HasSuffix(lower, ".disabled")
			cleanName := strings.TrimSuffix(name, ".disabled")
			for _, ext := range validExts {
				cleanName = strings.TrimSuffix(cleanName, ext)
			}

			parsedName, parsedVer := parseModNameAndVersion(cleanName)

			items = append(items, ContentItem{
				Filename: name,
				Name:     parsedName,
				Version:  parsedVer,
				Type:     itemType,
				Enabled:  enabled,
				Size:     info.Size(),
				ModTime:  info.ModTime().Unix(),
				Author:   "Uploaded",
			})
		}
	}

	scanFolder("mods", "mod", []string{".jar"})
	scanFolder("resourcepacks", "resourcepack", []string{".zip"})
	scanFolder("shaderpacks", "shaderpack", []string{".zip"})
	scanFolder("datapacks", "datapack", []string{".zip"})

	// Enrich mods with real icons, titles, and authors
	l.enrichModsMetadata(instDir, items)

	sort.Slice(items, func(i, j int) bool {
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})

	return items
}

func (l *Launcher) enrichModsMetadata(instDir string, items []ContentItem) {
	cacheFile := filepath.Join(instDir, ".wailauncher_mod_cache.json")
	iconsDir := filepath.Join(instDir, ".wailauncher_icons")
	_ = os.MkdirAll(iconsDir, 0755)

	diskCache := loadModCache(cacheFile)

	metaCacheMu.Lock()
	for k, v := range diskCache {
		metaMemCache[k] = v
	}
	metaCacheMu.Unlock()

	var needModrinthLookup []*ContentItem
	modsDir := filepath.Join(instDir, "mods")

	for i := range items {
		it := &items[i]
		if it.Type != "mod" {
			continue
		}

		metaCacheMu.RLock()
		meta, found := metaMemCache[it.Filename]
		metaCacheMu.RUnlock()

		if found && meta.Title != "" {
			it.Name = meta.Title
			it.Author = meta.Author
			it.AuthorAvatar = meta.AuthorAvatar
			if meta.Version != "" {
				it.Version = meta.Version
			}
			it.Sha1 = meta.Sha1
			it.Source = meta.Source
			it.ProjectID = meta.ProjectID

			if meta.IconURL != "" {
				it.IconURL = meta.IconURL
			} else if meta.HasLocalIcon && meta.Sha1 != "" {
				localIcon := filepath.Join(iconsDir, meta.Sha1+".png")
				if data, err := os.ReadFile(localIcon); err == nil {
					it.IconURL = "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)
				}
			}
			continue
		}

		// 1. Inspect jar internally for mods.toml / fabric.mod.json / icon.png
		jarPath := filepath.Join(modsDir, it.Filename)
		extracted, iconBytes := extractMetadataFromJar(jarPath)
		if extracted.Title != "" {
			it.Name = extracted.Title
		}
		if extracted.Author != "" {
			it.Author = extracted.Author
		}
		if extracted.Version != "" {
			it.Version = extracted.Version
		}

		// Calculate SHA1
		h, _ := calcFileSHA1(jarPath)
		it.Sha1 = h
		extracted.Sha1 = h
		it.Source = extracted.Source
		it.ProjectID = extracted.ProjectID

		// Save local icon PNG if found
		if len(iconBytes) > 0 && h != "" {
			localIcon := filepath.Join(iconsDir, h+".png")
			_ = os.WriteFile(localIcon, iconBytes, 0644)
			extracted.HasLocalIcon = true
			it.IconURL = "data:image/png;base64," + base64.StdEncoding.EncodeToString(iconBytes)
		}

		// Save in cache
		metaCacheMu.Lock()
		metaMemCache[it.Filename] = extracted
		diskCache[it.Filename] = extracted
		metaCacheMu.Unlock()

		// If still missing icon or author avatar, queue for Modrinth online lookup
		if (it.IconURL == "" || it.AuthorAvatar == "") && it.Name != "" {
			needModrinthLookup = append(needModrinthLookup, it)
		}
	}

	saveModCache(cacheFile, diskCache)

	// Quick background batch icon and author avatar resolution
	if len(needModrinthLookup) > 0 {
		go func(toLookup []*ContentItem, cFile string) {
			ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
			defer cancel()

			updated := false
			for _, it := range toLookup {
				iconURL, author, authorAvatar := searchModrinthDetailsByName(ctx, it.Name)
				metaCacheMu.Lock()
				cur := metaMemCache[it.Filename]
				changed := false
				if iconURL != "" && it.IconURL == "" {
					it.IconURL = iconURL
					cur.IconURL = iconURL
					changed = true
				}
				if authorAvatar != "" && it.AuthorAvatar == "" {
					it.AuthorAvatar = authorAvatar
					cur.AuthorAvatar = authorAvatar
					changed = true
				}
				if author != "" && (it.Author == "" || it.Author == "Uploaded") {
					it.Author = author
					cur.Author = author
					changed = true
				}
				if changed {
					metaMemCache[it.Filename] = cur
					diskCache[it.Filename] = cur
					updated = true
				}
				metaCacheMu.Unlock()
			}
			if updated {
				saveModCache(cFile, diskCache)
			}
		}(needModrinthLookup, cacheFile)
	}
}

func extractMetadataFromJar(jarPath string) (CachedModMeta, []byte) {
	var meta CachedModMeta

	r, err := zip.OpenReader(jarPath)
	if err != nil {
		return meta, nil
	}
	defer r.Close()

	var iconData []byte
	var logoPathInJar string

	for _, f := range r.File {
		name := f.Name
		lower := strings.ToLower(name)

		// 1. Forge / NeoForge mods.toml
		if lower == "meta-inf/mods.toml" || lower == "meta-inf/neoforge.mods.toml" {
			if rc, err := f.Open(); err == nil {
				buf, _ := io.ReadAll(rc)
				rc.Close()
				s := string(buf)

				nameRe := regexp.MustCompile(`displayName\s*=\s*"([^"]+)"`)
				if m := nameRe.FindStringSubmatch(s); len(m) > 1 {
					meta.Title = m[1]
				}
				authRe := regexp.MustCompile(`authors\s*=\s*"([^"]+)"`)
				if m := authRe.FindStringSubmatch(s); len(m) > 1 {
					meta.Author = m[1]
				}
				verRe := regexp.MustCompile(`version\s*=\s*"([^"]+)"`)
				if m := verRe.FindStringSubmatch(s); len(m) > 1 && !strings.Contains(m[1], "${") {
					meta.Version = m[1]
				}
				logoRe := regexp.MustCompile(`logoFile\s*=\s*"([^"]+)"`)
				if m := logoRe.FindStringSubmatch(s); len(m) > 1 {
					logoPathInJar = strings.TrimPrefix(m[1], "/")
				}
			}
		}

		// 2. Fabric fabric.mod.json
		if lower == "fabric.mod.json" {
			if rc, err := f.Open(); err == nil {
				var fab struct {
					Name    string `json:"name"`
					Version string `json:"version"`
					Icon    any    `json:"icon"`
					Authors any    `json:"authors"`
				}
				_ = json.NewDecoder(rc).Decode(&fab)
				rc.Close()

				if fab.Name != "" {
					meta.Title = fab.Name
				}
				if fab.Version != "" {
					meta.Version = fab.Version
				}
				if fab.Authors != nil {
					switch a := fab.Authors.(type) {
					case string:
						meta.Author = a
					case []any:
						var auths []string
						for _, item := range a {
							if s, ok := item.(string); ok {
								auths = append(auths, s)
							} else if m, ok := item.(map[string]any); ok {
								if n, ok := m["name"].(string); ok {
									auths = append(auths, n)
								}
							}
						}
						meta.Author = strings.Join(auths, ", ")
					}
				}
				if fab.Icon != nil {
					if s, ok := fab.Icon.(string); ok {
						logoPathInJar = strings.TrimPrefix(s, "/")
					} else if m, ok := fab.Icon.(map[string]any); ok {
						for _, v := range m {
							if s, ok := v.(string); ok {
								logoPathInJar = strings.TrimPrefix(s, "/")
								break
							}
						}
					}
				}
			}
		}

		// 3. Check for root icon.png or logo.png
		if iconData == nil && (lower == "icon.png" || lower == "logo.png" || strings.HasSuffix(lower, "/icon.png")) {
			if rc, err := f.Open(); err == nil {
				data, _ := io.ReadAll(rc)
				rc.Close()
				if len(data) > 0 && len(data) < 500000 {
					iconData = data
				}
			}
		}
	}

	// If logoPathInJar specified, extract that specific file
	if logoPathInJar != "" {
		for _, f := range r.File {
			if strings.EqualFold(strings.TrimPrefix(f.Name, "/"), logoPathInJar) {
				if rc, err := f.Open(); err == nil {
					data, _ := io.ReadAll(rc)
					rc.Close()
					if len(data) > 0 && len(data) < 500000 {
						iconData = data
					}
				}
				break
			}
		}
	}

	return meta, iconData
}

func searchModrinthDetailsByName(ctx context.Context, modName string) (iconURL, author, authorAvatar string) {
	clean := strings.TrimSpace(modName)
	if len(clean) < 3 {
		return "", "", ""
	}
	urlStr := fmt.Sprintf("%s/search?query=%s&limit=1", modrinthAPI, url.QueryEscape(clean))
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return "", "", ""
	}
	req.Header.Set("User-Agent", "WaiLauncher/0.1.0")
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", "", ""
	}
	defer resp.Body.Close()

	var searchRes ModrinthSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchRes); err == nil && len(searchRes.Hits) > 0 {
		hit := searchRes.Hits[0]
		iconURL = hit.IconURL
		author = hit.Author
		if author != "" {
			// Query user avatar
			uReq, _ := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/user/%s", modrinthAPI, url.PathEscape(author)), nil)
			if uReq != nil {
				uReq.Header.Set("User-Agent", "WaiLauncher/0.1.0")
				if uResp, uErr := httpClient.Do(uReq); uErr == nil {
					var userObj struct {
						AvatarURL string `json:"avatar_url"`
					}
					_ = json.NewDecoder(uResp.Body).Decode(&userObj)
					uResp.Body.Close()
					authorAvatar = userObj.AvatarURL
				}
			}
		}
	}
	return iconURL, author, authorAvatar
}

func loadModCache(path string) map[string]CachedModMeta {
	data, err := os.ReadFile(path)
	if err != nil {
		return make(map[string]CachedModMeta)
	}
	var res map[string]CachedModMeta
	if err := json.Unmarshal(data, &res); err != nil {
		return make(map[string]CachedModMeta)
	}
	return res
}

func saveModCache(path string, c map[string]CachedModMeta) {
	data, err := json.Marshal(c)
	if err == nil {
		_ = os.WriteFile(path, data, 0644)
	}
}

// CheckModUpdates queries Modrinth to see if newer versions are available for installed mods.
func (l *Launcher) CheckModUpdates(ctx context.Context, instDir, mcVer, loader string) (map[string]ContentItem, error) {
	modsDir := filepath.Join(instDir, "mods")
	entries, err := os.ReadDir(modsDir)
	if err != nil {
		return nil, err
	}

	type hashResult struct {
		filename string
		hash     string
	}
	var hashes []hashResult
	var hashList []string

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".jar") && !strings.HasSuffix(strings.ToLower(name), ".jar.disabled") {
			continue
		}
		filePath := filepath.Join(modsDir, name)
		h, err := calcFileSHA1(filePath)
		if err == nil && h != "" {
			hashes = append(hashes, hashResult{filename: name, hash: h})
			hashList = append(hashList, h)
		}
	}

	if len(hashList) == 0 {
		return nil, nil
	}

	// Prepare Modrinth update request
	reqPayload := map[string]any{
		"hashes":        hashList,
		"algorithm":     "sha1",
		"loaders":       []string{strings.ToLower(loader)},
		"game_versions": []string{mcVer},
	}
	reqBody, _ := json.Marshal(reqPayload)

	req, err := http.NewRequestWithContext(ctx, "POST", modrinthAPI+"/version_files/update", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "WaiLauncher/0.1.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("modrinth update status: %d", resp.StatusCode)
	}

	var updateRes map[string]ModrinthVersion
	if err := json.NewDecoder(resp.Body).Decode(&updateRes); err != nil {
		return nil, err
	}

	// Map updates back to filenames
	resultMap := make(map[string]ContentItem)
	for _, hr := range hashes {
		if v, ok := updateRes[hr.hash]; ok {
			var dlURL, fName, dlHash string
			for _, f := range v.Files {
				if f.Primary || dlURL == "" {
					dlURL = f.URL
					fName = f.Filename
					if f.Hashes != nil {
						dlHash = f.Hashes["sha1"]
					}
				}
			}
			// Only flag as an update if the target version's file hash differs from the current installed hash!
			if dlURL != "" && dlHash != "" && !strings.EqualFold(dlHash, hr.hash) {
				resultMap[hr.filename] = ContentItem{
					Filename:   hr.filename,
					HasUpdate:  true,
					UpdateVer:  v.VersionNum,
					UpdateURL:  dlURL,
					UpdateFile: fName,
				}
			}
		}
	}

	return resultMap, nil
}

// GetInstanceWorlds lists world saves in saves/ directory.
func (l *Launcher) GetInstanceWorlds(instDir string) []WorldItem {
	savesDir := filepath.Join(instDir, "saves")
	_ = os.MkdirAll(savesDir, 0755)
	entries, err := os.ReadDir(savesDir)
	if err != nil {
		return nil
	}

	var worlds []WorldItem
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		wPath := filepath.Join(savesDir, e.Name())
		levelDat := filepath.Join(wPath, "level.dat")
		if _, err := os.Stat(levelDat); err != nil {
			continue
		}

		info, _ := os.Stat(wPath)
		var lastPlayed int64
		if info != nil {
			lastPlayed = info.ModTime().Unix()
		}

		size := calculateDirSize(wPath)

		var iconBase64 string
		iconPath := filepath.Join(wPath, "icon.png")
		if imgData, err := os.ReadFile(iconPath); err == nil {
			iconBase64 = "data:image/png;base64," + base64.StdEncoding.EncodeToString(imgData)
		}

		worlds = append(worlds, WorldItem{
			Name:       e.Name(),
			FolderName: e.Name(),
			Size:       size,
			LastPlayed: lastPlayed,
			IconBase64: iconBase64,
		})
	}

	sort.Slice(worlds, func(i, j int) bool {
		return worlds[i].LastPlayed > worlds[j].LastPlayed
	})

	return worlds
}

// DeleteInstanceWorld deletes a world directory in saves/.
func (l *Launcher) DeleteInstanceWorld(instDir, folderName string) error {
	wPath := filepath.Join(instDir, "saves", filepath.Base(folderName))
	return os.RemoveAll(wPath)
}

func calcFileSHA1(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hasher := sha1.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func calculateDirSize(path string) int64 {
	var size int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}

func parseModNameAndVersion(raw string) (string, string) {
	s := strings.ReplaceAll(raw, "+", "-")
	s = strings.ReplaceAll(s, "_", "-")
	parts := strings.Split(s, "-")
	if len(parts) <= 1 {
		return raw, ""
	}

	nameParts := []string{}
	verParts := []string{}
	foundVer := false

	for _, p := range parts {
		if !foundVer && containsDigit(p) && (strings.HasPrefix(p, "v") || strings.Contains(p, ".") || len(p) >= 3) {
			foundVer = true
		}
		if foundVer {
			verParts = append(verParts, p)
		} else {
			nameParts = append(nameParts, p)
		}
	}

	if len(nameParts) == 0 {
		return raw, strings.Join(verParts, "-")
	}
	return strings.Join(nameParts, " "), strings.Join(verParts, "-")
}

func containsDigit(s string) bool {
	for _, c := range s {
		if c >= '0' && c <= '9' {
			return true
		}
	}
	return false
}

func cleanModSearchQuery(raw string) string {
	s := strings.TrimSuffix(raw, filepath.Ext(raw))
	s = strings.TrimSuffix(s, ".disabled")
	s = strings.TrimLeft(s, "_!.-#+ ")
	baseName, _ := parseModNameAndVersion(s)
	baseName = strings.TrimSpace(baseName)
	if baseName != "" && len(baseName) >= 2 {
		return baseName
	}
	return strings.TrimSpace(s)
}

// RecordModSource saves the source and project ID for an installed mod.
func (l *Launcher) RecordModSource(instDir, filename, source, projectID string) {
	cacheFile := filepath.Join(instDir, ".wailauncher_mod_cache.json")
	diskCache := loadModCache(cacheFile)
	cur := diskCache[filename]
	cur.Source = source
	cur.ProjectID = projectID
	diskCache[filename] = cur
	saveModCache(cacheFile, diskCache)

	metaCacheMu.Lock()
	m := metaMemCache[filename]
	m.Source = source
	m.ProjectID = projectID
	metaMemCache[filename] = m
	metaCacheMu.Unlock()
}

// GetModAvailableVersions discovers the mod on Modrinth or CurseForge and returns candidate versions.
func (l *Launcher) GetModAvailableVersions(ctx context.Context, instDir, filename, mcVer, loader string) (*ModVersionChangeInfo, error) {
	modsDir := filepath.Join(instDir, "mods")
	jarPath := filepath.Join(modsDir, filename)
	if _, err := os.Stat(jarPath); err != nil {
		if strings.HasSuffix(filename, ".disabled") {
			alt := strings.TrimSuffix(filename, ".disabled")
			if _, aErr := os.Stat(filepath.Join(modsDir, alt)); aErr == nil {
				jarPath = filepath.Join(modsDir, alt)
				filename = alt
			}
		} else {
			alt := filename + ".disabled"
			if _, aErr := os.Stat(filepath.Join(modsDir, alt)); aErr == nil {
				jarPath = filepath.Join(modsDir, alt)
				filename = alt
			}
		}
	}

	h, _ := calcFileSHA1(jarPath)

	cacheFile := filepath.Join(instDir, ".wailauncher_mod_cache.json")
	diskCache := loadModCache(cacheFile)
	meta := diskCache[filename]

	source := meta.Source
	projectID := meta.ProjectID
	modTitle := meta.Title
	if modTitle == "" {
		extracted, _ := extractMetadataFromJar(jarPath)
		if extracted.Title != "" {
			modTitle = extracted.Title
		} else {
			modTitle = strings.TrimSuffix(strings.TrimSuffix(filename, ".disabled"), filepath.Ext(filename))
		}
	}

	// 1. If source/projectID unknown, check Modrinth by hash
	if (source == "" || projectID == "") && h != "" {
		reqPayload := map[string]any{
			"hashes":    []string{h},
			"algorithm": "sha1",
		}
		reqBody, _ := json.Marshal(reqPayload)
		req, err := http.NewRequestWithContext(ctx, "POST", modrinthAPI+"/version_files", bytes.NewReader(reqBody))
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("User-Agent", "WaiLauncher/0.1.0")
			if resp, err := httpClient.Do(req); err == nil {
				if resp.StatusCode == http.StatusOK {
					var mrRes map[string]ModrinthVersion
					if err := json.NewDecoder(resp.Body).Decode(&mrRes); err == nil {
						if mv, ok := mrRes[h]; ok && mv.ProjectID != "" {
							source = "modrinth"
							projectID = mv.ProjectID
						}
					}
				}
				resp.Body.Close()
			}
		}
	}

	// 2. If still unknown, search Modrinth by clean title
	if source == "" || projectID == "" {
		cleanName := cleanModSearchQuery(modTitle)
		if cleanName != "" {
			sr, err := l.SearchModrinth(ctx, cleanName, "mod", "", "", 0, 5)
			if (err != nil || len(sr.Hits) == 0) && cleanName != modTitle {
				sr, err = l.SearchModrinth(ctx, modTitle, "mod", "", "", 0, 5)
			}
			if err == nil && len(sr.Hits) > 0 {
				source = "modrinth"
				projectID = sr.Hits[0].ProjectID
				if modTitle == "" {
					modTitle = sr.Hits[0].Title
				}
			}
		}
	}

	// 3. If still unknown, search CurseForge by clean title
	if source == "" || projectID == "" {
		cleanName := cleanModSearchQuery(modTitle)
		if cleanName != "" {
			sr, err := l.SearchCurseForge(ctx, cleanName, "mod", "", "", 0, 5)
			if (err != nil || len(sr.Hits) == 0) && cleanName != modTitle {
				sr, err = l.SearchCurseForge(ctx, modTitle, "mod", "", "", 0, 5)
			}
			if err == nil && len(sr.Hits) > 0 {
				source = "curseforge"
				projectID = sr.Hits[0].ProjectID
				if modTitle == "" {
					modTitle = sr.Hits[0].Title
				}
			}
		}
	}

	if source == "" || projectID == "" {
		return nil, fmt.Errorf("не удалось найти проект мода «%s» на Modrinth или CurseForge", cleanModSearchQuery(modTitle))
	}

	// Save resolved source & projectID in cache
	meta.Source = source
	meta.ProjectID = projectID
	meta.Title = modTitle
	diskCache[filename] = meta
	saveModCache(cacheFile, diskCache)
	metaCacheMu.Lock()
	metaMemCache[filename] = meta
	metaCacheMu.Unlock()

	var options []ModVersionOption

	if source == "modrinth" {
		versions, err := l.GetModrinthProjectVersions(ctx, projectID, "", "")
		if err != nil {
			return nil, fmt.Errorf("ошибка загрузки версий с Modrinth: %w", err)
		}
		for _, v := range versions {
			var dlURL, fName string
			var fSize int64
			var fHash string
			for _, f := range v.Files {
				if f.Primary || dlURL == "" {
					dlURL = f.URL
					fName = f.Filename
					fSize = f.Size
					if f.Hashes != nil {
						fHash = f.Hashes["sha1"]
					}
				}
			}
			isCur := false
			if (h != "" && fHash != "" && strings.EqualFold(h, fHash)) || fName == filename {
				isCur = true
			}
			vType := strings.ToLower(v.VersionType)
			if vType == "" {
				vType = "release"
			}

			isComp := true
			if mcVer != "" && len(v.GameVersions) > 0 {
				matched := false
				for _, gv := range v.GameVersions {
					if gv == mcVer || strings.HasPrefix(mcVer, gv) || strings.HasPrefix(gv, mcVer) {
						matched = true
						break
					}
				}
				if !matched {
					isComp = false
				}
			}
			if isComp && loader != "" && loader != "vanilla" && len(v.Loaders) > 0 {
				matched := false
				for _, ld := range v.Loaders {
					if strings.EqualFold(ld, loader) {
						matched = true
						break
					}
				}
				if !matched {
					isComp = false
				}
			}

			options = append(options, ModVersionOption{
				ID:            v.ID,
				Name:          v.Name,
				VersionNumber: v.VersionNum,
				GameVersions:  v.GameVersions,
				Loaders:       v.Loaders,
				DownloadURL:   dlURL,
				Filename:      fName,
				FileSize:      fSize,
				DatePublished: v.DatePublished,
				IsCurrent:     isCur,
				VersionType:   vType,
				Changelog:     v.Changelog,
				IsCompatible:  isComp,
			})
		}
	} else if source == "curseforge" {
		cfID, err := strconv.Atoi(projectID)
		if err != nil {
			return nil, fmt.Errorf("неверный ID CurseForge: %s", projectID)
		}
		files, err := l.GetCurseForgeModFiles(ctx, cfID, "", "")
		if err != nil {
			return nil, fmt.Errorf("ошибка загрузки файлов с CurseForge: %w", err)
		}
		for _, f := range files {
			dl := f.DownloadURL
			if dl == "" {
				dl = fmt.Sprintf("https://edge.forgecdn.net/files/%d/%d/%s", f.ID/1000, f.ID%1000, url.PathEscape(f.FileName))
			}
			vType := "release"
			switch f.ReleaseType {
			case 2:
				vType = "beta"
			case 3:
				vType = "alpha"
			}
			isCur := (f.FileName == filename || strings.TrimSuffix(filename, ".disabled") == f.FileName)

			isComp := true
			if mcVer != "" && len(f.GameVersions) > 0 {
				matched := false
				for _, gv := range f.GameVersions {
					if gv == mcVer || strings.HasPrefix(mcVer, gv) || strings.HasPrefix(gv, mcVer) {
						matched = true
						break
					}
				}
				if !matched {
					isComp = false
				}
			}

			options = append(options, ModVersionOption{
				ID:            fmt.Sprintf("%d", f.ID),
				Name:          f.DisplayName,
				VersionNumber: f.DisplayName,
				GameVersions:  f.GameVersions,
				DownloadURL:   dl,
				Filename:      f.FileName,
				FileSize:      f.FileLength,
				DatePublished: f.FileDate,
				IsCurrent:     isCur,
				VersionType:   vType,
				Changelog:     "",
				IsCompatible:  isComp,
			})
		}
	}

	return &ModVersionChangeInfo{
		Filename:       filename,
		ModName:        modTitle,
		Source:         source,
		CurrentVersion: meta.Version,
		CurrentSha1:    h,
		ProjectID:      projectID,
		Versions:       options,
	}, nil
}

// ChangeModVersion downloads a new version of a mod, removes the old file, and returns updated item.
func (l *Launcher) ChangeModVersion(ctx context.Context, instDir, oldFilename, downloadURL, newFilename, source, projectID string) (*ContentItem, error) {
	modsDir := filepath.Join(instDir, "mods")
	_ = os.MkdirAll(modsDir, 0755)
	destPath := filepath.Join(modsDir, newFilename)

	if err := l.DownloadModFile(ctx, downloadURL, destPath); err != nil {
		return nil, fmt.Errorf("download mod file: %w", err)
	}

	if oldFilename != "" && oldFilename != newFilename {
		_ = os.Remove(filepath.Join(modsDir, oldFilename))
		_ = os.Remove(filepath.Join(modsDir, oldFilename+".disabled"))
		_ = os.Remove(strings.TrimSuffix(filepath.Join(modsDir, oldFilename), ".disabled"))
	}

	h, _ := calcFileSHA1(destPath)
	extracted, iconBytes := extractMetadataFromJar(destPath)
	if extracted.Title == "" {
		extracted.Title = strings.TrimSuffix(newFilename, filepath.Ext(newFilename))
	}
	extracted.Sha1 = h
	extracted.Source = source
	extracted.ProjectID = projectID

	iconsDir := filepath.Join(instDir, ".wailauncher_icons")
	_ = os.MkdirAll(iconsDir, 0755)
	iconURL := ""
	if len(iconBytes) > 0 && h != "" {
		localIcon := filepath.Join(iconsDir, h+".png")
		_ = os.WriteFile(localIcon, iconBytes, 0644)
		extracted.HasLocalIcon = true
		iconURL = "data:image/png;base64," + base64.StdEncoding.EncodeToString(iconBytes)
	}

	cacheFile := filepath.Join(instDir, ".wailauncher_mod_cache.json")
	diskCache := loadModCache(cacheFile)
	if oldFilename != "" {
		delete(diskCache, oldFilename)
	}
	diskCache[newFilename] = extracted
	saveModCache(cacheFile, diskCache)

	metaCacheMu.Lock()
	if oldFilename != "" {
		delete(metaMemCache, oldFilename)
	}
	metaMemCache[newFilename] = extracted
	metaCacheMu.Unlock()

	fi, _ := os.Stat(destPath)
	var size, modTime int64
	if fi != nil {
		size = fi.Size()
		modTime = fi.ModTime().Unix()
	}

	return &ContentItem{
		Filename:  newFilename,
		Name:      extracted.Title,
		Version:   extracted.Version,
		Type:      "mod",
		Enabled:   !strings.HasSuffix(strings.ToLower(newFilename), ".disabled"),
		Size:      size,
		ModTime:   modTime,
		Author:    extracted.Author,
		IconURL:   iconURL,
		Sha1:      h,
		Source:    source,
		ProjectID: projectID,
	}, nil
}

