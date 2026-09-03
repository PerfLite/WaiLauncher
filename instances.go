package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"sort"
	"strings"
	"time"
	"unicode"

	"WaiLauncher/internal/launcher"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Instance is one user-created build ("сборка"): a version + modloader combo
// with its own game folder, like Modrinth App profiles.
type Instance struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	VersionID     string `json:"versionId"`
	Loader        string `json:"loader"`        // vanilla | fabric | forge | neoforge
	LoaderVersion string `json:"loaderVersion"` // e.g. 47.2.20; "" for vanilla
	Dir           string `json:"dir"`           // folder name inside instances/; "" = legacy shared game dir
	Created       int64  `json:"created"`
	Icon          string `json:"icon,omitempty"`          // URL or base64
	Group         string `json:"group,omitempty"`         // folder / category name
	SortOrder     int    `json:"sortOrder,omitempty"`     // custom sort index
	PlayTime      int64  `json:"playTime,omitempty"`      // in seconds
	PlayTimeToday int64  `json:"playTimeToday,omitempty"` // in seconds (current day)
	LastPlayDay   string `json:"lastPlayDay,omitempty"`   // "2006-01-02" of the last session
	LastPlayed    int64  `json:"lastPlayed,omitempty"`    // unix timestamp
	ServerAddress string `json:"serverAddress,omitempty"` // optional direct connect address

	// Upstream modpack tracking (for auto-updates)
	ModpackSource      string `json:"modpackSource,omitempty"`      // "modrinth" | "curseforge" | "ftb"
	ModpackID          string `json:"modpackId,omitempty"`          // project ID / slug
	ModpackVersionID   string `json:"modpackVersionId,omitempty"`   // installed version ID
	ModpackVersionName string `json:"modpackVersionName,omitempty"` // e.g. "1.0.0"

	// Per-instance launch overrides; empty/zero = inherit global settings.
	RAMMB           int    `json:"ramMb,omitempty"`
	JavaPath        string `json:"javaPath,omitempty"`
	JVMPreset       string `json:"jvmPreset,omitempty"` // "" or "global" = inherit, aikar, zgc, shenandoah, default, none
	JVMArgs         string `json:"jvmArgs,omitempty"`
	UseCustomWindow bool   `json:"useCustomWindow,omitempty"`
	Fullscreen      bool   `json:"fullscreen,omitempty"`
	WindowWidth     int    `json:"windowWidth,omitempty"`
	WindowHeight    int    `json:"windowHeight,omitempty"`
}

// VerifyResult reports the statistics of an instance verification scan.
type VerifyResult struct {
	TotalChecked int      `json:"totalChecked"`
	Repaired     int      `json:"repaired"`
	Failed       int      `json:"failed"`
	Details      []string `json:"details"`
}

func (a *App) instancesPath() string {
	return filepath.Join(a.l.Root, "instances.json")
}

func (a *App) loadInstances() []Instance {
	path := a.instancesPath()
	data, err := os.ReadFile(path)
	var list []Instance
	if err != nil || len(data) == 0 || json.Unmarshal(data, &list) != nil {
		// Attempt fallback recovery from .bak
		bakPath := path + ".bak"
		if bakData, bakErr := os.ReadFile(bakPath); bakErr == nil && len(bakData) > 0 {
			if json.Unmarshal(bakData, &list) == nil && len(list) > 0 {
				launcher.LogWarn("Recovered corrupted instances.json from instances.json.bak")
				_ = atomicSaveJSON(path, bakData)
			}
		}
	}
	if list == nil {
		return nil
	}
	hasMissingIcons := false
	for i := range list {
		inst := &list[i]
		if inst.Icon == "" {
			gameDir := a.instanceGameDir(*inst)
			for _, ext := range []string{"png", "jpg", "jpeg", "webp"} {
				iconPath := filepath.Join(gameDir, "icon."+ext)
				if imgData, err := os.ReadFile(iconPath); err == nil && len(imgData) > 0 {
					mime := "image/png"
					if ext == "jpg" || ext == "jpeg" {
						mime = "image/jpeg"
					} else if ext == "webp" {
						mime = "image/webp"
					}
					inst.Icon = fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(imgData))
					break
				}
			}
			if inst.Icon == "" {
				hasMissingIcons = true
			}
		}
	}
	if hasMissingIcons {
		a.enrichInstanceIconsBackground(list)
	}
	return list
}

func (a *App) enrichInstanceIconsBackground(instances []Instance) {
	go func(list []Instance) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		updated := false

		for i := range list {
			inst := &list[i]
			if inst.Icon != "" {
				continue
			}
			gameDir := a.instanceGameDir(*inst)
			iconPath := filepath.Join(gameDir, "icon.png")
			if _, err := os.Stat(iconPath); err == nil {
				continue
			}
			query := inst.Name
			if query == "" {
				query = inst.ID
			}
			packs, err := a.l.SearchModpacks(ctx, "modrinth", query, "", "", 0, 1)
			if err == nil && len(packs) > 0 && packs[0].IconURL != "" {
				req, _ := http.NewRequestWithContext(ctx, "GET", packs[0].IconURL, nil)
				if req != nil {
					req.Header.Set("User-Agent", "WaiLauncher/0.1.0")
					if resp, err := http.DefaultClient.Do(req); err == nil && resp.StatusCode == 200 {
						imgBytes, _ := io.ReadAll(resp.Body)
						resp.Body.Close()
						if len(imgBytes) > 0 {
							_ = os.MkdirAll(gameDir, 0o755)
							_ = os.WriteFile(iconPath, imgBytes, 0o644)
							inst.Icon = "data:image/png;base64," + base64.StdEncoding.EncodeToString(imgBytes)
							updated = true
						}
					}
				}
			}
		}
		if updated {
			_ = a.saveInstances(list)
			runtime.EventsEmit(a.ctx, "instances-updated", a.loadInstances())
		}
	}(instances)
}

func (a *App) saveInstances(list []Instance) error {
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return atomicSaveJSON(a.instancesPath(), data)
}

// atomicSaveJSON writes data atomically (.tmp -> targetPath) and keeps a .bak copy
// plus historical snapshots in <root>/backups/config_backups/.
func atomicSaveJSON(targetPath string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}

	// 1. If file already exists and is non-empty, copy to .bak and archive snapshot
	if info, err := os.Stat(targetPath); err == nil && info.Size() > 0 {
		oldData, err := os.ReadFile(targetPath)
		if err == nil && len(oldData) > 0 {
			// Write immediate .bak file
			_ = os.WriteFile(targetPath+".bak", oldData, 0o644)

			// Store rolling timestamped backup in backups/config_backups/
			backupDir := filepath.Join(filepath.Dir(targetPath), "backups", "config_backups")
			_ = os.MkdirAll(backupDir, 0o755)
			baseName := filepath.Base(targetPath)
			timestamp := time.Now().Format("20060102-150405")
			snapshotPath := filepath.Join(backupDir, fmt.Sprintf("%s.%s.bak", baseName, timestamp))
			_ = os.WriteFile(snapshotPath, oldData, 0o644)

			// Prune old snapshots (keep latest 10)
			pruneConfigBackups(backupDir, baseName, 10)
		}
	}

	// 2. Write to .tmp file
	tmpPath := targetPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return err
	}

	// 3. Atomically replace targetPath with .tmp
	if goruntime.GOOS == "windows" {
		_ = os.Remove(targetPath)
	}
	if err := os.Rename(tmpPath, targetPath); err != nil {
		// Fallback: direct write if rename fails
		return os.WriteFile(targetPath, data, 0o644)
	}
	return nil
}

func pruneConfigBackups(dir, baseName string, maxKeep int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var matches []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), baseName+".") && strings.HasSuffix(e.Name(), ".bak") {
			matches = append(matches, filepath.Join(dir, e.Name()))
		}
	}
	if len(matches) > maxKeep {
		sort.Strings(matches)
		for i := 0; i < len(matches)-maxKeep; i++ {
			_ = os.Remove(matches[i])
		}
	}
}

