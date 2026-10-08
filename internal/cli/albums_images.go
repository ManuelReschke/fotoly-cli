package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/ManuelReschke/fotoly-cli/internal/ui"
	"github.com/spf13/cobra"
)

func (a *App) albumsImagesCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "images", Short: "Manage images in an album"}
	cmd.AddCommand(a.albumsImagesLSCommand())
	cmd.AddCommand(a.albumsImagesAddCommand())
	cmd.AddCommand(a.albumsImagesDeleteCommand())
	return cmd
}

func (a *App) albumsImagesLSCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "ls <album-id>",
		Short: "List images in an album",
		Args:  cobra.ExactArgs(1),
		RunE:  a.albumsImagesLSRun,
	}
}

func (a *App) albumsImagesLSRun(cmd *cobra.Command, args []string) error {
	id, err := parsePositiveID(args[0], "album id")
	if err != nil {
		return err
	}
	c, vals, err := a.requireClient(cmd)
	if err != nil {
		return err
	}
	col, err := c.ListAlbumImages(cmd.Context(), id)
	if err != nil {
		return err
	}
	if a.flagJSON {
		return json.NewEncoder(a.Stdout).Encode(col)
	}
	fmt.Fprintf(a.Stdout, "Album: %s (%d)\n", col.Album.Title, col.Album.ID)
	rows := make([][]string, 0, len(col.Images))
	for _, img := range col.Images {
		rows = append(rows, []string{
			shortUUID(img.ImageUUID),
			img.FileName,
			ui.Bytes(img.FileSize),
			strconv.FormatBool(img.IsPublic),
			strconv.FormatBool(img.IsNSFW),
			strconv.FormatInt(img.ViewCount, 10),
			strings.Join(img.Tags, ","),
			resolvedShare(vals.BaseURL, img.ViewURL),
		})
	}
	fmt.Fprint(a.Stdout, ui.Table([]string{"UUID", "FILE", "SIZE", "PUBLIC", "NSFW", "VIEWS", "TAGS", "URL"}, rows))
	return nil
}

func (a *App) albumsImagesAddCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "add <album-id> <uuid...>",
		Short: "Add existing images to an album",
		Args:  cobra.MinimumNArgs(2),
		RunE:  a.albumsImagesAddRun,
	}
}

func (a *App) albumsImagesAddRun(cmd *cobra.Command, args []string) error {
	id, err := parsePositiveID(args[0], "album id")
	if err != nil {
		return err
	}
	uuids, err := parseUUIDs(args[1:])
	if err != nil {
		return err
	}
	c, _, err := a.requireClient(cmd)
	if err != nil {
		return err
	}
	res, err := c.AddAlbumImages(cmd.Context(), id, uuids)
	if err != nil {
		return err
	}
	if a.flagJSON {
		return json.NewEncoder(a.Stdout).Encode(res)
	}
	fmt.Fprintf(a.Stdout, "Added: %s\n", joinOrDash(res.Added))
	fmt.Fprintf(a.Stdout, "Already: %s\n", joinOrDash(res.AlreadyAssigned))
	fmt.Fprintf(a.Stdout, "Images: %d\n", res.ImageCount)
	return nil
}

func (a *App) albumsImagesDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <album-id> <uuid...>",
		Short: "Remove images from an album",
		Args:  cobra.MinimumNArgs(2),
		RunE:  a.albumsImagesDeleteRun,
	}
	cmd.Flags().Bool("yes", false, "do not prompt for confirmation")
	return cmd
}

type albumImageRemoval struct {
	AlbumID   int64  `json:"album_id"`
	ImageUUID string `json:"image_uuid"`
	Status    string `json:"status,omitempty"`
	Error     string `json:"error,omitempty"`
}

func (a *App) albumsImagesDeleteRun(cmd *cobra.Command, args []string) error {
	id, err := parsePositiveID(args[0], "album id")
	if err != nil {
		return err
	}
	uuids, err := parseUUIDs(args[1:])
	if err != nil {
		return err
	}
	ok, err := a.confirmDestructive(cmd, fmt.Sprintf("Remove %d image(s) from the album?", len(uuids)), "use --yes to remove")
	if err != nil || !ok {
		return err
	}
	c, _, err := a.requireClient(cmd)
	if err != nil {
		return err
	}
	results := make([]albumImageRemoval, 0, len(uuids))
	failed := 0
	for _, uuid := range uuids {
		if err := c.RemoveAlbumImage(cmd.Context(), id, uuid); err != nil {
			failed++
			results = append(results, albumImageRemoval{AlbumID: id, ImageUUID: uuid, Error: err.Error()})
			if !a.flagJSON {
				fmt.Fprintf(a.Stderr, "%s: %s\n", uuid, err.Error())
			}
			continue
		}
		results = append(results, albumImageRemoval{AlbumID: id, ImageUUID: uuid, Status: "removed"})
		if !a.flagJSON {
			fmt.Fprintf(a.Stdout, "%s: removed\n", uuid)
		}
	}
	if a.flagJSON {
		if err := json.NewEncoder(a.Stdout).Encode(results); err != nil {
			return err
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d image(s) failed to remove", failed)
	}
	return nil
}

func joinOrDash(items []string) string {
	if len(items) == 0 {
		return "-"
	}
	return strings.Join(items, ", ")
}
