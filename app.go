package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"
	"time"

	"WaiLauncher/internal/launcher"
	"WaiLauncher/internal/launcher/auth"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const launcherVersion = "1.1.0"

// FilePickResult holds local file path and base64 data URL from file dialog.
type FilePickResult struct {
	FilePath string `json:"filePath"`
	DataURL  string `json:"dataUrl"`
	FileName string `json:"fileName"`
}

// VersionEntry is one row of the frontend version dropdown.
type VersionEntry struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Installed bool   `json:"installed"`
}

// StatePayload is the bootstrap snapshot the UI loads on start.
type StatePayload struct {
	Settings       Settings       `json:"settings"`
	Versions       []VersionEntry `json:"versions"`
	LatestRelease  string         `json:"latestRelease"`
	LatestSnapshot string         `json:"latestSnapshot"`
	VersionsErr    string         `json:"versionsErr"`
	LauncherVer    string         `json:"launcherVer"`
	DataDir        string         `json:"dataDir"`
	Accounts       []auth.Account `json:"accounts"`
	ActiveID       string         `json:"activeId"`
	Instances      []Instance     `json:"instances"`
	ActiveInstance string         `json:"activeInstance"`
}

// App is the Wails-bound backend service.
type App struct {
	ctx      context.Context
	l        *launcher.Launcher
	set      *Settings
	accounts *auth.AccountManager

	mu         sync.Mutex
	cancel     context.CancelFunc
	working    bool // installing/downloading
	playing    bool // game process alive
	updating   bool // launcher self-update in progress
	gameHandle *launcher.GameHandle
	authCancel context.CancelFunc
	discordRPC *launcher.DiscordRPC
	trayStopItem interface {
		Enable()
		Disable()
	}
}

// NewApp creates a new App application struct.
func NewApp() *App {
	return &App{
		discordRPC: &launcher.DiscordRPC{ClientID: ""},
	}
}

// startup is called when the app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	root, err := launcher.DefaultRoot()
	if err != nil {
		root = "data"
	}
	_ = launcher.InitLogger(root)
	l, err := launcher.New(root)
	if err != nil {
		runtime.LogError(ctx, "init data dir: "+err.Error())
		return
	}
	a.l = l
	a.set = loadSettings(root)
	a.l.Lang = a.set.Language
	if a.discordRPC != nil {
		a.discordRPC.ClientID = strings.TrimSpace(a.set.DiscordAppID)
	}

	accMgr, err := auth.NewManager(root, a.set.Username)
	if err != nil {
		runtime.LogError(ctx, "init accounts: "+err.Error())
	}
	a.accounts = accMgr

	// Synchronize active account username with settings
	if a.accounts != nil {
		if active, err := a.accounts.GetActive(); err == nil && active != nil {
			a.set.Username = active.Username
			_ = a.set.save()
		}
	}

	cleanupOldExecutable()

	a.startTray()

	a.fitWindow()
	a.migrateLegacyInstance()
	a.migrateLegacyGameDirs()

	runtime.OnFileDrop(a.ctx, func(x, y int, paths []string) {
		for _, p := range paths {
			lp := strings.ToLower(p)
			if strings.HasSuffix(lp, ".mrpack") || strings.HasSuffix(lp, ".zip") {
				runtime.EventsEmit(a.ctx, "file-drop-import", p)
				return
			}
		}
	})
}

// currentScreenSize returns the size of the current (or primary) display.
func (a *App) currentScreenSize() (int, int) {
	screens, err := runtime.ScreenGetAll(a.ctx)
	if err != nil || len(screens) == 0 {
		return 0, 0
	}
	s := screens[0]
	for _, sc := range screens {
		if sc.IsCurrent {
			s = sc
			break
		}
	}
	return s.Size.Width, s.Size.Height
}

// fitWindow shrinks the default 1280x800 window to fit small displays
// (e.g. 1366x768 laptops with the taskbar visible).
func (a *App) fitWindow() {
	sw, sh := a.currentScreenSize()
	if sw == 0 || sh == 0 {
		return
	}
	w, h := 1280, 800
	if mw := sw - 48; mw < w {
		w = max(mw, 1024)
	}
	if mh := sh - 88; mh < h {
		h = max(mh, 640)
	}
	if w != 1280 || h != 800 {
		runtime.WindowSetSize(a.ctx, w, h)
	}
	// The 800px-tall initial window can sit partially above a 768p screen;
	// re-center so the title bar stays visible.
	runtime.WindowCenter(a.ctx)
}

