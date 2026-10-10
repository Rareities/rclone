package internxt

import (
	"errors"
	"time"

	"github.com/pquerna/otp/totp"
	"github.com/rclone/rclone/fs/config/obscure"
)

func generateTOTPCodeWithOffset(secret string, offset int64) (string, error) {
	return generateTOTPCodeAt(secret, time.Now(), offset)
}

func generateTOTPCodeAt(secret string, now time.Time, offset int64) (string, error) {
	if secret == "" {
		return "", errors.New("totp_secret is empty")
	}
	return totp.GenerateCode(secret, now.Add(time.Duration(offset)*30*time.Second))
}

func isBase32Secret(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if !(c >= 'A' && c <= 'Z') && !(c >= '2' && c <= '7') && c != '=' {
			return false
		}
	}
	return true
}

// revealTOTPSecret accepts legacy plaintext seeds and current obscured values.
func revealTOTPSecret(raw string) string {
	if raw == "" || isBase32Secret(raw) {
		return raw
	}
	revealed, err := obscure.Reveal(raw)
	if err != nil {
		return raw
	}
	return revealed
}
