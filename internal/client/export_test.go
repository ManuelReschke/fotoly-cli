package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCreateExportPollsUntilReady(t *testing.T) {
	var posts, gets int
	var slept []time.Duration
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/user/export" {
			t.Errorf("path=%s", r.URL.Path)
		}
		if r.Header.Get("X-API-Key") != "pxl_test" {
			t.Errorf("key=%q", r.Header.Get("X-API-Key"))
		}
		if r.Header.Get("Authorization") != "" {
			t.Errorf("Authorization=%q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodPost:
			posts++
			_, _ = io.WriteString(w, `{"id":1,"status":"pending","original_bytes":0,"includes_zip":false,"expires_at":null,"error":"","created":false}`)
		case http.MethodGet:
			gets++
			if gets == 1 {
				_, _ = io.WriteString(w, `{"id":1,"status":"processing","original_bytes":0,"includes_zip":false,"expires_at":null,"error":""}`)
				return
			}
			_, _ = io.WriteString(w, `{"id":1,"status":"ready","original_bytes":3,"includes_zip":false,"expires_at":null,"error":""}`)
		default:
			t.Errorf("method=%s", r.Method)
		}
	}))
	defer srv.Close()

	c := New(srv.URL, "pxl_test", "fotoly-cli/dev", srv.Client())
	c.Sleep = func(d time.Duration) { slept = append(slept, d) }
	st, err := c.CreateExport(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	st, err = c.WaitExport(context.Background(), st)
	if err != nil {
		t.Fatal(err)
	}
	if st.Status != "ready" || st.ID != 1 {
		t.Fatalf("status=%+v", st)
	}
	if posts != 1 || gets != 2 {
		t.Fatalf("posts=%d gets=%d", posts, gets)
	}
	if len(slept) != 2 || slept[0] != exportPollInterval || slept[1] != exportPollInterval {
		t.Fatalf("slept=%v", slept)
	}
}

func TestWaitExportReadyDoesNotPoll(t *testing.T) {
	c := New("https://example.test", "pxl_test", "fotoly-cli/dev", nil)
	c.Sleep = func(time.Duration) { t.Fatal("ready export must not sleep") }
	c.HTTP = &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("ready export must not poll")
		return nil, nil
	})}
	st, err := c.WaitExport(context.Background(), &ExportStatus{Status: "ready"})
	if err != nil {
		t.Fatal(err)
	}
	if st.Status != "ready" {
		t.Fatalf("status=%+v", st)
	}
}

func TestWaitExportFailedIncludesError(t *testing.T) {
	c := New("https://example.test", "pxl_test", "fotoly-cli/dev", nil)
	c.Sleep = func(time.Duration) { t.Fatal("failed export must not sleep") }
	_, err := c.WaitExport(context.Background(), &ExportStatus{Status: "failed", Error: "disk full"})
	if err == nil || !strings.Contains(err.Error(), "disk full") {
		t.Fatalf("err=%v", err)
	}
}

func TestWaitExportTimeoutIncludesError(t *testing.T) {
	var gets int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gets++
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":1,"status":"pending","error":"still working"}`)
	}))
	defer srv.Close()

	c := New(srv.URL, "pxl_test", "fotoly-cli/dev", srv.Client())
	c.Sleep = func(time.Duration) {}
	_, err := c.WaitExport(context.Background(), &ExportStatus{Status: "pending", Error: "still working"})
	if err == nil || !strings.Contains(err.Error(), "still working") || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("err=%v", err)
	}
	wantGets := int(exportPollTimeout / exportPollInterval)
	if gets != wantGets {
		t.Fatalf("gets=%d want=%d", gets, wantGets)
	}
}

func TestWaitExportGet404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":"not_found","message":"missing export"}`)
	}))
	defer srv.Close()

	c := New(srv.URL, "pxl_test", "fotoly-cli/dev", srv.Client())
	c.Sleep = func(time.Duration) {}
	_, err := c.WaitExport(context.Background(), &ExportStatus{Status: "pending"})
	if err == nil || !strings.Contains(err.Error(), "missing export") {
		t.Fatalf("err=%v", err)
	}
}

func TestGetExportAccount(t *testing.T) {
	const body = `{"exported_at":"2026-10-24T12:00:00Z","images":[{"uuid":"u","file_name":"cat.jpg","file_size":3,"sha256":"abc","download_path":"/user/export/files/u"}],"extra":true}`
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.Header.Get("X-API-Key") != "pxl_test" {
			t.Errorf("key=%q", r.Header.Get("X-API-Key"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()

	c := New(srv.URL, "pxl_test", "fotoly-cli/dev", srv.Client())
	raw, err := c.GetExportAccount(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/api/v1/user/export/account" {
		t.Fatalf("path=%s", gotPath)
	}
	var account ExportAccount
	if err := json.Unmarshal(raw, &account); err != nil {
		t.Fatal(err)
	}
	if account.ExportedAt != "2026-10-24T12:00:00Z" || len(account.Images) != 1 || account.Images[0].DownloadPath != "/user/export/files/u" {
		t.Fatalf("account=%+v", account)
	}
	if !bytes.Contains(raw, []byte(`"extra":true`)) {
		t.Fatalf("raw dropped unknown field: %s", raw)
	}
}

func TestDownloadExportFileIgnoresShortClientTimeout(t *testing.T) {
	const payload = "raw-bytes"
	var gotKey, gotPath, gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-API-Key")
		gotPath = r.URL.Path
		gotAccept = r.Header.Get("Accept")
		if r.Method != http.MethodGet {
			t.Errorf("method=%s", r.Method)
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = io.WriteString(w, payload)
	}))
	defer srv.Close()

	base := srv.Client()
	base.Timeout = time.Nanosecond
	c := New(srv.URL, "pxl_test", "fotoly-cli/test", base)
	var buf bytes.Buffer
	n, err := c.DownloadExportFile(context.Background(), "/user/export/files/11111111-1111-1111-1111-111111111111", &buf)
	if err != nil {
		t.Fatal(err)
	}
	if n != int64(len(payload)) || buf.String() != payload {
		t.Fatalf("n=%d body=%q", n, buf.String())
	}
	if gotKey != "pxl_test" || gotPath != "/api/v1/user/export/files/11111111-1111-1111-1111-111111111111" {
		t.Fatalf("key=%q path=%s", gotKey, gotPath)
	}
	if gotAccept == "application/json" {
		t.Fatal("file download must not ask for JSON")
	}
	if base.Timeout != time.Nanosecond {
		t.Fatalf("shared client timeout changed: %s", base.Timeout)
	}
}

func TestDownloadExportFileAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = io.WriteString(w, `{"error":"not_ready","message":"export is not ready"}`)
	}))
	defer srv.Close()

	c := New(srv.URL, "pxl_test", "fotoly-cli/dev", srv.Client())
	_, err := c.DownloadExportFile(context.Background(), "/user/export/files/u", &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "export is not ready") {
		t.Fatalf("err=%v", err)
	}
}
