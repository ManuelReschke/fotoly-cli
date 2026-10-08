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

type capturedRequest struct {
	Method string
	Path   string
	Body   string
	Query  url.Values
}

func scriptedClient(t *testing.T, status int, response string) (*Client, *capturedRequest) {
	t.Helper()
	got := &capturedRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got.Method = r.Method
		got.Path = r.URL.EscapedPath()
		got.Body = string(b)
		got.Query = r.URL.Query()
		if r.Header.Get("X-API-Key") != "pxl_test" {
			t.Errorf("api key=%q", r.Header.Get("X-API-Key"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, response)
	}))
	t.Cleanup(srv.Close)
	return New(srv.URL, "pxl_test", "fotoly-cli/dev", srv.Client()), got
}

const albumFixture = `{"id":7,"title":"Trip","description":"days","is_public":true,"is_nsfw":false,"share_link":"tok","view_url":"/a/tok","image_count":1,"image_sort_order":"asc","has_share_password":true,"cover_image_uuid":"img-1","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`

func TestCreateAlbumOmitsUnsetFields(t *testing.T) {
	c, got := scriptedClient(t, http.StatusCreated, albumFixture)
	desc := "days"
	album, err := c.CreateAlbum(context.Background(), AlbumCreate{Title: "Trip", Description: &desc})
	if err != nil {
		t.Fatal(err)
	}
	if got.Method != http.MethodPost || got.Path != "/api/v1/albums" {
		t.Fatalf("method=%s path=%s", got.Method, got.Path)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(got.Body), &body); err != nil {
		t.Fatal(err)
	}
	if body["title"] != "Trip" || body["description"] != "days" {
		t.Fatalf("body=%s", got.Body)
	}
	for _, key := range []string{"is_public", "is_nsfw", "share_password", "image_sort_order"} {
		if _, ok := body[key]; ok {
			t.Fatalf("unset %s was sent: %s", key, got.Body)
		}
	}
	if album.ID != 7 || album.Title != "Trip" || !album.IsPublic || album.ImageSortOrder != "asc" || album.HasSharePassword == nil || !*album.HasSharePassword {
		t.Fatalf("album=%+v", album)
	}
	if album.CoverImageUUID == nil || *album.CoverImageUUID != "img-1" {
		t.Fatalf("cover=%v", album.CoverImageUUID)
	}
}

func TestCreateAlbumSendsPasswordAndSort(t *testing.T) {
	c, got := scriptedClient(t, http.StatusCreated, albumFixture)
	public, nsfw := true, false
	pw, sort := "secret", "asc"
	_, err := c.CreateAlbum(context.Background(), AlbumCreate{
		Title: "Trip", IsPublic: &public, IsNSFW: &nsfw, SharePassword: &pw, ImageSortOrder: &sort,
	})
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(got.Body), &body); err != nil {
		t.Fatal(err)
	}
	if body["share_password"] != "secret" || body["image_sort_order"] != "asc" || body["is_public"] != true || body["is_nsfw"] != false {
		t.Fatalf("body=%s", got.Body)
	}
}

func TestUpdateAlbumSendsEmptyPassword(t *testing.T) {
	c, got := scriptedClient(t, http.StatusOK, albumFixture)
	title, pw := "Renamed", ""
	album, err := c.UpdateAlbum(context.Background(), 7, AlbumUpdate{Title: &title, SharePassword: &pw})
	if err != nil {
		t.Fatal(err)
	}
	if got.Method != http.MethodPatch || got.Path != "/api/v1/albums/7" {
		t.Fatalf("method=%s path=%s", got.Method, got.Path)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(got.Body), &body); err != nil {
		t.Fatal(err)
	}
	if body["title"] != "Renamed" || body["share_password"] != "" {
		t.Fatalf("body=%s", got.Body)
	}
	if _, ok := body["description"]; ok {
		t.Fatalf("unset description was sent: %s", got.Body)
	}
	if album.ID != 7 {
		t.Fatalf("album=%+v", album)
	}
}

func TestDeleteAlbumNoContent(t *testing.T) {
	c, got := scriptedClient(t, http.StatusNoContent, "")
	if err := c.DeleteAlbum(context.Background(), 7); err != nil {
		t.Fatal(err)
	}
	if got.Method != http.MethodDelete || got.Path != "/api/v1/albums/7" || got.Body != "" {
		t.Fatalf("method=%s path=%s body=%q", got.Method, got.Path, got.Body)
	}
}

