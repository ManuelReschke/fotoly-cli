package cli

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/ManuelReschke/fotoly-cli/internal/client"
	"github.com/ManuelReschke/fotoly-cli/internal/ui"
	"github.com/spf13/cobra"
)

func (a *App) albumsCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "albums", Short: "List albums"}
	cmd.AddCommand(a.albumsLSCommand())
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

func resolvedShare(base, viewURL string) string {
	u, err := client.ResolveShareURL(base, viewURL)
	if err != nil {
		return ""
	}
	return u
}