// emitState pushes the launch FSM state to the frontend.
func (a *App) emitState(state, stage string, pct float64, msg string) {
	runtime.EventsEmit(a.ctx, "launch", map[string]any{
		"state": state, "stage": stage, "percent": pct, "message": msg,
	})
}

// GetState returns settings + version list (cached manifest, offline-friendly) + accounts.
func (a *App) GetState() StatePayload {
	p := StatePayload{
		Settings:       *a.set,
		LauncherVer:    launcherVersion,
		DataDir:        a.l.Root,
		Instances:      a.loadInstances(),
		ActiveInstance: a.set.ActiveInstance,
	}
	if a.accounts != nil {
		accData := a.accounts.GetData()
		p.Accounts = accData.Accounts
		p.ActiveID = accData.ActiveID
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	m, err := a.l.GetManifest(ctx, false)
	if err != nil {
		p.VersionsErr = err.Error()
		return p
	}
	p.LatestRelease = m.Latest.Release
	p.LatestSnapshot = m.Latest.Snapshot
	p.Versions = a.versionEntries(m)
	return p
}

// RefreshVersions force-fetches the manifest from Mojang.
func (a *App) RefreshVersions() (StatePayload, error) {
	p := StatePayload{
		Settings:       *a.set,
		LauncherVer:    launcherVersion,
		DataDir:        a.l.Root,
		Instances:      a.loadInstances(),
		ActiveInstance: a.set.ActiveInstance,
	}
	if a.accounts != nil {
		accData := a.accounts.GetData()
		p.Accounts = accData.Accounts
		p.ActiveID = accData.ActiveID
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	m, err := a.l.GetManifest(ctx, true)
	if err != nil {
		return p, err
	}
	p.LatestRelease = m.Latest.Release
	p.LatestSnapshot = m.Latest.Snapshot
	p.Versions = a.versionEntries(m)
	return p, nil
}

// versionEntries filters the manifest: all releases, recent snapshots, plus
// anything installed locally.
func (a *App) versionEntries(m *launcher.VersionManifest) []VersionEntry {
	out := make([]VersionEntry, 0, 256)
	snapshots := 0
	for _, v := range m.Versions {
		installed := a.l.IsInstalled(v.ID)
		switch v.Type {
		case "release":
			out = append(out, VersionEntry{ID: v.ID, Type: v.Type, Installed: installed})
		case "snapshot":
			if snapshots < 40 || installed {
				out = append(out, VersionEntry{ID: v.ID, Type: v.Type, Installed: installed})
				snapshots++
			}
		default:
			if installed {
				out = append(out, VersionEntry{ID: v.ID, Type: v.Type, Installed: installed})
			}
		}
	}
	return out
}

// GetSettings returns the current settings.
func (a *App) GetSettings() Settings {
	return *a.set
}

// GetNews returns the official Minecraft launcher news (cached).
func (a *App) GetNews() ([]launcher.NewsEntry, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	return a.l.GetNews(ctx, false)
}

// GetArticle fetches and parses the full content of a minecraft.net article.
func (a *App) GetArticle(url string) (*launcher.ArticleDetails, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return a.l.GetArticle(ctx, url)
}

// TranslateArticle translates the article content to the specified language (e.g. "ru").
func (a *App) TranslateArticle(url string, lang string) (*launcher.ArticleDetails, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	return a.l.TranslateArticle(ctx, url, lang)
}

// SaveSettings validates and persists settings.
func (a *App) SaveSettings(s Settings) error {
	s.normalize()
	*a.set = s
	a.set.root = a.l.Root
	a.l.Lang = a.set.Language
	a.discordRPC.ClientID = strings.TrimSpace(a.set.DiscordAppID)
	return a.set.save()
}

// Play starts the install+launch pipeline for a build (instance)
// asynchronously. Progress is delivered via the "launch" event.
func (a *App) Play(instanceID string) error {
	a.mu.Lock()
	if a.working || a.playing {
		a.mu.Unlock()
		return fmt.Errorf("%s", launcher.T(a.set.Language, "launch.busy"))
	}

	instances := a.loadInstances()
	if len(instances) == 0 {
		a.mu.Unlock()
		return fmt.Errorf("%s", launcher.T(a.set.Language, "err.no_instance"))
	}

	instanceID = strings.TrimSpace(instanceID)
	if instanceID == "" {
		instanceID = a.set.ActiveInstance
	}

	var inst *Instance
	for _, it := range instances {
		if it.ID == instanceID {
			cp := it
			inst = &cp
			break
		}
	}
	if inst == nil {
		for _, it := range instances {
			if it.ID == a.set.ActiveInstance {
				cp := it
				inst = &cp
				break
			}
		}
		if inst == nil {
			cp := instances[0]
			inst = &cp
		}
	}
	versionID := inst.VersionID
	gameDir := a.instanceGameDir(*inst)

	a.working = true
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	a.mu.Unlock()

	a.set.ActiveInstance = inst.ID
	a.set.SelectedVersion = versionID
	_ = a.set.save()

	go func() {
		defer func() {
			a.mu.Lock()
			a.working = false
			a.playing = false
			a.cancel = nil
			a.mu.Unlock()
		}()

		emit := func(ev launcher.ProgressEvent) {
			a.emitState("working", ev.Stage, ev.Percent, ev.Message)
		}

		if a.accounts == nil {
			a.emitState("idle", "", 0, launcher.T(a.set.Language, "err.no_account"))
			return
		}

		activeAcc, err := a.accounts.GetActive()
		if err != nil || activeAcc == nil {
			a.emitState("idle", "", 0, launcher.T(a.set.Language, "err.no_account"))
			return
		}

		userType := "legacy"
		token := "0"
		uuid := activeAcc.UUID
		xuid := ""

		if activeAcc.Type == auth.AccountTypeMicrosoft {
			emit(launcher.ProgressEvent{Stage: "manifest", Message: "Microsoft Auth"})
			if err := a.accounts.EnsureValidAccount(ctx, activeAcc); err != nil {
				if ctx.Err() != nil {
					a.emitState("idle", "", 0, launcher.T(a.set.Language, "launch.cancelled"))
				} else {
					a.emitState("idle", "", 0, launcher.T(a.set.Language, "err.auth_failed", err.Error()))
				}
				return
			}
			userType = "msa"
			if activeAcc.MinecraftToken != nil {
				token = activeAcc.MinecraftToken.AccessToken
			}
			xuid = activeAcc.XUID
		}

		var w, h int
		var fullscreen bool
		if inst.UseCustomWindow {
			fullscreen = inst.Fullscreen
			w = inst.WindowWidth
			h = inst.WindowHeight
		} else if a.set.WindowCustom {
			fullscreen = a.set.Fullscreen
			w = a.set.WindowWidth
			h = a.set.WindowHeight
		} else {
			w, h = parseResolution(a.set.Resolution)
		}
		// A window larger than the display is unusable (it lands off-screen):
		// scale the requested resolution down to fit, keeping aspect ratio.
		if !fullscreen && w > 0 && h > 0 {
			if sw, sh := a.currentScreenSize(); sw > 0 && sh > 0 {
				maxW, maxH := sw-16, sh-48 // room for the frame and taskbar
				if w > maxW || h > maxH {
					scale := float64(maxW) / float64(w)
					if s2 := float64(maxH) / float64(h); s2 < scale {
						scale = s2
					}
					w = int(float64(w) * scale)
					h = int(float64(h) * scale)
					if w < 320 {
						w = 320
					}
					if h < 240 {
						h = 240
					}
				}
			}
		}
		// Per-instance overrides, falling back to global settings.
		ramMB := a.set.RAMMB
		if inst.RAMMB > 0 {
			ramMB = inst.RAMMB
		}
		javaPath := inst.JavaPath
		if javaPath == "" {
			javaPath = a.set.JavaPath
		}
		cfg := launcher.LaunchConfig{
			Username:    activeAcc.Username,
			UUID:        uuid,
			AccessToken: token,
			UserType:    userType,
			XUID:        xuid,
			RAMMB:       ramMB,
			JavaPath:    javaPath,
			ExtraJVM:    parseJVMArgs(inst.JVMArgs),
			GameDir:     gameDir,
			Width:       w,
			Height:      h,
			Fullscreen:  fullscreen,
			Server:      inst.ServerAddress,
		}

		launcher.LogInfo("Launching instance '%s' (ID: %s, Version: %s, Loader: %s, RAM: %dMB)", inst.Name, inst.ID, versionID, inst.Loader, ramMB)

		onLog := func(line string) {
			launcher.LogToFile("GAME", line)
			runtime.EventsEmit(a.ctx, "gamelog", line)
		}
		var handle *launcher.GameHandle
		if inst.Loader != "" && inst.Loader != "vanilla" {
			// Modloader build: generate/verify the merged version json first.
			var vj *launcher.VersionJSON
			vj, err = a.l.ResolveLoaderVersion(ctx, inst.Loader, inst.LoaderVersion, versionID, emit, onLog)
			if err == nil {
				handle, err = a.l.LaunchVersion(ctx, vj, cfg, emit, onLog)
			}
		} else {
			handle, err = a.l.Launch(ctx, versionID, cfg, emit, onLog)
		}
		if err != nil {
			launcher.LogError("Launch error: %v", err)
			if ctx.Err() != nil {
				a.emitState("idle", "", 0, launcher.T(a.set.Language, "launch.cancelled"))
			} else {
				a.emitState("idle", "", 0, launcher.T(a.set.Language, "launch.error", err.Error()))
			}
			return
		}
		launcher.LogInfo("Game process started successfully (PID: %d)", handle.Cmd.Process.Pid)

		a.mu.Lock()
		a.playing = true
		a.gameHandle = handle
		a.mu.Unlock()
		a.updateTrayPlaying(true)
		a.emitState("playing", "", 100, versionID)
		if a.set.CloseOnLaunch {
			runtime.WindowMinimise(a.ctx)
		}
		if a.set.DiscordRPC {
			a.discordRPC.SetActivity(inst.Name, versionID)
		}

		gameStartTime := time.Now()
		err = handle.Wait()
		a.discordRPC.Clear()
		sessionSec := int64(time.Since(gameStartTime).Seconds())

		// Update instance playtime (total + today) & lastPlayed timestamp
		today := time.Now().Format("2006-01-02")
		allInst := a.loadInstances()
		for i := range allInst {
			if allInst[i].ID == inst.ID {
				allInst[i].PlayTime += sessionSec
				if allInst[i].LastPlayDay != today {
					allInst[i].LastPlayDay = today
					allInst[i].PlayTimeToday = 0
				}
				allInst[i].PlayTimeToday += sessionSec
				allInst[i].LastPlayed = time.Now().Unix()
				break
			}
		}
		_ = a.saveInstances(allInst)
		runtime.EventsEmit(a.ctx, "instances-updated", a.loadInstances())

		a.mu.Lock()
		a.playing = false
		a.gameHandle = nil
		a.mu.Unlock()
		a.updateTrayPlaying(false)
		if a.set.CloseOnLaunch {
			runtime.WindowUnminimise(a.ctx)
			runtime.WindowShow(a.ctx)
		}
		if err != nil {
			// Surface a meaningful cause (OOM, mod conflict, ...) from the log
			// instead of a bare "the game crashed".
			crashMsg := launcher.T(a.set.Language, "launch.crash")
			if hint := a.detectCrashCause(*inst); hint != "" {
				crashMsg = hint
			}
			a.emitState("idle", "", 0, crashMsg)
		} else {
			a.emitState("idle", "", 0, launcher.T(a.set.Language, "launch.done"))
		}
	}()
	return nil
}

// parseJVMArgs splits a user-entered JVM argument string into tokens,
// respecting double quotes around values with spaces.
func parseJVMArgs(s string) []string {
	var out []string
	var cur strings.Builder
	inQuote := false
	for _, r := range strings.TrimSpace(s) {
		switch {
		case r == '"':
			inQuote = !inQuote
		case (r == ' ' || r == '\t') && !inQuote:
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// detectCrashCause scans the tail of latest.log for well-known crash
// signatures and returns a human-readable explanation, or "" when nothing
// specific was found.
func (a *App) detectCrashCause(inst Instance) string {
	data, err := os.ReadFile(filepath.Join(a.instanceGameDir(inst), "logs", "latest.log"))
	if err != nil || len(data) == 0 {
		return ""
	}
	tail := string(data)
	if len(tail) > 200000 {
		tail = tail[len(tail)-200000:]
	}
	lower := strings.ToLower(tail)

	lang := a.set.Language
	switch {
	case strings.Contains(lower, "outofmemoryerror") || strings.Contains(lower, "java.lang.outofmemoryerror"):
		return launcher.T(lang, "crash.oom")
	case strings.Contains(lower, "essential libraries are missing") ||
		strings.Contains(lower, "incompatible mods found") ||
		strings.Contains(lower, "duplicate mods found") ||
		strings.Contains(lower, "mixinapply"):
		return launcher.T(lang, "crash.mods")
	case strings.Contains(lower, "pixel format not accelerated") || strings.Contains(lower, "glfw error"):
		return launcher.T(lang, "crash.gpu")
	case strings.Contains(lower, "it appears your gpu is too weak"):
		return launcher.T(lang, "crash.glow")
	}
	// Try to quote the first FATAL/ERROR line as a hint.
	for _, line := range strings.Split(tail, "\n") {
		l := strings.TrimSpace(line)
		if li := strings.ToLower(l); strings.Contains(li, "fatal error") || strings.Contains(li, "exception in thread") {
			if len(l) > 160 {
				l = l[:160] + "…"
			}
			return launcher.T(lang, "crash.detail", l)
		}
	}
	return ""
}

// StopGame terminates a running Minecraft instance or cancels an in-progress launch.
func (a *App) StopGame() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.playing && a.gameHandle != nil && a.gameHandle.Cmd != nil && a.gameHandle.Cmd.Process != nil {
		_ = a.gameHandle.Cmd.Process.Kill()
	}
	if a.cancel != nil {
		a.cancel()
	}
}

// CancelPlay aborts an in-progress download/install or stops a running game.
func (a *App) CancelPlay() {
	a.StopGame()
}

// ---- Account Management API ----

// GetAccounts returns all configured accounts and active ID.
func (a *App) GetAccounts() auth.AccountsData {
	if a.accounts == nil {
		return auth.AccountsData{}
	}
	return a.accounts.GetData()
}

// AddOfflineAccount creates and selects a new offline account.
func (a *App) AddOfflineAccount(username string) (*auth.Account, error) {
	if a.accounts == nil {
		return nil, fmt.Errorf("accounts manager not initialized")
	}
	acc, err := a.accounts.AddOffline(username)
	if err != nil {
		return nil, err
	}
	a.set.Username = acc.Username
	_ = a.set.save()
	return acc, nil
}

// StartMicrosoftAuth initiates the Microsoft OAuth device code flow.
func (a *App) StartMicrosoftAuth() (*auth.DeviceCodeResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return auth.RequestDeviceCode(ctx, auth.DefaultMSAClientID)
}

// PollMicrosoftAuth waits for the user to complete login in the browser.
func (a *App) PollMicrosoftAuth(deviceCode string, interval int) (*auth.Account, error) {
	if a.accounts == nil {
		return nil, fmt.Errorf("accounts manager not initialized")
	}

	a.mu.Lock()
	if a.authCancel != nil {
		a.authCancel()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	a.authCancel = cancel
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		a.authCancel = nil
		a.mu.Unlock()
	}()

	msToken, err := auth.PollDeviceToken(ctx, auth.DefaultMSAClientID, deviceCode, interval)
	if err != nil {
		return nil, err
	}

	acc, err := auth.CompleteMicrosoftAuthFlow(ctx, msToken)
	if err != nil {
		return nil, err
	}

	if err := a.accounts.AddMicrosoft(acc); err != nil {
		return nil, err
	}

	a.set.Username = acc.Username
	_ = a.set.save()
	return acc, nil
}

// CancelMicrosoftAuth aborts any pending Microsoft device auth polling.
func (a *App) CancelMicrosoftAuth() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.authCancel != nil {
		a.authCancel()
		a.authCancel = nil
	}
}

// SelectAccount changes the active account.
func (a *App) SelectAccount(id string) (*auth.Account, error) {
	if a.accounts == nil {
		return nil, fmt.Errorf("accounts manager not initialized")
	}
	acc, err := a.accounts.SetActive(id)
	if err != nil {
		return nil, err
	}
	a.set.Username = acc.Username
	_ = a.set.save()
	return acc, nil
}

// RemoveAccount deletes an account by ID.
func (a *App) RemoveAccount(id string) error {
	if a.accounts == nil {
		return fmt.Errorf("accounts manager not initialized")
	}
	if err := a.accounts.Remove(id); err != nil {
		return err
	}
	if active, err := a.accounts.GetActive(); err == nil && active != nil {
		a.set.Username = active.Username
		_ = a.set.save()
	}
	return nil
}

// RefreshAccount refreshes account profile / tokens.
func (a *App) RefreshAccount(id string) (*auth.Account, error) {
	if a.accounts == nil {
		return nil, fmt.Errorf("accounts manager not initialized")
	}
	data := a.accounts.GetData()
	var target *auth.Account
	for i := range data.Accounts {
		if data.Accounts[i].ID == id {
			target = &data.Accounts[i]
			break
		}
	}
	if target == nil {
		return nil, fmt.Errorf("account not found")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err := a.accounts.EnsureValidAccount(ctx, target); err != nil {
		return nil, err
	}

	return target, nil
}

// PickSkinFile opens a dialog to select a skin PNG file.
func (a *App) PickSkinFile() (*FilePickResult, error) {
	p, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Выберите файл скина (.png)",
		Filters: []runtime.FileFilter{
			{DisplayName: "Minecraft Skin (*.png)", Pattern: "*.png"},
		},
	})
	if err != nil || p == "" {
		return nil, err
	}
	bytes, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(bytes)
	return &FilePickResult{
		FilePath: p,
		DataURL:  dataURL,
		FileName: filepath.Base(p),
	}, nil
}

// PickCapeFile opens a dialog to select a cape PNG file.
func (a *App) PickCapeFile() (*FilePickResult, error) {
	p, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Выберите файл плаща (.png)",
		Filters: []runtime.FileFilter{
			{DisplayName: "Minecraft Cape (*.png)", Pattern: "*.png"},
		},
	})
	if err != nil || p == "" {
		return nil, err
	}
	bytes, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(bytes)
	return &FilePickResult{
		FilePath: p,
		DataURL:  dataURL,
		FileName: filepath.Base(p),
	}, nil
}

