package cli

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/ManuelReschke/fotoly-cli/internal/client"
	"github.com/ManuelReschke/fotoly-cli/internal/ui"
	"github.com/spf13/cobra"
)

func (a *App) notificationsCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "notifications", Short: "List notifications"}
	cmd.AddCommand(a.notificationsLSCommand())
	return cmd
}

func (a *App) notificationsLSCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List notifications",
		Args:  cobra.NoArgs,
		RunE:  a.notificationsLSRun,
	}
	cmd.Flags().Int("limit", 30, "page size, from 1 to 100")
	cmd.Flags().String("cursor", "", "pagination cursor")
	return cmd
}

func (a *App) notificationsLSRun(cmd *cobra.Command, _ []string) error {
	limit, err := cmd.Flags().GetInt("limit")
	if err != nil {
		return err
	}
	cursor, err := cmd.Flags().GetString("cursor")
	if err != nil {
		return err
	}
	if limit < 1 || limit > 100 {
		return fmt.Errorf("%w: --limit must be from 1 to 100", ErrUsage)
	}
	c, vals, err := a.requireClient(cmd)
	if err != nil {
		return err
	}
	col, err := c.ListNotifications(cmd.Context(), client.NotificationQuery{Limit: &limit, Cursor: cursor})
	if err != nil {
		return err
	}
	if a.flagJSON {
		return json.NewEncoder(a.Stdout).Encode(col)
	}
	fmt.Fprintf(a.Stdout, "Unread: %d\n", col.UnreadCount)
	rows := make([][]string, 0, len(col.Items))
	for _, item := range col.Items {
		rows = append(rows, []string{
			strconv.FormatInt(item.ID, 10),
			strconv.FormatBool(item.IsRead),
			item.Type,
			item.Title,
			item.ActorName,
			resolvedShare(vals.BaseURL, item.TargetURL),
		})
	}
	fmt.Fprint(a.Stdout, ui.Table([]string{"ID", "READ", "TYPE", "TITLE", "ACTOR", "URL"}, rows))
	if col.HasMore && col.NextCursor != nil && *col.NextCursor != "" {
		fmt.Fprintf(a.Stderr, "next cursor: %s\n", *col.NextCursor)
	}
	return nil
}
