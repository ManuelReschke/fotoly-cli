package brand

import "strings"

type Brand struct {
	Name       string
	Binary     string
	DefaultURL string
	ConfigApp  string
	EnvPrefix  string
	UAPrefix   string
	Accent     string
}

func (b Brand) APIRoot(baseURL string) string {
	return strings.TrimRight(strings.TrimSpace(baseURL), "/") + "/api/v1"
}

func (b Brand) UserAgentString(version string) string {
	if strings.TrimSpace(version) == "" {
		version = "dev"
	}
	return b.UAPrefix + "/" + version
}

var Fotoly = Brand{
	Name:       "Fotoly",
	Binary:     "fotoly",
	DefaultURL: "https://fotoly.eu",
	ConfigApp:  "fotoly",
	EnvPrefix:  "FOTOLY",
	UAPrefix:   "fotoly-cli",
	Accent:     "#E85D04",
}

var PixelFox = Brand{
	Name:       "PixelFox",
	Binary:     "pixelfox",
	DefaultURL: "https://pixelfox.cc",
	ConfigApp:  "pixelfox",
	EnvPrefix:  "PIXELFOX",
	UAPrefix:   "pixelfox-cli",
	Accent:     "#F97316",
}