// SetAccountSkin updates the skin for the given account.
func (a *App) SetAccountSkin(accountID, filePathOrDataURL, variant string) (*auth.Account, error) {
	if a.accounts == nil {
		return nil, fmt.Errorf("accounts manager not initialized")
	}
	acc, err := a.accounts.GetAccount(accountID)
	if err != nil {
		return nil, err
	}

	variant = strings.ToLower(strings.TrimSpace(variant))
	if variant != "slim" {
		variant = "classic"
	}

	var rawBytes []byte
	var filename string

	if strings.HasPrefix(filePathOrDataURL, "data:image") {
		parts := strings.SplitN(filePathOrDataURL, ",", 2)
		if len(parts) == 2 {
			rawBytes, _ = base64.StdEncoding.DecodeString(parts[1])
		}
		filename = "skin.png"
	} else if _, statErr := os.Stat(filePathOrDataURL); statErr == nil {
		rawBytes, _ = os.ReadFile(filePathOrDataURL)
		filename = filepath.Base(filePathOrDataURL)
	}

	if acc.Type == auth.AccountTypeMicrosoft && acc.MicrosoftToken != nil && len(rawBytes) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := a.accounts.EnsureValidAccount(ctx, acc); err != nil {
			return nil, fmt.Errorf("auth error: %w", err)
		}
		if err := auth.UploadSkinToMojang(ctx, acc.MinecraftToken.AccessToken, rawBytes, filename, variant); err != nil {
			return nil, fmt.Errorf("mojang upload error: %w", err)
		}
		// Refresh profile to get updated official SkinURL
		if prof, err := auth.GetMinecraftProfile(ctx, acc.MinecraftToken.AccessToken); err == nil {
			for _, s := range prof.Skins {
				if s.State == "ACTIVE" || acc.SkinURL == "" {
					acc.SkinURL = s.URL
				}
			}
			acc.SkinModel = variant
			return a.accounts.UpdateSkin(acc.ID, acc.SkinURL, variant)
		}
	}

	// For Offline or local fallback: store as dataURL or URL
	skinVal := filePathOrDataURL
	if len(rawBytes) > 0 && !strings.HasPrefix(skinVal, "data:image") {
		skinVal = "data:image/png;base64," + base64.StdEncoding.EncodeToString(rawBytes)
	}

	return a.accounts.UpdateSkin(acc.ID, skinVal, variant)
}

