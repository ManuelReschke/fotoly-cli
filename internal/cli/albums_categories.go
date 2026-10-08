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

func (a *App) albumsCategoriesCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "categories", Short: "Manage album categories"}
	cmd.AddCommand(a.albumsCategoriesLSCommand())
	cmd.AddCommand(a.albumsCategoriesCreateCommand())
	cmd.AddCommand(a.albumsCategoriesSetCommand())
	return cmd
}

func (a *App) albumsCategoriesLSCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List album categories",
		Args:  cobra.NoArgs,
		RunE:  a.albumsCategoriesLSRun,
	}
}

func (a *App) albumsCategoriesLSRun(cmd *cobra.Command, _ []string) error {
	c, _, err := a.requireClient(cmd)
	if err != nil {
		return err
	}
	col, err := c.ListAlbumCategories(cmd.Context())
	if err != nil {
		return err
	}
	return a.writeCategories(col)
}

func (a *App) albumsCategoriesCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a private album category",
		Args:  cobra.NoArgs,
		RunE:  a.albumsCategoriesCreateRun,
	}
	cmd.Flags().String("name", "", "category name")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func (a *App) albumsCategoriesCreateRun(cmd *cobra.Command, _ []string) error {
	name, err := cmd.Flags().GetString("name")
	if err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("%w: --name cannot be empty", ErrUsage)
	}
	c, _, err := a.requireClient(cmd)
	if err != nil {
		return err
	}
	cat, err := c.CreateAlbumCategory(cmd.Context(), name)
	if err != nil {
		return err
	}
	if a.flagJSON {
		return json.NewEncoder(a.Stdout).Encode(cat)
	}
	fmt.Fprintf(a.Stdout, "ID: %d\n", cat.ID)
	fmt.Fprintf(a.Stdout, "Name: %s\n", cat.Name)
	fmt.Fprintf(a.Stdout, "Slug: %s\n", cat.Slug)
	fmt.Fprintf(a.Stdout, "Scope: %s\n", cat.Scope)
	return nil
}

func (a *App) albumsCategoriesSetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <album-id>",
		Short: "Replace categories on an album",
		Long:  "Replaces both category lists. Pass each flag, using an empty value to clear that side.",
		Args:  cobra.ExactArgs(1),
		RunE:  a.albumsCategoriesSetRun,
	}
	cmd.Flags().String("private", "", "comma-separated private category ids; empty clears them")
	cmd.Flags().String("public", "", "comma-separated public category ids; empty clears them")
	return cmd
}

func (a *App) albumsCategoriesSetRun(cmd *cobra.Command, args []string) error {
	id, err := parsePositiveID(args[0], "album id")
	if err != nil {
		return err
	}
	if !cmd.Flags().Changed("private") || !cmd.Flags().Changed("public") {
		return fmt.Errorf("%w: set both --private and --public; an empty value clears that side", ErrUsage)
	}
	privateRaw, err := cmd.Flags().GetString("private")
	if err != nil {
		return err
	}
	publicRaw, err := cmd.Flags().GetString("public")
	if err != nil {
		return err
	}
	privateIDs, err := parseIDList(privateRaw)
	if err != nil {
		return err
	}
	publicIDs, err := parseIDList(publicRaw)
	if err != nil {
		return err
	}
	c, _, err := a.requireClient(cmd)
	if err != nil {
		return err
	}
	col, err := c.SetAlbumCategories(cmd.Context(), id, privateIDs, publicIDs)
	if err != nil {
		return err
	}
	return a.writeCategories(col)
}

func (a *App) writeCategories(col *client.AlbumCategoryCollection) error {
	if a.flagJSON {
		return json.NewEncoder(a.Stdout).Encode(col)
	}
	rows := make([][]string, 0, len(col.Categories))
	for _, cat := range col.Categories {
		rows = append(rows, []string{
			strconv.FormatInt(cat.ID, 10),
			cat.Name,
			cat.Scope,
			strconv.FormatInt(cat.AlbumCount, 10),
		})
	}
	fmt.Fprint(a.Stdout, ui.Table([]string{"ID", "NAME", "SCOPE", "ALBUMS"}, rows))
	return nil
}
