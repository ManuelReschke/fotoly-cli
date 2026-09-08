package cli

import (
	"fmt"
	"strings"

	"github.com/ManuelReschke/fotoly-cli/internal/brand"
	"github.com/charmbracelet/huh"
)

type Prompter interface {
	PromptSetup(br brand.Brand, defaultURL string) (apiKey, baseURL string, err error)
	ConfirmDelete(n int) (bool, error)
}

type HuhPrompter struct{}

func (HuhPrompter) PromptSetup(br brand.Brand, defaultURL string) (string, string, error) {
	key, base := "", defaultURL
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("API key").EchoMode(huh.EchoModePassword).Value(&key),
			huh.NewInput().Title("Base URL").Value(&base),
		),
	)
	if err := form.Run(); err != nil {
		return "", "", err
	}
	return strings.TrimSpace(key), strings.TrimSpace(base), nil
}

func (HuhPrompter) ConfirmDelete(int) (bool, error) {
	return false, fmt.Errorf("not implemented")
}
