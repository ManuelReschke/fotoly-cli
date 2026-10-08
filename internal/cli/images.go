package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/ManuelReschke/fotoly-cli/internal/client"
	"github.com/ManuelReschke/fotoly-cli/internal/ui"
	"github.com/spf13/cobra"
)

func (a *App) imagesCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "images", Short: "Manage images"}
	cmd.AddCommand(a.imagesLSCommand())
	cmd.AddCommand(a.imagesGetCommand())
	cmd.AddCommand(a.imagesStatusCommand())
	cmd.AddCommand(a.imagesEditCommand())
	cmd.AddCommand(a.imagesDeleteCommand())
	cmd.AddCommand(a.imagesLikeCommand())
	cmd.AddCommand(a.imagesUnlikeCommand())
	cmd.AddCommand(a.imagesCommentsCommand())
	return cmd
}

func (a *App) imagesLSCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List images",
		RunE:  a.imagesLSRun,
	}
	cmd.Flags().Int("limit", 25, "page size")
	cmd.Flags().String("cursor", "", "pagination cursor")
	cmd.Flags().Int64("album", 0, "filter by album id")
	cmd.Flags().Bool("public", false, "only public images")
	cmd.Flags().Bool("private", false, "only private images")
	cmd.Flags().Bool("nsfw", false, "only NSFW images")
	cmd.Flags().Bool("sfw", false, "only SFW images")
	cmd.Flags().String("tag", "", "filter by tag")
	return cmd
}

func (a *App) imagesLSRun(cmd *cobra.Command, _ []string) error {
	public, err := cmd.Flags().GetBool("public")
	if err != nil {
		return err
	}
	private, err := cmd.Flags().GetBool("private")
	if err != nil {
		return err
	}
	if public && private {
		return fmt.Errorf("%w: --public and --private are mutually exclusive", ErrUsage)
	}
	nsfw, err := cmd.Flags().GetBool("nsfw")
	if err != nil {
		return err
	}
	sfw, err := cmd.Flags().GetBool("sfw")
	if err != nil {
		return err
	}
	if nsfw && sfw {
		return fmt.Errorf("%w: --nsfw and --sfw are mutually exclusive", ErrUsage)
	}

	c, vals, err := a.requireClient(cmd)
	if err != nil {
		return err
	}

	limit, err := cmd.Flags().GetInt("limit")
	if err != nil {
		return err
	}
	cursor, err := cmd.Flags().GetString("cursor")
	if err != nil {
		return err
	}
	albumID, err := cmd.Flags().GetInt64("album")
	if err != nil {
		return err
	}
	tag, err := cmd.Flags().GetString("tag")
	if err != nil {
		return err
	}

	q := client.ImageListQuery{
		Limit:   limit,
		Cursor:  cursor,
		AlbumID: albumID,
		Tag:     tag,
	}
	if public {
		q.IsPublic = boolPtr(true)
	} else if private {
		q.IsPublic = boolPtr(false)
	}
	if nsfw {
		q.IsNSFW = boolPtr(true)
	} else if sfw {
		q.IsNSFW = boolPtr(false)
	}

	col, err := c.ListImages(cmd.Context(), q)
	if err != nil {
		return err
	}
	if a.flagJSON {
		return json.NewEncoder(a.Stdout).Encode(col)
	}
	rows := make([][]string, 0, len(col.Items))
	for _, img := range col.Items {
		rows = append(rows, []string{
			shortUUID(img.ImageUUID),
			img.FileName,
			ui.Bytes(img.FileSize),
			strconv.FormatBool(img.IsPublic),
			strconv.FormatBool(img.IsNSFW),
			resolvedShare(vals.BaseURL, img.ViewURL),
		})
	}
	fmt.Fprint(a.Stdout, ui.Table([]string{"UUID", "FILE", "SIZE", "PUBLIC", "NSFW", "URL"}, rows))
	if col.HasMore && col.NextCursor != nil && *col.NextCursor != "" {
		fmt.Fprintf(a.Stderr, "next cursor: %s\n", *col.NextCursor)
	}
	return nil
}

func (a *App) imagesGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get <uuid>",
		Short: "Get image details",
		Args:  cobra.ExactArgs(1),
		RunE:  a.imagesGetRun,
	}
}

func (a *App) imagesGetRun(cmd *cobra.Command, args []string) error {
	c, vals, err := a.requireClient(cmd)
	if err != nil {
		return err
	}
	img, err := c.GetImage(cmd.Context(), args[0])
	if err != nil {
		return err
	}
	if a.flagJSON {
		return json.NewEncoder(a.Stdout).Encode(img)
	}
	fmt.Fprintf(a.Stdout, "UUID: %s\n", img.ImageUUID)
	fmt.Fprintf(a.Stdout, "URL: %s\n", resolvedShare(vals.BaseURL, img.ViewURL))
	fmt.Fprintf(a.Stdout, "Original: %s\n", img.URL)
	fmt.Fprintf(a.Stdout, "NSFW: %s\n", strconv.FormatBool(img.IsNSFW))
	if len(img.AvailableVariants) > 0 {
		fmt.Fprintf(a.Stdout, "Variants: %s\n", strings.Join(img.AvailableVariants, ", "))
	}
	return nil
}

