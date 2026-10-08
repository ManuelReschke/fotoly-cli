package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/ManuelReschke/fotoly-cli/internal/client"
	"github.com/ManuelReschke/fotoly-cli/internal/ui"
	"github.com/spf13/cobra"
)

func (a *App) albumsCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "albums", Short: "Manage albums"}
	cmd.AddCommand(a.albumsLSCommand())
	cmd.AddCommand(a.albumsCreateCommand())
	cmd.AddCommand(a.albumsEditCommand())
	cmd.AddCommand(a.albumsDeleteCommand())
	cmd.AddCommand(a.albumsImagesCommand())
	cmd.AddCommand(a.albumsCoverCommand())
	cmd.AddCommand(a.albumsMembersCommand())
	cmd.AddCommand(a.albumsCategoriesCommand())
	return cmd
}

func (a *App) albumsLSCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List albums",
		RunE:  a.albumsLSRun,
	}
}

func (a *App) albumsLSRun(cmd *cobra.Command, _ []string) error {
	c, vals, err := a.requireClient(cmd)
	if err != nil {
		return err
	}
	col, err := c.ListAlbums(cmd.Context())
	if err != nil {
		return err
	}
	if a.flagJSON {
		return json.NewEncoder(a.Stdout).Encode(col)
	}
	rows := make([][]string, 0, len(col.Albums))
	for _, album := range col.Albums {
		rows = append(rows, []string{
			strconv.FormatInt(album.ID, 10),
			album.Title,
			strconv.FormatInt(album.ImageCount, 10),
			strconv.FormatBool(album.IsPublic),
			resolvedShare(vals.BaseURL, album.ViewURL),
		})
	}
	fmt.Fprint(a.Stdout, ui.Table([]string{"ID", "TITLE", "IMAGES", "PUBLIC", "URL"}, rows))
	return nil
}

func (a *App) albumsCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an album",
		Args:  cobra.NoArgs,
		RunE:  a.albumsCreateRun,
	}
	addAlbumMetaFlags(cmd)
	_ = cmd.MarkFlagRequired("title")
	return cmd
}

func (a *App) albumsCreateRun(cmd *cobra.Command, _ []string) error {
	title, err := cmd.Flags().GetString("title")
	if err != nil {
		return err
	}
	if strings.TrimSpace(title) == "" {
		return fmt.Errorf("%w: --title cannot be empty", ErrUsage)
	}
	isPublic, isNSFW, err := optionalVisibility(cmd)
	if err != nil {
		return err
	}
	description, err := changedString(cmd, "description")
	if err != nil {
		return err
	}
	password, err := changedString(cmd, "password")
	if err != nil {
		return err
	}
	if password != nil && *password == "" {
		password = nil
	}
	sortOrder, err := changedSort(cmd)
	if err != nil {
		return err
	}
	c, vals, err := a.requireClient(cmd)
	if err != nil {
		return err
	}
	album, err := c.CreateAlbum(cmd.Context(), client.AlbumCreate{
		Title:          title,
		Description:    description,
		IsPublic:       isPublic,
		IsNSFW:         isNSFW,
		SharePassword:  password,
		ImageSortOrder: sortOrder,
	})
	if err != nil {
		return err
	}
	return a.writeAlbum(vals.BaseURL, album, false)
}

func (a *App) albumsEditCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit <id>",
		Short: "Update an album",
		Args:  cobra.ExactArgs(1),
		RunE:  a.albumsEditRun,
	}
	addAlbumMetaFlags(cmd)
	return cmd
}

func (a *App) albumsEditRun(cmd *cobra.Command, args []string) error {
	id, err := parsePositiveID(args[0], "album id")
	if err != nil {
		return err
	}
	if !albumMetaChanged(cmd) {
		return fmt.Errorf("%w: set at least one of --title, --description, --public, --private, --nsfw, --sfw, --password, or --sort", ErrUsage)
	}
	title, err := changedString(cmd, "title")
	if err != nil {
		return err
	}
	if title != nil && strings.TrimSpace(*title) == "" {
		return fmt.Errorf("%w: --title cannot be empty", ErrUsage)
	}
	description, err := changedString(cmd, "description")
	if err != nil {
		return err
	}
	isPublic, isNSFW, err := optionalVisibility(cmd)
	if err != nil {
		return err
	}
	password, err := changedString(cmd, "password")
	if err != nil {
		return err
	}
	sortOrder, err := changedSort(cmd)
	if err != nil {
		return err
	}
	c, vals, err := a.requireClient(cmd)
	if err != nil {
		return err
	}
	album, err := c.UpdateAlbum(cmd.Context(), id, client.AlbumUpdate{
		Title:          title,
		Description:    description,
		IsPublic:       isPublic,
		IsNSFW:         isNSFW,
		SharePassword:  password,
		ImageSortOrder: sortOrder,
	})
	if err != nil {
		return err
	}
	return a.writeAlbum(vals.BaseURL, album, false)
}

