// Package internxt provides authentication handling
package internxt

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/golang-jwt/jwt/v5"
	internxtauth "github.com/internxt/rclone-adapter/auth"
	internxtconfig "github.com/internxt/rclone-adapter/config"
	sdkerrors "github.com/internxt/rclone-adapter/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/lib/oauthutil"
	"golang.org/x/oauth2"
)

type userInfo struct {
	RootFolderID string
	Bucket       string
	BridgeUser   string
	UserID       string
	NewToken     string
}

type userInfoConfig struct {
	Token string
}

// getUserInfo fetches user metadata from the refresh endpoint
func getUserInfo(ctx context.Context, cfg *userInfoConfig) (*userInfo, error) {
	// Call the refresh endpoint to get all user metadata
	refreshCfg := internxtconfig.NewDefaultToken(cfg.Token)
	resp, err := internxtauth.RefreshToken(ctx, refreshCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user info: %w", err)
	}

	if resp.User.Bucket == "" {
		return nil, errors.New("API response missing user.bucket")
	}
	if resp.User.RootFolderID == "" {
		return nil, errors.New("API response missing user.rootFolderId")
	}
	if resp.User.BridgeUser == "" {
		return nil, errors.New("API response missing user.bridgeUser")
	}
	if resp.User.UserID == "" {
		return nil, errors.New("API response missing user.userId")
	}

	info := &userInfo{
		RootFolderID: resp.User.RootFolderID,
		Bucket:       resp.User.Bucket,
		BridgeUser:   resp.User.BridgeUser,
		UserID:       resp.User.UserID,
		NewToken:     responseToken(resp.NewToken, resp.Token),
	}

	fs.Debugf(nil, "User info: rootFolderId=%s, bucket=%s",
		info.RootFolderID, info.Bucket)

	return info, nil
}

// parseJWTExpiry extracts the expiry time from a JWT token string
func parseJWTExpiry(tokenString string) (time.Time, error) {
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	token, _, err := parser.ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return time.Time{}, errors.New("invalid token claims")
	}

	exp, ok := claims["exp"].(float64)
	if !ok {
		return time.Time{}, errors.New("token missing expiration")
	}

	return time.Unix(int64(exp), 0), nil
}

// jwtToOAuth2Token converts a JWT string to an oauth2.Token with expiry
func jwtToOAuth2Token(jwtString string) (*oauth2.Token, error) {
	expiry, err := parseJWTExpiry(jwtString)
	if err != nil {
		return nil, err
	}

	return &oauth2.Token{
		AccessToken: jwtString,
		TokenType:   "Bearer",
		Expiry:      expiry,
	}, nil
}

// computeBasicAuthHeader creates the BasicAuthHeader for bucket operations
func computeBasicAuthHeader(bridgeUser, userID string) string {
	sum := sha256.Sum256([]byte(userID))
	hexPass := hex.EncodeToString(sum[:])
	creds := fmt.Sprintf("%s:%s", bridgeUser, hexPass)
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(creds))
}

// refreshJWTToken refreshes the token using Internxt's refresh endpoint
func refreshJWTToken(ctx context.Context, name string, m configmap.Mapper) error {
	currentToken, err := oauthutil.GetToken(name, m)
	if err != nil {
		return fmt.Errorf("failed to get current token: %w", err)
	}

	cfg := internxtconfig.NewDefaultToken(currentToken.AccessToken)
	resp, err := internxtauth.RefreshToken(ctx, cfg)
	if err != nil {
		return fmt.Errorf("refresh request failed: %w", err)
	}

	newToken := responseToken(resp.NewToken, resp.Token)
	if newToken == "" {
		return errors.New("refresh response missing newToken")
	}

	// Convert JWT to oauth2.Token format
	token, err := jwtToOAuth2Token(newToken)
	if err != nil {
		return fmt.Errorf("failed to parse refreshed token: %w", err)
	}

	err = oauthutil.PutToken(name, m, token, false)
	if err != nil {
		return fmt.Errorf("failed to save token: %w", err)
	}

	if resp.User.Bucket != "" {
		m.Set("bucket", resp.User.Bucket)
	}

	fs.Debugf(name, "Token refreshed successfully, new expiry: %v", token.Expiry)
	return nil
}

