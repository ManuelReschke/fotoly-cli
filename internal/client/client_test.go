package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetProfileSuccessSendsAPIKey(t *testing.T) {
	var gotKey, gotUA, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-API-Key")
		gotUA = r.Header.Get("User-Agent")
		gotPath = r.URL.Path
		if r.Header.Get("Authorization") != "" {
			t.Errorf("profile must not send Authorization, got %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": 1, "username": "pixelpete", "email": "pete@example.com",
			"status": "active", "plan": "premium", "created_at": "2026-01-01T00:00:00Z",
			"stats":       map[string]any{"images": map[string]any{"count": 3, "storage_used_bytes": 100}, "albums": map[string]any{"count": 1}},
			"limits":      map[string]any{"max_upload_bytes": 50, "storage_quota_bytes": 200, "can_multi_upload": true, "image_upload_enabled": true, "direct_upload_enabled": true, "allowed_thumbnail_formats": []string{"original"}},
			"preferences": map[string]any{"upload_nsfw_by_default": false, "thumbnail_original": true, "thumbnail_webp": false, "thumbnail_avif": false},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "pxl_test", "fotoly-cli/dev", srv.Client())
	acc, err := c.GetProfile(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if acc.Username != "pixelpete" || acc.Plan != "premium" {
		t.Fatalf("account=%+v", acc)
	}
	if gotKey != "pxl_test" || !strings.Contains(gotUA, "fotoly-cli") || gotPath != "/api/v1/user/profile" {
		t.Fatalf("key=%q ua=%q path=%q", gotKey, gotUA, gotPath)
	}
}

func TestGetProfileUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":"unauthorized","message":"Invalid API key"}`)
	}))
	defer srv.Close()

	c := New(srv.URL, "bad", "fotoly-cli/dev", srv.Client())
	_, err := c.GetProfile(context.Background())
	if !IsUnauthorized(err) {
		t.Fatalf("err=%v", err)
	}
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.Message != "Invalid API key" || apiErr.Status != 401 {
		t.Fatalf("apiErr=%v", err)
	}
}
