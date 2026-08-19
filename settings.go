package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Settings is persisted to <root>/settings.json and bound to the frontend.
type Settings struct {
	Username        string `json:"username"`
	RAMMB           int    `json:"ramMb"`
	Resolution      string `json:"resolution"`
	JavaPath        string `json:"javaPath"`
	CloseOnLaunch   bool   `json:"closeOnLaunch"`
	ShowSnapshots   bool   `json:"showSnapshots"`
	DiscordRPC      bool   `json:"discordRpc"`
	DiscordAppID    string `json:"discordAppId"` // Discord Developer Portal application id for Rich Presence
	AutoUpdate      bool   `json:"autoUpdate"`
	LauncherUpdates bool   `json:"launcherUpdates"` // self-update: check GitHub Releases on start
	SelectedVersion string `json:"selectedVersion"`
	ActiveInstance  string `json:"activeInstance"` // id of the build ИГРАТЬ launches
	Language        string `json:"language"`       // "ru" (default) or "en"

	CenterWindow  bool   `json:"centerWindow"`

	// Window settings override
	WindowCustom bool `json:"windowCustom"`
	Fullscreen   bool `json:"fullscreen"`
	WindowWidth  int  `json:"windowWidth"`
	WindowHeight int  `json:"windowHeight"`

	root string `json:"-"`
}

func defaultSettings(root string) *Settings {
	return &Settings{
		Username:      "Player",
		RAMMB:         4096,
		Resolution:    "1920 × 1080",
		DiscordRPC:    true,
		AutoUpdate:    true,
		LauncherUpdates: true,
		CloseOnLaunch: false,
		Language:      "ru",
		CenterWindow:  true,
		WindowCustom:  false,
		Fullscreen:    false,
		WindowWidth:   854,
		WindowHeight:  480,
		root:          root,
	}
}

func loadSettings(root string) *Settings {
	s := defaultSettings(root)
	data, err := os.ReadFile(filepath.Join(root, "settings.json"))
	if err == nil {
		json.Unmarshal(data, s)
	}
	s.root = root
	return s
}

func (s *Settings) save() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.root, "settings.json"), data, 0o644)
}

// normalize clamps invalid values before saving/launching.
func (s *Settings) normalize() {
	s.Username = strings.TrimSpace(s.Username)
	if s.Username == "" {
		s.Username = "Player"
	}
	if s.RAMMB < 1024 {
		s.RAMMB = 1024
	}
	if s.RAMMB > 32768 {
		s.RAMMB = 32768
	}
	if s.Language != "ru" && s.Language != "en" {
		s.Language = "ru"
	}
	s.DiscordAppID = strings.TrimSpace(s.DiscordAppID)
}

// parseResolution turns "1920 × 1080" into (1920, 1080); (0,0) on failure.
func parseResolution(r string) (int, int) {
	r = strings.ReplaceAll(r, "×", "x")
	var w, h int
	if _, err := fmt.Sscanf(r, "%d x %d", &w, &h); err != nil || w <= 0 || h <= 0 {
		return 0, 0
	}
	return w, h
}