// instanceGameDir resolves the per-instance game folder. Dir == "" means the
// legacy shared <root>/game folder (used by the auto-migrated first build
// until migrateLegacyGameDirs moves it into instances/).
func (a *App) instanceGameDir(inst Instance) string {
	if inst.Dir != "" {
		return a.l.InstanceDir(inst.Dir)
	}
	if inst.ID != "" && inst.ID != "default" {
		return a.l.InstanceDir(inst.ID)
	}
	return a.l.GameDir()
}

// migrateLegacyGameDirs moves builds that still use the shared <root>/game
// folder into their own instances/<id> folder, so every build lives under
// instances/ like in Modrinth App.
func (a *App) migrateLegacyGameDirs() {
	list := a.loadInstances()
	changed := false
	for i := range list {
		if list[i].Dir == "" {
			if list[i].ID != "" {
				list[i].Dir = list[i].ID
				changed = true
			}
		}
	}
	if changed {
		_ = a.saveInstances(list)
	}
}

// migrateLegacyInstance turns the pre-instance setup (selected version +
// shared game folder with existing worlds) into a "default" build so nothing
// is lost on upgrade.
func (a *App) migrateLegacyInstance() {
	if _, err := os.Stat(a.instancesPath()); err == nil {
		return
	}
	if a.set.SelectedVersion == "" {
		return
	}
	inst := Instance{
		ID:        "default",
		Name:      a.set.SelectedVersion,
		VersionID: a.set.SelectedVersion,
		Loader:    "vanilla",
		Dir:       "", // migrateLegacyGameDirs moves the shared game dir into instances/
		Created:   time.Now().Unix(),
	}
	if a.saveInstances([]Instance{inst}) == nil {
		a.set.ActiveInstance = "default"
		_ = a.set.save()
	}
}

// GetInstances returns all created builds.
func (a *App) GetInstances() []Instance {
	return a.loadInstances()
}

// CreateInstance adds a new build with its own folder and selects it.
// For modloader builds loaderVersion may be empty — the recommended or
// newest version is picked automatically.
func (a *App) CreateInstance(name, versionID, loader, loaderVersion string) (*Instance, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("%s", launcher.T(a.set.Language, "err.pick_version"))
	}
	loader = strings.TrimSpace(loader)
	if loader == "" {
		loader = "vanilla"
	}
	loaderVersion = strings.TrimSpace(loaderVersion)
	if loader == "vanilla" {
		loaderVersion = ""
	} else if loaderVersion == "" {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		v, err := a.l.PickDefaultLoaderVersion(ctx, loader, versionID)
		if err != nil {
			return nil, err
		}
		loaderVersion = v
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = versionID
	}
	list := a.loadInstances()
	id := instanceSlug(name, list)
	if err := os.MkdirAll(a.l.InstanceDir(id), 0o755); err != nil {
		return nil, err
	}
	inst := Instance{
		ID:            id,
		Name:          name,
		VersionID:     versionID,
		Loader:        loader,
		LoaderVersion: loaderVersion,
		Dir:           id,
		Created:       time.Now().Unix(),
	}
	list = append(list, inst)
	if err := a.saveInstances(list); err != nil {
		return nil, err
	}
	a.set.ActiveInstance = id
	a.set.SelectedVersion = versionID
	_ = a.set.save()
	return &inst, nil
}

// DeleteInstance removes a build and its folder, returning the id of the
// build that becomes active afterwards ("" if none left).
func (a *App) DeleteInstance(id string) (string, error) {
	list := a.loadInstances()
	kept := make([]Instance, 0, len(list))
	var removed *Instance
	for i := range list {
		if list[i].ID == id {
			removed = &list[i]
			continue
		}
		kept = append(kept, list[i])
	}
	if removed == nil {
		return "", fmt.Errorf("%s", launcher.T(a.set.Language, "err.no_instance"))
	}
	if err := a.saveInstances(kept); err != nil {
		return "", err
	}
	if removed.Dir != "" && removed.Dir == filepath.Base(removed.Dir) {
		// Dir is a plain folder name written by CreateInstance; InstanceDir
		// keeps it inside <root>/instances.
		_ = os.RemoveAll(a.l.InstanceDir(removed.Dir))
	}
	if a.set.ActiveInstance == id {
		a.set.ActiveInstance = ""
		if len(kept) > 0 {
			a.set.ActiveInstance = kept[0].ID
		}
		_ = a.set.save()
	}
	return a.set.ActiveInstance, nil
}

// SetActiveInstance selects the build the ИГРАТЬ button launches.
func (a *App) SetActiveInstance(id string) error {
	for _, inst := range a.loadInstances() {
		if inst.ID == id {
			a.set.ActiveInstance = id
			a.set.SelectedVersion = inst.VersionID
			return a.set.save()
		}
	}
	return fmt.Errorf("%s", launcher.T(a.set.Language, "err.no_instance"))
}

// GetLoaderVersions exposes the modloader version lists to the UI.
func (a *App) GetLoaderVersions(loader, mcVersion string) ([]launcher.LoaderVersionEntry, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	return a.l.GetLoaderVersions(ctx, loader, mcVersion)
}

// instanceSlug builds a unique folder-safe id from the display name.
func instanceSlug(name string, existing []Instance) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteRune('-')
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		slug = "instance"
	}
	if len(slug) > 32 {
		slug = strings.TrimRight(slug[:32], "-")
	}
	used := make(map[string]bool, len(existing))
	for _, i := range existing {
		used[i.ID] = true
	}
	if !used[slug] {
		return slug
	}
	for n := 2; ; n++ {
		c := fmt.Sprintf("%s-%d", slug, n)
		if !used[c] {
			return c
		}
	}
}

// OpenInstanceDir opens the instance folder in Explorer.
func (a *App) OpenInstanceDir(id string) error {
	for _, inst := range a.loadInstances() {
		if inst.ID == id {
			dir := a.instanceGameDir(inst)
			_ = os.MkdirAll(dir, 0o755)
			if goruntime.GOOS == "windows" {
				return exec.Command("explorer.exe", dir).Start()
			}
			a.OpenURL(dir)
			return nil
		}
	}
	return fmt.Errorf("instance not found")
}

// GetInstalledMods returns a list of installed mods in the instance's mods folder.
func (a *App) GetInstalledMods(id string) ([]launcher.ModItem, error) {
	for _, inst := range a.loadInstances() {
		if inst.ID == id {
			gameDir := a.instanceGameDir(inst)
			launcher.OrganizeInstanceFolders(gameDir)
			modsDir := filepath.Join(gameDir, "mods")
			_ = os.MkdirAll(modsDir, 0o755)
			entries, err := os.ReadDir(modsDir)
			if err != nil {
				return nil, err
			}
			var list []launcher.ModItem
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				name := e.Name()
				lower := strings.ToLower(name)
				if !strings.HasSuffix(lower, ".jar") && !strings.HasSuffix(lower, ".jar.disabled") {
					continue
				}
				info, err := e.Info()
				if err != nil {
					continue
				}
				enabled := !strings.HasSuffix(lower, ".disabled")
				cleanName := strings.TrimSuffix(name, ".disabled")
				cleanName = strings.TrimSuffix(cleanName, ".jar")
				list = append(list, launcher.ModItem{
					Filename: name,
					Name:     cleanName,
					Enabled:  enabled,
					Size:     info.Size(),
					ModTime:  info.ModTime().Unix(),
				})
			}
			sort.Slice(list, func(i, j int) bool {
				return strings.ToLower(list[i].Name) < strings.ToLower(list[j].Name)
			})
			return list, nil
		}
	}
	return nil, fmt.Errorf("instance not found")
}

// ToggleMod enables or disables a mod by renaming .jar <-> .jar.disabled.
func (a *App) ToggleMod(instanceID, filename string, enable bool) error {
	for _, inst := range a.loadInstances() {
		if inst.ID == instanceID {
			modsDir := filepath.Join(a.instanceGameDir(inst), "mods")
			oldPath := filepath.Join(modsDir, filename)
			var newPath string
			if enable {
				newPath = strings.TrimSuffix(oldPath, ".disabled")
			} else {
				if !strings.HasSuffix(oldPath, ".disabled") {
					newPath = oldPath + ".disabled"
				} else {
					newPath = oldPath
				}
			}
			if oldPath == newPath {
				return nil
			}
			return os.Rename(oldPath, newPath)
		}
	}
	return fmt.Errorf("instance not found")
}

