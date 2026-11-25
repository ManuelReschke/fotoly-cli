package client

import (
	"context"
	"net/http"
)

type UploadCheck struct {
	StatusCode   int    `json:"status_code"`
	AllowOrigin  string `json:"allow_origin,omitempty"`
	AllowMethods string `json:"allow_methods,omitempty"`
	AllowHeaders string `json:"allow_headers,omitempty"`
}

func (c *Client) CheckUpload(ctx context.Context, targetURL, origin string) (UploadCheck, error) {
	if targetURL == "" {
		targetURL = c.apiURL("/upload")
	}
	method := http.MethodHead
	if origin != "" {
		method = http.MethodOptions
	}
	req, err := http.NewRequestWithContext(ctx, method, targetURL, nil)
	if err != nil {
		return UploadCheck{}, err
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
		req.Header.Set("Access-Control-Request-Method", http.MethodPost)
		req.Header.Set("Access-Control-Request-Headers", "authorization, content-type")
	}
	resp, err := c.doWithoutRedirect(req)
	if err != nil {
		return UploadCheck{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return UploadCheck{}, decodeAPIError(resp)
	}
	return UploadCheck{
		StatusCode:   resp.StatusCode,
		AllowOrigin:  resp.Header.Get("Access-Control-Allow-Origin"),
		AllowMethods: resp.Header.Get("Access-Control-Allow-Methods"),
		AllowHeaders: resp.Header.Get("Access-Control-Allow-Headers"),
	}, nil
}
