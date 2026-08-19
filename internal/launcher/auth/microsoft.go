package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	DefaultMSAClientID = "c36a9fb6-4f2a-41ff-90bd-ae7cc92031eb"
	deviceCodeURL      = "https://login.microsoftonline.com/consumers/oauth2/v2.0/devicecode"
	tokenURL           = "https://login.microsoftonline.com/consumers/oauth2/v2.0/token"
	xblAuthURL         = "https://user.auth.xboxlive.com/user/authenticate"
	xstsAuthURL        = "https://xsts.auth.xboxlive.com/xsts/authorize"
	mcLoginURL         = "https://api.minecraftservices.com/authentication/login_with_xbox"
	mcProfileURL       = "https://api.minecraftservices.com/minecraft/profile"
	mcStoreURL         = "https://api.minecraftservices.com/entitlements/mcstore"
)

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

// RequestDeviceCode initiates Microsoft Device Authorization Grant.
func RequestDeviceCode(ctx context.Context, clientID string) (*DeviceCodeResponse, error) {
	if clientID == "" {
		clientID = DefaultMSAClientID
	}

	form := url.Values{
		"client_id": {clientID},
		"scope":     {"XboxLive.SignIn offline_access"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, deviceCodeURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create device code request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request device code: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read device code response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("device code request failed (status %d): %s", resp.StatusCode, string(body))
	}

	var raw struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURI string `json:"verification_uri"`
		ExpiresIn       int    `json:"expires_in"`
		Interval        int    `json:"interval"`
		Message         string `json:"message"`
		Error           string `json:"error"`
		ErrorDesc       string `json:"error_description"`
	}

	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse device code json: %w", err)
	}

	if raw.Error != "" {
		return nil, fmt.Errorf("device code error: %s - %s", raw.Error, raw.ErrorDesc)
	}

	interval := raw.Interval
	if interval <= 0 {
		interval = 5
	}

	return &DeviceCodeResponse{
		DeviceCode:      raw.DeviceCode,
		UserCode:        raw.UserCode,
		VerificationURI: raw.VerificationURI,
		ExpiresIn:       raw.ExpiresIn,
		Interval:        interval,
		Message:         raw.Message,
	}, nil
}

// PollDeviceToken polls the token endpoint until authorized, timed out, or canceled.
func PollDeviceToken(ctx context.Context, clientID, deviceCode string, intervalSec int) (*MSTokenData, error) {
	if clientID == "" {
		clientID = DefaultMSAClientID
	}
	if intervalSec < 1 {
		intervalSec = 5
	}

	ticker := time.NewTicker(time.Duration(intervalSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			form := url.Values{
				"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
				"client_id":   {clientID},
				"device_code": {deviceCode},
			}

			req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
			if err != nil {
				return nil, err
			}
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("Accept", "application/json")

			resp, err := httpClient.Do(req)
			if err != nil {
				// transient network error, continue polling
				continue
			}

			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				continue
			}

			var raw struct {
				AccessToken  string `json:"access_token"`
				RefreshToken string `json:"refresh_token"`
				ExpiresIn    int    `json:"expires_in"`
				Error        string `json:"error"`
				ErrorDesc    string `json:"error_description"`
			}

			if err := json.Unmarshal(body, &raw); err != nil {
				continue
			}

			switch raw.Error {
			case "":
				if raw.AccessToken != "" {
					exp := raw.ExpiresIn
					if exp <= 0 {
						exp = 3600
					}
					return &MSTokenData{
						AccessToken:  raw.AccessToken,
						RefreshToken: raw.RefreshToken,
						ExpiresAt:    time.Now().Add(time.Duration(exp) * time.Second),
					}, nil
				}
			case "authorization_pending":
				// User hasn't finished browser auth yet, keep waiting
				continue
			case "slow_down":
				// Server asked to slow down polling rate
				ticker.Reset(time.Duration(intervalSec+5) * time.Second)
				continue
			case "expired_token":
				return nil, errors.New("authorization code expired, please try again")
			case "access_denied":
				return nil, errors.New("authorization was denied by the user")
			default:
				return nil, fmt.Errorf("microsoft auth error: %s (%s)", raw.Error, raw.ErrorDesc)
			}
		}
	}
}