// DeleteMod deletes a mod jar file from the instance.
func (a *App) DeleteMod(instanceID, filename string) error {
	for _, inst := range a.loadInstances() {
		if inst.ID == instanceID {
			modsDir := filepath.Join(a.instanceGameDir(inst), "mods")
			return os.Remove(filepath.Join(modsDir, filename))
		}
	}
	return fmt.Errorf("instance not found")
}

// SearchModrinthMods searches for projects (mods, resourcepacks, shaders) on Modrinth API.
func (a *App) SearchModrinthMods(query, projectType, loader, mcVersion string, offset, limit int) (*launcher.ModrinthSearchResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return a.l.SearchModrinth(ctx, query, projectType, loader, mcVersion, offset, limit)
}

// InstallModrinthMod installs the latest compatible project version from Modrinth directly into the instance.
func (a *App) InstallModrinthMod(instanceID, projectIDOrSlug, projectType string) (*launcher.ModItem, error) {
	var targetInst *Instance
	for _, inst := range a.loadInstances() {
		if inst.ID == instanceID {
			targetInst = &inst
			break
		}
	}
	if targetInst == nil {
		return nil, fmt.Errorf("instance not found")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	ld := targetInst.Loader
	if projectType == "resourcepack" || projectType == "shader" {
		ld = "" // Resource packs and shaders don't depend on loader
	}

	versions, err := a.l.GetModrinthProjectVersions(ctx, projectIDOrSlug, ld, targetInst.VersionID)
	if err != nil {
		return nil, fmt.Errorf("lookup versions: %w", err)
	}
	if len(versions) == 0 {
		return nil, fmt.Errorf("no compatible versions found for Minecraft %s", targetInst.VersionID)
	}
	v := versions[0]
	if len(v.Files) == 0 {
		return nil, fmt.Errorf("selected version has no files")
	}
	var chosenFile launcher.ModrinthVersionFile
	for _, f := range v.Files {
		if f.Primary {
			chosenFile = f
			break
		}
	}
	if chosenFile.URL == "" {
		chosenFile = v.Files[0]
	}

	subFolder := "mods"
	switch projectType {
	case "resourcepack":
		subFolder = "resourcepacks"
	case "shader":
		subFolder = "shaderpacks"
	}

	targetDir := filepath.Join(a.instanceGameDir(*targetInst), subFolder)
	destPath := filepath.Join(targetDir, chosenFile.Filename)
	if err := a.l.DownloadModFile(ctx, chosenFile.URL, destPath); err != nil {
		return nil, fmt.Errorf("download file: %w", err)
	}
	cleanName := strings.TrimSuffix(chosenFile.Filename, filepath.Ext(chosenFile.Filename))
	return &launcher.ModItem{
		Filename: chosenFile.Filename,
		Name:     cleanName,
		Enabled:  true,
		Size:     chosenFile.Size,
		ModTime:  time.Now().Unix(),
	}, nil
}

// CheckModDependencies checks if the mod has any missing required or optional dependencies.
func (a *App) CheckModDependencies(instanceID, projectIDOrSlug string) ([]launcher.ResolvedDependency, error) {
	var targetInst *Instance
	for _, inst := range a.loadInstances() {
		if inst.ID == instanceID {
			targetInst = &inst
			break
		}
	}
	if targetInst == nil {
		return nil, fmt.Errorf("instance not found")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	ld := targetInst.Loader
	versions, err := a.l.GetModrinthProjectVersions(ctx, projectIDOrSlug, ld, targetInst.VersionID)
	if err != nil || len(versions) == 0 {
		return nil, nil
	}

	var installedFiles []string
	modsDir := filepath.Join(a.instanceGameDir(*targetInst), "mods")
	if entries, err := os.ReadDir(modsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				installedFiles = append(installedFiles, e.Name())
			}
		}
	}

	return a.l.ResolveModDependencies(ctx, versions[0].ID, ld, targetInst.VersionID, installedFiles)
}

