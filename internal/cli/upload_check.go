package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func (a *App) uploadCheckCommand() *cobra.Command {
	var targetURL, origin string
	cmd := &cobra.Command{
		Use:   "upload-check",
		Short: "Check upload reachability or CORS preflight",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := a.configuredClient()
			if err != nil {
				return err
			}
			out, err := c.CheckUpload(cmd.Context(), targetURL, origin)
			if err != nil {
				return err
			}
			display := fmt.Sprintf("HTTP %d", out.StatusCode)
			if origin != "" {
				display += fmt.Sprintf("\nAllow origin: %s\nAllow methods: %s\nAllow headers: %s",
					out.AllowOrigin, out.AllowMethods, out.AllowHeaders)
			}
			return a.writeAPIResult(out, display)
		},
	}
	cmd.Flags().StringVar(&targetURL, "url", "", "storage upload URL (defaults to the configured API's /upload)")
	cmd.Flags().StringVar(&origin, "origin", "", "send an OPTIONS preflight for this origin instead of HEAD")
	return cmd
}
