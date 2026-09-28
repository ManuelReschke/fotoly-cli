package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
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
	Sleep     func(time.Duration)
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
		Sleep:     time.Sleep,
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

func (c *Client) UpdateImage(ctx context.Context, uuid string, req ImageUpdate) (*ImageEdit, error) {
	var img ImageEdit
	if err := c.doJSONBody(ctx, http.MethodPatch, "/images/"+url.PathEscape(uuid), req, &img); err != nil {
		return nil, err
	}
	return &img, nil
}

func (c *Client) DeleteImage(ctx context.Context, uuid string) (*ImageDeletionAccepted, error) {
	var acc ImageDeletionAccepted
	if err := c.doJSON(ctx, http.MethodDelete, "/images/"+url.PathEscape(uuid), &acc); err != nil {
		return nil, err
	}
	return &acc, nil
}

func (c *Client) CreateUploadSession(ctx context.Context, req UploadSessionRequest) (*UploadSessionResponse, error) {
	var sess UploadSessionResponse
	if err := c.doJSONBody(ctx, http.MethodPost, "/upload/sessions", req, &sess); err != nil {
		return nil, err
	}
	return &sess, nil
}

func (c *Client) UploadFile(ctx context.Context, uploadURL, token, filename string, r io.Reader, size int64, progress func(sent, total int64)) (*StorageUploadResponse, error) {
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		var err error
		defer func() {
			if closeErr := mw.Close(); err == nil {
				err = closeErr
			}
			_ = pw.CloseWithError(err)
		}()
		fw, ferr := mw.CreateFormFile("file", filename)
		if ferr != nil {
			err = ferr
			return
		}
		src := io.Reader(r)
		if progress != nil {
			src = io.TeeReader(r, &progressWriter{total: size, fn: progress})
		}
		_, err = io.Copy(fw, src)
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, pr)
	if err != nil {
		_ = pr.Close()
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)

	transport := c.HTTP.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	httpClient := &http.Client{
		Timeout:   10 * time.Minute,
		Transport: transport,
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		_ = pr.Close()
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, decodeAPIError(resp)
	}
	var out StorageUploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

type progressWriter struct {
	sent, total int64
	fn          func(sent, total int64)
}

func (w *progressWriter) Write(p []byte) (int, error) {
	n := len(p)
	w.sent += int64(n)
	w.fn(w.sent, w.total)
	return n, nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, dest any) error {
	return c.doJSONBody(ctx, method, path, nil, dest)
}

func (c *Client) doJSONBody(ctx context.Context, method, path string, body, dest any) error {
	u := strings.TrimRight(c.BaseURL, "/") + "/api/v1" + path
	var payload []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		payload = b
	}

	var delay time.Duration
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 && delay > 0 {
			c.sleep(delay)
		}
		var rdr io.Reader
		if payload != nil {
			rdr = bytes.NewReader(payload)
		}
		req, err := http.NewRequestWithContext(ctx, method, u, rdr)
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("X-API-Key", c.APIKey)
		req.Header.Set("User-Agent", c.UserAgent)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.HTTP.Do(req)
		if err != nil {
			if attempt == 0 && ctx.Err() == nil {
				delay = 0
				continue
			}
			return err
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			defer resp.Body.Close()
			return json.NewDecoder(resp.Body).Decode(dest)
		}

		if attempt == 0 && resp.StatusCode == http.StatusTooManyRequests {
			delay = retryAfterDelay(resp.Header.Get("Retry-After"))
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			continue
		}
		if attempt == 0 && resp.StatusCode >= 500 {
			delay = 0
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			continue
		}

		defer resp.Body.Close()
		return decodeAPIError(resp)
	}
	return errors.New("retry exhausted")
}

func (c *Client) sleep(d time.Duration) {
	if d <= 0 {
		return
	}
	fn := c.Sleep
	if fn == nil {
		fn = time.Sleep
	}
	fn(d)
}

func retryAfterDelay(h string) time.Duration {
	h = strings.TrimSpace(h)
	if h == "" {
		return 2 * time.Second
	}
	if secs, err := strconv.Atoi(h); err == nil {
		if secs < 0 {
			return 0
		}
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(h); err == nil {
		d := time.Until(t)
		if d < 0 {
			return 0
		}
		return d
	}
	return 2 * time.Second
}

func decodeAPIError(resp *http.Response) error {
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