// InstallModWithDependencies installs a main mod and any selected dependency URLs into the instance's mods folder.
func (a *App) InstallModWithDependencies(instanceID, projectIDOrSlug, projectType string, depDownloadUrls []string) (*launcher.ModItem, error) {
	var targetInst *Instance
	for _, inst := range a.loadInstances() {
		if inst.ID == instanceID {
			targetInst = &inst
			break
		}
	}
	if targetInst == nil {
		return nil, fmt.Errorf("instance not found")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	modsDir := filepath.Join(a.instanceGameDir(*targetInst), "mods")
	_ = os.MkdirAll(modsDir, 0o755)

	for _, rawURL := range depDownloadUrls {
		if strings.TrimSpace(rawURL) == "" {
			continue
		}
		u, err := url.Parse(rawURL)
		if err != nil {
			continue
		}
		fn := filepath.Base(u.Path)
		if fn == "" || fn == "." || fn == "/" {
			continue
		}
		dest := filepath.Join(modsDir, fn)
		_ = a.l.DownloadModFile(ctx, rawURL, dest)
	}

	return a.InstallModrinthMod(instanceID, projectIDOrSlug, projectType)
}

// SearchCurseForgeMods searches for projects (mods, resourcepacks, shaders) on CurseForge API.
func (a *App) SearchCurseForgeMods(query, projectType, loader, mcVersion string, offset, limit int) (*launcher.ModrinthSearchResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return a.l.SearchCurseForge(ctx, query, projectType, loader, mcVersion, offset, limit)
}

// InstallCurseForgeMod installs the latest compatible project version from CurseForge directly into the instance.
func (a *App) InstallCurseForgeMod(instanceID, modIDStr, projectType string) (*launcher.ModItem, error) {
	var targetInst *Instance
	for _, inst := range a.loadInstances() {
		if inst.ID == instanceID {
			targetInst = &inst
			break
		}
	}
	if targetInst == nil {
		return nil, fmt.Errorf("instance not found")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	modID, err := launcher.ParseCurseForgeID(modIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid curseforge mod id: %s", modIDStr)
	}

	ld := targetInst.Loader
	if projectType == "resourcepack" || projectType == "shader" {
		ld = ""
	}

	files, err := a.l.GetCurseForgeModFiles(ctx, modID, ld, targetInst.VersionID)
	if err != nil {
		return nil, fmt.Errorf("lookup curseforge files: %w", err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no compatible versions found on CurseForge for Minecraft %s", targetInst.VersionID)
	}

	chosenFile := files[0]
	if chosenFile.DownloadURL == "" {
		chosenFile.DownloadURL = fmt.Sprintf("https://edge.forgecdn.net/files/%d/%d/%s", chosenFile.ID/1000, chosenFile.ID%1000, url.PathEscape(chosenFile.FileName))
	}

	subFolder := "mods"
	switch projectType {
	case "resourcepack":
		subFolder = "resourcepacks"
	case "shader":
		subFolder = "shaderpacks"
	}

	targetDir := filepath.Join(a.instanceGameDir(*targetInst), subFolder)
	destPath := filepath.Join(targetDir, chosenFile.FileName)
	if err := a.l.DownloadModFile(ctx, chosenFile.DownloadURL, destPath); err != nil {
		return nil, fmt.Errorf("download curseforge file: %w", err)
	}
	cleanName := strings.TrimSuffix(chosenFile.FileName, filepath.Ext(chosenFile.FileName))
	return &launcher.ModItem{
		Filename: chosenFile.FileName,
		Name:     cleanName,
		Enabled:  true,
		Size:     chosenFile.FileLength,
		ModTime:  time.Now().Unix(),
	}, nil
}

// CheckCurseForgeDependencies checks if the CurseForge mod has any missing dependencies.
func (a *App) CheckCurseForgeDependencies(instanceID, modIDStr string) ([]launcher.ResolvedDependency, error) {
	var targetInst *Instance
	for _, inst := range a.loadInstances() {
		if inst.ID == instanceID {
			targetInst = &inst
			break
		}
	}
	if targetInst == nil {
		return nil, fmt.Errorf("instance not found")
	}

	modID, err := launcher.ParseCurseForgeID(modIDStr)
	if err != nil {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	ld := targetInst.Loader
	files, err := a.l.GetCurseForgeModFiles(ctx, modID, ld, targetInst.VersionID)
	if err != nil || len(files) == 0 {
		return nil, nil
	}

	var installedFiles []string
	modsDir := filepath.Join(a.instanceGameDir(*targetInst), "mods")
	if entries, err := os.ReadDir(modsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				installedFiles = append(installedFiles, e.Name())
			}
		}
	}

	return a.l.ResolveCurseForgeDependencies(ctx, modID, &files[0], ld, targetInst.VersionID, installedFiles)
}

// InstallCurseForgeModWithDependencies installs a CurseForge mod and its dependencies.
func (a *App) InstallCurseForgeModWithDependencies(instanceID, modIDStr, projectType string, depDownloadUrls []string) (*launcher.ModItem, error) {
	var targetInst *Instance
	for _, inst := range a.loadInstances() {
		if inst.ID == instanceID {
			targetInst = &inst
			break
		}
	}
	if targetInst == nil {
		return nil, fmt.Errorf("instance not found")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	modsDir := filepath.Join(a.instanceGameDir(*targetInst), "mods")
	_ = os.MkdirAll(modsDir, 0o755)

	for _, rawURL := range depDownloadUrls {
		if strings.TrimSpace(rawURL) == "" {
			continue
		}
		u, err := url.Parse(rawURL)
		if err != nil {
			continue
		}
		fn := filepath.Base(u.Path)
		if fn == "" || fn == "." || fn == "/" {
			continue
		}
		dest := filepath.Join(modsDir, fn)
		_ = a.l.DownloadModFile(ctx, rawURL, dest)
	}

	return a.InstallCurseForgeMod(instanceID, modIDStr, projectType)
}

// GetInstanceLogs reads the latest log of the instance.
func (a *App) GetInstanceLogs(instanceID string) (string, error) {
	for _, inst := range a.loadInstances() {
		if inst.ID == instanceID {
			logPath := filepath.Join(a.instanceGameDir(inst), "logs", "latest.log")
			data, err := os.ReadFile(logPath)
			if err != nil {
				return "", nil
			}
			s := string(data)
			if len(s) > 100000 {
				s = s[len(s)-100000:]
			}
			return s, nil
		}
	}
	return "", fmt.Errorf("instance not found")
}

// SearchModpacks queries Modrinth or CurseForge for modpacks.
func (a *App) SearchModpacks(source, query, mcVersion, loader string, offset, limit int) ([]launcher.ModpackItem, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	return a.l.SearchModpacks(ctx, source, query, mcVersion, loader, offset, limit)
}

// GetModpackDetails fetches full details, description, and versions for a modpack.
func (a *App) GetModpackDetails(source, idOrSlug string) (*launcher.ModpackDetails, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	return a.l.GetModpackDetails(ctx, source, idOrSlug)
}

// InstallModpack downloads and installs a modpack from Modrinth, CurseForge or FTB.
func (a *App) InstallModpack(source, downloadURL, customName, packID, versionID, versionName string) (*Instance, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	progressFn := func(p launcher.ModpackProgress) {
		runtime.EventsEmit(a.ctx, "modpack-progress", p)
	}

	info, err := a.l.InstallModpackFromURL(ctx, source, downloadURL, customName, progressFn)
	if err != nil {
		progressFn(launcher.ModpackProgress{Stage: "error", Percent: 0, Message: err.Error()})
		return nil, err
	}

	list := a.loadInstances()
	newInst := Instance{
		ID:                 info.ID,
		Name:               info.Name,
		VersionID:          info.VersionID,
		Loader:             info.Loader,
		LoaderVersion:      info.LoaderVersion,
		Dir:                info.ID,
		Created:            time.Now().Unix(),
		ModpackSource:      source,
		ModpackID:          packID,
		ModpackVersionID:   versionID,
		ModpackVersionName: versionName,
	}

	gameDir := a.instanceGameDir(newInst)
	iconPath := filepath.Join(gameDir, "icon.png")
	if data, err := os.ReadFile(iconPath); err == nil && len(data) > 0 {
		newInst.Icon = "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)
	} else {
		packs, err := a.l.SearchModpacks(ctx, source, newInst.Name, "", "", 0, 1)
		if err == nil && len(packs) > 0 && packs[0].IconURL != "" {
			if req, _ := http.NewRequestWithContext(ctx, "GET", packs[0].IconURL, nil); req != nil {
				req.Header.Set("User-Agent", "WaiLauncher/0.1.0")
				if resp, err := http.DefaultClient.Do(req); err == nil && resp.StatusCode == 200 {
					imgBytes, _ := io.ReadAll(resp.Body)
					resp.Body.Close()
					if len(imgBytes) > 0 {
						_ = os.MkdirAll(gameDir, 0o755)
						_ = os.WriteFile(iconPath, imgBytes, 0o644)
						newInst.Icon = "data:image/png;base64," + base64.StdEncoding.EncodeToString(imgBytes)
					}
				}
			}
		}
	}

	list = append(list, newInst)
	_ = a.saveInstances(list)
	a.set.ActiveInstance = info.ID
	a.set.SelectedVersion = info.VersionID
	_ = a.set.save()
	runtime.EventsEmit(a.ctx, "instances-updated", a.loadInstances())

	return &newInst, nil
}

type ModpackUpdateInfo struct {
	HasUpdate       bool   `json:"hasUpdate"`
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion"`
	LatestVersionID string `json:"latestVersionId"`
	DownloadURL     string `json:"downloadUrl"`
	Changelog       string `json:"changelog"`
	ReleaseDate     string `json:"releaseDate"`
}

func (a *App) findInstance(id string) (*Instance, error) {
	for _, inst := range a.loadInstances() {
		if inst.ID == id {
			return &inst, nil
		}
	}
	return nil, fmt.Errorf("instance %q not found", id)
}

// CheckInstanceModpackUpdate checks if there is a newer version of the upstream modpack.
func (a *App) CheckInstanceModpackUpdate(instanceID string) (*ModpackUpdateInfo, error) {
	inst, err := a.findInstance(instanceID)
	if err != nil {
		return nil, err
	}

	if inst.ModpackSource == "" || inst.ModpackID == "" {
		return &ModpackUpdateInfo{HasUpdate: false}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	details, err := a.l.GetModpackDetails(ctx, inst.ModpackSource, inst.ModpackID)
	if err != nil || details == nil || len(details.Versions) == 0 {
		return &ModpackUpdateInfo{HasUpdate: false}, nil
	}

	latest := details.Versions[0]
	hasUpdate := false
	if inst.ModpackVersionID != "" && latest.ID != inst.ModpackVersionID {
		hasUpdate = true
	} else if inst.ModpackVersionName != "" && latest.VersionNumber != "" && latest.VersionNumber != inst.ModpackVersionName {
		hasUpdate = true
	}

	return &ModpackUpdateInfo{
		HasUpdate:       hasUpdate,
		CurrentVersion:  inst.ModpackVersionName,
		LatestVersion:   latest.VersionNumber,
		LatestVersionID: latest.ID,
		DownloadURL:     latest.DownloadURL,
		Changelog:       latest.Changelog,
		ReleaseDate:     latest.DatePublished,
	}, nil
}

// UpdateInstanceModpack updates an existing instance to a newer version of the modpack, preserving saves and settings.
func (a *App) UpdateInstanceModpack(instanceID, newDownloadURL, newVersionID, newVersionName string) (*Instance, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	inst, err := a.findInstance(instanceID)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	progressFn := func(p launcher.ModpackProgress) {
		runtime.EventsEmit(a.ctx, "modpack-progress", p)
	}

	progressFn(launcher.ModpackProgress{
		Stage:   "downloading",
		Percent: 0.05,
		Message: fmt.Sprintf("Подготовка обновления сборки «%s»...", inst.Name),
	})

	gameDir := a.instanceGameDir(*inst)

	// 1. Temporarily backup mods in case of rollback
	modsDir := filepath.Join(gameDir, "mods")
	oldModsBackup := filepath.Join(gameDir, ".wailauncher", "mods_backup_before_update")
	_ = os.RemoveAll(oldModsBackup)
	_ = os.MkdirAll(filepath.Dir(oldModsBackup), 0755)
	if st, err := os.Stat(modsDir); err == nil && st.IsDir() {
		_ = os.Rename(modsDir, oldModsBackup)
	}
	_ = os.MkdirAll(modsDir, 0755)

	// 2. Perform installation of new version files
	info, err := a.l.InstallModpackFromURL(ctx, inst.ModpackSource, newDownloadURL, inst.Name, progressFn)
	if err != nil {
		// Rollback old mods on error
		_ = os.RemoveAll(modsDir)
		_ = os.Rename(oldModsBackup, modsDir)
		progressFn(launcher.ModpackProgress{Stage: "error", Percent: 0, Message: fmt.Sprintf("Ошибка обновления: %v", err)})
		return nil, err
	}

	// 3. Move newly downloaded pack files from its temp folder into this instance's gameDir
	newGameDir := a.l.InstanceDir(info.ID)
	if newGameDir != gameDir {
		entries, err := os.ReadDir(newGameDir)
		if err == nil {
			for _, entry := range entries {
				name := entry.Name()
				if name == "saves" || name == "screenshots" || name == "options.txt" || name == "servers.dat" {
					continue // preserve user's personal data
				}
				srcItem := filepath.Join(newGameDir, name)
				dstItem := filepath.Join(gameDir, name)
				if entry.IsDir() {
					_ = copyDirectory(srcItem, dstItem)
				} else {
					_ = copySingleFile(srcItem, dstItem)
				}
			}
		}
		_ = os.RemoveAll(newGameDir)
	}

	// Remove old mods backup
	_ = os.RemoveAll(oldModsBackup)

	// 4. Update instance metadata
	list := a.loadInstances()
	for i := range list {
		if list[i].ID == instanceID {
			list[i].VersionID = info.VersionID
			list[i].Loader = info.Loader
			list[i].LoaderVersion = info.LoaderVersion
			list[i].ModpackVersionID = newVersionID
			list[i].ModpackVersionName = newVersionName
			inst = &list[i]
			break
		}
	}

	_ = a.saveInstances(list)
	runtime.EventsEmit(a.ctx, "instances-updated", a.loadInstances())

	progressFn(launcher.ModpackProgress{
		Stage:   "done",
		Percent: 1.0,
		Message: fmt.Sprintf("Сборка «%s» успешно обновлена до версии %s!", inst.Name, newVersionName),
	})

	return inst, nil
}

// GetInstanceAllContent retrieves all mods, resourcepacks, shaderpacks, and datapacks.
func (a *App) GetInstanceAllContent(id string) ([]launcher.ContentItem, error) {
	for _, inst := range a.loadInstances() {
		if inst.ID == id {
			gameDir := a.instanceGameDir(inst)
			return a.l.GetInstanceAllContent(gameDir), nil
		}
	}
	return nil, fmt.Errorf("instance not found")
}

// ToggleInstanceContent toggles enabled/disabled state of a content item.
func (a *App) ToggleInstanceContent(instanceID, itemType, filename string, enable bool) error {
	for _, inst := range a.loadInstances() {
		if inst.ID == instanceID {
			gameDir := a.instanceGameDir(inst)
			subFolder := "mods"
			switch itemType {
			case "resourcepack":
				subFolder = "resourcepacks"
			case "shaderpack":
				subFolder = "shaderpacks"
			case "datapack":
				subFolder = "datapacks"
			}
			folderPath := filepath.Join(gameDir, subFolder)
			oldPath := filepath.Join(folderPath, filename)
			var newPath string
			if enable {
				newPath = strings.TrimSuffix(oldPath, ".disabled")
			} else {
				if !strings.HasSuffix(oldPath, ".disabled") {
					newPath = oldPath + ".disabled"
				} else {
					newPath = oldPath
				}
			}
			if oldPath == newPath {
				return nil
			}
			return os.Rename(oldPath, newPath)
		}
	}
	return fmt.Errorf("instance not found")
}

// DeleteInstanceContent removes a file from instance folder.
func (a *App) DeleteInstanceContent(instanceID, itemType, filename string) error {
	for _, inst := range a.loadInstances() {
		if inst.ID == instanceID {
			gameDir := a.instanceGameDir(inst)
			subFolder := "mods"
			switch itemType {
			case "resourcepack":
				subFolder = "resourcepacks"
			case "shaderpack":
				subFolder = "shaderpacks"
			case "datapack":
				subFolder = "datapacks"
			}
			return os.Remove(filepath.Join(gameDir, subFolder, filename))
		}
	}
	return fmt.Errorf("instance not found")
}

// CheckInstanceModUpdates checks for mod updates via Modrinth.
func (a *App) CheckInstanceModUpdates(id string) (map[string]launcher.ContentItem, error) {
	for _, inst := range a.loadInstances() {
		if inst.ID == id {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			gameDir := a.instanceGameDir(inst)
			return a.l.CheckModUpdates(ctx, gameDir, inst.VersionID, inst.Loader)
		}
	}
	return nil, fmt.Errorf("instance not found")
}

// UpdateInstanceMod downloads the new version of a mod and removes the old jar.
func (a *App) UpdateInstanceMod(instanceID, oldFilename, newURL, newFilename string) error {
	for _, inst := range a.loadInstances() {
		if inst.ID == instanceID {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			gameDir := a.instanceGameDir(inst)
			modsDir := filepath.Join(gameDir, "mods")
			destPath := filepath.Join(modsDir, newFilename)
			if err := a.l.DownloadModFile(ctx, newURL, destPath); err != nil {
				return err
			}
			if oldFilename != newFilename {
				_ = os.Remove(filepath.Join(modsDir, oldFilename))
			}
			return nil
		}
	}
	return fmt.Errorf("instance not found")
}

// GetInstanceWorlds returns saved worlds for an instance.
func (a *App) GetInstanceWorlds(id string) ([]launcher.WorldItem, error) {
	for _, inst := range a.loadInstances() {
		if inst.ID == id {
			gameDir := a.instanceGameDir(inst)
			return a.l.GetInstanceWorlds(gameDir), nil
		}
	}
	return nil, fmt.Errorf("instance not found")
}

// DeleteInstanceWorld deletes a world directory in saves/.
func (a *App) DeleteInstanceWorld(instanceID, folderName string) error {
	for _, inst := range a.loadInstances() {
		if inst.ID == instanceID {
			gameDir := a.instanceGameDir(inst)
			return a.l.DeleteInstanceWorld(gameDir, folderName)
		}
	}
	return fmt.Errorf("instance not found")
}

// OpenInstanceSubFolder opens a specific subfolder (e.g. "mods", "saves", "resourcepacks", "shaderpacks") in Explorer.
func (a *App) OpenInstanceSubFolder(instanceID, subFolder string) error {
	for _, inst := range a.loadInstances() {
		if inst.ID == instanceID {
			gameDir := a.instanceGameDir(inst)
			target := filepath.Join(gameDir, subFolder)
			_ = os.MkdirAll(target, 0o755)
			if goruntime.GOOS == "windows" {
				return exec.Command("explorer.exe", target).Start()
			}
			a.OpenURL(target)
			return nil
		}
	}
	return fmt.Errorf("instance not found")
}

// ShowFileInExplorer opens Windows Explorer with the specific file selected.
func (a *App) ShowFileInExplorer(instanceID, itemType, filename string) error {
	for _, inst := range a.loadInstances() {
		if inst.ID == instanceID {
			gameDir := a.instanceGameDir(inst)
			subFolder := "mods"
			switch itemType {
			case "resourcepack":
				subFolder = "resourcepacks"
			case "shaderpack":
				subFolder = "shaderpacks"
			case "datapack":
				subFolder = "datapacks"
			}
			fullPath := filepath.Join(gameDir, subFolder, filename)
			if goruntime.GOOS == "windows" {
				return exec.Command("explorer.exe", "/select,"+fullPath).Start()
			}
			a.OpenURL(filepath.Dir(fullPath))
			return nil
		}
	}
	return fmt.Errorf("instance not found")
}

// PickInstanceIcon opens a file dialog to choose an icon for an instance.
func (a *App) PickInstanceIcon(instanceID string) (string, error) {
	list := a.loadInstances()
	var target *Instance
	for i := range list {
		if list[i].ID == instanceID {
			target = &list[i]
			break
		}
	}
	if target == nil {
		return "", fmt.Errorf("instance not found")
	}

	p, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Выберите иконку сборки",
		Filters: []runtime.FileFilter{
			{DisplayName: "Изображения (*.png, *.jpg, *.webp)", Pattern: "*.png;*.jpg;*.jpeg;*.webp"},
		},
	})
	if err != nil || p == "" {
		return "", nil
	}

	data, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}

	gameDir := a.instanceGameDir(*target)
	_ = os.MkdirAll(gameDir, 0o755)
	destIcon := filepath.Join(gameDir, "icon.png")
	_ = os.WriteFile(destIcon, data, 0o644)

	base64Icon := "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)
	target.Icon = base64Icon
	_ = a.saveInstances(list)

	return base64Icon, nil
}

// UpdateInstanceSettings updates name, server address, version, loader, and loader version.
func (a *App) UpdateInstanceSettings(instanceID, name, serverAddress, versionID, loader, loaderVersion, group string) (*Instance, error) {
	list := a.loadInstances()
	var target *Instance
	for i := range list {
		if list[i].ID == instanceID {
			target = &list[i]
			break
		}
	}
	if target == nil {
		return nil, fmt.Errorf("instance not found")
	}
	name = strings.TrimSpace(name)
	if name != "" {
		target.Name = name
	}
	target.ServerAddress = strings.TrimSpace(serverAddress)
	target.Group = strings.TrimSpace(group)
	versionID = strings.TrimSpace(versionID)
	if versionID != "" {
		target.VersionID = versionID
	}
	loader = strings.TrimSpace(loader)
	if loader != "" {
		target.Loader = loader
		if loader == "vanilla" {
			target.LoaderVersion = ""
		} else {
			target.LoaderVersion = strings.TrimSpace(loaderVersion)
		}
	}
	_ = a.saveInstances(list)
	runtime.EventsEmit(a.ctx, "instances-updated", a.loadInstances())
	return target, nil
}

// ScreenshotItem represents one screenshot taken in the instance.
type ScreenshotItem struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	ModTime  int64  `json:"modTime"`
	DataURL  string `json:"dataUrl"`
}