// reLogin performs a full re-login using stored email+password credentials.
// Returns the AccessResponse on success, or an error if 2FA is required or login fails.
func (f *Fs) reLogin(ctx context.Context) (*internxtauth.AccessResponse, error) {
	password, err := obscure.Reveal(f.opt.Pass)
	if err != nil {
		return nil, fmt.Errorf("couldn't decrypt password: %w", err)
	}

	cfg := internxtconfig.NewDefaultToken("")
	cfg.HTTPClient = fshttp.NewClient(ctx)

	loginResp, err := internxtauth.Login(ctx, cfg, f.opt.Email)
	if err != nil {
		return nil, fmt.Errorf("re-login check failed: %w", err)
	}

	if loginResp.TFA {
		secret := revealTOTPSecret(f.opt.TOTPSecret)
		if secret == "" {
			return nil, errors.New("account requires 2FA - please run: rclone config reconnect " + f.name + ":")
		}
		var lastErr error
		for _, offset := range []int64{0, -1, 1} {
			code, err := generateTOTPCodeWithOffset(secret, offset)
			if err != nil {
				return nil, err
			}
			resp, err := internxtauth.DoLogin(ctx, cfg, f.opt.Email, password, code)
			if err == nil {
				return resp, nil
			}
			var httpErr *sdkerrors.HTTPError
			if !errors.As(err, &httpErr) || (httpErr.StatusCode() != 401 && httpErr.StatusCode() != 403) {
				return nil, fmt.Errorf("re-login failed: %w", err)
			}
			lastErr = err
		}
		return nil, fmt.Errorf("re-login failed in all TOTP windows: %w", lastErr)
	}

	resp, err := internxtauth.DoLogin(ctx, cfg, f.opt.Email, password, "")
	if err != nil {
		return nil, fmt.Errorf("re-login failed: %w", err)
	}

	return resp, nil
}

// refreshOrReLogin tries to refresh the JWT token first; if that fails with 401,
// it falls back to a full re-login using stored credentials.
func (f *Fs) refreshOrReLogin(ctx context.Context) error {
	refreshErr := refreshJWTToken(ctx, f.name, f.m)
	if refreshErr == nil {
		newToken, err := oauthutil.GetToken(f.name, f.m)
		if err != nil {
			return fmt.Errorf("failed to get refreshed token: %w", err)
		}
		f.cfg.Token = newToken.AccessToken
		f.cfg.BasicAuthHeader = computeBasicAuthHeader(f.bridgeUser, f.userID)
		fs.Debugf(f, "Token refresh succeeded")
		return nil
	}

	var httpErr *sdkerrors.HTTPError
	if !errors.As(refreshErr, &httpErr) || httpErr.StatusCode() != 401 {
		return refreshErr
	}

	fs.Debugf(f, "Token refresh returned 401, attempting re-login with stored credentials")

	resp, err := f.reLogin(ctx)
	if err != nil {
		return fmt.Errorf("re-login fallback failed: %w", err)
	}

	oauthToken, err := jwtToOAuth2Token(responseToken(resp.NewToken, resp.Token))
	if err != nil {
		return fmt.Errorf("failed to parse re-login token: %w", err)
	}
	err = oauthutil.PutToken(f.name, f.m, oauthToken, true)
	if err != nil {
		return fmt.Errorf("failed to save re-login token: %w", err)
	}

	f.cfg.Token = oauthToken.AccessToken
	f.bridgeUser = resp.User.BridgeUser
	f.userID = resp.User.UserID
	f.cfg.BasicAuthHeader = computeBasicAuthHeader(f.bridgeUser, f.userID)
	f.cfg.Bucket = resp.User.Bucket
	f.cfg.RootFolderID = resp.User.RootFolderID

	fs.Debugf(f, "Re-login succeeded, new token expiry: %v", oauthToken.Expiry)
	return nil
}

func responseToken(newToken, token string) string {
	if newToken != "" {
		return newToken
	}
	return token
}

// getBackoffDuration returns a capped delay with up to ten percent jitter.
func getBackoffDuration(attempt int) time.Duration {
	base := time.Minute
	switch attempt {
	case 2:
		base = 5 * time.Minute
	case 3:
		base = 15 * time.Minute
	default:
		if attempt >= 4 {
			base = time.Hour
		}
	}
	return base - time.Duration(rand.Int64N(int64(base/10)))
}

// reAuthorize serializes authentication and bounds retries after failures.
func (f *Fs) reAuthorize(ctx context.Context) error {
	return f.reAuthorizeWith(ctx, f.refreshOrReLogin, time.Now())
}

func (f *Fs) reAuthorizeWith(ctx context.Context, authorize func(context.Context) error, now time.Time) error {
	f.authMu.Lock()
	defer f.authMu.Unlock()
	if f.authFailCount >= 5 {
		return errors.New("auth exceeded max retries: manual re-auth required")
	}
	if now.Before(f.nextAuthAllowed) {
		return fmt.Errorf("re-authorization blocked until %v (attempt %d/5)", f.nextAuthAllowed, f.authFailCount)
	}
	if err := authorize(ctx); err != nil {
		f.authFailCount++
		f.nextAuthAllowed = now.Add(getBackoffDuration(f.authFailCount))
		if f.authFailCount >= 5 {
			return fmt.Errorf("auth exceeded max retries: manual re-auth required: %w", err)
		}
		return err
	}
	f.authFailCount = 0
	f.nextAuthAllowed = time.Time{}
	return nil
}
