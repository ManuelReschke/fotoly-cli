package cli

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ManuelReschke/fotoly-cli/internal/brand"
)

type apiCapture struct {
	Method string
	Path   string
	Body   string
	Query  url.Values
}

func scriptedAPI(t *testing.T, status int, response string) (*httptest.Server, *apiCapture) {
	t.Helper()
	got := &apiCapture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got.Method = r.Method
		got.Path = r.URL.EscapedPath()
		got.Body = string(b)
		got.Query = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, response)
	}))
	t.Cleanup(srv.Close)
	return srv, got
}

const albumResponse = `{"id":7,"title":"Trip","description":"days","is_public":true,"is_nsfw":false,"share_link":"tok","view_url":"/a/tok","image_count":1,"image_sort_order":"asc","has_share_password":true,"cover_image_uuid":"img-1","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`

func TestAlbumsCreate(t *testing.T) {
	a, stdout, stderr := newTestApp(t, brand.Fotoly)
	srv, got := scriptedAPI(t, http.StatusCreated, albumResponse)
	attachServer(a, srv)
	saveTestAPIKey(t, a, srv.URL)
	a.Root().SetArgs([]string{"albums", "create", "--title", "Trip", "--description", "days", "--public", "--password", "secret", "--sort", "asc"})
	if code := Run(a); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if got.Method != http.MethodPost || got.Path != "/api/v1/albums" {
		t.Fatalf("method=%s path=%s", got.Method, got.Path)
	}
	for _, part := range []string{`"title":"Trip"`, `"description":"days"`, `"is_public":true`, `"share_password":"secret"`, `"image_sort_order":"asc"`} {
		if !strings.Contains(got.Body, part) {
			t.Fatalf("body=%s missing %s", got.Body, part)
		}
	}
	if strings.Contains(got.Body, "is_nsfw") {
		t.Fatalf("unset nsfw was sent: %s", got.Body)
	}
	if !strings.Contains(stdout.String(), "ID: 7") || !strings.Contains(stdout.String(), "Trip") {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestAlbumsCreateRejectsVisibilityConflict(t *testing.T) {
	a, _, stderr := newTestApp(t, brand.Fotoly)
	a.Root().SetArgs([]string{"albums", "create", "--title", "Trip", "--public", "--private"})
	if code := Run(a); code != 2 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "--public and --private are mutually exclusive") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestAlbumsCreateRequiresTitle(t *testing.T) {
	a, _, stderr := newTestApp(t, brand.Fotoly)
	a.Root().SetArgs([]string{"albums", "create"})
	if code := Run(a); code != 2 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
}

func TestAlbumsEditRequiresAFlag(t *testing.T) {
	a, _, stderr := newTestApp(t, brand.Fotoly)
	a.Root().SetArgs([]string{"albums", "edit", "7"})
	if code := Run(a); code != 2 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "at least one") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestAlbumsEditClearsPassword(t *testing.T) {
	a, stdout, stderr := newTestApp(t, brand.Fotoly)
	srv, got := scriptedAPI(t, http.StatusOK, albumResponse)
	attachServer(a, srv)
	saveTestAPIKey(t, a, srv.URL)
	a.Root().SetArgs([]string{"albums", "edit", "7", "--password", ""})
	if code := Run(a); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if got.Method != http.MethodPatch || got.Path != "/api/v1/albums/7" || got.Body != `{"share_password":""}` {
		t.Fatalf("method=%s path=%s body=%s", got.Method, got.Path, got.Body)
	}
	if !strings.Contains(stdout.String(), "Password: true") {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestAlbumsEditRejectsBadSort(t *testing.T) {
	a, _, stderr := newTestApp(t, brand.Fotoly)
	a.Root().SetArgs([]string{"albums", "edit", "7", "--sort", "sideways"})
	if code := Run(a); code != 2 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
}

func TestAlbumsDeleteRequiresYes(t *testing.T) {
	a, _, stderr := newTestApp(t, brand.Fotoly)
	srv, got := scriptedAPI(t, http.StatusNoContent, "")
	attachServer(a, srv)
	saveTestAPIKey(t, a, srv.URL)
	a.Root().SetArgs([]string{"albums", "delete", "7"})
	if code := Run(a); code != 1 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "use --yes to delete") {
		t.Fatalf("stderr=%q", stderr.String())
	}
	if got.Method != "" {
		t.Fatalf("api was called: %+v", got)
	}
}

func TestAlbumsDeleteYes(t *testing.T) {
	a, stdout, stderr := newTestApp(t, brand.Fotoly)
	srv, got := scriptedAPI(t, http.StatusNoContent, "")
	attachServer(a, srv)
	saveTestAPIKey(t, a, srv.URL)
	a.Root().SetArgs([]string{"albums", "delete", "7", "--yes", "--json"})
	if code := Run(a); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if got.Method != http.MethodDelete || got.Path != "/api/v1/albums/7" {
		t.Fatalf("method=%s path=%s", got.Method, got.Path)
	}
	if !strings.Contains(stdout.String(), `"status":"deleted"`) || !strings.Contains(stdout.String(), `"id":7`) {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestAlbumsImagesLS(t *testing.T) {
	a, stdout, stderr := newTestApp(t, brand.Fotoly)
	body := `{"album":{"id":7,"title":"Trip","description":"","is_public":true,"is_nsfw":false,"share_link":"tok","view_url":"/a/tok","image_count":1,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"},"images":[{"id":1,"image_uuid":"12345678-aaaa-bbbb-cccc-ddddeeeeffff","title":"","description":"","file_name":"a.jpg","file_size":10,"file_type":"image/jpeg","width":1,"height":1,"is_public":true,"is_nsfw":false,"share_link":"s","view_url":"/i/s","view_count":3,"download_count":1,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}]}`
	srv, got := scriptedAPI(t, http.StatusOK, body)
	attachServer(a, srv)
	saveTestAPIKey(t, a, srv.URL)
	a.Root().SetArgs([]string{"albums", "images", "ls", "7"})
	if code := Run(a); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if got.Method != http.MethodGet || got.Path != "/api/v1/albums/7/images" {
		t.Fatalf("method=%s path=%s", got.Method, got.Path)
	}
	if !strings.Contains(stdout.String(), "a.jpg") || !strings.Contains(stdout.String(), "12345678") {
		t.Fatalf("stdout=%q", stdout.String())
	}
	if strings.Contains(stdout.String(), testImageUUID) {
		t.Fatalf("human output must truncate uuid: %q", stdout.String())
	}
}

func TestAlbumsImagesAddAndList(t *testing.T) {
	a, stdout, stderr := newTestApp(t, brand.Fotoly)
	srv, got := scriptedAPI(t, http.StatusOK, `{"added":["img-1"],"already_assigned":[],"image_count":1}`)
	attachServer(a, srv)
	saveTestAPIKey(t, a, srv.URL)
	a.Root().SetArgs([]string{"albums", "images", "add", "7", "img-1"})
	if code := Run(a); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if got.Method != http.MethodPost || got.Path != "/api/v1/albums/7/images" || got.Body != `{"image_uuids":["img-1"]}` {
		t.Fatalf("method=%s path=%s body=%s", got.Method, got.Path, got.Body)
	}
	if !strings.Contains(stdout.String(), "img-1") {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestAlbumsImagesDeleteRequiresYes(t *testing.T) {
	a, _, stderr := newTestApp(t, brand.Fotoly)
	srv, got := scriptedAPI(t, http.StatusNoContent, "")
	attachServer(a, srv)
	saveTestAPIKey(t, a, srv.URL)
	a.Root().SetArgs([]string{"albums", "images", "delete", "7", "img-1"})
	if code := Run(a); code != 1 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "use --yes to remove") || got.Method != "" {
		t.Fatalf("stderr=%q method=%s", stderr.String(), got.Method)
	}
}

func TestAlbumsCoverClearJSON(t *testing.T) {
	a, stdout, stderr := newTestApp(t, brand.Fotoly)
	cleared := `{"id":7,"title":"Trip","description":"","is_public":false,"is_nsfw":false,"share_link":"tok","view_url":"/a/tok","image_count":1,"cover_image_uuid":null,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`
	srv, got := scriptedAPI(t, http.StatusOK, cleared)
	attachServer(a, srv)
	saveTestAPIKey(t, a, srv.URL)
	a.Root().SetArgs([]string{"albums", "cover", "7", "--clear", "--json"})
	if code := Run(a); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if got.Method != http.MethodPut || got.Path != "/api/v1/albums/7/cover" || got.Body != `{"image_uuid":""}` {
		t.Fatalf("method=%s path=%s body=%s", got.Method, got.Path, got.Body)
	}
	if !strings.Contains(stdout.String(), `"cover_image_uuid":null`) {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestAlbumsMembersAddUsername(t *testing.T) {
	a, stdout, stderr := newTestApp(t, brand.Fotoly)
	srv, got := scriptedAPI(t, http.StatusCreated, `{"user_id":3,"username":"ada","role":"viewer","origin":"invite","created_at":"2026-01-01T00:00:00Z","already_member":false}`)
	attachServer(a, srv)
	saveTestAPIKey(t, a, srv.URL)
	a.Root().SetArgs([]string{"albums", "members", "add", "7", "--username", "Ada"})
	if code := Run(a); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if got.Method != http.MethodPost || got.Path != "/api/v1/albums/7/members" || got.Body != `{"username":"Ada"}` {
		t.Fatalf("method=%s path=%s body=%s", got.Method, got.Path, got.Body)
	}
	if !strings.Contains(stdout.String(), "ada") || !strings.Contains(stdout.String(), "3") {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestAlbumsMembersAddRejectsBothIdentifiers(t *testing.T) {
	a, _, stderr := newTestApp(t, brand.Fotoly)
	a.Root().SetArgs([]string{"albums", "members", "add", "7", "--user-id", "3", "--username", "Ada"})
	if code := Run(a); code != 2 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
}

func TestAlbumsMembersInviteShowsCandidates(t *testing.T) {
	a, _, stderr := newTestApp(t, brand.Fotoly)
	srv, _ := scriptedAPI(t, http.StatusConflict, `{"error":"conflict","message":"Username is ambiguous","candidates":[{"user_id":3,"username":"ada"},{"user_id":9,"username":"ada"}]}`)
	attachServer(a, srv)
	saveTestAPIKey(t, a, srv.URL)
	a.Root().SetArgs([]string{"albums", "members", "add", "7", "--username", "ada"})
	if code := Run(a); code != 1 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "ada (3)") || !strings.Contains(stderr.String(), "ada (9)") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestAlbumsCategoriesSetRequiresBothSides(t *testing.T) {
	a, _, stderr := newTestApp(t, brand.Fotoly)
	srv, got := scriptedAPI(t, http.StatusOK, `{"categories":[]}`)
	attachServer(a, srv)
	saveTestAPIKey(t, a, srv.URL)
	a.Root().SetArgs([]string{"albums", "categories", "set", "7", "--private", "4"})
	if code := Run(a); code != 2 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if got.Method != "" {
		t.Fatalf("api was called: %+v", got)
	}
}

func TestAlbumsCategoriesSetEmptyClears(t *testing.T) {
	a, _, stderr := newTestApp(t, brand.Fotoly)
	srv, got := scriptedAPI(t, http.StatusOK, `{"categories":[]}`)
	attachServer(a, srv)
	saveTestAPIKey(t, a, srv.URL)
	a.Root().SetArgs([]string{"albums", "categories", "set", "7", "--private", "", "--public", "4, 5"})
	if code := Run(a); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if got.Method != http.MethodPut || got.Path != "/api/v1/albums/7/categories" || got.Body != `{"private_category_ids":[],"public_category_ids":[4,5]}` {
		t.Fatalf("method=%s path=%s body=%s", got.Method, got.Path, got.Body)
	}
}

func TestImagesLike(t *testing.T) {
	a, stdout, stderr := newTestApp(t, brand.Fotoly)
	srv, got := scriptedAPI(t, http.StatusOK, `{"image_uuid":"img-1","liked":true,"like_count":2}`)
	attachServer(a, srv)
	saveTestAPIKey(t, a, srv.URL)
	a.Root().SetArgs([]string{"images", "like", "img-1"})
	if code := Run(a); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if got.Method != http.MethodPut || got.Path != "/api/v1/images/img-1/like" || got.Body != "" {
		t.Fatalf("method=%s path=%s body=%q", got.Method, got.Path, got.Body)
	}
	if !strings.Contains(stdout.String(), "Liked: true") || !strings.Contains(stdout.String(), "Likes: 2") {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestNotificationsLS(t *testing.T) {
	a, stdout, stderr := newTestApp(t, brand.Fotoly)
	srv, got := scriptedAPI(t, http.StatusOK, `{"unread_count":4,"has_more":true,"next_cursor":"cur","items":[{"id":9,"type":"image_like","title":"New like","body":"ada liked","target_url":"/i/s","is_read":false,"actor_name":"ada","actor_count":1,"event_count":1,"last_event_at":"2026-01-01T00:00:00Z"}]}`)
	attachServer(a, srv)
	saveTestAPIKey(t, a, srv.URL)
	a.Root().SetArgs([]string{"notifications", "ls", "--limit", "10", "--cursor", "abc"})
	if code := Run(a); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if got.Path != "/api/v1/notifications" || got.Query.Get("limit") != "10" || got.Query.Get("cursor") != "abc" {
		t.Fatalf("path=%s query=%v", got.Path, got.Query)
	}
	if !strings.Contains(stdout.String(), "New like") || !strings.Contains(stdout.String(), "Unread: 4") {
		t.Fatalf("stdout=%q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "cur") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestImageCommentsAddReply(t *testing.T) {
	a, stdout, stderr := newTestApp(t, brand.Fotoly)
	srv, got := scriptedAPI(t, http.StatusCreated, `{"id":13,"user_id":3,"username":"ada","content":"yo","created_at":"2026-01-01T00:00:00Z","like_count":0,"liked":false,"reply_count":0,"can_delete":true,"deleted":false,"parent_id":12,"replies":[]}`)
	attachServer(a, srv)
	saveTestAPIKey(t, a, srv.URL)
	a.Root().SetArgs([]string{"images", "comments", "add", "img-1", "--content", "yo", "--reply-to", "12"})
	if code := Run(a); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if got.Method != http.MethodPost || got.Path != "/api/v1/images/img-1/comments" || got.Body != `{"content":"yo","parent_id":12}` {
		t.Fatalf("method=%s path=%s body=%s", got.Method, got.Path, got.Body)
	}
	if !strings.Contains(stdout.String(), "yo") || !strings.Contains(stdout.String(), "13") {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestAlbumsEditRejectsFalseVisibility(t *testing.T) {
	a, _, stderr := newTestApp(t, brand.Fotoly)
	srv, got := scriptedAPI(t, http.StatusOK, albumResponse)
	attachServer(a, srv)
	saveTestAPIKey(t, a, srv.URL)
	a.Root().SetArgs([]string{"albums", "edit", "7", "--public=false"})
	if code := Run(a); code != 2 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "--private") || got.Method != "" {
		t.Fatalf("stderr=%q method=%s", stderr.String(), got.Method)
	}
}

func TestNotificationsRejectsBadLimit(t *testing.T) {
	a, _, stderr := newTestApp(t, brand.Fotoly)
	srv, got := scriptedAPI(t, http.StatusOK, `{"unread_count":0,"has_more":false,"next_cursor":null,"items":[]}`)
	attachServer(a, srv)
	saveTestAPIKey(t, a, srv.URL)
	a.Root().SetArgs([]string{"notifications", "ls", "--limit", "0"})
	if code := Run(a); code != 2 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if got.Method != "" {
		t.Fatalf("api was called: %+v", got)
	}
	if !strings.Contains(stderr.String(), "--limit") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestImageCommentsAddRejectsLongContent(t *testing.T) {
	a, _, stderr := newTestApp(t, brand.Fotoly)
	a.Root().SetArgs([]string{"images", "comments", "add", "img-1", "--content", strings.Repeat("ä", 2001)})
	if code := Run(a); code != 2 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "2000") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestAlbumsImagesLSJSONKeepsFields(t *testing.T) {
	a, stdout, stderr := newTestApp(t, brand.Fotoly)
	body := `{"album":{"id":7,"title":"Trip","description":"","is_public":true,"is_nsfw":false,"share_link":"tok","view_url":"/a/tok","image_count":1,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"},"images":[{"id":1,"image_uuid":"img-1","title":"","description":"","file_name":"a.jpg","file_size":10,"file_type":"image/jpeg","width":1,"height":1,"is_public":true,"is_nsfw":false,"tags":[],"share_link":"s","view_url":"/i/s","view_count":3,"download_count":1,"processing_profile":"default","last_viewed_at":null,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}]}`
	srv, _ := scriptedAPI(t, http.StatusOK, body)
	attachServer(a, srv)
	saveTestAPIKey(t, a, srv.URL)
	a.Root().SetArgs([]string{"albums", "images", "ls", "7", "--json"})
	if code := Run(a); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	for _, part := range []string{`"tags":[]`, `"processing_profile":"default"`, `"last_viewed_at":null`} {
		if !strings.Contains(out, part) {
			t.Fatalf("stdout=%s missing %s", out, part)
		}
	}
}

func TestImageCommentsLSWarnsWhenTruncated(t *testing.T) {
	a, _, stderr := newTestApp(t, brand.Fotoly)
	body := `{"image_uuid":"img-1","total_count":5,"comments":[{"id":12,"user_id":3,"username":"ada","content":"hi","created_at":"2026-01-01T00:00:00Z","like_count":0,"liked":false,"reply_count":0,"can_delete":true,"deleted":false,"replies":[]}]}`
	srv, _ := scriptedAPI(t, http.StatusOK, body)
	attachServer(a, srv)
	saveTestAPIKey(t, a, srv.URL)
	a.Root().SetArgs([]string{"images", "comments", "ls", "img-1"})
	if code := Run(a); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "older comments omitted") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestImageCommentsDeleteRequiresYes(t *testing.T) {
	a, _, stderr := newTestApp(t, brand.Fotoly)
	srv, got := scriptedAPI(t, http.StatusNoContent, "")
	attachServer(a, srv)
	saveTestAPIKey(t, a, srv.URL)
	a.Root().SetArgs([]string{"images", "comments", "delete", "13"})
	if code := Run(a); code != 1 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "use --yes to delete") || got.Method != "" {
		t.Fatalf("stderr=%q method=%s", stderr.String(), got.Method)
	}
}
