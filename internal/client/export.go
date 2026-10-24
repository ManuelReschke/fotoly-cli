package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	exportStatusReady     = "ready"
	exportStatusFailed    = "failed"
	exportPollInterval    = 2 * time.Second
	exportPollTimeout     = 30 * time.Minute
	exportDownloadTimeout = 10 * time.Minute
)

func (c *Client) CreateExport(ctx context.Context) (*ExportStatus, error) {
	var st ExportStatus
	if err := c.doJSON(ctx, http.MethodPost, "/user/export", &st); err != nil {
		return nil, err
	}
	return &st, nil
}

func (c *Client) GetExport(ctx context.Context) (*ExportStatus, error) {
	var st ExportStatus
	if err := c.doJSON(ctx, http.MethodGet, "/user/export", &st); err != nil {
		return nil, err
	}
	return &st, nil
}

// WaitExport polls GET /user/export until the status is ready or failed.
// A ready or failed status returns immediately and does not poll.
// The deadline is exportPollTimeout of Client.Sleep intervals, not wall time.
func (c *Client) WaitExport(ctx context.Context, st *ExportStatus) (*ExportStatus, error) {
	if st == nil {
		return nil, errors.New("export status missing")
	}
	var waited time.Duration
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		switch st.Status {
		case exportStatusReady:
			return st, nil
		case exportStatusFailed:
			return nil, exportStatusError("export failed", st)
		}
		if waited >= exportPollTimeout {
			return nil, exportStatusError("export timed out", st)
		}
		c.sleep(exportPollInterval)
		waited += exportPollInterval
		next, err := c.GetExport(ctx)
		if err != nil {
			return nil, err
		}
		if next == nil {
			return nil, errors.New("export status missing")
		}
		st = next
	}
}

func exportStatusError(prefix string, st *ExportStatus) error {
	if st == nil {
		return errors.New(prefix)
	}
	msg := strings.TrimSpace(st.Error)
	if msg == "" {
		return errors.New(prefix)
	}
	return fmt.Errorf("%s: %s", prefix, msg)
}

// GetExportAccount returns the raw account document. Call it only after the export is ready.
func (c *Client) GetExportAccount(ctx context.Context) (json.RawMessage, error) {
	var raw json.RawMessage
	if err := c.doJSON(ctx, http.MethodGet, "/user/export/account", &raw); err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, errors.New("export account is empty")
	}
	return raw, nil
}

// DownloadExportFile writes one raw export object. downloadPath is an API path
// such as /user/export/files/<uuid>. The body is not JSON.
// Each call uses its own timeout of at least exportDownloadTimeout and does not
// reuse the 30s timeout from New.
func (c *Client) DownloadExportFile(ctx context.Context, downloadPath string, dst io.Writer) (int64, error) {
	if dst == nil {
		return 0, errors.New("export download destination is nil")
	}
	path := downloadPath
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	u := strings.TrimRight(c.BaseURL, "/") + "/api/v1" + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("X-API-Key", c.APIKey)
	req.Header.Set("User-Agent", c.UserAgent)

	hc := http.Client{Timeout: exportDownloadTimeout}
	if c.HTTP != nil {
		hc = *c.HTTP
		hc.Timeout = exportDownloadTimeout
	}
	resp, err := hc.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, decodeAPIError(resp)
	}
	return io.Copy(dst, resp.Body)
}