func (a *App) imagesStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status <uuid>",
		Short: "Show image processing status",
		Args:  cobra.ExactArgs(1),
		RunE:  a.imagesStatusRun,
	}
}

func (a *App) imagesStatusRun(cmd *cobra.Command, args []string) error {
	c, vals, err := a.requireClient(cmd)
	if err != nil {
		return err
	}
	st, err := c.GetImageStatus(cmd.Context(), args[0])
	if err != nil {
		return err
	}
	if a.flagJSON {
		return json.NewEncoder(a.Stdout).Encode(st)
	}
	fmt.Fprintf(a.Stdout, "Complete: %s\n", strconv.FormatBool(st.Complete))
	fmt.Fprintf(a.Stdout, "Failed: %s\n", strconv.FormatBool(st.Failed))
	if st.ViewURL != nil && *st.ViewURL != "" {
		fmt.Fprintf(a.Stdout, "URL: %s\n", resolvedShare(vals.BaseURL, *st.ViewURL))
	}
	return nil
}

func (a *App) imagesEditCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit <uuid>",
		Short: "Update image metadata",
		Args:  cobra.ExactArgs(1),
		RunE:  a.imagesEditRun,
	}
	cmd.Flags().String("title", "", "new title")
	cmd.Flags().String("description", "", "new description")
	cmd.Flags().Bool("public", false, "make the image public")
	cmd.Flags().Bool("private", false, "make the image private")
	cmd.Flags().Bool("nsfw", false, "mark the image NSFW")
	cmd.Flags().Bool("sfw", false, "mark the image SFW")
	cmd.Flags().StringArray("tag", nil, "replace all tags; repeat for each tag")
	cmd.Flags().Bool("clear-tags", false, "remove all tags")
	return cmd
}

func (a *App) imagesEditRun(cmd *cobra.Command, args []string) error {
	public, err := cmd.Flags().GetBool("public")
	if err != nil {
		return err
	}
	private, err := cmd.Flags().GetBool("private")
	if err != nil {
		return err
	}
	if public && private {
		return fmt.Errorf("%w: --public and --private are mutually exclusive", ErrUsage)
	}
	nsfw, err := cmd.Flags().GetBool("nsfw")
	if err != nil {
		return err
	}
	sfw, err := cmd.Flags().GetBool("sfw")
	if err != nil {
		return err
	}
	if nsfw && sfw {
		return fmt.Errorf("%w: --nsfw and --sfw are mutually exclusive", ErrUsage)
	}
	clearTags, err := cmd.Flags().GetBool("clear-tags")
	if err != nil {
		return err
	}
	tags, err := cmd.Flags().GetStringArray("tag")
	if err != nil {
		return err
	}
	if clearTags && cmd.Flags().Changed("tag") {
		return fmt.Errorf("%w: --tag and --clear-tags are mutually exclusive", ErrUsage)
	}
	titleChanged := cmd.Flags().Changed("title")
	descriptionChanged := cmd.Flags().Changed("description")
	if !titleChanged && !descriptionChanged && !public && !private && !nsfw && !sfw && !clearTags && !cmd.Flags().Changed("tag") {
		return fmt.Errorf("%w: set at least one of --title, --description, --public, --private, --nsfw, --sfw, --tag, or --clear-tags", ErrUsage)
	}

	var req client.ImageUpdate
	if titleChanged {
		title, err := cmd.Flags().GetString("title")
		if err != nil {
			return err
		}
		req.Title = &title
	}
	if descriptionChanged {
		description, err := cmd.Flags().GetString("description")
		if err != nil {
			return err
		}
		req.Description = &description
	}
	if public {
		req.IsPublic = boolPtr(true)
	} else if private {
		req.IsPublic = boolPtr(false)
	}
	if nsfw {
		req.IsNSFW = boolPtr(true)
	} else if sfw {
		req.IsNSFW = boolPtr(false)
	}
	if clearTags {
		empty := []string{}
		req.Tags = &empty
	} else if cmd.Flags().Changed("tag") {
		req.Tags = &tags
	}

	c, vals, err := a.requireClient(cmd)
	if err != nil {
		return err
	}
	img, err := c.UpdateImage(cmd.Context(), args[0], req)
	if err != nil {
		return err
	}
	if a.flagJSON {
		return json.NewEncoder(a.Stdout).Encode(img)
	}
	fmt.Fprintf(a.Stdout, "UUID: %s\n", img.ImageUUID)
	fmt.Fprintf(a.Stdout, "Title: %s\n", img.Title)
	fmt.Fprintf(a.Stdout, "Description: %s\n", img.Description)
	fmt.Fprintf(a.Stdout, "Public: %s\n", strconv.FormatBool(img.IsPublic))
	fmt.Fprintf(a.Stdout, "NSFW: %s\n", strconv.FormatBool(img.IsNSFW))
	fmt.Fprintf(a.Stdout, "Tags: %s\n", strings.Join(img.Tags, ", "))
	fmt.Fprintf(a.Stdout, "URL: %s\n", resolvedShare(vals.BaseURL, img.ViewURL))
	return nil
}