// SetAccountCape updates or activates a cape for the given account.
func (a *App) SetAccountCape(accountID, capeURLOrDataURL, capeID string) (*auth.Account, error) {
	if a.accounts == nil {
		return nil, fmt.Errorf("accounts manager not initialized")
	}
	acc, err := a.accounts.GetAccount(accountID)
	if err != nil {
		return nil, err
	}

	if acc.Type == auth.AccountTypeMicrosoft && acc.MicrosoftToken != nil && capeID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := a.accounts.EnsureValidAccount(ctx, acc); err != nil {
			return nil, fmt.Errorf("auth error: %w", err)
		}
		if err := auth.SetActiveMojangCape(ctx, acc.MinecraftToken.AccessToken, capeID); err != nil {
			return nil, fmt.Errorf("mojang cape error: %w", err)
		}
		if prof, err := auth.GetMinecraftProfile(ctx, acc.MinecraftToken.AccessToken); err == nil {
			acc.Capes = prof.Capes
			for _, c := range prof.Capes {
				if c.ID == capeID || c.State == "ACTIVE" {
					return a.accounts.UpdateCape(acc.ID, c.URL, prof.Capes)
				}
			}
		}
	}

	return a.accounts.UpdateCape(acc.ID, capeURLOrDataURL, nil)
}

