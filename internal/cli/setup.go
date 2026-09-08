package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ManuelReschke/fotoly-cli/internal/client"
	"github.com/ManuelReschke/fotoly-cli/internal/config"
	"github.com/spf13/cobra"
)

func (a *App) setupCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Configure API key",
		RunE:  a.setupRun,
	}
	cmd.Flags().Bool("reset", false, "remove stored config")
	return cmd
}

func (a *App) setupRun(cmd *cobra.Command, _ []string) error {
	dir, err := a.UserConfigDir()
	if err != nil {
		return err
	}

	reset, err := cmd.Flags().GetBool("reset")
	if err != nil {
		return err
	}
	if reset {
		if err := config.Reset(a.configPath(dir)); err != nil {
			return err
		}
		fmt.Fprintln(a.Stdout, "Config removed.")
		return nil
	}

	vals, err := config.Load(a.Brand, config.Source{
		UserConfigDir: dir,
		LookupEnv:     a.LookupEnv,
		FlagConfig:    a.flagConfig,
		FlagAPIKey:    a.flagAPIKey,
	})
	if err != nil {
		return err
	}

	key := strings.TrimSpace(a.flagAPIKey)
	if key == "" && a.LookupEnv != nil {
		key = strings.TrimSpace(a.LookupEnv(a.Brand.EnvPrefix + "_API_KEY"))
	}

	base := vals.BaseURL
	if key == "" && a.IsTTY != nil && a.IsTTY() && a.Prompter != nil {
		var url string
		key, url, err = a.Prompter.PromptSetup(a.Brand, base)
		if err != nil {
			return err
		}
		key = strings.TrimSpace(key)
		if u := strings.TrimSpace(url); u != "" {
			base = u
		}
	}
	if key == "" {
		return errors.New("No API key. Use --api-key or run setup from a terminal.")
	}
	return a.runSetup(cmd.Context(), key, base)
}

func (a *App) runSetup(ctx context.Context, key, base string) error {
	key = strings.TrimSpace(key)
	if !strings.HasPrefix(key, "pxl_") {
		fmt.Fprintln(a.Stderr, "Warning: API keys usually start with pxl_")
	}

	c := client.New(base, key, a.Brand.UserAgentString(a.Version), a.HTTPClient)
	if a.Sleep != nil {
		c.Sleep = a.Sleep
	}
	acc, err := c.GetProfile(ctx)
	if err != nil {
		if client.IsUnauthorized(err) {
			return fmt.Errorf("Invalid API key. Run '%s setup'.", a.Brand.Binary)
		}
		return err
	}
	dir, err := a.UserConfigDir()
	if err != nil {
		return err
	}
	if err := config.Save(a.configPath(dir), config.File{BaseURL: base, APIKey: key}); err != nil {
		return err
	}
	out := a.Stdout
	if a.flagJSON {
		out = a.Stderr
	}
	fmt.Fprintf(out, "Logged in as %s (%s) on %s\n", acc.Username, acc.Plan, base)
	return nil
}

func (a *App) configPath(userConfigDir string) string {
	path := strings.TrimSpace(a.flagConfig)
	if path == "" && a.LookupEnv != nil {
		path = strings.TrimSpace(a.LookupEnv(a.Brand.EnvPrefix + "_CONFIG"))
	}
	if path == "" {
		path = config.FilePath(a.Brand, userConfigDir)
	}
	return path
}
