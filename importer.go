package main

import (
	"archive/zip"
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type DetectedInstance struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	VersionID          string `json:"versionId"`
	Loader             string `json:"loader"`
	LoaderVersion      string `json:"loaderVersion,omitempty"`
	Path               string `json:"path"`
	Icon               string `json:"icon,omitempty"`
	ModpackSource      string `json:"modpackSource,omitempty"`
	ModpackID          string `json:"modpackId,omitempty"`
	ModpackVersionID   string `json:"modpackVersionId,omitempty"`
	ModpackVersionName string `json:"modpackVersionName,omitempty"`
}

type DetectedLauncher struct {
	ID        string             `json:"id"`
	Name      string             `json:"name"`
	BasePath  string             `json:"basePath"`
	Found     bool               `json:"found"`
	Instances []DetectedInstance `json:"instances"`
}

// DetectInstalledLaunchers scans default locations for popular Minecraft launchers.
func (a *App) DetectInstalledLaunchers() ([]DetectedLauncher, error) {
	var result []DetectedLauncher

	appData := os.Getenv("APPDATA")
	userProfile := os.Getenv("USERPROFILE")

	// 1. Prism Launcher
	if appData != "" {
		prismDir := filepath.Join(appData, "PrismLauncher", "instances")
		if dl := scanPrismInstances("prism", "Prism Launcher", prismDir); dl.Found && len(dl.Instances) > 0 {
			result = append(result, dl)
		}
	}

	// 2. CurseForge
	if userProfile != "" {
		cfDir := filepath.Join(userProfile, "curseforge", "minecraft", "Instances")
		if dl := scanCurseForgeInstances("curseforge", "CurseForge", cfDir); dl.Found && len(dl.Instances) > 0 {
			result = append(result, dl)
		}
	} else if appData != "" {
		cfDir := filepath.Join(appData, "curseforge", "minecraft", "Instances")
		if dl := scanCurseForgeInstances("curseforge", "CurseForge", cfDir); dl.Found && len(dl.Instances) > 0 {
			result = append(result, dl)
		}
	}

	// 3. Modrinth App
	var mrPaths []string
	if appData != "" {
		mrPaths = append(mrPaths, filepath.Join(appData, "ModrinthApp", "profiles"))
		mrPaths = append(mrPaths, filepath.Join(appData, "com.modrinth.theseus", "profiles"))
		mrPaths = append(mrPaths, filepath.Join(appData, "Modrinth App", "profiles"))
	}
	if userProfile != "" {
		mrPaths = append(mrPaths, filepath.Join(userProfile, "AppData", "Roaming", "ModrinthApp", "profiles"))
		mrPaths = append(mrPaths, filepath.Join(userProfile, "AppData", "Roaming", "com.modrinth.theseus", "profiles"))
	}
	for _, mp := range mrPaths {
		if dl := scanModrinthAppInstances("modrinth", "Modrinth App", mp); dl.Found && len(dl.Instances) > 0 {
			result = append(result, dl)
			break
		}
	}

	// 4. MultiMC
	if appData != "" {
		mmcDir := filepath.Join(appData, "MultiMC", "instances")
		if dl := scanPrismInstances("multimc", "MultiMC", mmcDir); dl.Found && len(dl.Instances) > 0 {
			result = append(result, dl)
		}
	}

	// 5. ATLauncher
	if appData != "" {
		atlDir := filepath.Join(appData, "ATLauncher", "instances")
		if dl := scanATLauncherInstances("atlauncher", "ATLauncher", atlDir); dl.Found && len(dl.Instances) > 0 {
			result = append(result, dl)
		}
	}

	// 6. FTB App (Feed The Beast)
	localAppData := os.Getenv("LOCALAPPDATA")
	var ftbPaths []string
	if localAppData != "" {
		ftbPaths = append(ftbPaths, filepath.Join(localAppData, ".ftba", "instances"))
	}
	if userProfile != "" {
		ftbPaths = append(ftbPaths, filepath.Join(userProfile, ".ftba", "instances"))
	}
	if appData != "" {
		ftbPaths = append(ftbPaths, filepath.Join(appData, "FTBA", "instances"))
	}
	for _, fp := range ftbPaths {
		if dl := scanFTBInstances("ftb", "FTB App", fp); dl.Found && len(dl.Instances) > 0 {
			result = append(result, dl)
			break
		}
	}

	return result, nil
}

