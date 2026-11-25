package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type Pong struct {
	Ping string `json:"ping"`
}

type AuthCredentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthRegistration struct {
	Username string `json:"username"`
	AuthCredentials
}

type AuthRegistrationResult struct {
	Email   string `json:"email"`
	Message string `json:"message"`
}

type AuthSession struct {
	Token     string      `json:"token"`
	TokenType string      `json:"token_type"`
	ExpiresAt string      `json:"expires_at"`
	User      UserAccount `json:"user"`
}

type AuthProvider struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type AuthProviders struct {
	Providers []AuthProvider `json:"providers"`
}

type AuthURL struct {
	URL string `json:"url"`
}

func (c *Client) Ping(ctx context.Context) (Pong, error) {
	var out Pong
	err := c.authJSON(ctx, http.MethodGet, "/ping", nil, &out, false)
	return out, err
}

func (c *Client) ListAuthProviders(ctx context.Context) (AuthProviders, error) {
	var out AuthProviders
	err := c.authJSON(ctx, http.MethodGet, "/auth/providers", nil, &out, false)
	return out, err
}

func (c *Client) Login(ctx context.Context, credentials AuthCredentials) (AuthSession, error) {
	var out AuthSession
	err := c.authJSON(ctx, http.MethodPost, "/auth/login", credentials, &out, false)
	return out, err
}

func (c *Client) Register(ctx context.Context, registration AuthRegistration) (AuthRegistrationResult, error) {
	var out AuthRegistrationResult
	err := c.authJSON(ctx, http.MethodPost, "/auth/register", registration, &out, false)
	return out, err
}

func (c *Client) ExchangeAuthCode(ctx context.Context, code string) (AuthSession, error) {
	body := struct {
		Code string `json:"code"`
	}{Code: code}
	var out AuthSession
	err := c.authJSON(ctx, http.MethodPost, "/auth/token", body, &out, false)
	return out, err
}

func (c *Client) Logout(ctx context.Context) error {
	if c.SessionToken == "" {
		return fmt.Errorf("app session token required")
	}
	session := *c
	session.APIKey = ""
	return session.authJSON(ctx, http.MethodPost, "/auth/logout", nil, nil, true)
}

func (c *Client) CreateWebHandoff(ctx context.Context, next *string) (AuthURL, error) {
	var body any
	if next != nil {
		body = struct {
			Next string `json:"next"`
		}{Next: *next}
	}
	var out AuthURL
	err := c.authJSON(ctx, http.MethodPost, "/auth/web-handoff", body, &out, true)
	return out, err
}

func (c *Client) StartAuthProvider(ctx context.Context, provider, redirectURI string) (AuthURL, error) {
	query := url.Values{"redirect_uri": {redirectURI}}
	path := "/auth/" + url.PathEscape(provider) + "/start?" + query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiURL(path), nil)
	if err != nil {
		return AuthURL{}, err
	}
	resp, err := c.doWithoutRedirect(req)
	if err != nil {
		return AuthURL{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTemporaryRedirect {
		return AuthURL{}, decodeAPIError(resp)
	}
	location, err := resp.Location()
	if err != nil {
		return AuthURL{}, fmt.Errorf("OAuth start returned an invalid Location: %w", err)
	}
	return AuthURL{URL: location.String()}, nil
}

// Authorization codes and auth writes are sent once because a lost response
// can occur after the server has already consumed a code or created a session.
func (c *Client) authJSON(ctx context.Context, method, path string, body, dest any, authenticated bool) error {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.apiURL(path), reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if authenticated {
		c.setAuth(req)
	}
	resp, err := c.doWithoutRedirect(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return decodeAPIError(resp)
	}
	if dest == nil {
		_, err := io.Copy(io.Discard, resp.Body)
		return err
	}
	return json.NewDecoder(resp.Body).Decode(dest)
}

func (c *Client) doWithoutRedirect(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", c.UserAgent)
	httpClient := *c.HTTP
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return httpClient.Do(req)
}
