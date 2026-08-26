package auth

import "time"

// AccountType identifies whether an account is offline or authenticated via Microsoft.
type AccountType string

const (
	AccountTypeOffline   AccountType = "offline"
	AccountTypeMicrosoft AccountType = "microsoft"
)

// MSTokenData stores Microsoft OAuth tokens.
type MSTokenData struct {
	AccessToken  string    `json:"accessToken"`
	RefreshToken string    `json:"refreshToken"`
	ExpiresAt    time.Time `json:"expiresAt"`
}

// MCTokenData stores Minecraft Services access tokens.
type MCTokenData struct {
	AccessToken string    `json:"accessToken"`
	ExpiresAt   time.Time `json:"expiresAt"`
}

// MojangCape represents a cape associated with a Minecraft profile.
type MojangCape struct {
	ID    string `json:"id"`
	State string `json:"state"` // "ACTIVE" | "INACTIVE"
	URL   string `json:"url"`
	Alias string `json:"alias"`
}

// PresetCape represents a predefined popular cape.
type PresetCape struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
	URL      string `json:"url"`
}

// Account represents a user profile in the launcher.
type Account struct {
	ID        string       `json:"id"`
	Type      AccountType  `json:"type"`
	Username  string       `json:"username"`
	UUID      string       `json:"uuid"` // Minecraft UUID
	SkinURL   string       `json:"skinUrl,omitempty"`
	SkinModel string       `json:"skinModel,omitempty"` // "classic" | "slim"
	CapeURL   string       `json:"capeUrl,omitempty"`
	Capes     []MojangCape `json:"capes,omitempty"`
	AvatarURL string       `json:"avatarUrl,omitempty"`
	CreatedAt time.Time    `json:"createdAt"`
	LastUsed  time.Time    `json:"lastUsed"`

	// Microsoft specific fields
	MicrosoftToken *MSTokenData `json:"msToken,omitempty"`
	MinecraftToken *MCTokenData `json:"mcToken,omitempty"`
	XUID           string       `json:"xuid,omitempty"`
	OwnsGame       bool         `json:"ownsGame,omitempty"`
}

// AccountsData represents the persisted accounts file payload.
type AccountsData struct {
	Accounts []Account `json:"accounts"`
	ActiveID string    `json:"activeId"`
}

// DeviceCodeResponse is returned by Microsoft OAuth device code endpoint.
type DeviceCodeResponse struct {
	DeviceCode      string `json:"deviceCode"`
	UserCode        string `json:"userCode"`
	VerificationURI string `json:"verificationUri"`
	ExpiresIn       int    `json:"expiresIn"`
	Interval        int    `json:"interval"`
	Message         string `json:"message"`
}
