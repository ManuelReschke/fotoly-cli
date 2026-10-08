package cli

import (
	"strings"

	"github.com/ManuelReschke/fotoly-cli/internal/brand"
	"github.com/charmbracelet/huh"
)

type Prompter interface {
	PromptSetup(br brand.Brand, defaultURL string) (apiKey, baseURL string, err error)
	Confirm(prompt string) (bool, error)
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

func (HuhPrompter) Confirm(prompt string) (bool, error) {
	ok := false
	form := huh.NewForm(huh.NewGroup(huh.NewConfirm().Title(prompt).Affirmative("Yes").Negative("No").Value(&ok)))
	if err := form.Run(); err != nil {
		return false, err
	}
	return ok, nil
}
