package auth

import (
	"os"
	"testing"
)

func TestOfflineUUID(t *testing.T) {
	tests := []struct {
		username string
		expected string
	}{
		{"Steve", "5627dd98-e6be-3c21-b8a8-e92344183641"},
		{"Player", "a01e3843-e521-3998-958a-f459800e4d11"},
	}

	for _, tt := range tests {
		got := OfflineUUID(tt.username)
		if got != tt.expected {
			t.Errorf("OfflineUUID(%q) = %q; want %q", tt.username, got, tt.expected)
		}
	}
}

func TestValidateUsername(t *testing.T) {
	if err := ValidateUsername("Player_123"); err != nil {
		t.Errorf("expected Player_123 to be valid, got %v", err)
	}
	if err := ValidateUsername(""); err == nil {
		t.Errorf("expected empty string to be invalid")
	}
	if err := ValidateUsername("TooLongUsernameForMinecraft"); err == nil {
		t.Errorf("expected >16 chars to be invalid")
	}
	if err := ValidateUsername("Invalid-Char!"); err == nil {
		t.Errorf("expected special chars to be invalid")
	}
}

func TestAccountManager(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wailauncher-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mgr, err := NewManager(tempDir, "TestPlayer")
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	data := mgr.GetData()
	if len(data.Accounts) != 1 {
		t.Fatalf("expected 1 initial account, got %d", len(data.Accounts))
	}
	if data.Accounts[0].Username != "TestPlayer" {
		t.Errorf("expected username TestPlayer, got %s", data.Accounts[0].Username)
	}
	if data.Accounts[0].Type != AccountTypeOffline {
		t.Errorf("expected account type offline, got %s", data.Accounts[0].Type)
	}

	// Add second offline account
	acc2, err := mgr.AddOffline("PlayerTwo")
	if err != nil {
		t.Fatalf("failed to add offline account: %v", err)
	}
	if acc2.Username != "PlayerTwo" {
		t.Errorf("expected username PlayerTwo, got %s", acc2.Username)
	}

	active, err := mgr.GetActive()
	if err != nil {
		t.Fatalf("failed to get active account: %v", err)
	}
	if active.ID != acc2.ID {
		t.Errorf("expected newly added account to be active, got %s", active.ID)
	}

	// Re-load from disk
	mgr2, err := NewManager(tempDir, "Default")
	if err != nil {
		t.Fatalf("failed to reload manager: %v", err)
	}
	data2 := mgr2.GetData()
	if len(data2.Accounts) != 2 {
		t.Errorf("expected 2 accounts after reload, got %d", len(data2.Accounts))
	}

	// Remove account
	if err := mgr2.Remove(acc2.ID); err != nil {
		t.Fatalf("failed to remove account: %v", err)
	}
	data3 := mgr2.GetData()
	if len(data3.Accounts) != 1 {
		t.Errorf("expected 1 account after removal, got %d", len(data3.Accounts))
	}
}