func (a *App) imagesDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <uuid...>",
		Short: "Delete images",
		Args:  cobra.MinimumNArgs(1),
		RunE:  a.imagesDeleteRun,
	}
	cmd.Flags().Bool("yes", false, "do not prompt for confirmation")
	return cmd
}

type imageDeleteResult struct {
	ImageUUID string `json:"image_uuid"`
	Status    string `json:"status,omitempty"`
	Message   string `json:"message,omitempty"`
	Error     string `json:"error,omitempty"`
}

func (a *App) imagesDeleteRun(cmd *cobra.Command, args []string) error {
	ok, err := a.confirmDestructive(cmd, fmt.Sprintf("Delete %d image(s)?", len(args)), "use --yes to delete")
	if err != nil || !ok {
		return err
	}

	c, _, err := a.requireClient(cmd)
	if err != nil {
		return err
	}

	results := make([]imageDeleteResult, 0, len(args))
	failed := 0
	for _, uuid := range args {
		acc, err := c.DeleteImage(cmd.Context(), uuid)
		if err != nil {
			failed++
			results = append(results, imageDeleteResult{ImageUUID: uuid, Error: err.Error()})
			if !a.flagJSON {
				fmt.Fprintf(a.Stderr, "%s: %s\n", uuid, err.Error())
			}
			continue
		}
		results = append(results, imageDeleteResult{
			ImageUUID: acc.ImageUUID,
			Status:    acc.Status,
			Message:   acc.Message,
		})
		if !a.flagJSON {
			fmt.Fprintf(a.Stdout, "%s: %s\n", acc.ImageUUID, acc.Status)
		}
	}
	if a.flagJSON {
		if err := json.NewEncoder(a.Stdout).Encode(results); err != nil {
			return err
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d image(s) failed to delete", failed)
	}
	return nil
}

func (a *App) imagesLikeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "like <uuid>",
		Short: "Like an image",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.imagesLikeRun(cmd, args[0], true)
		},
	}
}

func (a *App) imagesUnlikeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "unlike <uuid>",
		Short: "Remove your like from an image",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.imagesLikeRun(cmd, args[0], false)
		},
	}
}

func (a *App) imagesLikeRun(cmd *cobra.Command, raw string, like bool) error {
	uuids, err := parseUUIDs([]string{raw})
	if err != nil {
		return err
	}
	c, _, err := a.requireClient(cmd)
	if err != nil {
		return err
	}
	var st *client.ImageLikeState
	if like {
		st, err = c.LikeImage(cmd.Context(), uuids[0])
	} else {
		st, err = c.UnlikeImage(cmd.Context(), uuids[0])
	}
	if err != nil {
		return err
	}
	if a.flagJSON {
		return json.NewEncoder(a.Stdout).Encode(st)
	}
	fmt.Fprintf(a.Stdout, "UUID: %s\n", st.ImageUUID)
	fmt.Fprintf(a.Stdout, "Liked: %s\n", strconv.FormatBool(st.Liked))
	fmt.Fprintf(a.Stdout, "Likes: %d\n", st.LikeCount)
	return nil
}

func (a *App) imagesCommentsCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "comments", Short: "Manage image comments"}
	cmd.AddCommand(a.imagesCommentsLSCommand())
	cmd.AddCommand(a.imagesCommentsAddCommand())
	cmd.AddCommand(a.imagesCommentsDeleteCommand())
	return cmd
}

func (a *App) imagesCommentsLSCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "ls <uuid>",
		Short: "List the newest comments on an image",
		Long:  "Shows the newest 30 top-level comments and their replies. Older roots are omitted. total_count still counts every comment that is not deleted.",
		Args:  cobra.ExactArgs(1),
		RunE:  a.imagesCommentsLSRun,
	}
}