// PickLauncherFolder opens directory picker for custom launcher path.
func (a *App) PickLauncherFolder() (string, error) {
	p, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Выберите папку сборок лаунчера (instances / profiles)",
	})
	if err != nil {
		return "", err
	}
	return p, nil
}

// ScanCustomLauncherDir scans a user-selected folder for instances.
func (a *App) ScanCustomLauncherDir(dirPath string) (DetectedLauncher, error) {
	dirPath = strings.TrimSpace(dirPath)
	if dirPath == "" {
		return DetectedLauncher{}, fmt.Errorf("empty path")
	}

	name := filepath.Base(dirPath)
	if name == "" || name == "." || name == "/" || name == "\\" {
		name = "Custom Launcher"
	}

	// 1. If the selected folder IS an instance itself (contains profile.json, instance.cfg, mods/, options.txt, etc.)
	if isInstanceDir(dirPath) {
		meta := detectInstanceMeta(dirPath)
		return DetectedLauncher{
			ID:        "custom",
			Name:      meta.Name,
			BasePath:  dirPath,
			Found:     true,
			Instances: []DetectedInstance{meta},
		}, nil
	}

	// 2. Otherwise, treat dirPath as a parent launcher directory containing instances in subfolders
	// Try Prism / MMC style first
	dl := scanPrismInstances("custom", name, dirPath)
	if len(dl.Instances) > 0 {
		return dl, nil
	}

	// Try CurseForge style
	dl = scanCurseForgeInstances("custom", name, dirPath)
	if len(dl.Instances) > 0 {
		return dl, nil
	}

	// Try Modrinth App style
	dl = scanModrinthAppInstances("custom", name, dirPath)
	if len(dl.Instances) > 0 {
		return dl, nil
	}

	// Try FTB style
	dl = scanFTBInstances("custom", name, dirPath)
	if len(dl.Instances) > 0 {
		return dl, nil
	}

	// Generic folder scan
	dl = scanGenericInstances("custom", name, dirPath)
	return dl, nil
}

// ImportSelectedInstances imports one or multiple detected instance directories into WaiLauncher with progress.
func (a *App) ImportSelectedInstances(instPaths []string) ([]*Instance, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	var imported []*Instance
	list := a.loadInstances()
	total := len(instPaths)

	for i, p := range instPaths {
		inst, err := a.importSingleInstanceDirWithProgress(a.ctx, p, i+1, total)
		if err != nil {
			continue
		}
		list = append(list, *inst)
		imported = append(imported, inst)
	}

	if len(imported) > 0 {
		_ = a.saveInstances(list)
		last := imported[len(imported)-1]
		a.set.ActiveInstance = last.ID
		a.set.SelectedVersion = last.VersionID
		_ = a.set.save()
		runtime.EventsEmit(a.ctx, "instances-updated", a.loadInstances())
		runtime.EventsEmit(a.ctx, "settings-updated", a.set)
	}

	runtime.EventsEmit(a.ctx, "import-progress", map[string]any{
		"instance": "",
		"status":   "done",
		"percent":  100,
		"index":    total,
		"total":    total,
	})

	return imported, nil
}

