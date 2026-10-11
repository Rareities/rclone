package internxt

import (
	"encoding/base32"
	"errors"
	"strings"
	"time"

	"github.com/pquerna/otp/totp"
	"github.com/rclone/rclone/fs/config/obscure"
)

func generateTOTPCode(secret string) (string, error) {
	return generateTOTPCodeWithOffset(secret, 0)
}

func generateTOTPCodeWithOffset(secret string, offset int64) (string, error) {
	if secret == "" {
		return "", errors.New("totp_secret is empty")
	}
	return totp.GenerateCode(secret, time.Now().Add(time.Duration(offset)*30*time.Second))
}

func isBase32Secret(raw string) bool {
	if raw == "" || raw != strings.ToUpper(raw) {
		return false
	}
	_, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.TrimRight(raw, "="))
	return err == nil
}

// Plaintext base32 supports legacy CloudBridge configs. New values are obscured
// through rclone's IsPassword option and never included in task options or logs.
func revealTOTPSecret(raw string) string {
	if isBase32Secret(raw) {
		return strings.TrimRight(raw, "=")
	}
	value, err := obscure.Reveal(raw)
	if err != nil || !isBase32Secret(value) {
		return ""
	}
	return strings.TrimRight(value, "=")
}