// RefreshMSToken refreshes Microsoft OAuth token using refresh token.
func RefreshMSToken(ctx context.Context, clientID, refreshToken string) (*MSTokenData, error) {
	if clientID == "" {
		clientID = DefaultMSAClientID
	}

	form := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {clientID},
		"refresh_token": {refreshToken},
		"scope":         {"XboxLive.SignIn offline_access"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send refresh request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read refresh response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("refresh token failed (status %d): %s", resp.StatusCode, string(body))
	}

	var raw struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Error        string `json:"error"`
		ErrorDesc    string `json:"error_description"`
	}

	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse refresh json: %w", err)
	}

	if raw.Error != "" {
		return nil, fmt.Errorf("refresh error: %s (%s)", raw.Error, raw.ErrorDesc)
	}

	exp := raw.ExpiresIn
	if exp <= 0 {
		exp = 3600
	}
	newRefresh := raw.RefreshToken
	if newRefresh == "" {
		newRefresh = refreshToken
	}

	return &MSTokenData{
		AccessToken:  raw.AccessToken,
		RefreshToken: newRefresh,
		ExpiresAt:    time.Now().Add(time.Duration(exp) * time.Second),
	}, nil
}

// AuthenticateXboxLive acquires an Xbox Live User Token (RPS).
func AuthenticateXboxLive(ctx context.Context, msAccessToken string) (xblToken string, uhs string, err error) {
	reqBody := map[string]any{
		"Properties": map[string]string{
			"AuthMethod": "RPS",
			"SiteName":   "user.auth.xboxlive.com",
			"RpsTicket":  "d=" + msAccessToken,
		},
		"RelyingParty": "http://auth.xboxlive.com",
		"TokenType":    "JWT",
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, xblAuthURL, bytes.NewReader(data))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("xbox live auth request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("xbox live auth failed (status %d): %s", resp.StatusCode, string(body))
	}

	var raw struct {
		Token         string `json:"Token"`
		DisplayClaims struct {
			XUI []struct {
				UHS string `json:"uhs"`
			} `json:"xui"`
		} `json:"DisplayClaims"`
	}

	if err := json.Unmarshal(body, &raw); err != nil {
		return "", "", fmt.Errorf("parse xbox live auth response: %w", err)
	}

	if raw.Token == "" || len(raw.DisplayClaims.XUI) == 0 || raw.DisplayClaims.XUI[0].UHS == "" {
		return "", "", errors.New("xbox live response missing token or user hash")
	}

	return raw.Token, raw.DisplayClaims.XUI[0].UHS, nil
}

