package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AccountManager manages local accounts persistence and lifecycle.
type AccountManager struct {
	mu       sync.RWMutex
	root     string
	data     AccountsData
	clientID string
}

// NewManager creates a new AccountManager rooted at dataDir.
func NewManager(root string, defaultUsername string) (*AccountManager, error) {
	if root == "" {
		root = "data"
	}
	m := &AccountManager{
		root:     root,
		clientID: DefaultMSAClientID,
	}

	if err := m.load(defaultUsername); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *AccountManager) accountsFilePath() string {
	return filepath.Join(m.root, "accounts.json")
}

// load reads accounts.json or initializes a default offline account.
func (m *AccountManager) load(defaultUsername string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p := m.accountsFilePath()
	bytes, err := os.ReadFile(p)
	if err == nil {
		if err := json.Unmarshal(bytes, &m.data); err == nil && len(m.data.Accounts) > 0 {
			// Ensure valid active ID
			found := false
			for _, a := range m.data.Accounts {
				if a.ID == m.data.ActiveID {
					found = true
					break
				}
			}
			if !found && len(m.data.Accounts) > 0 {
				m.data.ActiveID = m.data.Accounts[0].ID
			}
			return nil
		}
	}

	// Create initial offline account
	if defaultUsername == "" {
		defaultUsername = "Player"
	}
	uuid := OfflineUUID(defaultUsername)
	initialAcc := Account{
		ID:        "off-" + uuid,
		Type:      AccountTypeOffline,
		Username:  defaultUsername,
		UUID:      uuid,
		AvatarURL: AvatarURL(defaultUsername),
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
	}

	m.data = AccountsData{
		Accounts: []Account{initialAcc},
		ActiveID: initialAcc.ID,
	}

	return m.saveLocked()
}

func (m *AccountManager) saveLocked() error {
	if err := os.MkdirAll(m.root, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.accountsFilePath(), data, 0o644)
}

// GetData returns a copy of current accounts and active ID.
func (m *AccountManager) GetData() AccountsData {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := AccountsData{
		ActiveID: m.data.ActiveID,
		Accounts: make([]Account, len(m.data.Accounts)),
	}
	copy(out.Accounts, m.data.Accounts)
	return out
}

// GetActive returns a pointer to the active account, or nil if none.
func (m *AccountManager) GetActive() (*Account, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, a := range m.data.Accounts {
		if a.ID == m.data.ActiveID {
			cpy := a
			return &cpy, nil
		}
	}

	if len(m.data.Accounts) > 0 {
		cpy := m.data.Accounts[0]
		return &cpy, nil
	}

	return nil, errors.New("no accounts available")
}

// SetActive marks an account as active and updates LastUsed.
func (m *AccountManager) SetActive(id string) (*Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var active *Account
	for i := range m.data.Accounts {
		if m.data.Accounts[i].ID == id {
			m.data.Accounts[i].LastUsed = time.Now()
			m.data.ActiveID = id
			cpy := m.data.Accounts[i]
			active = &cpy
			break
		}
	}

	if active == nil {
		return nil, fmt.Errorf("account not found: %s", id)
	}

	if err := m.saveLocked(); err != nil {
		return nil, err
	}
	return active, nil
}

// AddOffline creates or activates an offline account with given username.
func (m *AccountManager) AddOffline(username string) (*Account, error) {
	if err := ValidateUsername(username); err != nil {
		return nil, err
	}

	uuid := OfflineUUID(username)
	accID := "off-" + uuid

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already exists
	for i := range m.data.Accounts {
		if m.data.Accounts[i].Type == AccountTypeOffline && m.data.Accounts[i].Username == username {
			m.data.Accounts[i].LastUsed = time.Now()
			m.data.ActiveID = m.data.Accounts[i].ID
			_ = m.saveLocked()
			cpy := m.data.Accounts[i]
			return &cpy, nil
		}
	}

	newAcc := Account{
		ID:        accID,
		Type:      AccountTypeOffline,
		Username:  username,
		UUID:      uuid,
		AvatarURL: AvatarURL(username),
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
	}

	m.data.Accounts = append(m.data.Accounts, newAcc)
	m.data.ActiveID = newAcc.ID

	if err := m.saveLocked(); err != nil {
		return nil, err
	}

	return &newAcc, nil
}

// AddMicrosoft adds or updates a Microsoft account and sets it as active.
func (m *AccountManager) AddMicrosoft(acc *Account) error {
	if acc == nil || acc.Type != AccountTypeMicrosoft {
		return errors.New("invalid microsoft account")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	found := false
	for i := range m.data.Accounts {
		if m.data.Accounts[i].ID == acc.ID || (m.data.Accounts[i].Type == AccountTypeMicrosoft && m.data.Accounts[i].UUID == acc.UUID) {
			m.data.Accounts[i] = *acc
			m.data.Accounts[i].LastUsed = time.Now()
			m.data.ActiveID = m.data.Accounts[i].ID
			found = true
			break
		}
	}

	if !found {
		m.data.Accounts = append(m.data.Accounts, *acc)
		m.data.ActiveID = acc.ID
	}

	return m.saveLocked()
}

// Remove deletes an account by ID.
func (m *AccountManager) Remove(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	idx := -1
	for i, a := range m.data.Accounts {
		if a.ID == id {
			idx = i
			break
		}
	}

	if idx == -1 {
		return errors.New("account not found")
	}

	m.data.Accounts = append(m.data.Accounts[:idx], m.data.Accounts[idx+1:]...)

	if len(m.data.Accounts) == 0 {
		// Create a fallback offline account
		fallback := Account{
			ID:        "off-" + OfflineUUID("Player"),
			Type:      AccountTypeOffline,
			Username:  "Player",
			UUID:      OfflineUUID("Player"),
			AvatarURL: AvatarURL("Player"),
			CreatedAt: time.Now(),
			LastUsed:  time.Now(),
		}
		m.data.Accounts = []Account{fallback}
		m.data.ActiveID = fallback.ID
	} else if m.data.ActiveID == id {
		m.data.ActiveID = m.data.Accounts[0].ID
	}

	return m.saveLocked()
}

// EnsureValidAccount checks and refreshes tokens if the account is Microsoft and expired.
func (m *AccountManager) EnsureValidAccount(ctx context.Context, acc *Account) error {
	if acc == nil {
		return errors.New("account is nil")
	}

	if acc.Type == AccountTypeOffline {
		return nil
	}

	if acc.Type == AccountTypeMicrosoft {
		needsRefresh := false
		if acc.MinecraftToken == nil || time.Until(acc.MinecraftToken.ExpiresAt) < 5*time.Minute {
			needsRefresh = true
		}

		if needsRefresh {
			if err := RefreshAccountTokens(ctx, m.clientID, acc); err != nil {
				return fmt.Errorf("token refresh failed: %w", err)
			}

			// Persist updated tokens
			m.mu.Lock()
			for i := range m.data.Accounts {
				if m.data.Accounts[i].ID == acc.ID {
					m.data.Accounts[i] = *acc
					break
				}
			}
			_ = m.saveLocked()
			m.mu.Unlock()
		}
	}

	return nil
}
