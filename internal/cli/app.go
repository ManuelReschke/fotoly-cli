package cli

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ManuelReschke/fotoly-cli/internal/brand"
	"github.com/ManuelReschke/fotoly-cli/internal/client"
	"github.com/ManuelReschke/fotoly-cli/internal/config"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type App struct {
	Brand         brand.Brand
	Version       string
	Commit        string
	Stdout        io.Writer
	Stderr        io.Writer
	LookupEnv     func(string) string
	UserConfigDir func() (string, error)
	HTTPClient    *http.Client
	IsTTY         func() bool
	Prompter      Prompter
	Clipboard     func(string) error
	Sleep         func(time.Duration)

	root       *cobra.Command
	flagJSON   bool
	flagConfig string
	flagAPIKey string
}

func New(b brand.Brand) *App {
	return &App{
		Brand:   b,
		Version: "dev",
		Commit:  "unknown",
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
		LookupEnv: func(k string) string {
			v, _ := os.LookupEnv(k)
			return v
		},
		UserConfigDir: os.UserConfigDir,
		IsTTY: func() bool {
			return term.IsTerminal(int(os.Stderr.Fd()))
		},
		Prompter: HuhPrompter{},
		Clipboard: func(string) error {
			return errors.New("clipboard unavailable")
		},
		Sleep: time.Sleep,
	}
}

func (a *App) Root() *cobra.Command {
	if a.root == nil {
		a.root = a.buildRoot()
	}
	return a.root
}

func (a *App) requireClient(cmd *cobra.Command) (*client.Client, config.Values, error) {
	dir, err := a.UserConfigDir()
	if err != nil {
		return nil, config.Values{}, err
	}
	src := config.Source{
		UserConfigDir: dir,
		LookupEnv:     a.LookupEnv,
		FlagConfig:    a.flagConfig,
		FlagAPIKey:    a.flagAPIKey,
	}
	vals, err := config.Load(a.Brand, src)
	if err != nil {
		return nil, config.Values{}, err
	}
	if vals.APIKey == "" && a.IsTTY != nil && a.IsTTY() && a.Prompter != nil {
		key, url, err := a.Prompter.PromptSetup(a.Brand, vals.BaseURL)
		if err != nil {
			return nil, config.Values{}, err
		}
		key = strings.TrimSpace(key)
		base := vals.BaseURL
		if u := strings.TrimSpace(url); u != "" {
			base = u
		}
		if key != "" {
			if err := a.runSetup(cmd.Context(), key, base); err != nil {
				return nil, config.Values{}, err
			}
			vals, err = config.Load(a.Brand, src)
			if err != nil {
				return nil, config.Values{}, err
			}
		}
	}
	if vals.APIKey == "" {
		return nil, config.Values{}, fmt.Errorf("No API key. Run '%s setup'.", a.Brand.Binary)
	}
	c := client.New(vals.BaseURL, vals.APIKey, a.Brand.UserAgentString(a.Version), a.HTTPClient)
	return c, vals, nil
}