// ClearAccountCape removes/hides the cape from the given account.
func (a *App) ClearAccountCape(accountID string) (*auth.Account, error) {
	if a.accounts == nil {
		return nil, fmt.Errorf("accounts manager not initialized")
	}
	acc, err := a.accounts.GetAccount(accountID)
	if err != nil {
		return nil, err
	}

	if acc.Type == auth.AccountTypeMicrosoft && acc.MicrosoftToken != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := a.accounts.EnsureValidAccount(ctx, acc); err == nil {
			_ = auth.HideMojangCape(ctx, acc.MinecraftToken.AccessToken)
			if prof, err := auth.GetMinecraftProfile(ctx, acc.MinecraftToken.AccessToken); err == nil {
				return a.accounts.UpdateCape(acc.ID, "", prof.Capes)
			}
		}
	}

	return a.accounts.UpdateCape(acc.ID, "", nil)
}

// GetPresetCapes returns curated list of preset capes.
func (a *App) GetPresetCapes() []auth.PresetCape {
	return auth.GetPresetCapes()
}


// ---- window controls for the custom titlebar ----

func (a *App) WindowMinimize() {
	runtime.WindowMinimise(a.ctx)
}

// WindowToggleMaximize toggles maximization and returns the new state.
func (a *App) WindowToggleMaximize() bool {
	runtime.WindowToggleMaximise(a.ctx)
	return runtime.WindowIsMaximised(a.ctx)
}