func (a *App) importSingleInstanceDirWithProgress(ctx context.Context, srcPath string, index, total int) (*Instance, error) {
	meta := detectInstanceMeta(srcPath)

	runtime.EventsEmit(ctx, "import-progress", map[string]any{
		"instance": meta.Name,
		"status":   fmt.Sprintf("Подготовка сборки «%s»...", meta.Name),
		"percent":  10,
		"index":    index,
		"total":    total,
	})

	instID := fmt.Sprintf("imported_%d_%s", time.Now().UnixNano()%1000000, sanitizeName(meta.Name))
	instDir := a.l.InstanceDir(instID)
	if err := os.MkdirAll(instDir, 0755); err != nil {
		return nil, err
	}

	// Determine actual game folder (Prism/MultiMC have .minecraft or minecraft inside)
	sourceGameDir := srcPath
	for _, sub := range []string{".minecraft", "minecraft"} {
		chk := filepath.Join(srcPath, sub)
		if st, err := os.Stat(chk); err == nil && st.IsDir() {
			sourceGameDir = chk
			break
		}
	}

	// Read all items from sourceGameDir
	entries, err := os.ReadDir(sourceGameDir)
	if err == nil && len(entries) > 0 {
		totalEntries := len(entries)
		for eIdx, entry := range entries {
			name := entry.Name()
			// Skip launcher internal files and temp caches
			if name == ".wailauncher" || name == ".fabric" || name == ".cache" ||
				name == "instance.cfg" || name == "mmc-pack.json" ||
				name == "minecraftinstance.json" || name == "profile.json" {
				continue
			}

			pct := 10 + int(float64(eIdx+1)/float64(totalEntries)*80)
			runtime.EventsEmit(ctx, "import-progress", map[string]any{
				"instance": meta.Name,
				"status":   fmt.Sprintf("Копирование: %s", name),
				"percent":  pct,
				"index":    index,
				"total":    total,
			})

			srcItem := filepath.Join(sourceGameDir, name)
			dstItem := filepath.Join(instDir, name)
			if entry.IsDir() {
				_ = copyDirectory(srcItem, dstItem)
			} else {
				_ = copySingleFile(srcItem, dstItem)
			}
		}
	}

	// Copy icon if present
	iconData := ""
	for _, icName := range []string{"icon.png", "icon.ico", "instance.png", "minecraft.png"} {
		icPath := filepath.Join(srcPath, icName)
		if b, err := os.ReadFile(icPath); err == nil && len(b) > 0 {
			iconData = "data:image/png;base64," + base64.StdEncoding.EncodeToString(b)
			_ = copySingleFile(icPath, filepath.Join(instDir, "icon.png"))
			break
		}
	}

	runtime.EventsEmit(ctx, "import-progress", map[string]any{
		"instance": meta.Name,
		"status":   "Завершение импорта...",
		"percent":  95,
		"index":    index,
		"total":    total,
	})

	inst := &Instance{
		ID:                 instID,
		Name:               meta.Name,
		VersionID:          meta.VersionID,
		Loader:             meta.Loader,
		LoaderVersion:      meta.LoaderVersion,
		Dir:                instID,
		Icon:               iconData,
		Created:            time.Now().Unix(),
		ModpackSource:      meta.ModpackSource,
		ModpackID:          meta.ModpackID,
		ModpackVersionID:   meta.ModpackVersionID,
		ModpackVersionName: meta.ModpackVersionName,
	}

	return inst, nil
}

func sanitizeName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	res := b.String()
	if len(res) > 20 {
		res = res[:20]
	}
	if res == "" {
		res = "instance"
	}
	return res
}

