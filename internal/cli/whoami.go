package cli

import (
	"encoding/json"
	"fmt"

	"github.com/ManuelReschke/fotoly-cli/internal/client"
	"github.com/ManuelReschke/fotoly-cli/internal/ui"
	"github.com/spf13/cobra"
)

func (a *App) whoamiCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "whoami",
		Aliases: []string{"me"},
		Short:   "Show the authenticated account",
		RunE:    a.whoamiRun,
	}
}

func (a *App) whoamiRun(cmd *cobra.Command, _ []string) error {
	c, _, err := a.requireClient(cmd)
	if err != nil {
		return err
	}
	acc, err := c.GetProfile(cmd.Context())
	if err != nil {
		if client.IsUnauthorized(err) {
			return fmt.Errorf("Invalid API key. Run '%s setup'.", a.Brand.Binary)
		}
		return err
	}
	if a.flagJSON {
		return json.NewEncoder(a.Stdout).Encode(acc)
	}
	fmt.Fprintf(a.Stdout, "Username: %s\n", acc.Username)
	fmt.Fprintf(a.Stdout, "Email: %s\n", acc.Email)
	fmt.Fprintf(a.Stdout, "Plan: %s\n", acc.Plan)
	fmt.Fprintf(a.Stdout, "Images: %d\n", acc.Stats.Images.Count)
	fmt.Fprintf(a.Stdout, "Storage: %s\n", ui.Bytes(acc.Stats.Images.StorageUsedBytes))
	return nil
}
