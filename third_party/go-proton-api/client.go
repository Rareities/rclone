package proton

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-resty/resty/v2"
)

// clientID is a unique identifier for a client.
var clientID uint64

// AuthHandler is given any new auths that are returned from the API due to an unexpected auth refresh.
type AuthHandler func(Auth)

// Handler is a generic function that can be registered for a certain event (e.g. deauth, API code).
type Handler func()

// AuthRefreshHook is consulted before a client refreshes its session, and
// again if that refresh is rejected. It returns the most recently persisted
// credentials for the session, so that a client whose refresh token has been
// rotated by another client (or another process) can adopt them instead of
// failing. ok is false when nothing is available.
type AuthRefreshHook func() (uid, acc, ref string, ok bool)

// AuthPersistenceHook reports whether newly rotated credentials were durably saved.
type AuthPersistenceHook func() error

// Client is the proton client.
type Client struct {
	m *Manager

	// clientID is this client's unique ID.
	clientID uint64

	uid      string
	acc      string
	ref      string
	authLock sync.RWMutex

	authHandlers         []AuthHandler
	deauthHandlers       []Handler
	authRefreshHooks     []AuthRefreshHook
	authPersistenceHooks []AuthPersistenceHook
	hookLock             sync.RWMutex

	deauthOnce       sync.Once
	oldRefreshTokens map[string]struct{}
}

func newClient(m *Manager, uid string) *Client {
	c := &Client{
		m:        m,
		uid:      uid,
		clientID: atomic.AddUint64(&clientID, 1),
	}

	return c
}

func (c *Client) AddAuthHandler(handler AuthHandler) {
	c.hookLock.Lock()
	defer c.hookLock.Unlock()

	c.authHandlers = append(c.authHandlers, handler)
}

func (c *Client) AddDeauthHandler(handler Handler) {
	c.hookLock.Lock()
	defer c.hookLock.Unlock()

	c.deauthHandlers = append(c.deauthHandlers, handler)
}

func (c *Client) AddAuthRefreshHook(hook AuthRefreshHook) {
	c.hookLock.Lock()
	defer c.hookLock.Unlock()

	c.authRefreshHooks = append(c.authRefreshHooks, hook)
}

// AddAuthPersistenceHook registers a durability check after authentication callbacks.
func (c *Client) AddAuthPersistenceHook(hook AuthPersistenceHook) {
	c.hookLock.Lock()
	defer c.hookLock.Unlock()
	c.authPersistenceHooks = append(c.authPersistenceHooks, hook)
}