func detectInstanceMeta(instPath string) DetectedInstance {
	meta := DetectedInstance{
		ID:        filepath.Base(instPath),
		Name:      filepath.Base(instPath),
		VersionID: "1.21.4",
		Loader:    "vanilla",
		Path:      instPath,
	}

	// Helper to search JSON files in instPath and subfolders
	searchFiles := []string{instPath}
	for _, sub := range []string{".minecraft", "minecraft"} {
		chk := filepath.Join(instPath, sub)
		if st, err := os.Stat(chk); err == nil && st.IsDir() {
			searchFiles = append(searchFiles, chk)
		}
	}

	for _, basePath := range searchFiles {
		// 1. Check Prism/MultiMC mmc-pack.json
		mmcPack := filepath.Join(basePath, "mmc-pack.json")
		if data, err := os.ReadFile(mmcPack); err == nil {
			var pack struct {
				Components []struct {
					UID     string `json:"uid"`
					Version string `json:"version"`
				} `json:"components"`
			}
			if json.Unmarshal(data, &pack) == nil {
				for _, comp := range pack.Components {
					switch comp.UID {
					case "net.minecraft":
						if comp.Version != "" {
							meta.VersionID = comp.Version
						}
					case "net.fabricmc.fabric-loader":
						meta.Loader = "fabric"
						meta.LoaderVersion = comp.Version
					case "net.minecraftforge":
						meta.Loader = "forge"
						meta.LoaderVersion = comp.Version
					case "net.neoforged.neoforge":
						meta.Loader = "neoforge"
						meta.LoaderVersion = comp.Version
					case "org.quiltmc.quilt-loader":
						meta.Loader = "fabric"
						meta.LoaderVersion = comp.Version
					}
				}
			}
		}

		// 2. Check instance.cfg for Name / IntendedVersion
		instCfg := filepath.Join(basePath, "instance.cfg")
		if f, err := os.Open(instCfg); err == nil {
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if strings.HasPrefix(line, "name=") {
					val := strings.TrimPrefix(line, "name=")
					if val != "" {
						meta.Name = val
					}
				} else if strings.HasPrefix(line, "IntendedVersion=") {
					val := strings.TrimPrefix(line, "IntendedVersion=")
					if val != "" && meta.VersionID == "1.21.4" {
						meta.VersionID = val
					}
				}
			}
			f.Close()
		}

		// 3. Check CurseForge minecraftinstance.json
		cfJSON := filepath.Join(basePath, "minecraftinstance.json")
		if data, err := os.ReadFile(cfJSON); err == nil {
			var cf struct {
				Name          string `json:"name"`
				GameVersion   string `json:"gameVersion"`
				ProjectID     int    `json:"projectID"`
				FileID        int    `json:"fileID"`
				BaseModLoader struct {
					Name string `json:"name"`
				} `json:"baseModLoader"`
			}
			if json.Unmarshal(data, &cf) == nil {
				if cf.Name != "" {
					meta.Name = cf.Name
				}
				if cf.GameVersion != "" {
					meta.VersionID = cf.GameVersion
				}
				if cf.ProjectID > 0 {
					meta.ModpackSource = "curseforge"
					meta.ModpackID = fmt.Sprintf("%d", cf.ProjectID)
					if cf.FileID > 0 {
						meta.ModpackVersionID = fmt.Sprintf("%d", cf.FileID)
					}
				}
				ldr := strings.ToLower(cf.BaseModLoader.Name)
				if strings.HasPrefix(ldr, "forge") {
					meta.Loader = "forge"
					meta.LoaderVersion = strings.TrimPrefix(ldr, "forge-")
				} else if strings.HasPrefix(ldr, "fabric") {
					meta.Loader = "fabric"
					meta.LoaderVersion = strings.TrimPrefix(ldr, "fabric-")
				} else if strings.HasPrefix(ldr, "neoforge") {
					meta.Loader = "neoforge"
					meta.LoaderVersion = strings.TrimPrefix(ldr, "neoforge-")
				}
			}
		}

		// 4. Check CurseForge manifest.json
		cfManifest := filepath.Join(basePath, "manifest.json")
		if data, err := os.ReadFile(cfManifest); err == nil {
			var m struct {
				Name      string `json:"name"`
				Version   string `json:"version"`
				Minecraft struct {
					Version    string `json:"version"`
					ModLoaders []struct {
						ID      string `json:"id"`
						Primary bool   `json:"primary"`
					} `json:"modLoaders"`
				} `json:"minecraft"`
			}
			if json.Unmarshal(data, &m) == nil {
				if m.Name != "" {
					meta.Name = m.Name
				}
				if m.Minecraft.Version != "" {
					meta.VersionID = m.Minecraft.Version
				}
				for _, ml := range m.Minecraft.ModLoaders {
					id := strings.ToLower(ml.ID)
					if strings.HasPrefix(id, "forge") {
						meta.Loader = "forge"
						meta.LoaderVersion = strings.TrimPrefix(id, "forge-")
						break
					} else if strings.HasPrefix(id, "fabric") {
						meta.Loader = "fabric"
						meta.LoaderVersion = strings.TrimPrefix(id, "fabric-")
						break
					} else if strings.HasPrefix(id, "neoforge") {
						meta.Loader = "neoforge"
						meta.LoaderVersion = strings.TrimPrefix(id, "neoforge-")
						break
					}
				}
			}
		}

		// 5. Check Modrinth profile.json (all schema variants)
		mrJSON := filepath.Join(basePath, "profile.json")
		if data, err := os.ReadFile(mrJSON); err == nil {
			var mr struct {
				Name             string `json:"name"`
				Title            string `json:"title"`
				GameVersion      string `json:"game_version"`
				GameVerCamel     string `json:"gameVersion"`
				MinecraftVersion string `json:"minecraft_version"`
				Loader           string `json:"loader"`
				ModLoader        string `json:"modLoader"`
				LoaderVersion    string `json:"loader_version"`
				LoaderVerCamel   string `json:"loaderVersion"`
				ProjectID        string `json:"project_id"`
				VersionID        string `json:"version_id"`
				Metadata         struct {
					GameVersion   string `json:"game_version"`
					Loader        string `json:"loader"`
					LoaderVersion string `json:"loader_version"`
				} `json:"metadata"`
			}
			if json.Unmarshal(data, &mr) == nil {
				if mr.Name != "" {
					meta.Name = mr.Name
				} else if mr.Title != "" {
					meta.Name = mr.Title
				}

				gv := mr.GameVersion
				if gv == "" {
					gv = mr.GameVerCamel
				}
				if gv == "" {
					gv = mr.MinecraftVersion
				}
				if gv == "" {
					gv = mr.Metadata.GameVersion
				}
				if gv != "" {
					meta.VersionID = gv
				}

				ldr := mr.Loader
				if ldr == "" {
					ldr = mr.ModLoader
				}
				if ldr == "" {
					ldr = mr.Metadata.Loader
				}
				if ldr != "" {
					meta.Loader = strings.ToLower(ldr)
				}

				lv := mr.LoaderVersion
				if lv == "" {
					lv = mr.LoaderVerCamel
				}
				if lv == "" {
					lv = mr.Metadata.LoaderVersion
				}
				if lv != "" {
					meta.LoaderVersion = lv
				}

				if mr.ProjectID != "" {
					meta.ModpackSource = "modrinth"
					meta.ModpackID = mr.ProjectID
					meta.ModpackVersionID = mr.VersionID
				}
			}
		}

		// 6. Check Modrinth modrinth.index.json
		mrIndex := filepath.Join(basePath, "modrinth.index.json")
		if data, err := os.ReadFile(mrIndex); err == nil {
			var idx struct {
				Name         string            `json:"name"`
				Dependencies map[string]string `json:"dependencies"`
			}
			if json.Unmarshal(data, &idx) == nil {
				if idx.Name != "" {
					meta.Name = idx.Name
				}
				if mv, ok := idx.Dependencies["minecraft"]; ok && mv != "" {
					meta.VersionID = mv
				}
				if fv, ok := idx.Dependencies["forge"]; ok {
					meta.Loader = "forge"
					meta.LoaderVersion = fv
				} else if fab, ok := idx.Dependencies["fabric-loader"]; ok {
					meta.Loader = "fabric"
					meta.LoaderVersion = fab
				} else if neo, ok := idx.Dependencies["neoforge"]; ok {
					meta.Loader = "neoforge"
					meta.LoaderVersion = neo
				}
			}
		}

		// 7. Check FTB App instance.json
		ftbJSON := filepath.Join(basePath, "instance.json")
		if data, err := os.ReadFile(ftbJSON); err == nil {
			var ftb struct {
				ID          int    `json:"id"`
				Parent      int    `json:"parent"`
				Name        string `json:"name"`
				GameVersion string `json:"gameVersion"`
				ModLoader   string `json:"modLoader"`
				LoaderVer   string `json:"loaderVersion"`
				Targets     []struct {
					Name    string `json:"name"`
					Version string `json:"version"`
					Type    string `json:"type"`
				} `json:"targets"`
			}
			if json.Unmarshal(data, &ftb) == nil {
				if ftb.Name != "" {
					meta.Name = ftb.Name
				}
				if ftb.GameVersion != "" {
					meta.VersionID = ftb.GameVersion
				}
				if ftb.Parent > 0 {
					meta.ModpackSource = "ftb"
					meta.ModpackID = fmt.Sprintf("%d", ftb.Parent)
					if ftb.ID > 0 {
						meta.ModpackVersionID = fmt.Sprintf("%d", ftb.ID)
					}
				} else if ftb.ID > 0 {
					meta.ModpackSource = "ftb"
					meta.ModpackID = fmt.Sprintf("%d", ftb.ID)
				}
				if ftb.ModLoader != "" {
					meta.Loader = strings.ToLower(ftb.ModLoader)
					meta.LoaderVersion = ftb.LoaderVer
				}
				for _, t := range ftb.Targets {
					if t.Name == "minecraft" && t.Version != "" {
						meta.VersionID = t.Version
					} else if strings.Contains(strings.ToLower(t.Name), "forge") || strings.Contains(strings.ToLower(t.Name), "fabric") {
						meta.Loader = strings.ToLower(t.Name)
						meta.LoaderVersion = t.Version
					}
				}
			}
		}
	}

	// 8. Deep inspect mods/ folder if loader is still vanilla or version is 1.21.4 default
	detectMetadataFromModsDir(instPath, &meta)

	return meta
}

