package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
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

func (c *Client) ListAlbums(ctx context.Context) (*AlbumCollection, error) {
	var col AlbumCollection
	if err := c.doJSON(ctx, http.MethodGet, "/albums", &col); err != nil {
		return nil, err
	}
	return &col, nil
}

func (c *Client) ListImages(ctx context.Context, q ImageListQuery) (*ImageCollection, error) {
	v := url.Values{}
	if q.Limit > 0 {
		v.Set("limit", strconv.Itoa(q.Limit))
	}
	if q.Cursor != "" {
		v.Set("cursor", q.Cursor)
	}
	if q.AlbumID > 0 {
		v.Set("album_id", strconv.FormatInt(q.AlbumID, 10))
	}
	if q.IsPublic != nil {
		v.Set("is_public", strconv.FormatBool(*q.IsPublic))
	}
	if q.IsNSFW != nil {
		v.Set("is_nsfw", strconv.FormatBool(*q.IsNSFW))
	}
	if q.Tag != "" {
		v.Set("tag", q.Tag)
	}
	path := "/images"
	if encoded := v.Encode(); encoded != "" {
		path += "?" + encoded
	}
	var col ImageCollection
	if err := c.doJSON(ctx, http.MethodGet, path, &col); err != nil {
		return nil, err
	}
	return &col, nil
}

func (c *Client) GetImage(ctx context.Context, uuid string) (*ImageResource, error) {
	var img ImageResource
	if err := c.doJSON(ctx, http.MethodGet, "/images/"+url.PathEscape(uuid), &img); err != nil {
		return nil, err
	}
	return &img, nil
}

func (c *Client) GetImageStatus(ctx context.Context, uuid string) (*ImageStatus, error) {
	var st ImageStatus
	if err := c.doJSON(ctx, http.MethodGet, "/images/"+url.PathEscape(uuid)+"/status", &st); err != nil {
		return nil, err
	}
	return &st, nil
}

func (c *Client) DeleteImage(ctx context.Context, uuid string) (*ImageDeletionAccepted, error) {
	var acc ImageDeletionAccepted
	if err := c.doJSON(ctx, http.MethodDelete, "/images/"+url.PathEscape(uuid), &acc); err != nil {
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
