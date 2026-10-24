package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/ManuelReschke/fotoly-cli/internal/client"
	"github.com/spf13/cobra"
)

// exportResult is the only stdout object when --json is set.
// Files is the number of images in the account document.
// Skipped is how many of those were already on disk.
type exportResult struct {
	Dir     string `json:"dir"`
	Files   int    `json:"files"`
	Skipped int    `json:"skipped"`
	Status  string `json:"status"`
}

func (a *App) exportCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export original images",
		Args:  cobra.NoArgs,
		RunE:  a.exportRun,
	}
	cmd.Flags().String("dir", "./"+a.Brand.Binary+"-export", "output directory")
	return cmd
}

func (a *App) exportRun(cmd *cobra.Command, _ []string) error {
	dir, err := cmd.Flags().GetString("dir")
	if err != nil {
		return err
	}
	if strings.TrimSpace(dir) == "" {
		return fmt.Errorf("%w: --dir is required", ErrUsage)
	}
	c, _, err := a.requireClient(cmd)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	st, err := c.CreateExport(ctx)
	if err != nil {
		return err
	}
	if _, err := c.WaitExport(ctx, st); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	raw, err := c.GetExportAccount(ctx)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "account.json"), raw, 0o644); err != nil {
		return err
	}
	var account client.ExportAccount
	if err := json.Unmarshal(raw, &account); err != nil {
		return err
	}
	total := len(account.Images)
	if total > 0 {
		if err := os.MkdirAll(filepath.Join(dir, "files"), 0o755); err != nil {
			return err
		}
	}
	names := exportFileNames(account.Images)
	skipped := 0
	for i, img := range account.Images {
		name := names[i]
		rel := filepath.Join("files", name)
		dest := filepath.Join(dir, rel)
		match, err := exportFileMatches(dest, img.FileSize, img.SHA256)
		if err != nil {
			return err
		}
		if match {
			skipped++
			if !a.flagJSON {
				fmt.Fprintf(a.Stdout, "skip %s\n", rel)
			}
			continue
		}
		if err := saveExportFile(ctx, c, img, dest, rel); err != nil {
			return err
		}
		if !a.flagJSON {
			fmt.Fprintf(a.Stdout, "saved %s (%d/%d)\n", rel, i+1, total)
		}
	}
	if a.flagJSON {
		return json.NewEncoder(a.Stdout).Encode(exportResult{
			Dir:     dir,
			Files:   total,
			Skipped: skipped,
			Status:  "ready",
		})
	}
	fmt.Fprintf(a.Stdout, "export ready: %s (%d files)\n", dir, total)
	return nil
}

func saveExportFile(ctx context.Context, c *client.Client, img client.ExportImage, dest, rel string) error {
	if strings.TrimSpace(img.DownloadPath) == "" {
		id := strings.TrimSpace(img.UUID)
		if id == "" {
			id = rel
		}
		return fmt.Errorf("export image %s is missing download_path", id)
	}
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := c.DownloadExportFile(ctx, img.DownloadPath, f)
	closeErr := f.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	ok, err := exportFileMatches(dest, img.FileSize, img.SHA256)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	want := strings.ToLower(strings.TrimSpace(img.SHA256))
	if want == "" {
		return fmt.Errorf("%s: size does not match file_size %d", rel, img.FileSize)
	}
	return fmt.Errorf("%s: does not match file_size %d or sha256 %s", rel, img.FileSize, want)
}

func exportFileMatches(path string, size int64, sum string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if !info.Mode().IsRegular() || info.Size() != size {
		return false, nil
	}
	if strings.TrimSpace(sum) == "" {
		return true, nil
	}
	got, err := fileSHA256Hex(path)
	if err != nil {
		return false, err
	}
	return strings.EqualFold(got, strings.TrimSpace(sum)), nil
}

func fileSHA256Hex(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// exportFileNames builds a stable file name for each image.
// A base name used more than once is suffixed for every image that uses it,
// so a later run does not depend on which duplicate appeared first.
func exportFileNames(images []client.ExportImage) []string {
	bases := make([]string, len(images))
	counts := make(map[string]int, len(images))
	for i, img := range images {
		bases[i] = exportBaseName(img.FileName, img.UUID)
		counts[bases[i]]++
	}
	used := make(map[string]struct{}, len(images))
	names := make([]string, len(images))
	for i, img := range images {
		name := bases[i]
		if counts[name] > 1 {
			name = exportWithUUID(bases[i], img.UUID)
		}
		if _, taken := used[name]; taken {
			name = exportDisambiguate(bases[i], img.UUID, used)
		}
		used[name] = struct{}{}
		names[i] = name
	}
	return names
}

func exportBaseName(fileName, uuid string) string {
	name := strings.ReplaceAll(fileName, `\`, "/")
	name = path.Base(name)
	if strings.TrimSpace(name) == "" || name == "." || name == ".." {
		uuid = strings.TrimSpace(uuid)
		if uuid == "" {
			return "file"
		}
		return uuid
	}
	return name
}

func exportDisambiguate(name, uuid string, used map[string]struct{}) string {
	candidate := exportWithUUID(name, uuid)
	if _, taken := used[candidate]; !taken {
		return candidate
	}
	stem, ext := splitExportExt(candidate)
	for n := 2; ; n++ {
		next := fmt.Sprintf("%s-%d%s", stem, n, ext)
		if _, taken := used[next]; !taken {
			return next
		}
	}
}

func exportWithUUID(name, uuid string) string {
	stem, ext := splitExportExt(name)
	return stem + "__" + exportUUIDPrefix(uuid) + ext
}

func exportUUIDPrefix(uuid string) string {
	uuid = strings.TrimSpace(uuid)
	if len(uuid) >= 8 {
		return uuid[:8]
	}
	if uuid == "" {
		return "unknown"
	}
	return uuid
}

func splitExportExt(name string) (string, string) {
	ext := path.Ext(name)
	if ext == "" || ext == name {
		return name, ""
	}
	return strings.TrimSuffix(name, ext), ext
}