// GetInstanceScreenshots scans the instance's screenshots directory.
func (a *App) GetInstanceScreenshots(instanceID string) ([]ScreenshotItem, error) {
	for _, inst := range a.loadInstances() {
		if inst.ID == instanceID {
			gameDir := a.instanceGameDir(inst)
			sDir := filepath.Join(gameDir, "screenshots")
			entries, err := os.ReadDir(sDir)
			if err != nil {
				return []ScreenshotItem{}, nil
			}
			var items []ScreenshotItem
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				name := e.Name()
				lower := strings.ToLower(name)
				if strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") || strings.HasSuffix(lower, ".webp") {
					info, err := e.Info()
					if err != nil {
						continue
					}
					fullPath := filepath.Join(sDir, name)
					imgBytes, err := os.ReadFile(fullPath)
					if err != nil {
						continue
					}
					mime := "image/png"
					if strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") {
						mime = "image/jpeg"
					} else if strings.HasSuffix(lower, ".webp") {
						mime = "image/webp"
					}
					dataURL := fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(imgBytes))
					items = append(items, ScreenshotItem{
						Filename: name,
						Size:     info.Size(),
						ModTime:  info.ModTime().Unix(),
						DataURL:  dataURL,
					})
				}
			}
			sort.Slice(items, func(i, j int) bool {
				return items[i].ModTime > items[j].ModTime
			})
			return items, nil
		}
	}
	return nil, fmt.Errorf("instance not found")
}

