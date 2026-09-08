package brand

import "testing"

func TestBrandsAreIsolated(t *testing.T) {
	if Fotoly.DefaultURL == PixelFox.DefaultURL {
		t.Fatal("brands must not share DefaultURL")
	}
	if Fotoly.ConfigApp == PixelFox.ConfigApp {
		t.Fatal("brands must not share ConfigApp")
	}
	if Fotoly.EnvPrefix == PixelFox.EnvPrefix {
		t.Fatal("brands must not share EnvPrefix")
	}
}

func TestFotolyDefaults(t *testing.T) {
	if Fotoly.Name != "Fotoly" || Fotoly.Binary != "fotoly" {
		t.Fatalf("fotoly identity: %+v", Fotoly)
	}
	if Fotoly.DefaultURL != "https://fotoly.eu" {
		t.Fatalf("DefaultURL=%q", Fotoly.DefaultURL)
	}
	if Fotoly.ConfigApp != "fotoly" || Fotoly.EnvPrefix != "FOTOLY" {
		t.Fatalf("config/env %+v", Fotoly)
	}
	if Fotoly.Accent != "#E85D04" {
		t.Fatalf("Accent=%q", Fotoly.Accent)
	}
}

func TestPixelFoxDefaults(t *testing.T) {
	if PixelFox.Name != "PixelFox" || PixelFox.Binary != "pixelfox" {
		t.Fatalf("pixelfox identity: %+v", PixelFox)
	}
	if PixelFox.DefaultURL != "https://pixelfox.cc" {
		t.Fatalf("DefaultURL=%q", PixelFox.DefaultURL)
	}
	if PixelFox.ConfigApp != "pixelfox" || PixelFox.EnvPrefix != "PIXELFOX" {
		t.Fatalf("config/env %+v", PixelFox)
	}
	if PixelFox.Accent != "#F97316" {
		t.Fatalf("Accent=%q", PixelFox.Accent)
	}
}

func TestUserAgentString(t *testing.T) {
	if got := Fotoly.UserAgentString("1.0.0"); got != "fotoly-cli/1.0.0" {
		t.Fatalf("got %q", got)
	}
	if got := PixelFox.UserAgentString(""); got != "pixelfox-cli/dev" {
		t.Fatalf("got %q", got)
	}
}

func TestAPIRoot(t *testing.T) {
	if got := Fotoly.APIRoot("https://fotoly.eu"); got != "https://fotoly.eu/api/v1" {
		t.Fatalf("got %q", got)
	}
	if got := Fotoly.APIRoot("https://fotoly.eu/"); got != "https://fotoly.eu/api/v1" {
		t.Fatalf("got %q", got)
	}
}