// AuthorizeXSTS acquires an XSTS token for Minecraft services.
func AuthorizeXSTS(ctx context.Context, xblToken string) (xstsToken string, uhs string, xuid string, err error) {
	reqBody := map[string]any{
		"Properties": map[string]any{
			"SandboxId":  "RETAIL",
			"UserTokens": []string{xblToken},
		},
		"RelyingParty": "rp://api.minecraftservices.com/",
		"TokenType":    "JWT",
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", "", "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, xstsAuthURL, bytes.NewReader(data))
	if err != nil {
		return "", "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", "", "", fmt.Errorf("xsts request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", "", err
	}

	if resp.StatusCode != http.StatusOK {
		var errObj struct {
			XErr int64  `json:"XErr"`
			Msg  string `json:"Message"`
		}
		json.Unmarshal(body, &errObj)
		switch errObj.XErr {
		case 2148916233:
			return "", "", "", errors.New("this Microsoft account does not have an Xbox profile. Please create one on xbox.com")
		case 2148916238:
			return "", "", "", errors.New("this Microsoft account is a child account and must be added to a Family")
		default:
			return "", "", "", fmt.Errorf("xsts auth failed (status %d, error %d): %s", resp.StatusCode, errObj.XErr, string(body))
		}
	}

	var raw struct {
		Token         string `json:"Token"`
		DisplayClaims struct {
			XUI []struct {
				UHS string `json:"uhs"`
				XID string `json:"xid"`
			} `json:"xui"`
		} `json:"DisplayClaims"`
	}

	if err := json.Unmarshal(body, &raw); err != nil {
		return "", "", "", fmt.Errorf("parse xsts response: %w", err)
	}

	if raw.Token == "" || len(raw.DisplayClaims.XUI) == 0 || raw.DisplayClaims.XUI[0].UHS == "" {
		return "", "", "", errors.New("xsts response missing token or user hash")
	}

	xid := ""
	if len(raw.DisplayClaims.XUI) > 0 {
		xid = raw.DisplayClaims.XUI[0].XID
	}

	return raw.Token, raw.DisplayClaims.XUI[0].UHS, xid, nil
}

// LoginMinecraftWithXbox exchanges XSTS identity token for Minecraft Services access token.
func LoginMinecraftWithXbox(ctx context.Context, uhs, xstsToken string) (*MCTokenData, error) {
	identityToken := fmt.Sprintf("XBL3.0 x=%s;%s", uhs, xstsToken)
	reqBody := map[string]string{
		"identityToken": identityToken,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, mcLoginURL, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("minecraft login request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("minecraft login failed (status %d): %s", resp.StatusCode, string(body))
	}

	var raw struct {
		AccessToken  string `json:"access_token"`
		ExpiresIn    int    `json:"expires_in"`
		Error        string `json:"error"`
		ErrorMessage string `json:"errorMessage"`
	}

	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse minecraft login response: %w", err)
	}

	if raw.Error != "" {
		return nil, fmt.Errorf("minecraft login error: %s (%s)", raw.Error, raw.ErrorMessage)
	}

	if raw.AccessToken == "" {
		return nil, errors.New("minecraft login response missing access token")
	}

	exp := raw.ExpiresIn
	if exp <= 0 {
		exp = 86400
	}

	return &MCTokenData{
		AccessToken: raw.AccessToken,
		ExpiresAt:   time.Now().Add(time.Duration(exp) * time.Second),
	}, nil
}

// MinecraftProfile holds player details returned by Minecraft Services.
type MinecraftProfile struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Skins []struct {
		ID      string `json:"id"`
		State   string `json:"state"`
		URL     string `json:"url"`
		Variant string `json:"variant"`
	} `json:"skins"`
}

// GetMinecraftProfile fetches player UUID, nickname, and active skin.
func GetMinecraftProfile(ctx context.Context, mcAccessToken string) (*MinecraftProfile, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mcProfileURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+mcAccessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("profile request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, errors.New("Minecraft profile not found. Make sure this account owns Minecraft Java Edition")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("profile fetch failed (status %d): %s", resp.StatusCode, string(body))
	}

	var p MinecraftProfile
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, fmt.Errorf("parse profile response: %w", err)
	}

	return &p, nil
}

// CheckMinecraftEntitlements checks if the account owns Minecraft.
func CheckMinecraftEntitlements(ctx context.Context, mcAccessToken string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mcStoreURL, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+mcAccessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, nil
	}

	var raw struct {
		Items []struct {
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return false, nil
	}

	for _, item := range raw.Items {
		if strings.Contains(item.Name, "minecraft") || strings.Contains(item.Name, "product_gamepass") {
			return true, nil
		}
	}
	return len(raw.Items) > 0, nil
}

