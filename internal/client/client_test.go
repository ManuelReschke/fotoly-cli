package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
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

func TestListAlbums(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.Method != http.MethodGet {
			t.Fatalf("method=%s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"albums":[{"id":42,"title":"Cats","description":"","is_public":true,"is_nsfw":false,"share_link":"s","view_url":"/a/s","image_count":2,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}]}`)
	}))
	defer srv.Close()

	c := New(srv.URL, "pxl_test", "fotoly-cli/dev", srv.Client())
	col, err := c.ListAlbums(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/api/v1/albums" {
		t.Fatalf("path=%s", gotPath)
	}
	if len(col.Albums) != 1 || col.Albums[0].ID != 42 || col.Albums[0].Title != "Cats" || col.Albums[0].ImageCount != 2 || !col.Albums[0].IsPublic || col.Albums[0].ViewURL != "/a/s" {
		t.Fatalf("col=%+v", col)
	}
}

func TestListImagesQuery(t *testing.T) {
	pub, nsfw := true, false
	var got url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/images" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		got = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"items":[{"image_uuid":"u1","title":"","description":"","file_name":"a.jpg","file_size":10,"file_type":"image/jpeg","width":1,"height":1,"is_public":true,"is_nsfw":false,"share_link":"s","view_url":"/i/s","stable_url":"https://x/s","view_count":0,"download_count":0,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}],"has_more":true,"next_cursor":"cur1"}`)
	}))
	defer srv.Close()
	c := New(srv.URL, "pxl_test", "fotoly-cli/dev", srv.Client())
	col, err := c.ListImages(context.Background(), ImageListQuery{Limit: 10, Cursor: "abc", AlbumID: 42, IsPublic: &pub, IsNSFW: &nsfw, Tag: "cat"})
	if err != nil {
		t.Fatal(err)
	}
	if !col.HasMore || col.NextCursor == nil || *col.NextCursor != "cur1" || len(col.Items) != 1 {
		t.Fatalf("col=%+v", col)
	}
	if got.Get("limit") != "10" || got.Get("cursor") != "abc" || got.Get("album_id") != "42" || got.Get("is_public") != "true" || got.Get("is_nsfw") != "false" || got.Get("tag") != "cat" {
		t.Fatalf("query=%v", got)
	}
}

func TestListImagesQueryOmitsUnset(t *testing.T) {
	var got url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/images" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		got = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"items":[],"has_more":false,"next_cursor":null}`)
	}))
	defer srv.Close()
	c := New(srv.URL, "pxl_test", "fotoly-cli/dev", srv.Client())
	col, err := c.ListImages(context.Background(), ImageListQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if col.HasMore || col.NextCursor != nil || len(col.Items) != 0 {
		t.Fatalf("col=%+v", col)
	}
	if got.Get("limit") != "" || got.Get("cursor") != "" || got.Get("album_id") != "" || got.Get("is_public") != "" || got.Get("is_nsfw") != "" || got.Get("tag") != "" {
		t.Fatalf("query=%v", got)
	}
}

func TestGetImage(t *testing.T) {
	var gotPath, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"image_uuid":"u","view_url":"/i/s","url":"https://cdn.example/orig.jpg","is_nsfw":false}`)
	}))
	defer srv.Close()
	c := New(srv.URL, "pxl_test", "fotoly-cli/dev", srv.Client())
	img, err := c.GetImage(context.Background(), "u")
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodGet || gotPath != "/api/v1/images/u" {
		t.Fatalf("method=%s path=%s", gotMethod, gotPath)
	}
	if img.ImageUUID != "u" || img.ViewURL != "/i/s" || img.URL != "https://cdn.example/orig.jpg" {
		t.Fatalf("img=%+v", img)
	}
}

func TestGetImageStatus(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"complete":true,"failed":false,"view_url":"/i/s"}`)
	}))
	defer srv.Close()
	c := New(srv.URL, "pxl_test", "fotoly-cli/dev", srv.Client())
	st, err := c.GetImageStatus(context.Background(), "u")
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/api/v1/images/u/status" {
		t.Fatalf("path=%s", gotPath)
	}
	if !st.Complete || st.Failed || st.ViewURL == nil || *st.ViewURL != "/i/s" {
		t.Fatalf("status=%+v", st)
	}
}

func TestDeleteImage(t *testing.T) {
	var gotPath, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, `{"image_uuid":"u","status":"accepted","message":"queued"}`)
	}))
	defer srv.Close()
	c := New(srv.URL, "pxl_test", "fotoly-cli/dev", srv.Client())
	acc, err := c.DeleteImage(context.Background(), "u")
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodDelete || gotPath != "/api/v1/images/u" {
		t.Fatalf("method=%s path=%s", gotMethod, gotPath)
	}
	if acc.ImageUUID != "u" || acc.Status != "accepted" || acc.Message != "queued" {
		t.Fatalf("acc=%+v", acc)
	}
}

func TestGetImageNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":"not_found","message":"Image not found"}`)
	}))
	defer srv.Close()
	c := New(srv.URL, "pxl_test", "fotoly-cli/dev", srv.Client())
	_, err := c.GetImage(context.Background(), "missing")
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.Status != 404 {
		t.Fatalf("err=%v", err)
	}
}
