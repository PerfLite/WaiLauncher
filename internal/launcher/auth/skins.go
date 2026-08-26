package auth

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"
)

var (
	textureCache   = make(map[string]string)
	textureCacheMu sync.RWMutex
)

// FetchTextureBase64 downloads an image from a URL with authentic headers and returns a data:image/png;base64 string.
func FetchTextureBase64(ctx context.Context, imgURL string) (string, error) {
	if imgURL == "" {
		return "", fmt.Errorf("empty url")
	}
	if strings.HasPrefix(imgURL, "data:image") {
		return imgURL, nil
	}

	textureCacheMu.RLock()
	if cached, ok := textureCache[imgURL]; ok {
		textureCacheMu.RUnlock()
		return cached, nil
	}
	textureCacheMu.RUnlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imgURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "image/png,image/*;q=0.9,*/*;q=0.8")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download texture error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("texture download failed HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read texture body: %w", err)
	}

	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)

	textureCacheMu.Lock()
	textureCache[imgURL] = dataURL
	textureCacheMu.Unlock()

	return dataURL, nil
}

// UploadSkinToMojang uploads skin PNG image bytes to official Mojang servers.
func UploadSkinToMojang(ctx context.Context, mcToken string, fileBytes []byte, filename, variant string) error {
	if len(fileBytes) == 0 {
		return fmt.Errorf("skin file data is empty")
	}

	variant = strings.ToLower(strings.TrimSpace(variant))
	if variant != "slim" {
		variant = "classic"
	}

	if filename == "" {
		filename = "skin.png"
	}

	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	if err := w.WriteField("variant", variant); err != nil {
		return err
	}

	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		return err
	}
	if _, err := part.Write(fileBytes); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.minecraftservices.com/minecraft/profile/skins", &b)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+mcToken)
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("upload skin request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("mojang skin upload failed (%d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// SetActiveMojangCape sets the active cape on Mojang servers for the authenticated user.
func SetActiveMojangCape(ctx context.Context, mcToken, capeID string) error {
	payload, _ := json.Marshal(map[string]string{
		"capeId": capeID,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, "https://api.minecraftservices.com/minecraft/profile/capes/active", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+mcToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("set active cape request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("mojang set cape failed (%d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// HideMojangCape disables/hides the active cape on Mojang servers.
func HideMojangCape(ctx context.Context, mcToken string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, "https://api.minecraftservices.com/minecraft/profile/capes/active", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+mcToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("hide cape request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("mojang hide cape failed (%d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// GetPresetCapes returns the list of popular community and historical capes.
func GetPresetCapes() []PresetCape {
	return []PresetCape{
		{
			ID:       "15th_anniversary",
			Name:     "15th Anniversary",
			Category: "Special",
			URL:      "https://textures.minecraft.net/texture/77e74dc48ef348f98d022b70417937dae6b91176b6d5102a35b1d3d63b2f29",
		},
		{
			ID:       "cherry_blossom",
			Name:     "Cherry Blossom",
			Category: "Special",
			URL:      "https://textures.minecraft.net/texture/242fb5364177b94916a037803a6fd349e54a5585ee5222cf64adff7d08c69ca",
		},
		{
			ID:       "vanilla",
			Name:     "Vanilla Cape",
			Category: "Special",
			URL:      "https://textures.minecraft.net/texture/3e6dd35a29ca4a387ee09e4a30e8c7159c3a3efbfa2d5d8518e388d7515b6",
		},
		{
			ID:       "migrator",
			Name:     "Migrator Cape",
			Category: "Special",
			URL:      "https://textures.minecraft.net/texture/2340c0e03dd66ff1169ded25fba270a77b85f473dd59d563236e96fa27b3",
		},
		{
			ID:       "twitch",
			Name:     "Purple Heart (Twitch)",
			Category: "Special",
			URL:      "https://textures.minecraft.net/texture/e55d2bbca7fbdfa07dfd8a87b1ef017c6999a41982b6b553ad504ccaeae81b",
		},
		{
			ID:       "tiktok",
			Name:     "Follower's Cape (TikTok)",
			Category: "Special",
			URL:      "https://textures.minecraft.net/texture/5e5dd0a248eb3a958e0a811c79e7c5364a66a7ec6a6ecb05b5f63d6b0542ab7",
		},
		{
			ID:       "minecon_2016",
			Name:     "MINECON 2016 (Enderman)",
			Category: "MINECON",
			URL:      "https://textures.minecraft.net/texture/ba3d386221d6046e727ec23f39a04a625f463c6c06a9289d045863c0a52bd48",
		},
		{
			ID:       "minecon_2015",
			Name:     "MINECON 2015 (Iron Golem)",
			Category: "MINECON",
			URL:      "https://textures.minecraft.net/texture/723e750a97bf9738ef9188e45dd980a37e584f23ee6404780bb4b313ef046ec",
		},
		{
			ID:       "minecon_2013",
			Name:     "MINECON 2013 (Piston)",
			Category: "MINECON",
			URL:      "https://textures.minecraft.net/texture/4531be4139cf1399435422896504a377fc0d099ec1b7be9e65fe8fcfc88e51",
		},
		{
			ID:       "minecon_2012",
			Name:     "MINECON 2012 (Pickaxe)",
			Category: "MINECON",
			URL:      "https://textures.minecraft.net/texture/44585145b23d9193235b8ff4fccab6b553ad504ccaeae81b5ad1bb190df0eb",
		},
		{
			ID:       "minecon_2011",
			Name:     "MINECON 2011 (Classic Red)",
			Category: "MINECON",
			URL:      "https://textures.minecraft.net/texture/8a68ca2a64e1c2e42ef998d89e51e3e576f7b19904ea8b1a8080f55502b6ec64",
		},
		{
			ID:       "optifine_white",
			Name:     "OptiFine Classic",
			Category: "Mods",
			URL:      "https://textures.minecraft.net/texture/c6ec588e89547d2f9540b6e9275b22b647f3b8e7c10b06b99be449ec936d538e",
		},
	}
}