func (a *App) albumsDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <id...>",
		Short: "Delete albums",
		Args:  cobra.MinimumNArgs(1),
		RunE:  a.albumsDeleteRun,
	}
	cmd.Flags().Bool("yes", false, "do not prompt for confirmation")
	return cmd
}

type albumDeleteResult struct {
	ID     int64  `json:"id"`
	Status string `json:"status,omitempty"`
	Error  string `json:"error,omitempty"`
}

func (a *App) albumsDeleteRun(cmd *cobra.Command, args []string) error {
	ids := make([]int64, len(args))
	for i, raw := range args {
		id, err := parsePositiveID(raw, "album id")
		if err != nil {
			return err
		}
		ids[i] = id
	}
	ok, err := a.confirmDestructive(cmd, fmt.Sprintf("Delete %d album(s)?", len(ids)), "use --yes to delete")
	if err != nil || !ok {
		return err
	}
	c, _, err := a.requireClient(cmd)
	if err != nil {
		return err
	}
	results := make([]albumDeleteResult, 0, len(ids))
	failed := 0
	for _, id := range ids {
		if err := c.DeleteAlbum(cmd.Context(), id); err != nil {
			failed++
			results = append(results, albumDeleteResult{ID: id, Error: err.Error()})
			if !a.flagJSON {
				fmt.Fprintf(a.Stderr, "%d: %s\n", id, err.Error())
			}
			continue
		}
		results = append(results, albumDeleteResult{ID: id, Status: "deleted"})
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
		return fmt.Errorf("%d album(s) failed to delete", failed)
	}
	return nil
}

func (a *App) albumsCoverCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cover <id>",
		Short: "Set or clear an album cover",
		Args:  cobra.ExactArgs(1),
		RunE:  a.albumsCoverRun,
	}
	cmd.Flags().String("image", "", "uuid of an image already in the album")
	cmd.Flags().Bool("clear", false, "remove the cover")
	return cmd
}

func (a *App) albumsCoverRun(cmd *cobra.Command, args []string) error {
	id, err := parsePositiveID(args[0], "album id")
	if err != nil {
		return err
	}
	clear, err := cmd.Flags().GetBool("clear")
	if err != nil {
		return err
	}
	imageChanged := cmd.Flags().Changed("image")
	if clear == imageChanged {
		return fmt.Errorf("%w: set exactly one of --image or --clear", ErrUsage)
	}
	imageUUID := ""
	if imageChanged {
		imageUUID, err = cmd.Flags().GetString("image")
		if err != nil {
			return err
		}
		imageUUID = strings.TrimSpace(imageUUID)
		if imageUUID == "" || len(imageUUID) > 36 {
			return fmt.Errorf("%w: --image must be 1 to 36 characters", ErrUsage)
		}
	}
	c, vals, err := a.requireClient(cmd)
	if err != nil {
		return err
	}
	album, err := c.SetAlbumCover(cmd.Context(), id, imageUUID)
	if err != nil {
		return err
	}
	return a.writeAlbum(vals.BaseURL, album, true)
}

func (a *App) writeAlbum(base string, album *client.AlbumSummary, forceCover bool) error {
	if a.flagJSON {
		if forceCover {
			payload := struct {
				client.AlbumSummary
				CoverImageUUID *string `json:"cover_image_uuid"`
			}{AlbumSummary: *album, CoverImageUUID: album.CoverImageUUID}
			return json.NewEncoder(a.Stdout).Encode(payload)
		}
		return json.NewEncoder(a.Stdout).Encode(album)
	}
	fmt.Fprintf(a.Stdout, "ID: %d\n", album.ID)
	fmt.Fprintf(a.Stdout, "Title: %s\n", album.Title)
	fmt.Fprintf(a.Stdout, "Description: %s\n", album.Description)
	fmt.Fprintf(a.Stdout, "Public: %s\n", strconv.FormatBool(album.IsPublic))
	fmt.Fprintf(a.Stdout, "NSFW: %s\n", strconv.FormatBool(album.IsNSFW))
	if album.ImageSortOrder != "" {
		fmt.Fprintf(a.Stdout, "Sort: %s\n", album.ImageSortOrder)
	}
	if album.HasSharePassword != nil {
		fmt.Fprintf(a.Stdout, "Password: %s\n", strconv.FormatBool(*album.HasSharePassword))
	}
	if album.CoverImageUUID != nil {
		fmt.Fprintf(a.Stdout, "Cover: %s\n", *album.CoverImageUUID)
	} else if forceCover {
		fmt.Fprintf(a.Stdout, "Cover: cleared\n")
	}
	fmt.Fprintf(a.Stdout, "Images: %d\n", album.ImageCount)
	fmt.Fprintf(a.Stdout, "URL: %s\n", resolvedShare(base, album.ViewURL))
	return nil
}

func resolvedShare(base, viewURL string) string {
	u, err := client.ResolveShareURL(base, viewURL)
	if err != nil {
		return ""
	}
	return u
}