func detectMetadataFromModsDir(instPath string, meta *DetectedInstance) {
	var possibleModsDirs []string
	for _, sub := range []string{"mods", ".minecraft/mods", "minecraft/mods"} {
		chk := filepath.Join(instPath, sub)
		if st, err := os.Stat(chk); err == nil && st.IsDir() {
			possibleModsDirs = append(possibleModsDirs, chk)
		}
	}

	if len(possibleModsDirs) == 0 {
		return
	}

	mcVerRegex := regexp.MustCompile(`(?:mc|minecraft|_|-|\.)?(1\.\d{1,2}(?:\.\d{1,2})?)`)
	versionCounts := make(map[string]int)
	detectedLoader := ""
	hasJars := false

	for _, modsDir := range possibleModsDirs {
		entries, err := os.ReadDir(modsDir)
		if err != nil {
			continue
		}

		checkedJars := 0
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".jar") {
				continue
			}
			hasJars = true
			name := entry.Name()
			nameLower := strings.ToLower(name)

			// Extract MC version from filename
			matches := mcVerRegex.FindAllStringSubmatch(nameLower, -1)
			for _, m := range matches {
				if len(m) > 1 && m[1] != "" {
					v := m[1]
					if strings.HasPrefix(v, "1.") && len(v) >= 4 {
						versionCounts[v]++
					}
				}
			}

			if strings.Contains(nameLower, "fabric") {
				if detectedLoader == "" {
					detectedLoader = "fabric"
				}
			} else if strings.Contains(nameLower, "neoforge") {
				if detectedLoader == "" {
					detectedLoader = "neoforge"
				}
			} else if strings.Contains(nameLower, "forge") {
				if detectedLoader == "" {
					detectedLoader = "forge"
				}
			}

			// Open first few jars to inspect manifests
			if checkedJars < 8 {
				checkedJars++
				jarPath := filepath.Join(modsDir, name)
				if zr, err := zip.OpenReader(jarPath); err == nil {
					for _, f := range zr.File {
						fn := f.Name
						if fn == "mcmod.info" {
							detectedLoader = "forge"
							if rc, err := f.Open(); err == nil {
								data, _ := io.ReadAll(rc)
								rc.Close()
								var infos []struct {
									MCVersion string `json:"mcversion"`
								}
								if json.Unmarshal(data, &infos) == nil && len(infos) > 0 && infos[0].MCVersion != "" {
									versionCounts[infos[0].MCVersion] += 5
								}
							}
							break
						} else if fn == "fabric.mod.json" {
							detectedLoader = "fabric"
							break
						} else if fn == "META-INF/neoforge.mods.toml" {
							detectedLoader = "neoforge"
							break
						} else if fn == "META-INF/mods.toml" {
							if detectedLoader == "" {
								detectedLoader = "forge"
							}
						}
					}
					zr.Close()
				}
			}
		}
	}

	// Pick the most common Minecraft version
	var bestVer string
	var maxCount int
	for v, c := range versionCounts {
		if c > maxCount {
			maxCount = c
			bestVer = v
		}
	}

	if bestVer != "" && (meta.VersionID == "1.21.4" || meta.VersionID == "") {
		meta.VersionID = bestVer
	}

	if hasJars && (meta.Loader == "vanilla" || meta.Loader == "") {
		if detectedLoader != "" {
			meta.Loader = detectedLoader
		} else {
			meta.Loader = "forge"
		}
	}
}

