package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ManuelReschke/fotoly-cli/internal/brand"
	"github.com/pelletier/go-toml/v2"
)

type File struct {
	BaseURL string `toml:"base_url"`
	APIKey  string `toml:"api_key"`
}

type Values struct {
	Path    string
	BaseURL string
	APIKey  string
}

type Source struct {
	UserConfigDir string
	LookupEnv     func(key string) string
	FlagConfig    string
	FlagAPIKey    string
}

func FilePath(b brand.Brand, userConfigDir string) string {
	return filepath.Join(userConfigDir, b.ConfigApp, "config.toml")
}

func Save(path string, file File) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := toml.Marshal(file)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

func Load(b brand.Brand, src Source) (Values, error) {
	lookup := src.LookupEnv
	if lookup == nil {
		lookup = func(string) string { return "" }
	}

	path := strings.TrimSpace(src.FlagConfig)
	if path == "" {
		path = strings.TrimSpace(lookup(b.EnvPrefix + "_CONFIG"))
	}
	if path == "" {
		path = FilePath(b, src.UserConfigDir)
	}

	var file File
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return Values{}, err
		}
	} else {
		if err := toml.Unmarshal(data, &file); err != nil {
			return Values{}, err
		}
	}

	apiKey := strings.TrimSpace(src.FlagAPIKey)
	if apiKey == "" {
		apiKey = strings.TrimSpace(lookup(b.EnvPrefix + "_API_KEY"))
	}
	if apiKey == "" {
		apiKey = strings.TrimSpace(file.APIKey)
	}

	baseURL := strings.TrimSpace(lookup(b.EnvPrefix + "_BASE_URL"))
	if baseURL == "" {
		baseURL = strings.TrimSpace(file.BaseURL)
	}
	if baseURL == "" {
		baseURL = b.DefaultURL
	}

	return Values{
		Path:    path,
		BaseURL: baseURL,
		APIKey:  apiKey,
	}, nil
}

func Reset(path string) error {
	err := os.Remove(path)
	if err != nil && os.IsNotExist(err) {
		return nil
	}
	return err
}
