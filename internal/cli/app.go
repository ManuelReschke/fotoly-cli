package cli

import (
	"errors"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/ManuelReschke/fotoly-cli/internal/brand"
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