// DeleteInstanceScreenshot removes a screenshot.
func (a *App) DeleteInstanceScreenshot(instanceID, filename string) error {
	for _, inst := range a.loadInstances() {
		if inst.ID == instanceID {
			gameDir := a.instanceGameDir(inst)
			target := filepath.Join(gameDir, "screenshots", filename)
			return os.Remove(target)
		}
	}
	return fmt.Errorf("instance not found")
}

// OpenScreenshotsFolder opens Windows Explorer inside instance's screenshots folder.
func (a *App) OpenScreenshotsFolder(instanceID string) error {
	return a.OpenInstanceSubFolder(instanceID, "screenshots")
}

// CloneInstance duplicates a build, copying its game folder to a new instance.
func (a *App) CloneInstance(instanceID string) (*Instance, error) {
	list := a.loadInstances()
	var src *Instance
	for i := range list {
		if list[i].ID == instanceID {
			cp := list[i]
			src = &cp
			break
		}
	}
	if src == nil {
		return nil, fmt.Errorf("instance not found")
	}

	newName := strings.TrimSpace(src.Name) + " (Copy)"
	newID := instanceSlug(newName, list)
	newDir := a.l.InstanceDir(newID)

	srcDir := a.instanceGameDir(*src)
	if err := copyTree(srcDir, newDir); err != nil {
		return nil, fmt.Errorf("copy game folder: %w", err)
	}
	// Remove volatile/crash artifacts from the clone; keep icon.
	for _, p := range []string{"logs", "crash-reports"} {
		_ = os.RemoveAll(filepath.Join(newDir, p))
	}

	newInst := Instance{
		ID:              newID,
		Name:            newName,
		VersionID:       src.VersionID,
		Loader:          src.Loader,
		LoaderVersion:   src.LoaderVersion,
		Dir:             newID,
		Created:         time.Now().Unix(),
		ServerAddress:   src.ServerAddress,
		RAMMB:           src.RAMMB,
		JavaPath:        src.JavaPath,
		JVMArgs:         src.JVMArgs,
		UseCustomWindow: src.UseCustomWindow,
		Fullscreen:      src.Fullscreen,
		WindowWidth:     src.WindowWidth,
		WindowHeight:    src.WindowHeight,
	}
	if iconData, err := os.ReadFile(filepath.Join(newDir, "icon.png")); err == nil && len(iconData) > 0 {
		newInst.Icon = "data:image/png;base64," + base64.StdEncoding.EncodeToString(iconData)
	}

	list = append(list, newInst)
	if err := a.saveInstances(list); err != nil {
		return nil, err
	}
	runtime.EventsEmit(a.ctx, "instances-updated", a.loadInstances())
	return &newInst, nil
}

// copyTree recursively copies src into dst (dst is created if missing).
func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable entries rather than fail the whole clone
		}
		rel, rerr := filepath.Rel(src, path)
		if rerr != nil {
			return nil
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm()|0o111)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return nil
		}
		in, ierr := os.Open(path)
		if ierr != nil {
			return nil
		}
		defer in.Close()
		out, oerr := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
		if oerr != nil {
			return nil
		}
		defer out.Close()
		_, _ = io.Copy(out, in)
		return nil
	})
}

// CrashReport is one entry from the crash-reports/ folder.
type CrashReport struct {
	Filename string `json:"filename"`
	ModTime  int64  `json:"modTime"`
	Size     int64  `json:"size"`
	Summary  string `json:"summary"` // first meaningful error line
	Content  string `json:"content"` // full text (bounded)
}

// GetInstanceCrashReports returns crash reports for the instance.
func (a *App) GetInstanceCrashReports(instanceID string) ([]CrashReport, error) {
	for _, inst := range a.loadInstances() {
		if inst.ID == instanceID {
			gameDir := a.instanceGameDir(inst)
			crashDir := filepath.Join(gameDir, "crash-reports")
			entries, err := os.ReadDir(crashDir)
			if err != nil {
				return []CrashReport{}, nil
			}
			var items []CrashReport
			for _, e := range entries {
				if e.IsDir() || !strings.HasSuffix(e.Name(), ".txt") {
					continue
				}
				info, ierr := e.Info()
				if ierr != nil {
					continue
				}
				data, derr := os.ReadFile(filepath.Join(crashDir, e.Name()))
				if derr != nil {
					continue
				}
				content := string(data)
				summary := crashSummary(content)
				if content != "" && len(content) > 30000 {
					content = content[len(content)-30000:]
				}
				items = append(items, CrashReport{
					Filename: e.Name(),
					ModTime:  info.ModTime().Unix(),
					Size:     info.Size(),
					Summary:  summary,
					Content:  content,
				})
			}
			sort.Slice(items, func(i, j int) bool { return items[i].ModTime > items[j].ModTime })
			return items, nil
		}
	}
	return nil, fmt.Errorf("instance not found")
}

