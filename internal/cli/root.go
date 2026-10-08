package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/ManuelReschke/fotoly-cli/internal/brand"
	"github.com/ManuelReschke/fotoly-cli/internal/client"
	"github.com/spf13/cobra"
)

var ErrUsage = errors.New("usage")

func Execute(b brand.Brand) int {
	return Run(New(b))
}

func Run(a *App) int {
	cmd := a.Root()
	cmd.SetOut(a.Stdout)
	cmd.SetErr(a.Stderr)
	cmd.SilenceErrors = true
	if err := cmd.Execute(); err != nil {
		if isUsageErr(err) {
			fmt.Fprintln(a.Stderr, err.Error())
			return 2
		}
		writeCommandError(a.Stderr, err, a.flagJSON)
		return 1
	}
	return 0
}

func writeCommandError(w io.Writer, err error, asJSON bool) {
	if !asJSON {
		fmt.Fprintln(w, err.Error())
		return
	}
	payload := struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}{
		Error:   "error",
		Message: err.Error(),
	}
	var apiErr *client.APIError
	if errors.As(err, &apiErr) {
		if apiErr.Code != "" {
			payload.Error = apiErr.Code
		} else {
			payload.Error = fmt.Sprintf("http %d", apiErr.Status)
		}
	}
	_ = json.NewEncoder(w).Encode(payload)
}

func isUsageErr(err error) bool {
	if errors.Is(err, ErrUsage) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unknown command") ||
		strings.Contains(msg, "unknown flag") ||
		strings.Contains(msg, "required flag") ||
		strings.Contains(msg, "accepts") ||
		strings.Contains(msg, "arg") && strings.Contains(msg, "received")
}

func (a *App) buildRoot() *cobra.Command {
	cmd := &cobra.Command{
		Use:           a.Brand.Binary,
		Short:         fmt.Sprintf("Official CLI for %s (%s)", a.Brand.Name, a.Brand.DefaultURL),
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			a.flagJSON, _ = cmd.Flags().GetBool("json")
			a.flagConfig, _ = cmd.Flags().GetString("config")
			a.flagAPIKey, _ = cmd.Flags().GetString("api-key")
		},
	}
	cmd.SetOut(a.Stdout)
	cmd.SetErr(a.Stderr)
	cmd.PersistentFlags().Bool("json", false, "output JSON")
	cmd.PersistentFlags().String("config", "", "config file path")
	cmd.PersistentFlags().String("api-key", "", "API key (overrides config)")
	cmd.AddCommand(a.versionCommand())
	cmd.AddCommand(a.setupCommand())
	cmd.AddCommand(a.whoamiCommand())
	cmd.AddCommand(a.uploadCommand())
	cmd.AddCommand(a.imagesCommand())
	cmd.AddCommand(a.albumsCommand())
	cmd.AddCommand(a.notificationsCommand())
	return cmd
}

func (a *App) versionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(a.Stdout, "%s %s (%s)\n", a.Brand.Binary, a.Version, a.Commit)
		},
	}
}