func (a *App) WindowClose() {
	runtime.Quit(a.ctx)
}

// OpenURL opens a link in the system browser.
func (a *App) OpenURL(url string) {
	runtime.BrowserOpenURL(a.ctx, url)
}

// PickJavaPath shows a file dialog to select the java executable.
func (a *App) PickJavaPath() string {
	defaultDir := ""
	if a.l != nil && a.l.Root != "" {
		javaDir := filepath.Join(a.l.Root, "java")
		_ = os.MkdirAll(javaDir, 0o755)
		defaultDir = javaDir
	}
	p, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		DefaultDirectory: defaultDir,
		Title:            launcher.T(a.set.Language, "dialog.java"),
		Filters: []runtime.FileFilter{
			{DisplayName: "Java (javaw.exe, java.exe)", Pattern: "*.exe;*"},
		},
	})
	if err != nil {
		return ""
	}
	return p
}

// PickDataDir shows a directory picker dialog to select the data folder.
func (a *App) PickDataDir() string {
	defaultDir := ""
	if a.l != nil && a.l.Root != "" {
		defaultDir = a.l.Root
	}
	p, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		DefaultDirectory: defaultDir,
		Title:            launcher.T(a.set.Language, "dialog.dataDir"),
	})
	if err != nil {
		return ""
	}
	return p
}

