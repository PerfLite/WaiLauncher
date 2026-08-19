package auth

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var validUsernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_]{1,16}$`)

// ValidateUsername checks if a Minecraft username meets standard player name criteria.
func ValidateUsername(username string) error {
	trimmed := strings.TrimSpace(username)
	if trimmed == "" {
		return errors.New("username cannot be empty")
	}
	if !validUsernameRegex.MatchString(trimmed) {
		return errors.New("username must be 1-16 characters and contain only letters, numbers, and underscores")
	}
	return nil
}

// OfflineUUID computes the standard Minecraft offline UUID matching Java's
// UUID.nameUUIDFromBytes(("OfflinePlayer:" + username).getBytes(StandardCharsets.UTF_8)).
// It uses MD5 and sets the version to 3 and variant to IETF RFC 4122.
func OfflineUUID(username string) string {
	hasher := md5.New()
	hasher.Write([]byte("OfflinePlayer:" + username))
	digest := hasher.Sum(nil)

	// Clear version bits and set to version 3 (MD5-based name UUID)
	digest[6] = (digest[6] & 0x0f) | 0x30
	// Clear variant bits and set to IETF variant (0b10xxxxxx)
	digest[8] = (digest[8] & 0x3f) | 0x80

	h := hex.EncodeToString(digest)
	return fmt.Sprintf("%s-%s-%s-%s-%s", h[0:8], h[8:12], h[12:16], h[16:20], h[20:32])
}

// AvatarURL returns the URL to fetch a 64x64 head avatar of the player.
func AvatarURL(nameOrUUID string) string {
	if nameOrUUID == "" {
		return ""
	}
	return fmt.Sprintf("https://mc-heads.net/avatar/%s/64", nameOrUUID)
}