func TestListAlbumImages(t *testing.T) {
	c, got := scriptedClient(t, http.StatusOK, `{"album":{"id":7,"title":"Trip","description":"","is_public":true,"is_nsfw":false,"share_link":"tok","view_url":"/a/tok","image_count":1,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"},"images":[{"id":1,"image_uuid":"img-1","title":"","description":"","file_name":"a.jpg","file_size":10,"file_type":"image/jpeg","width":1,"height":1,"is_public":true,"is_nsfw":false,"share_link":"s","view_url":"/i/s","view_count":3,"download_count":1,"tags":["beach"],"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}]}`)
	col, err := c.ListAlbumImages(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if got.Method != http.MethodGet || got.Path != "/api/v1/albums/7/images" {
		t.Fatalf("method=%s path=%s", got.Method, got.Path)
	}
	if col.Album.ID != 7 || len(col.Images) != 1 || col.Images[0].ImageUUID != "img-1" || col.Images[0].ViewCount != 3 || len(col.Images[0].Tags) != 1 {
		t.Fatalf("col=%+v", col)
	}
}

func TestAddAlbumImages(t *testing.T) {
	c, got := scriptedClient(t, http.StatusOK, `{"added":["img-1"],"already_assigned":["img-2"],"image_count":2}`)
	res, err := c.AddAlbumImages(context.Background(), 7, []string{"img-1", "img-2"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Method != http.MethodPost || got.Path != "/api/v1/albums/7/images" {
		t.Fatalf("method=%s path=%s", got.Method, got.Path)
	}
	if got.Body != `{"image_uuids":["img-1","img-2"]}` {
		t.Fatalf("body=%s", got.Body)
	}
	if len(res.Added) != 1 || res.Added[0] != "img-1" || len(res.AlreadyAssigned) != 1 || res.ImageCount != 2 {
		t.Fatalf("res=%+v", res)
	}
}

func TestRemoveAlbumImage(t *testing.T) {
	c, got := scriptedClient(t, http.StatusNoContent, "")
	if err := c.RemoveAlbumImage(context.Background(), 7, "img/1"); err != nil {
		t.Fatal(err)
	}
	if got.Method != http.MethodDelete || got.Path != "/api/v1/albums/7/images/img%2F1" {
		t.Fatalf("method=%s path=%s", got.Method, got.Path)
	}
}

func TestSetAlbumCover(t *testing.T) {
	c, got := scriptedClient(t, http.StatusOK, albumFixture)
	if _, err := c.SetAlbumCover(context.Background(), 7, "img-1"); err != nil {
		t.Fatal(err)
	}
	if got.Method != http.MethodPut || got.Path != "/api/v1/albums/7/cover" || got.Body != `{"image_uuid":"img-1"}` {
		t.Fatalf("method=%s path=%s body=%s", got.Method, got.Path, got.Body)
	}
}

func TestClearAlbumCoverSendsEmptyUUID(t *testing.T) {
	cleared := `{"id":7,"title":"Trip","description":"","is_public":false,"is_nsfw":false,"share_link":"tok","view_url":"/a/tok","image_count":1,"cover_image_uuid":null,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"}`
	c, got := scriptedClient(t, http.StatusOK, cleared)
	album, err := c.SetAlbumCover(context.Background(), 7, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Body != `{"image_uuid":""}` {
		t.Fatalf("body=%s", got.Body)
	}
	if album.CoverImageUUID != nil {
		t.Fatalf("cover=%v", album.CoverImageUUID)
	}
}

func TestAlbumMembers(t *testing.T) {
	c, got := scriptedClient(t, http.StatusOK, `{"members":[{"user_id":3,"username":"ada","role":"viewer","origin":"invite","created_at":"2026-01-01T00:00:00Z"}]}`)
	col, err := c.ListAlbumMembers(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if got.Path != "/api/v1/albums/7/members" || len(col.Members) != 1 || col.Members[0].UserID != 3 || col.Members[0].Origin != "invite" {
		t.Fatalf("path=%s col=%+v", got.Path, col)
	}
}

func TestInviteAlbumMemberByUsername(t *testing.T) {
	c, got := scriptedClient(t, http.StatusCreated, `{"user_id":3,"username":"ada","role":"viewer","origin":"invite","created_at":"2026-01-01T00:00:00Z","already_member":false}`)
	name := "Ada"
	res, err := c.InviteAlbumMember(context.Background(), 7, AlbumMemberInvite{Username: &name})
	if err != nil {
		t.Fatal(err)
	}
	if got.Method != http.MethodPost || got.Path != "/api/v1/albums/7/members" || got.Body != `{"username":"Ada"}` {
		t.Fatalf("method=%s path=%s body=%s", got.Method, got.Path, got.Body)
	}
	if res.UserID != 3 || res.AlreadyMember {
		t.Fatalf("res=%+v", res)
	}
}

func TestInviteAlbumMemberAmbiguousUsername(t *testing.T) {
	c, _ := scriptedClient(t, http.StatusConflict, `{"error":"conflict","message":"Username is ambiguous","candidates":[{"user_id":3,"username":"ada"},{"user_id":9,"username":"ada"}]}`)
	name := "ada"
	_, err := c.InviteAlbumMember(context.Background(), 7, AlbumMemberInvite{Username: &name})
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.Status != http.StatusConflict || len(apiErr.Candidates) != 2 || apiErr.Candidates[0].UserID != 3 {
		t.Fatalf("err=%v", err)
	}
	if !strings.Contains(err.Error(), "ada (3)") || !strings.Contains(err.Error(), "ada (9)") {
		t.Fatalf("error=%q", err.Error())
	}
}

func TestRemoveAlbumMember(t *testing.T) {
	c, got := scriptedClient(t, http.StatusNoContent, "")
	if err := c.RemoveAlbumMember(context.Background(), 7, 3); err != nil {
		t.Fatal(err)
	}
	if got.Method != http.MethodDelete || got.Path != "/api/v1/albums/7/members/3" {
		t.Fatalf("method=%s path=%s", got.Method, got.Path)
	}
}

func TestAlbumCategories(t *testing.T) {
	c, got := scriptedClient(t, http.StatusOK, `{"categories":[{"id":4,"name":"travel","slug":"travel","scope":"private","album_count":2}]}`)
	col, err := c.ListAlbumCategories(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Method != http.MethodGet || got.Path != "/api/v1/album-categories" {
		t.Fatalf("method=%s path=%s", got.Method, got.Path)
	}
	if len(col.Categories) != 1 || col.Categories[0].Scope != "private" || col.Categories[0].AlbumCount != 2 {
		t.Fatalf("col=%+v", col)
	}
}

func TestCreateAlbumCategory(t *testing.T) {
	c, got := scriptedClient(t, http.StatusCreated, `{"id":4,"name":"travel","slug":"travel","scope":"private","album_count":0}`)
	cat, err := c.CreateAlbumCategory(context.Background(), "travel")
	if err != nil {
		t.Fatal(err)
	}
	if got.Method != http.MethodPost || got.Body != `{"name":"travel"}` {
		t.Fatalf("method=%s body=%s", got.Method, got.Body)
	}
	if cat.ID != 4 || cat.Name != "travel" {
		t.Fatalf("cat=%+v", cat)
	}
}

func TestSetAlbumCategoriesSendsEmptyLists(t *testing.T) {
	c, got := scriptedClient(t, http.StatusOK, `{"categories":[]}`)
	col, err := c.SetAlbumCategories(context.Background(), 7, nil, []int64{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Method != http.MethodPut || got.Path != "/api/v1/albums/7/categories" {
		t.Fatalf("method=%s path=%s", got.Method, got.Path)
	}
	if got.Body != `{"private_category_ids":[],"public_category_ids":[]}` {
		t.Fatalf("body=%s", got.Body)
	}
	if col.Categories == nil || len(col.Categories) != 0 {
		t.Fatalf("col=%+v", col)
	}
}

func TestLikeAndUnlikeImage(t *testing.T) {
	c, got := scriptedClient(t, http.StatusOK, `{"image_uuid":"img-1","liked":true,"like_count":2}`)
	st, err := c.LikeImage(context.Background(), "img-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Method != http.MethodPut || got.Path != "/api/v1/images/img-1/like" || got.Body != "" {
		t.Fatalf("method=%s path=%s body=%q", got.Method, got.Path, got.Body)
	}
	if !st.Liked || st.LikeCount != 2 {
		t.Fatalf("state=%+v", st)
	}

	c2, got2 := scriptedClient(t, http.StatusOK, `{"image_uuid":"img-1","liked":false,"like_count":1}`)
	st, err = c2.UnlikeImage(context.Background(), "img/1")
	if err != nil {
		t.Fatal(err)
	}
	if got2.Method != http.MethodDelete || got2.Path != "/api/v1/images/img%2F1/like" || st.Liked || st.LikeCount != 1 {
		t.Fatalf("method=%s path=%s state=%+v", got2.Method, got2.Path, st)
	}
}

func TestListNotifications(t *testing.T) {
	c, got := scriptedClient(t, http.StatusOK, `{"unread_count":4,"has_more":true,"next_cursor":"cur","items":[{"id":9,"type":"image_like","title":"New like","body":"ada liked","target_url":"/i/s","is_read":false,"actor_name":"ada","actor_count":1,"event_count":1,"last_event_at":"2026-01-01T00:00:00Z"}]}`)
	limit := 10
	col, err := c.ListNotifications(context.Background(), NotificationQuery{Limit: &limit, Cursor: "abc"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Path != "/api/v1/notifications" || got.Query.Get("limit") != "10" || got.Query.Get("cursor") != "abc" {
		t.Fatalf("path=%s query=%v", got.Path, got.Query)
	}
	if col.UnreadCount != 4 || !col.HasMore || col.NextCursor == nil || *col.NextCursor != "cur" || col.Items[0].Type != "image_like" {
		t.Fatalf("col=%+v", col)
	}
}

func TestListNotificationsSendsExplicitZeroLimit(t *testing.T) {
	limit := 0
	c, got := scriptedClient(t, http.StatusOK, `{"unread_count":0,"has_more":false,"next_cursor":null,"items":[]}`)
	if _, err := c.ListNotifications(context.Background(), NotificationQuery{Limit: &limit}); err != nil {
		t.Fatal(err)
	}
	if got.Query.Get("limit") != "0" {
		t.Fatalf("query=%v", got.Query)
	}
}

func TestListNotificationsOmitsUnsetQuery(t *testing.T) {
	c, got := scriptedClient(t, http.StatusOK, `{"unread_count":0,"has_more":false,"next_cursor":null,"items":[]}`)
	col, err := c.ListNotifications(context.Background(), NotificationQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Query.Get("limit") != "" || got.Query.Get("cursor") != "" || col.NextCursor != nil || len(col.Items) != 0 {
		t.Fatalf("query=%v col=%+v", got.Query, col)
	}
}

func TestImageComments(t *testing.T) {
	body := `{"image_uuid":"img-1","total_count":2,"comments":[{"id":12,"user_id":3,"username":"ada","content":"hi","created_at":"2026-01-01T00:00:00Z","like_count":1,"liked":true,"reply_count":1,"can_delete":true,"deleted":false,"replies":[{"id":13,"user_id":4,"username":"bob","content":"yo","created_at":"2026-01-01T00:00:00Z","like_count":0,"liked":false,"reply_count":0,"can_delete":false,"deleted":false,"parent_id":12,"replies":[]}]}]}`
	c, got := scriptedClient(t, http.StatusOK, body)
	col, err := c.ListImageComments(context.Background(), "img-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Path != "/api/v1/images/img-1/comments" || col.TotalCount != 2 || col.Comments[0].Replies[0].ParentID == nil || *col.Comments[0].Replies[0].ParentID != 12 {
		t.Fatalf("path=%s col=%+v", got.Path, col)
	}
}

func TestCreateImageCommentReply(t *testing.T) {
	c, got := scriptedClient(t, http.StatusCreated, `{"id":13,"user_id":3,"username":"ada","content":"yo","created_at":"2026-01-01T00:00:00Z","like_count":0,"liked":false,"reply_count":0,"can_delete":true,"deleted":false,"parent_id":12,"replies":[]}`)
	parent := int64(12)
	comment, err := c.CreateImageComment(context.Background(), "img-1", ImageCommentCreate{Content: "yo", ParentID: &parent})
	if err != nil {
		t.Fatal(err)
	}
	if got.Method != http.MethodPost || got.Path != "/api/v1/images/img-1/comments" || got.Body != `{"content":"yo","parent_id":12}` {
		t.Fatalf("method=%s path=%s body=%s", got.Method, got.Path, got.Body)
	}
	if comment.ID != 13 || comment.ParentID == nil || *comment.ParentID != 12 {
		t.Fatalf("comment=%+v", comment)
	}
}

func TestDeleteComment(t *testing.T) {
	c, got := scriptedClient(t, http.StatusNoContent, "")
	if err := c.DeleteComment(context.Background(), 13); err != nil {
		t.Fatal(err)
	}
	if got.Method != http.MethodDelete || got.Path != "/api/v1/comments/13" {
		t.Fatalf("method=%s path=%s", got.Method, got.Path)
	}
}
