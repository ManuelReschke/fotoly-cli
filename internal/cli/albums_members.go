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

func (a *App) albumsMembersCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "members", Short: "Manage album members"}
	cmd.AddCommand(a.albumsMembersLSCommand())
	cmd.AddCommand(a.albumsMembersAddCommand())
	cmd.AddCommand(a.albumsMembersDeleteCommand())
	return cmd
}

func (a *App) albumsMembersLSCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "ls <album-id>",
		Short: "List album members",
		Args:  cobra.ExactArgs(1),
		RunE:  a.albumsMembersLSRun,
	}
}

func (a *App) albumsMembersLSRun(cmd *cobra.Command, args []string) error {
	id, err := parsePositiveID(args[0], "album id")
	if err != nil {
		return err
	}
	c, _, err := a.requireClient(cmd)
	if err != nil {
		return err
	}
	col, err := c.ListAlbumMembers(cmd.Context(), id)
	if err != nil {
		return err
	}
	if a.flagJSON {
		return json.NewEncoder(a.Stdout).Encode(col)
	}
	rows := make([][]string, 0, len(col.Members))
	for _, member := range col.Members {
		rows = append(rows, []string{
			strconv.FormatInt(member.UserID, 10),
			member.Username,
			member.Role,
			member.Origin,
		})
	}
	fmt.Fprint(a.Stdout, ui.Table([]string{"USER", "NAME", "ROLE", "ORIGIN"}, rows))
	return nil
}

func (a *App) albumsMembersAddCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <album-id>",
		Short: "Invite a member to an album",
		Args:  cobra.ExactArgs(1),
		RunE:  a.albumsMembersAddRun,
	}
	cmd.Flags().Int64("user-id", 0, "user id to invite")
	cmd.Flags().String("username", "", "username to invite")
	return cmd
}

func (a *App) albumsMembersAddRun(cmd *cobra.Command, args []string) error {
	id, err := parsePositiveID(args[0], "album id")
	if err != nil {
		return err
	}
	userChanged := cmd.Flags().Changed("user-id")
	nameChanged := cmd.Flags().Changed("username")
	if userChanged == nameChanged {
		return fmt.Errorf("%w: set exactly one of --user-id or --username", ErrUsage)
	}
	var req client.AlbumMemberInvite
	if userChanged {
		userID, err := cmd.Flags().GetInt64("user-id")
		if err != nil {
			return err
		}
		if userID < 1 {
			return fmt.Errorf("%w: --user-id must be a positive integer", ErrUsage)
		}
		req.UserID = &userID
	} else {
		name, err := cmd.Flags().GetString("username")
		if err != nil {
			return err
		}
		name = strings.TrimSpace(name)
		if name == "" {
			return fmt.Errorf("%w: --username cannot be empty", ErrUsage)
		}
		req.Username = &name
	}
	c, _, err := a.requireClient(cmd)
	if err != nil {
		return err
	}
	res, err := c.InviteAlbumMember(cmd.Context(), id, req)
	if err != nil {
		return err
	}
	if a.flagJSON {
		return json.NewEncoder(a.Stdout).Encode(res)
	}
	fmt.Fprintf(a.Stdout, "User: %s (%d)\n", res.Username, res.UserID)
	fmt.Fprintf(a.Stdout, "Role: %s\n", res.Role)
	fmt.Fprintf(a.Stdout, "Origin: %s\n", res.Origin)
	fmt.Fprintf(a.Stdout, "Already: %s\n", strconv.FormatBool(res.AlreadyMember))
	return nil
}

func (a *App) albumsMembersDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <album-id> <user-id...>",
		Short: "Remove members from an album",
		Args:  cobra.MinimumNArgs(2),
		RunE:  a.albumsMembersDeleteRun,
	}
	cmd.Flags().Bool("yes", false, "do not prompt for confirmation")
	return cmd
}

type memberRemoval struct {
	AlbumID int64  `json:"album_id"`
	UserID  int64  `json:"user_id"`
	Status  string `json:"status,omitempty"`
	Error   string `json:"error,omitempty"`
}

func (a *App) albumsMembersDeleteRun(cmd *cobra.Command, args []string) error {
	albumID, err := parsePositiveID(args[0], "album id")
	if err != nil {
		return err
	}
	userIDs := make([]int64, len(args)-1)
	for i, raw := range args[1:] {
		userID, err := parsePositiveID(raw, "user id")
		if err != nil {
			return err
		}
		userIDs[i] = userID
	}
	ok, err := a.confirmDestructive(cmd, fmt.Sprintf("Remove %d member(s)?", len(userIDs)), "use --yes to remove")
	if err != nil || !ok {
		return err
	}
	c, _, err := a.requireClient(cmd)
	if err != nil {
		return err
	}
	results := make([]memberRemoval, 0, len(userIDs))
	failed := 0
	for _, userID := range userIDs {
		if err := c.RemoveAlbumMember(cmd.Context(), albumID, userID); err != nil {
			failed++
			results = append(results, memberRemoval{AlbumID: albumID, UserID: userID, Error: err.Error()})
			if !a.flagJSON {
				fmt.Fprintf(a.Stderr, "%d: %s\n", userID, err.Error())
			}
			continue
		}
		results = append(results, memberRemoval{AlbumID: albumID, UserID: userID, Status: "removed"})
		if !a.flagJSON {
			fmt.Fprintf(a.Stdout, "%d: removed\n", userID)
		}
	}
	if a.flagJSON {
		if err := json.NewEncoder(a.Stdout).Encode(results); err != nil {
			return err
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d member(s) failed to remove", failed)
	}
	return nil
}