func scanFTBInstances(id, name, baseDir string) DetectedLauncher {
	dl := DetectedLauncher{ID: id, Name: name, BasePath: baseDir}
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return dl
	}
	dl.Found = true

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		instDir := filepath.Join(baseDir, entry.Name())
		ftbFile := filepath.Join(instDir, "instance.json")
		if fileExists(ftbFile) {
			meta := detectInstanceMeta(instDir)
			dl.Instances = append(dl.Instances, meta)
		}
	}
	return dl
}

func scanPrismInstances(id, name, baseDir string) DetectedLauncher {
	dl := DetectedLauncher{ID: id, Name: name, BasePath: baseDir}
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return dl
	}
	dl.Found = true

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		instDir := filepath.Join(baseDir, entry.Name())
		cfgFile := filepath.Join(instDir, "instance.cfg")
		mmcFile := filepath.Join(instDir, "mmc-pack.json")
		if fileExists(cfgFile) || fileExists(mmcFile) {
			meta := detectInstanceMeta(instDir)
			dl.Instances = append(dl.Instances, meta)
		}
	}
	return dl
}

func scanCurseForgeInstances(id, name, baseDir string) DetectedLauncher {
	dl := DetectedLauncher{ID: id, Name: name, BasePath: baseDir}
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return dl
	}
	dl.Found = true

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		instDir := filepath.Join(baseDir, entry.Name())
		cfFile := filepath.Join(instDir, "minecraftinstance.json")
		if fileExists(cfFile) {
			meta := detectInstanceMeta(instDir)
			dl.Instances = append(dl.Instances, meta)
		}
	}
	return dl
}