// crashSummary extracts a short human-readable line from a crash report.
func crashSummary(content string) string {
	for _, line := range strings.Split(content, "\n") {
		l := strings.TrimSpace(line)
		if strings.HasPrefix(l, "Description: ") {
			return strings.TrimPrefix(l, "Description: ")
		}
		if strings.Contains(strings.ToLower(l), "crash report") {
			continue
		}
		if strings.HasPrefix(l, "---- ") && strings.Contains(l, "----") && !strings.HasPrefix(l, "---- Minecraft") {
			continue
		}
	}
	return ""
}

// UpdateInstanceLaunchConfig persists per-instance launch overrides.
func (a *App) UpdateInstanceLaunchConfig(instanceID string, ramMB int, javaPath, jvmArgs, jvmPreset string, useCustomWindow, fullscreen bool, winW, winH int) (*Instance, error) {
	list := a.loadInstances()
	var target *Instance
	for i := range list {
		if list[i].ID == instanceID {
			target = &list[i]
			break
		}
	}
	if target == nil {
		return nil, fmt.Errorf("instance not found")
	}
	if ramMB < 0 {
		ramMB = 0 // 0 = inherit global
	}
	if ramMB > 0 && ramMB < 512 {
		ramMB = 512
	}
	if ramMB > 32768 {
		ramMB = 32768
	}
	target.RAMMB = ramMB
	target.JavaPath = strings.TrimSpace(javaPath)
	target.JVMPreset = strings.TrimSpace(jvmPreset)
	target.JVMArgs = strings.TrimSpace(jvmArgs)
	target.UseCustomWindow = useCustomWindow
	target.Fullscreen = fullscreen
	if winW >= 320 {
		target.WindowWidth = winW
	}
	if winH >= 240 {
		target.WindowHeight = winH
	}
	_ = a.saveInstances(list)
	runtime.EventsEmit(a.ctx, "instances-updated", a.loadInstances())
	return target, nil
}

// ExportInstance packages the instance into an archive (.mrpack or .zip).
func (a *App) ExportInstance(instanceID string) (string, error) {
	var target *Instance
	list := a.loadInstances()
	for i := range list {
		if list[i].ID == instanceID {
			target = &list[i]
			break
		}
	}
	if target == nil {
		return "", fmt.Errorf("instance not found")
	}

	savePath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Экспорт сборки",
		DefaultFilename: target.Name + ".mrpack",
		Filters: []runtime.FileFilter{
			{DisplayName: "Modrinth Modpack (*.mrpack)", Pattern: "*.mrpack"},
			{DisplayName: "Zip Archive (*.zip)", Pattern: "*.zip"},
		},
	})
	if err != nil || savePath == "" {
		return "", nil
	}

	gameDir := a.instanceGameDir(*target)
	outFile, err := os.Create(savePath)
	if err != nil {
		return "", err
	}
	defer outFile.Close()

	zw := zip.NewWriter(outFile)
	defer zw.Close()

	// Folders to include: mods, config, resourcepacks, shaderpacks, defaultconfigs, kubejs, scripts
	foldersToInclude := []string{"mods", "config", "resourcepacks", "shaderpacks", "defaultconfigs", "kubejs", "scripts"}
	for _, folder := range foldersToInclude {
		fPath := filepath.Join(gameDir, folder)
		if st, err := os.Stat(fPath); err == nil && st.IsDir() {
			_ = filepath.Walk(fPath, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				rel, err := filepath.Rel(gameDir, path)
				if err != nil {
					return nil
				}
				if strings.Contains(rel, ".wailauncher") {
					return nil
				}
				zipRel := filepath.ToSlash(filepath.Join("overrides", rel))
				w, err := zw.Create(zipRel)
				if err != nil {
					return nil
				}
				data, err := os.ReadFile(path)
				if err == nil {
					_, _ = w.Write(data)
				}
				return nil
			})
		}
	}

	// Include icon.png if exists
	iconPath := filepath.Join(gameDir, "icon.png")
	if iconData, err := os.ReadFile(iconPath); err == nil && len(iconData) > 0 {
		w, err := zw.Create("icon.png")
		if err == nil {
			_, _ = w.Write(iconData)
		}
	}

	// Create modrinth.index.json
	deps := map[string]string{
		"minecraft": target.VersionID,
	}
	if target.Loader != "" && target.Loader != "vanilla" {
		if target.Loader == "fabric" {
			deps["fabric-loader"] = target.LoaderVersion
		} else if target.Loader == "forge" {
			deps["forge"] = target.LoaderVersion
		} else if target.Loader == "neoforge" {
			deps["neoforge"] = target.LoaderVersion
		}
	}

	indexManifest := map[string]any{
		"formatVersion": 1,
		"game":          "minecraft",
		"versionId":     target.VersionID,
		"name":          target.Name,
		"summary":       "Exported from WaiLauncher",
		"files":         []any{},
		"dependencies":  deps,
	}

	manifestBytes, _ := json.MarshalIndent(indexManifest, "", "  ")
	if w, err := zw.Create("modrinth.index.json"); err == nil {
		_, _ = w.Write(manifestBytes)
	}

	return savePath, nil
}

// ImportInstanceDialog opens file dialog to pick an instance archive to import.
func (a *App) ImportInstanceDialog() (*Instance, error) {
	p, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Импорт сборки",
		Filters: []runtime.FileFilter{
			{DisplayName: "Сборки Minecraft (*.mrpack, *.zip)", Pattern: "*.mrpack;*.zip"},
		},
	})
	if err != nil || p == "" {
		return nil, nil
	}
	return a.ImportInstanceFile(p)
}

