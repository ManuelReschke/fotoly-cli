package ui

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/charmbracelet/lipgloss"
)

func Enabled(noColorEnv string, stdoutIsTTY bool) bool {
	return noColorEnv == "" && stdoutIsTTY
}

func Header(accent, text string, colorOn bool) string {
	if !colorOn {
		return text
	}
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(accent)).Render(text)
}

func Table(headers []string, rows [][]string) string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, strings.Join(headers, "\t"))
	for _, row := range rows {
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}
	_ = w.Flush()
	return b.String()
}