// OpenDataDir opens the launcher data directory in Windows Explorer / system file manager.
func (a *App) OpenDataDir() {
	if a.l != nil && a.l.Root != "" {
		_ = os.MkdirAll(a.l.Root, 0o755)
		if goruntime.GOOS == "windows" {
			exec.Command("explorer.exe", a.l.Root).Start()
		} else {
			runtime.BrowserOpenURL(a.ctx, a.l.Root)
		}
	}
}

// JavaRuntimeStatus describes the installation status of a major Java version.
type JavaRuntimeStatus struct {
	Major   int    `json:"major"`
	Found   bool   `json:"found"`
	Path    string `json:"path"`
	Managed bool   `json:"managed"`
}

// GetJavaRuntimesStatus checks availability for Java 8, 17, and 21.
func (a *App) GetJavaRuntimesStatus() []JavaRuntimeStatus {
	majors := []int{8, 17, 21}
	var res []JavaRuntimeStatus
	for _, m := range majors {
		p := a.l.FindJavaForMajor(m)
		found := p != ""
		managed := strings.Contains(strings.ToLower(p), strings.ToLower(a.l.Root))
		res = append(res, JavaRuntimeStatus{
			Major:   m,
			Found:   found,
			Path:    p,
			Managed: managed,
		})
	}
	return res
}

