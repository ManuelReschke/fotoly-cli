package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/ManuelReschke/fotoly-cli/internal/brand"
)

func TestFilePath(t *testing.T) {
	got := FilePath(brand.Fotoly, "/tmp/cfg")
	want := filepath.Join("/tmp/cfg", "fotoly", "config.toml")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if FilePath(brand.Fotoly, "/tmp/cfg") == FilePath(brand.PixelFox, "/tmp/cfg") {
		t.Fatal("brands must not share config path")
	}
}

func TestLoadMissingFileUsesBrandDefault(t *testing.T) {
	dir := t.TempDir()
	vals, err := Load(brand.Fotoly, Source{UserConfigDir: dir, LookupEnv: func(string) string { return "" }})
	if err != nil {
		t.Fatal(err)
	}
	if vals.APIKey != "" {
		t.Fatalf("APIKey=%q", vals.APIKey)
	}
	if vals.BaseURL != brand.Fotoly.DefaultURL {
		t.Fatalf("BaseURL=%q", vals.BaseURL)
	}
	if vals.Path != FilePath(brand.Fotoly, dir) {
		t.Fatalf("Path=%q", vals.Path)
	}
}

func TestSaveRoundTripAndMode(t *testing.T) {
	dir := t.TempDir()
	path := FilePath(brand.Fotoly, dir)
	if err := Save(path, File{BaseURL: "https://fotoly.eu", APIKey: "pxl_secret"}); err != nil {
		t.Fatal(err)
	}
	vals, err := Load(brand.Fotoly, Source{UserConfigDir: dir, LookupEnv: func(string) string { return "" }})
	if err != nil {
		t.Fatal(err)
	}
	if vals.APIKey != "pxl_secret" || vals.BaseURL != "https://fotoly.eu" {
		t.Fatalf("vals=%+v", vals)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("perm=%o", info.Mode().Perm())
		}
		dirInfo, err := os.Stat(filepath.Dir(path))
		if err != nil {
			t.Fatal(err)
		}
		if dirInfo.Mode().Perm() != 0o700 {
			t.Fatalf("dir perm=%o", dirInfo.Mode().Perm())
		}
	}
}

func TestEnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	path := FilePath(brand.Fotoly, dir)
	if err := Save(path, File{BaseURL: "https://fotoly.eu", APIKey: "pxl_file"}); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{
		"FOTOLY_API_KEY":  "pxl_env",
		"FOTOLY_BASE_URL": "http://localhost:8080",
	}
	vals, err := Load(brand.Fotoly, Source{
		UserConfigDir: dir,
		LookupEnv:     func(k string) string { return env[k] },
	})
	if err != nil {
		t.Fatal(err)
	}
	if vals.APIKey != "pxl_env" {
		t.Fatalf("APIKey=%q", vals.APIKey)
	}
	if vals.BaseURL != "http://localhost:8080" {
		t.Fatalf("BaseURL=%q", vals.BaseURL)
	}
}

func TestFlagOverridesEnv(t *testing.T) {
	env := map[string]string{"FOTOLY_API_KEY": "pxl_env"}
	vals, err := Load(brand.Fotoly, Source{
		UserConfigDir: t.TempDir(),
		LookupEnv:     func(k string) string { return env[k] },
		FlagAPIKey:    "pxl_flag",
	})
	if err != nil {
		t.Fatal(err)
	}
	if vals.APIKey != "pxl_flag" {
		t.Fatalf("APIKey=%q", vals.APIKey)
	}
}

func TestFotolyEnvDoesNotLeakIntoPixelFox(t *testing.T) {
	env := map[string]string{"FOTOLY_API_KEY": "pxl_fotoly"}
	vals, err := Load(brand.PixelFox, Source{
		UserConfigDir: t.TempDir(),
		LookupEnv:     func(k string) string { return env[k] },
	})
	if err != nil {
		t.Fatal(err)
	}
	if vals.APIKey != "" {
		t.Fatalf("PixelFox picked up Fotoly key: %q", vals.APIKey)
	}
	if vals.BaseURL != brand.PixelFox.DefaultURL {
		t.Fatalf("BaseURL=%q", vals.BaseURL)
	}
}

func TestFlagConfigPath(t *testing.T) {
	dir := t.TempDir()
	custom := filepath.Join(dir, "custom.toml")
	if err := Save(custom, File{BaseURL: "https://fotoly.eu", APIKey: "pxl_custom"}); err != nil {
		t.Fatal(err)
	}
	vals, err := Load(brand.Fotoly, Source{
		UserConfigDir: dir,
		LookupEnv:     func(string) string { return "" },
		FlagConfig:    custom,
	})
	if err != nil {
		t.Fatal(err)
	}
	if vals.APIKey != "pxl_custom" || vals.Path != custom {
		t.Fatalf("vals=%+v", vals)
	}
}

func TestEnvConfigPath(t *testing.T) {
	dir := t.TempDir()
	custom := filepath.Join(dir, "from-env.toml")
	if err := Save(custom, File{BaseURL: "https://fotoly.eu", APIKey: "pxl_envpath"}); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{"FOTOLY_CONFIG": custom}
	vals, err := Load(brand.Fotoly, Source{
		UserConfigDir: dir,
		LookupEnv:     func(k string) string { return env[k] },
	})
	if err != nil {
		t.Fatal(err)
	}
	if vals.APIKey != "pxl_envpath" {
		t.Fatalf("APIKey=%q", vals.APIKey)
	}
}

func TestResetMissingIsSuccess(t *testing.T) {
	if err := Reset(filepath.Join(t.TempDir(), "missing.toml")); err != nil {
		t.Fatal(err)
	}
}

func TestResetDeletesFile(t *testing.T) {
	dir := t.TempDir()
	path := FilePath(brand.Fotoly, dir)
	if err := Save(path, File{APIKey: "pxl_x", BaseURL: "https://fotoly.eu"}); err != nil {
		t.Fatal(err)
	}
	if err := Reset(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("file still exists: %v", err)
	}
}