// ImportInstanceFile imports a modpack from a local file path (e.g. from dialog or drag-and-drop).
func (a *App) ImportInstanceFile(filePath string) (*Instance, error) {
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(fileData), int64(len(fileData)))
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}

	var mrIndex *launcher.ModrinthIndexManifest
	var cfManifest *launcher.CurseForgeManifest

	for _, f := range zr.File {
		if f.Name == "modrinth.index.json" {
			rc, err := f.Open()
			if err == nil {
				var idx launcher.ModrinthIndexManifest
				if json.NewDecoder(rc).Decode(&idx) == nil {
					mrIndex = &idx
				}
				rc.Close()
			}
		} else if f.Name == "manifest.json" {
			rc, err := f.Open()
			if err == nil {
				var mf launcher.CurseForgeManifest
				if json.NewDecoder(rc).Decode(&mf) == nil {
					cfManifest = &mf
				}
				rc.Close()
			}
		}
	}

	baseName := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	packName := baseName
	mcVer := a.set.SelectedVersion
	if mcVer == "" {
		mcVer = "1.21.4"
	}
	loader := "vanilla"
	loaderVer := ""

	if mrIndex != nil {
		if mrIndex.Name != "" {
			packName = mrIndex.Name
		}
		if mrIndex.Dependencies["minecraft"] != "" {
			mcVer = mrIndex.Dependencies["minecraft"]
		} else if mrIndex.VersionID != "" {
			mcVer = mrIndex.VersionID
		}
		if v, ok := mrIndex.Dependencies["fabric-loader"]; ok && v != "" {
			loader = "fabric"
			loaderVer = v
		} else if v, ok := mrIndex.Dependencies["neoforge"]; ok && v != "" {
			loader = "neoforge"
			loaderVer = v
		} else if v, ok := mrIndex.Dependencies["forge"]; ok && v != "" {
			loader = "forge"
			loaderVer = v
		}
	} else if cfManifest != nil {
		if cfManifest.Name != "" {
			packName = cfManifest.Name
		}
		if cfManifest.Minecraft.Version != "" {
			mcVer = cfManifest.Minecraft.Version
		}
		for _, ml := range cfManifest.Minecraft.ModLoaders {
			if strings.HasPrefix(ml.ID, "forge-") {
				loader = "forge"
				loaderVer = strings.TrimPrefix(ml.ID, "forge-")
			} else if strings.HasPrefix(ml.ID, "neoforge-") {
				loader = "neoforge"
				loaderVer = strings.TrimPrefix(ml.ID, "neoforge-")
			} else if strings.HasPrefix(ml.ID, "fabric-") {
				loader = "fabric"
				loaderVer = strings.TrimPrefix(ml.ID, "fabric-")
			}
		}
	}

	list := a.loadInstances()
	instID := instanceSlug(packName, list)
	instDir := a.l.InstanceDir(instID)
	if err := os.MkdirAll(instDir, 0o755); err != nil {
		return nil, err
	}
	_ = os.MkdirAll(filepath.Join(instDir, "mods"), 0o755)

	// Extract files
	for _, f := range zr.File {
		cleanName := filepath.Clean(f.Name)
		var relPath string
		if strings.HasPrefix(cleanName, "overrides/") || strings.HasPrefix(cleanName, "overrides\\") {
			relPath = cleanName[10:]
		} else if strings.HasPrefix(cleanName, "client-overrides/") || strings.HasPrefix(cleanName, "client-overrides\\") {
			relPath = cleanName[17:]
		} else if !strings.Contains(cleanName, "manifest.json") && !strings.Contains(cleanName, "modrinth.index.json") {
			relPath = cleanName
		}
		if relPath != "" {
			target := filepath.Join(instDir, relPath)
			if f.FileInfo().IsDir() {
				_ = os.MkdirAll(target, 0o755)
			} else {
				_ = os.MkdirAll(filepath.Dir(target), 0o755)
				rc, err := f.Open()
				if err == nil {
					out, err := os.Create(target)
					if err == nil {
						_, _ = io.Copy(out, rc)
						out.Close()
					}
					rc.Close()
				}
			}
		}
	}

	newInst := Instance{
		ID:            instID,
		Name:          packName,
		VersionID:     mcVer,
		Loader:        loader,
		LoaderVersion: loaderVer,
		Dir:           instID,
		Created:       time.Now().Unix(),
	}

	// Check icon
	iconPng := filepath.Join(instDir, "icon.png")
	if iconBytes, err := os.ReadFile(iconPng); err == nil && len(iconBytes) > 0 {
		newInst.Icon = "data:image/png;base64," + base64.StdEncoding.EncodeToString(iconBytes)
	}

	list = append(list, newInst)
	_ = a.saveInstances(list)
	a.set.ActiveInstance = instID
	a.set.SelectedVersion = mcVer
	_ = a.set.save()
	runtime.EventsEmit(a.ctx, "instances-updated", a.loadInstances())

	return &newInst, nil
}

// CreateGroup creates a new custom folder / group if it doesn't already exist.
func (a *App) CreateGroup(name string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("group name cannot be empty")
	}
	for _, g := range a.set.Groups {
		if strings.EqualFold(g, name) {
			return nil
		}
	}
	a.set.Groups = append(a.set.Groups, name)
	if err := a.set.save(); err != nil {
		return err
	}
	runtime.EventsEmit(a.ctx, "settings-updated", a.set)
	return nil
}

// RenameGroup renames a group and updates all instances belonging to it.
func (a *App) RenameGroup(oldName, newName string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)
	if oldName == "" || newName == "" {
		return fmt.Errorf("group names cannot be empty")
	}

	found := false
	for i, g := range a.set.Groups {
		if strings.EqualFold(g, oldName) {
			a.set.Groups[i] = newName
			found = true
			break
		}
	}
	if !found {
		a.set.Groups = append(a.set.Groups, newName)
	}
	_ = a.set.save()

	list := a.loadInstances()
	changed := false
	for i := range list {
		if strings.EqualFold(list[i].Group, oldName) {
			list[i].Group = newName
			changed = true
		}
	}
	if changed {
		_ = a.saveInstances(list)
	}

	runtime.EventsEmit(a.ctx, "settings-updated", a.set)
	runtime.EventsEmit(a.ctx, "instances-updated", a.loadInstances())
	return nil
}

// DeleteGroup removes a group and unassigns instances belonging to it.
func (a *App) DeleteGroup(name string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}

	newGroups := make([]string, 0, len(a.set.Groups))
	for _, g := range a.set.Groups {
		if !strings.EqualFold(g, name) {
			newGroups = append(newGroups, g)
		}
	}
	a.set.Groups = newGroups
	_ = a.set.save()

	list := a.loadInstances()
	changed := false
	for i := range list {
		if strings.EqualFold(list[i].Group, name) {
			list[i].Group = ""
			changed = true
		}
	}
	if changed {
		_ = a.saveInstances(list)
	}

	runtime.EventsEmit(a.ctx, "settings-updated", a.set)
	runtime.EventsEmit(a.ctx, "instances-updated", a.loadInstances())
	return nil
}

// UpdateInstanceGroup updates the folder/category name of an instance.
func (a *App) UpdateInstanceGroup(instanceID, group string) (*Instance, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	group = strings.TrimSpace(group)
	list := a.loadInstances()
	var found *Instance
	for i := range list {
		if list[i].ID == instanceID {
			list[i].Group = group
			found = &list[i]
			break
		}
	}
	if found == nil {
		return nil, fmt.Errorf("instance not found: %s", instanceID)
	}
	if err := a.saveInstances(list); err != nil {
		return nil, err
	}

	if group != "" {
		hasGroup := false
		for _, g := range a.set.Groups {
			if strings.EqualFold(g, group) {
				hasGroup = true
				break
			}
		}
		if !hasGroup {
			a.set.Groups = append(a.set.Groups, group)
			_ = a.set.save()
			runtime.EventsEmit(a.ctx, "settings-updated", a.set)
		}
	}

	runtime.EventsEmit(a.ctx, "instances-updated", a.loadInstances())
	return found, nil
}

// ReorderInstances updates the sort order of instances based on the ordered list of IDs.
func (a *App) ReorderInstances(orderedIDs []string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	list := a.loadInstances()
	idMap := make(map[string]int, len(orderedIDs))
	for idx, id := range orderedIDs {
		idMap[id] = idx
	}

	for i := range list {
		if idx, ok := idMap[list[i].ID]; ok {
			list[i].SortOrder = idx
		}
	}

	sort.SliceStable(list, func(i, j int) bool {
		return list[i].SortOrder < list[j].SortOrder
	})

	if err := a.saveInstances(list); err != nil {
		return err
	}
	runtime.EventsEmit(a.ctx, "instances-updated", a.loadInstances())
	return nil
}

// VerifyInstanceFiles forcibly checks SHA1 and sizes of client jar, libraries, and assets for an instance.
func (a *App) VerifyInstanceFiles(instanceID string) (VerifyResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	var target *Instance
	for _, inst := range a.loadInstances() {
		if inst.ID == instanceID {
			target = &inst
			break
		}
	}
	if target == nil {
		return VerifyResult{}, fmt.Errorf("instance not found: %s", instanceID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	emit := func(p launcher.ProgressEvent) {
		runtime.EventsEmit(a.ctx, "verify-progress", p)
	}

	var v *launcher.VersionJSON
	var err error
	if target.Loader != "" && target.Loader != "vanilla" {
		v, err = a.l.ResolveLoaderVersion(ctx, target.Loader, target.LoaderVersion, target.VersionID, emit, nil)
	} else {
		v, err = a.l.LoadVersion(ctx, target.VersionID)
	}
	if err != nil {
		return VerifyResult{}, fmt.Errorf("load version manifest: %w", err)
	}

	total, repaired, err := a.l.VerifyAndRepair(ctx, v, emit)
	if err != nil {
		return VerifyResult{TotalChecked: total, Repaired: repaired, Failed: 1, Details: []string{err.Error()}}, err
	}

	return VerifyResult{
		TotalChecked: total,
		Repaired:     repaired,
		Failed:       0,
		Details:      []string{fmt.Sprintf("Проверено %d файлов. Все компоненты сборки целостны.", total)},
	}, nil
}
