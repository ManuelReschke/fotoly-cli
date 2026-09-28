package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

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
	yes, err := cmd.Flags().GetBool("yes")
	if err != nil {
		return err
	}
	if !yes {
		if a.IsTTY != nil && a.IsTTY() && a.Prompter != nil {
			ok, err := a.Prompter.ConfirmDelete(len(args))
			if err != nil {
				return err
			}
			if !ok {
				fmt.Fprintln(a.Stdout, "Aborted.")
				return nil
			}
		} else {
			return errors.New("use --yes to delete")
		}
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

func boolPtr(v bool) *bool { return &v }

func shortUUID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}