func (c *Client) persistenceError() error {
	for _, hook := range c.authPersistenceHooks {
		if err := hook(); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) AddPreRequestHook(hook resty.RequestMiddleware) {
	c.hookLock.Lock()
	defer c.hookLock.Unlock()

	c.m.rc.OnBeforeRequest(func(rc *resty.Client, r *resty.Request) error {
		if clientID, ok := ClientIDFromContext(r.Context()); !ok || clientID != c.clientID {
			return nil
		}

		return hook(rc, r)
	})
}

func (c *Client) AddPostRequestHook(hook resty.ResponseMiddleware) {
	c.hookLock.Lock()
	defer c.hookLock.Unlock()

	c.m.rc.OnAfterResponse(func(rc *resty.Client, r *resty.Response) error {
		if clientID, ok := ClientIDFromContext(r.Request.Context()); !ok || clientID != c.clientID {
			return nil
		}

		return hook(rc, r)
	})
}

func (c *Client) Close() {
	c.authLock.Lock()
	defer c.authLock.Unlock()

	c.uid = ""
	c.acc = ""
	c.ref = ""

	c.hookLock.Lock()
	defer c.hookLock.Unlock()

	c.authHandlers = nil
	c.deauthHandlers = nil
	c.authRefreshHooks = nil
	c.authPersistenceHooks = nil
	c.oldRefreshTokens = nil
}

func (c *Client) withAuth(acc, ref string) *Client {
	c.acc = acc
	c.ref = ref

	return c
}

func (c *Client) do(ctx context.Context, fn func(*resty.Request) (*resty.Response, error)) error {
	if _, err := c.doRes(ctx, fn); err != nil {
		return err
	}

	return nil
}

func (c *Client) doRes(ctx context.Context, fn func(*resty.Request) (*resty.Response, error)) (*resty.Response, error) {

	res, err := c.exec(ctx, fn)

	if res != nil {
		// If we receive no response, we can't do anything.
		if res.RawResponse == nil {
			return nil, newNetError(err, "received no response from API")
		}

		// If we receive a net error, we can't do anything.
		if resErr, ok := err.(*resty.ResponseError); ok {
			if netErr := new(net.OpError); errors.As(resErr.Err, &netErr) {
				return nil, newNetError(netErr, "network error while communicating with API")
			}
		}

		// If we receive a 401, we need to refresh the auth.
		if res.StatusCode() == http.StatusUnauthorized {
			if err := c.authRefresh(ctx); err != nil {
				return nil, fmt.Errorf("failed to refresh auth: %w", err)
			}

			if res, err = c.exec(ctx, fn); err != nil {
				return nil, fmt.Errorf("failed to retry request: %w", err)
			}
		}
	}

	return res, err
}

func (c *Client) exec(ctx context.Context, fn func(*resty.Request) (*resty.Response, error)) (*resty.Response, error) {
	c.authLock.RLock()
	defer c.authLock.RUnlock()

	r := c.m.r(WithClient(ctx, c.clientID))

	if c.uid != "" {
		r.SetHeader("x-pm-uid", c.uid)
	}

	if c.acc != "" {
		r.SetAuthToken(c.acc)
	}

	return fn(r)
}

func (c *Client) authRefresh(ctx context.Context) error {
	c.authLock.Lock()
	defer c.authLock.Unlock()

	c.hookLock.RLock()
	defer c.hookLock.RUnlock()

	// Refresh tokens are single use. If another client has already refreshed
	// this session, our copy of the token is spent and the refresh would be
	// rejected, so adopt the rotated credentials instead.
	if c.adoptNewerAuth() {
		return c.persistenceError()
	}

	auth, err := c.m.authRefresh(ctx, c.uid, c.ref)

	if err != nil {
		if respErr, ok := err.(*resty.ResponseError); ok {

			switch respErr.Response.StatusCode() {
			case http.StatusBadRequest, http.StatusUnprocessableEntity:
				// The rotation may have happened after we checked above.
				if c.adoptAfterRejectedAuth(ctx) {
					return c.persistenceError()
				}
				if ctx.Err() != nil {
					return ctx.Err()
				}

				c.deauthOnce.Do(func() {
					for _, handler := range c.deauthHandlers {
						handler()
					}
				})

				return fmt.Errorf("failed to refresh auth, de-auth: %w", err)
			case http.StatusConflict, http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusServiceUnavailable:
				return fmt.Errorf("failed to refresh auth, server issues: %w", err)
			default:
				//
			}
		}

		return fmt.Errorf("failed to refresh auth: %w", err)
	}

	c.rememberRefreshToken()
	c.acc = auth.AccessToken
	c.ref = auth.RefreshToken

	for _, handler := range c.authHandlers {
		handler(auth)
	}

	return c.persistenceError()
}

// adoptNewerAuth asks the refresh hooks for persisted credentials and adopts
// them if they carry a refresh token other than the one this client holds.
// Must be called with authLock and hookLock held. Credentials must belong to
// the same session UID; hooks must reject stale configuration generations.
func (c *Client) adoptNewerAuth() bool {
	for _, hook := range c.authRefreshHooks {
		uid, acc, ref, ok := hook()
		if !ok || uid != c.uid || acc == "" || ref == "" || ref == c.ref {
			continue
		}

		if _, stale := c.oldRefreshTokens[ref]; stale {
			continue
		}
		c.rememberRefreshToken()
		c.uid = uid
		c.acc = acc
		c.ref = ref

		auth := Auth{UID: uid, AccessToken: acc, RefreshToken: ref}

		for _, handler := range c.authHandlers {
			handler(auth)
		}

		return true
	}

	return false
}

// rememberRefreshToken prevents a hook from cycling back to a spent token.
func (c *Client) rememberRefreshToken() {
	if c.oldRefreshTokens == nil {
		c.oldRefreshTokens = make(map[string]struct{})
	}
	c.oldRefreshTokens[c.ref] = struct{}{}
}

func (c *Client) adoptAfterRejectedAuth(ctx context.Context) bool {
	if c.adoptNewerAuth() {
		return true
	}
	if len(c.authRefreshHooks) == 0 {
		return false
	}
	// Allow an in-flight winner to persist its rotated token before revoking.
	for _, delay := range []time.Duration{25 * time.Millisecond, 50 * time.Millisecond, 100 * time.Millisecond} {
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return false
		case <-timer.C:
		}
		if c.adoptNewerAuth() {
			return true
		}
	}
	return false
}