// CompleteMicrosoftAuthFlow executes the remaining Xbox + Minecraft steps from an MS token.
func CompleteMicrosoftAuthFlow(ctx context.Context, msToken *MSTokenData) (*Account, error) {
	xblToken, _, err := AuthenticateXboxLive(ctx, msToken.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("xbox live auth: %w", err)
	}

	xstsToken, uhs, xuid, err := AuthorizeXSTS(ctx, xblToken)
	if err != nil {
		return nil, fmt.Errorf("xsts auth: %w", err)
	}

	mcToken, err := LoginMinecraftWithXbox(ctx, uhs, xstsToken)
	if err != nil {
		return nil, fmt.Errorf("minecraft services login: %w", err)
	}

	profile, err := GetMinecraftProfile(ctx, mcToken.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("fetch profile: %w", err)
	}

	skinURL := ""
	for _, s := range profile.Skins {
		if s.State == "ACTIVE" || skinURL == "" {
			skinURL = s.URL
		}
	}

	owns, _ := CheckMinecraftEntitlements(ctx, mcToken.AccessToken)

	// Format UUID with hyphens if not formatted
	formattedUUID := formatUUID(profile.ID)

	acc := &Account{
		ID:             "msa-" + profile.ID,
		Type:           AccountTypeMicrosoft,
		Username:       profile.Name,
		UUID:           formattedUUID,
		SkinURL:        skinURL,
		AvatarURL:      AvatarURL(formattedUUID),
		CreatedAt:      time.Now(),
		LastUsed:       time.Now(),
		MicrosoftToken: msToken,
		MinecraftToken: mcToken,
		XUID:           xuid,
		OwnsGame:       owns,
	}

	return acc, nil
}

// RefreshAccountTokens refreshes MS and Minecraft tokens for an existing Microsoft account.
func RefreshAccountTokens(ctx context.Context, clientID string, acc *Account) error {
	if acc.Type != AccountTypeMicrosoft || acc.MicrosoftToken == nil {
		return errors.New("not a microsoft account")
	}

	// 1. Refresh MSA token if needed (or if expires in under 5 minutes)
	if time.Until(acc.MicrosoftToken.ExpiresAt) < 5*time.Minute {
		if acc.MicrosoftToken.RefreshToken == "" {
			return errors.New("no refresh token available, please log in again")
		}
		newMSToken, err := RefreshMSToken(ctx, clientID, acc.MicrosoftToken.RefreshToken)
		if err != nil {
			return fmt.Errorf("refresh microsoft token: %w", err)
		}
		acc.MicrosoftToken = newMSToken
	}

	// 2. Perform Xbox + Minecraft auth
	xblToken, _, err := AuthenticateXboxLive(ctx, acc.MicrosoftToken.AccessToken)
	if err != nil {
		return fmt.Errorf("xbox live auth: %w", err)
	}

	xstsToken, uhs, xuid, err := AuthorizeXSTS(ctx, xblToken)
	if err != nil {
		return fmt.Errorf("xsts auth: %w", err)
	}

	mcToken, err := LoginMinecraftWithXbox(ctx, uhs, xstsToken)
	if err != nil {
		return fmt.Errorf("minecraft login: %w", err)
	}

	acc.MinecraftToken = mcToken
	if xuid != "" {
		acc.XUID = xuid
	}

	// 3. Update profile details (skin, username)
	profile, err := GetMinecraftProfile(ctx, mcToken.AccessToken)
	if err == nil {
		acc.Username = profile.Name
		if profile.ID != "" {
			acc.UUID = formatUUID(profile.ID)
		}
		for _, s := range profile.Skins {
			if s.State == "ACTIVE" || acc.SkinURL == "" {
				acc.SkinURL = s.URL
			}
		}
		acc.AvatarURL = AvatarURL(acc.UUID)
	}

	acc.LastUsed = time.Now()
	return nil
}

func formatUUID(raw string) string {
	raw = strings.ReplaceAll(raw, "-", "")
	if len(raw) != 32 {
		return raw
	}
	return fmt.Sprintf("%s-%s-%s-%s-%s", raw[0:8], raw[8:12], raw[12:16], raw[16:20], raw[20:32])
}