func scanModrinthAppInstances(id, name, baseDir string) DetectedLauncher {
	dl := DetectedLauncher{ID: id, Name: name, BasePath: baseDir}
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return dl
	}
	dl.Found = true

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		instDir := filepath.Join(baseDir, entry.Name())
		mrFile := filepath.Join(instDir, "profile.json")
		if fileExists(mrFile) {
			meta := detectInstanceMeta(instDir)
			dl.Instances = append(dl.Instances, meta)
		}
	}
	return dl
}

func scanATLauncherInstances(id, name, baseDir string) DetectedLauncher {
	dl := DetectedLauncher{ID: id, Name: name, BasePath: baseDir}
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return dl
	}
	dl.Found = true

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		instDir := filepath.Join(baseDir, entry.Name())
		meta := detectInstanceMeta(instDir)
		dl.Instances = append(dl.Instances, meta)
	}
	return dl
}

func scanGenericInstances(id, name, baseDir string) DetectedLauncher {
	dl := DetectedLauncher{ID: id, Name: name, BasePath: baseDir, Found: true}
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return dl
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		instDir := filepath.Join(baseDir, entry.Name())
		if isInstanceDir(instDir) {
			meta := detectInstanceMeta(instDir)
			dl.Instances = append(dl.Instances, meta)
		}
	}
	return dl
}

func isInstanceDir(p string) bool {
	checks := []string{
		"profile.json",
		"minecraftinstance.json",
		"instance.cfg",
		"mmc-pack.json",
		"instance.json",
		"options.txt",
	}
	for _, c := range checks {
		if fileExists(filepath.Join(p, c)) {
			return true
		}
	}
	// Check subfolders that indicate a Minecraft instance directory
	for _, sub := range []string{"mods", "saves", "config", "resourcepacks", "shaderpacks"} {
		chk := filepath.Join(p, sub)
		if st, err := os.Stat(chk); err == nil && st.IsDir() {
			return true
		}
	}
	return false
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func copySingleFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func copyDirectory(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return nil
		}
		targetPath := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}
		return copySingleFile(path, targetPath)
	})
}