// InstallJavaRuntime triggers 1-click download of Adoptium OpenJDK for the specified major version.
func (a *App) InstallJavaRuntime(major int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	emit := func(ev launcher.ProgressEvent) {
		runtime.EventsEmit(a.ctx, "java-install-progress", map[string]any{
			"major":   major,
			"percent": ev.Percent,
			"message": ev.Message,
			"stage":   ev.Stage,
		})
	}
	return a.l.DownloadTemurinDirect(ctx, major, emit)
}

// CheckJavaUpdates checks managed Temurin installs against the latest Adoptium releases.
func (a *App) CheckJavaUpdates() []launcher.JavaUpdateInfo {
	if a.l == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return a.l.CheckTemurinUpdate(ctx, []int{8, 17, 21})
}

// UninstallJavaRuntime removes the managed Adoptium OpenJDK for the specified major version.
func (a *App) UninstallJavaRuntime(major int) error {
	if a.l == nil {
		return fmt.Errorf("launcher not initialized")
	}
	if err := a.l.DeleteManagedJava(major); err != nil {
		return err
	}
	managedPrefix := filepath.Join(a.l.Root, "java", fmt.Sprintf("temurin-%d", major))
	if strings.HasPrefix(strings.ToLower(a.set.JavaPath), strings.ToLower(managedPrefix)) {
		a.set.JavaPath = ""
		_ = a.SaveSettings(*a.set)
	}
	return nil
}

// OpenLogsFolder opens the directory containing launcher.log in Windows File Explorer.
func (a *App) OpenLogsFolder() error {
	if a.l == nil {
		return fmt.Errorf("launcher not initialized")
	}
	logPath := filepath.Join(a.l.Root, "launcher.log")
	if _, err := os.Stat(logPath); err == nil {
		return exec.Command("explorer", "/select,", logPath).Start()
	}
	return exec.Command("explorer", a.l.Root).Start()
}

// GetLauncherLogs returns recent content of launcher.log.
func (a *App) GetLauncherLogs() (string, error) {
	if a.l == nil {
		return "", fmt.Errorf("launcher not initialized")
	}
	logPath := filepath.Join(a.l.Root, "launcher.log")
	b, err := os.ReadFile(logPath)
	if err != nil {
		return "", err
	}
	s := string(b)
	if len(s) > 100000 {
		s = s[len(s)-100000:]
	}
	return s, nil
}