func (a *App) imagesCommentsLSRun(cmd *cobra.Command, args []string) error {
	uuids, err := parseUUIDs(args)
	if err != nil {
		return err
	}
	c, _, err := a.requireClient(cmd)
	if err != nil {
		return err
	}
	col, err := c.ListImageComments(cmd.Context(), uuids[0])
	if err != nil {
		return err
	}
	if col.TotalCount > int64(countVisibleComments(col.Comments)) {
		fmt.Fprintln(a.Stderr, "older comments omitted")
	}
	if a.flagJSON {
		return json.NewEncoder(a.Stdout).Encode(col)
	}
	for _, comment := range col.Comments {
		writeComment(a.Stdout, comment, "")
	}
	fmt.Fprintf(a.Stdout, "Total: %d\n", col.TotalCount)
	return nil
}

func countVisibleComments(comments []client.ImageComment) int {
	n := 0
	for _, comment := range comments {
		if !comment.Deleted {
			n++
		}
		n += countVisibleComments(comment.Replies)
	}
	return n
}

func writeComment(w io.Writer, comment client.ImageComment, indent string) {
	content := comment.Content
	if comment.Deleted {
		content = "(deleted)"
	}
	fmt.Fprintf(w, "%s#%d %s\n%s%s\n", indent, comment.ID, comment.Username, indent, content)
	for _, reply := range comment.Replies {
		writeComment(w, reply, indent+"  ")
	}
}

func (a *App) imagesCommentsAddCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <uuid>",
		Short: "Comment on an image",
		Args:  cobra.ExactArgs(1),
		RunE:  a.imagesCommentsAddRun,
	}
	cmd.Flags().String("content", "", "comment text, at most 2000 characters")
	cmd.Flags().Int64("reply-to", 0, "top-level comment id to reply to")
	return cmd
}

func (a *App) imagesCommentsAddRun(cmd *cobra.Command, args []string) error {
	uuids, err := parseUUIDs(args)
	if err != nil {
		return err
	}
	if !cmd.Flags().Changed("content") {
		return fmt.Errorf("%w: --content is required", ErrUsage)
	}
	content, err := cmd.Flags().GetString("content")
	if err != nil {
		return err
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return fmt.Errorf("%w: --content cannot be empty", ErrUsage)
	}
	if utf8.RuneCountInString(content) > 2000 {
		return fmt.Errorf("%w: --content must be at most 2000 characters", ErrUsage)
	}
	var parentID *int64
	if cmd.Flags().Changed("reply-to") {
		parent, err := cmd.Flags().GetInt64("reply-to")
		if err != nil {
			return err
		}
		if parent < 1 {
			return fmt.Errorf("%w: --reply-to must be a positive integer", ErrUsage)
		}
		parentID = &parent
	}
	c, _, err := a.requireClient(cmd)
	if err != nil {
		return err
	}
	comment, err := c.CreateImageComment(cmd.Context(), uuids[0], client.ImageCommentCreate{Content: content, ParentID: parentID})
	if err != nil {
		return err
	}
	if a.flagJSON {
		return json.NewEncoder(a.Stdout).Encode(comment)
	}
	writeComment(a.Stdout, *comment, "")
	return nil
}

func (a *App) imagesCommentsDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <id...>",
		Short: "Delete comments",
		Args:  cobra.MinimumNArgs(1),
		RunE:  a.imagesCommentsDeleteRun,
	}
	cmd.Flags().Bool("yes", false, "do not prompt for confirmation")
	return cmd
}

type commentDeleteResult struct {
	ID     int64  `json:"id"`
	Status string `json:"status,omitempty"`
	Error  string `json:"error,omitempty"`
}

func (a *App) imagesCommentsDeleteRun(cmd *cobra.Command, args []string) error {
	ids := make([]int64, len(args))
	for i, raw := range args {
		id, err := parsePositiveID(raw, "comment id")
		if err != nil {
			return err
		}
		ids[i] = id
	}
	ok, err := a.confirmDestructive(cmd, fmt.Sprintf("Delete %d comment(s)?", len(ids)), "use --yes to delete")
	if err != nil || !ok {
		return err
	}
	c, _, err := a.requireClient(cmd)
	if err != nil {
		return err
	}
	results := make([]commentDeleteResult, 0, len(ids))
	failed := 0
	for _, id := range ids {
		if err := c.DeleteComment(cmd.Context(), id); err != nil {
			failed++
			results = append(results, commentDeleteResult{ID: id, Error: err.Error()})
			if !a.flagJSON {
				fmt.Fprintf(a.Stderr, "%d: %s\n", id, err.Error())
			}
			continue
		}
		results = append(results, commentDeleteResult{ID: id, Status: "deleted"})
		if !a.flagJSON {
			fmt.Fprintf(a.Stdout, "%d: deleted\n", id)
		}
	}
	if a.flagJSON {
		if err := json.NewEncoder(a.Stdout).Encode(results); err != nil {
			return err
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d comment(s) failed to delete", failed)
	}
	return nil
}

func boolPtr(v bool) *bool { return &v }

func shortUUID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}
