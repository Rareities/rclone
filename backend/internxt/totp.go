package internxt

import (
	"encoding/base32"
	"fmt"
	"strings"
	"time"

	"github.com/pquerna/otp/totp"
	"github.com/rclone/rclone/fs/config/obscure"
)

func generateTOTPCodeAt(secret string, at time.Time) (string, error) {
	if secret == "" {
		return "", fmt.Errorf("totp_secret is empty")
	}
	code, err := totp.GenerateCode(secret, at)
	if err != nil {
		return "", fmt.Errorf("failed to generate TOTP code: %w", err)
	}
	return code, nil
}

// revealTOTPSecret accepts normal rclone-obscured values and legacy plaintext
// Base32 seeds. IsPassword: true protects new config writes, but existing app
// profiles may predate that option and cannot safely be decoded as obscured
// data without first distinguishing a valid Base32 seed.
func revealTOTPSecret(raw string) string {
	if raw == "" {
		return ""
	}
	if isBase32Secret(raw) {
		return strings.ToUpper(raw)
	}
	revealed, err := obscure.Reveal(raw)
	if err != nil {
		// Preserve legacy raw values; generateTOTPCodeAt validates them before
		// any authentication request is made.
		return raw
	}
	return revealed
}

func isBase32Secret(secret string) bool {
	if secret == "" {
		return false
	}
	// TOTP seeds are conventionally all upper- or all lowercase. Obscured
	// values are base64url and may contain mixed case; normalizing those before
	// testing would occasionally mistake ciphertext for a plaintext seed.
	if secret != strings.ToUpper(secret) && secret != strings.ToLower(secret) {
		return false
	}
	secret = strings.ToUpper(strings.TrimRight(secret, "="))
	if secret == "" {
		return false
	}
	_, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	return err == nil
}
