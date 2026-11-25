package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/ManuelReschke/fotoly-cli/internal/client"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func (a *App) configuredClient() (*client.Client, error) {
	vals, err := a.loadConfig()
	if err != nil {
		return nil, err
	}
	return a.apiClient(vals), nil
}

func (a *App) writeAPIResult(value any, display string) error {
	if a.flagJSON {
		return json.NewEncoder(a.Stdout).Encode(value)
	}
	_, err := fmt.Fprintln(a.Stdout, display)
	return err
}

func (a *App) pingCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "ping",
		Short: "Check API availability",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := a.configuredClient()
			if err != nil {
				return err
			}
			out, err := c.Ping(cmd.Context())
			if err != nil {
				return err
			}
			return a.writeAPIResult(out, out.Ping)
		},
	}
}

func (a *App) authCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "auth", Short: "Manage app sessions and social login"}
	cmd.AddCommand(
		a.authProvidersCommand(),
		a.authStartCommand(),
		a.authCredentialsCommand(false),
		a.authCredentialsCommand(true),
		a.authTokenCommand(),
		a.authLogoutCommand(),
		a.authWebHandoffCommand(),
	)
	return cmd
}

func (a *App) authProvidersCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "providers",
		Short: "List available social login providers",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := a.configuredClient()
			if err != nil {
				return err
			}
			out, err := c.ListAuthProviders(cmd.Context())
			if err != nil {
				return err
			}
			lines := make([]string, 0, len(out.Providers))
			for _, provider := range out.Providers {
				lines = append(lines, fmt.Sprintf("%s\t%s", provider.ID, provider.Name))
			}
			if len(lines) == 0 {
				lines = append(lines, "No social login providers available.")
			}
			return a.writeAPIResult(out, strings.Join(lines, "\n"))
		},
	}
}

func (a *App) authStartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start <provider>",
		Short: "Print the browser URL to start social login",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			provider := strings.ToLower(args[0])
			switch provider {
			case "google", "facebook", "discord", "patreon":
			default:
				return fmt.Errorf("%w: provider must be google, facebook, discord, or patreon", ErrUsage)
			}
			redirectURI, err := cmd.Flags().GetString("redirect-uri")
			if err != nil {
				return err
			}
			if redirectURI != "fotoly://auth/callback" && redirectURI != "pixelfox://auth/callback" {
				return fmt.Errorf("%w: --redirect-uri must be fotoly://auth/callback or pixelfox://auth/callback", ErrUsage)
			}
			c, err := a.configuredClient()
			if err != nil {
				return err
			}
			out, err := c.StartAuthProvider(cmd.Context(), provider, redirectURI)
			if err != nil {
				return err
			}
			return a.writeAPIResult(out, out.URL)
		},
	}
	cmd.Flags().String("redirect-uri", a.Brand.Binary+"://auth/callback", "registered app callback URI")
	return cmd
}

func (a *App) authCredentialsCommand(register bool) *cobra.Command {
	var email, username string
	var fromStdin bool
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in with email and password and print the app session token",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			email := strings.TrimSpace(email)
			username := strings.TrimSpace(username)
			if email == "" {
				return fmt.Errorf("%w: --email cannot be empty", ErrUsage)
			}
			if register && username == "" {
				return fmt.Errorf("%w: --username cannot be empty", ErrUsage)
			}
			c, err := a.configuredClient()
			if err != nil {
				return err
			}
			password, err := a.readAuthSecret(cmd, "Password", "password-stdin", fromStdin)
			if err != nil {
				return err
			}
			credentials := client.AuthCredentials{Email: email, Password: password}
			if register {
				out, err := c.Register(cmd.Context(), client.AuthRegistration{
					Username: username, AuthCredentials: credentials,
				})
				if err != nil {
					return err
				}
				return a.writeAPIResult(out, out.Message)
			}
			out, err := c.Login(cmd.Context(), credentials)
			if err != nil {
				return err
			}
			return a.writeAPIResult(out, out.Token)
		},
	}
	if register {
		cmd.Use = "register"
		cmd.Short = "Create an account and send the activation email"
		cmd.Flags().StringVar(&username, "username", "", "account username")
		_ = cmd.MarkFlagRequired("username")
	}
	cmd.Flags().StringVar(&email, "email", "", "account email address")
	cmd.Flags().BoolVar(&fromStdin, "password-stdin", false, "read one password line from stdin")
	_ = cmd.MarkFlagRequired("email")
	return cmd
}

func (a *App) authTokenCommand() *cobra.Command {
	var fromStdin bool
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Exchange a social login code and print the app session token",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := a.configuredClient()
			if err != nil {
				return err
			}
			code, err := a.readAuthSecret(cmd, "OAuth code", "code-stdin", fromStdin)
			if err != nil {
				return err
			}
			out, err := c.ExchangeAuthCode(cmd.Context(), strings.TrimSpace(code))
			if err != nil {
				return err
			}
			return a.writeAPIResult(out, out.Token)
		},
	}
	cmd.Flags().BoolVar(&fromStdin, "code-stdin", false, "read one authorization code line from stdin")
	return cmd
}

func (a *App) authLogoutCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Revoke the current app session",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := a.configuredClient()
			if err != nil {
				return err
			}
			if c.SessionToken == "" {
				return fmt.Errorf("%w: set %s_SESSION_TOKEN to the app session to revoke", ErrUsage, a.Brand.EnvPrefix)
			}
			if err := c.Logout(cmd.Context()); err != nil {
				return err
			}
			return a.writeAPIResult(map[string]string{"status": "revoked"}, "Session revoked.")
		},
	}
}

func (a *App) authWebHandoffCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "web-handoff",
		Short: "Print a one-time website login URL",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			next, err := changedString(cmd, "next")
			if err != nil {
				return err
			}
			c, _, err := a.requireClient(cmd)
			if err != nil {
				return err
			}
			out, err := c.CreateWebHandoff(cmd.Context(), next)
			if err != nil {
				return err
			}
			return a.writeAPIResult(out, out.URL)
		},
	}
	cmd.Flags().String("next", "", "website path to open after login (defaults to /pricing)")
	return cmd
}

func (a *App) readAuthSecret(cmd *cobra.Command, label, stdinFlag string, fromStdin bool) (string, error) {
	input := cmd.InOrStdin()
	var secret string
	if fromStdin {
		lines := bufio.NewScanner(input)
		if lines.Scan() {
			secret = lines.Text()
		}
		if err := lines.Err(); err != nil {
			return "", err
		}
	} else {
		f, ok := input.(*os.File)
		if !ok || !term.IsTerminal(int(f.Fd())) {
			return "", fmt.Errorf("%w: use --%s when stdin is not a terminal", ErrUsage, stdinFlag)
		}
		fmt.Fprintf(a.Stderr, "%s: ", label)
		line, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(a.Stderr)
		if err != nil {
			return "", err
		}
		secret = string(line)
	}
	if secret == "" {
		return "", fmt.Errorf("%w: %s cannot be empty", ErrUsage, strings.ToLower(label))
	}
	return secret, nil
}
