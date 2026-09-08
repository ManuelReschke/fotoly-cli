package client

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	BaseURL   string
	APIKey    string
	UserAgent string
	HTTP      *http.Client
}

func New(baseURL, apiKey, userAgent string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		BaseURL:   baseURL,
		APIKey:    apiKey,
		UserAgent: userAgent,
		HTTP:      httpClient,
	}
}

func (c *Client) GetProfile(ctx context.Context) (*UserAccount, error) {
	var acc UserAccount
	if err := c.doJSON(ctx, http.MethodGet, "/user/profile", &acc); err != nil {
		return nil, err
	}
	return &acc, nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, dest any) error {
	u := strings.TrimRight(c.BaseURL, "/") + "/api/v1" + path
	req, err := http.NewRequestWithContext(ctx, method, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-API-Key", c.APIKey)
	req.Header.Set("User-Agent", c.UserAgent)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var body struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&body)
		return &APIError{
			Status:  resp.StatusCode,
			Code:    body.Error,
			Message: body.Message,
		}
	}
	return json.NewDecoder(resp.Body).Decode(dest)
}
