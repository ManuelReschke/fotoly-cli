package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ManuelReschke/fotoly-cli/internal/client"
	"github.com/spf13/cobra"
)

const (
	statusPollAttempts = 45
	processingSpinner  = `|/-\`
	processingTick     = 100 * time.Millisecond
)

type uploadResult struct {
	File      string `json:"file"`
	OK        bool   `json:"ok"`
	ImageUUID string `json:"image_uuid,omitempty"`
	URL       string `json:"url,omitempty"`
	Duplicate bool   `json:"duplicate,omitempty"`
	Error     string `json:"error,omitempty"`
}

func (a *App) uploadCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "upload <files...>",
		Aliases: []string{"up"},
		Short:   "Upload images",
		RunE:    a.uploadRun,
	}
	cmd.Flags().Int64("album", 0, "album id")
	cmd.Flags().Bool("nsfw", false, "mark as NSFW")
	cmd.Flags().String("processing", "default", "processing profile (default or original_only)")
	cmd.Flags().Bool("no-wait", false, "skip processing poll")
	cmd.Flags().Bool("copy", false, "copy share URL to clipboard")
	cmd.Flags().Bool("no-copy", false, "do not copy to clipboard")
	return cmd
}

func (a *App) uploadRun(cmd *cobra.Command, args []string) error {
	processing, err := cmd.Flags().GetString("processing")
	if err != nil {
		return err
	}
	if processing != "default" && processing != "original_only" {
		return fmt.Errorf("%w: --processing must be default or original_only", ErrUsage)
	}
	copyFlag, err := cmd.Flags().GetBool("copy")
	if err != nil {
		return err
	}
	noCopyFlag, err := cmd.Flags().GetBool("no-copy")
	if err != nil {
		return err
	}
	if copyFlag && noCopyFlag {
		return fmt.Errorf("%w: --copy and --no-copy are mutually exclusive", ErrUsage)
	}
	if len(args) == 0 {
		return fmt.Errorf("%w: upload requires at least one file", ErrUsage)
	}

	albumID, err := cmd.Flags().GetInt64("album")
	if err != nil {
		return err
	}
	nsfw, err := cmd.Flags().GetBool("nsfw")
	if err != nil {
		return err
	}
	noWait, err := cmd.Flags().GetBool("no-wait")
	if err != nil {
		return err
	}

	c, vals, err := a.requireClient(cmd)
	if err != nil {
		return err
	}

	results := make([]uploadResult, 0, len(args))
	successCount := 0
	lastURL := ""
	for _, path := range args {
		res := a.uploadOne(cmd, c, vals.BaseURL, path, albumID, nsfw, processing, noWait)
		results = append(results, res)
		if res.OK {
			successCount++
			lastURL = res.URL
			if !a.flagJSON {
				fmt.Fprintln(a.Stdout, res.URL)
			}
		} else if !a.flagJSON {
			fmt.Fprintf(a.Stderr, "%s: %s\n", res.File, res.Error)
		}
	}
	failed := len(args) - successCount

	if a.flagJSON {
		if err := json.NewEncoder(a.Stdout).Encode(results); err != nil {
			return err
		}
	}

	copyEnabled := a.isTTY() && successCount == 1
	if copyFlag {
		copyEnabled = true
	}
	if noCopyFlag {
		copyEnabled = false
	}
	if copyEnabled && lastURL != "" && a.Clipboard != nil {
		if err := a.Clipboard(lastURL); err != nil {
			fmt.Fprintf(a.Stderr, "warning: could not copy to clipboard: %s\n", err)
		}
	}

	if failed > 0 {
		return fmt.Errorf("uploaded %d, failed %d", successCount, failed)
	}
	if len(args) > 1 {
		fmt.Fprintf(a.Stderr, "uploaded %d, failed %d\n", successCount, failed)
	}
	return nil
}

func (a *App) uploadOne(cmd *cobra.Command, c *client.Client, baseURL, path string, albumID int64, nsfw bool, processing string, noWait bool) uploadResult {
	res := uploadResult{File: path}
	info, err := os.Stat(path)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	if info.IsDir() {
		res.Error = "is a directory"
		return res
	}
	if info.Size() == 0 {
		res.Error = "file is empty"
		return res
	}

	req := client.UploadSessionRequest{
		FileSize: info.Size(),
		Processing: &struct {
			Profile string `json:"profile"`
		}{Profile: processing},
	}
	if albumID != 0 {
		id := albumID
		req.AlbumID = &id
	}
	if nsfw {
		req.IsNSFW = boolPtr(true)
	}

	sess, err := c.CreateUploadSession(cmd.Context(), req)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	if sess.MaxBytes > 0 && info.Size() > sess.MaxBytes {
		res.Error = fmt.Sprintf("file exceeds max_bytes (%d > %d)", info.Size(), sess.MaxBytes)
		return res
	}
	if albumID != 0 && sess.AlbumID == nil {
		fmt.Fprintf(a.Stderr, "album %d was not bound; uploading without album\n", albumID)
	}

	f, err := os.Open(path)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	defer f.Close()

	up, err := c.UploadFile(cmd.Context(), sess.UploadURL, sess.Token, filepath.Base(path), f, info.Size(), a.uploadProgress(filepath.Base(path)))
	if err != nil {
		res.Error = err.Error()
		return res
	}
	if up.ImageUUID == nil || *up.ImageUUID == "" {
		res.Error = "upload response missing image_uuid"
		return res
	}
	res.ImageUUID = *up.ImageUUID
	if up.Duplicate != nil {
		res.Duplicate = *up.Duplicate
	}

	if !noWait {
		if err := a.waitForImage(cmd, c, res.ImageUUID, filepath.Base(path)); err != nil {
			res.Error = err.Error()
			return res
		}
	}

	var viewFromImage, urlFromImage string
	img, err := c.GetImage(cmd.Context(), res.ImageUUID)
	if err != nil {
		if !noWait {
			res.Error = err.Error()
			return res
		}
	} else {
		viewFromImage = img.ViewURL
		urlFromImage = img.URL
	}
	viewFromUpload := ""
	if up.ViewURL != nil {
		viewFromUpload = *up.ViewURL
	}
	share, err := client.ResolveShareURL(baseURL, viewFromImage, viewFromUpload, urlFromImage)
	if err != nil {
		if noWait {
			res.OK = true
			return res
		}
		res.Error = err.Error()
		return res
	}
	res.URL = share
	res.OK = true
	return res
}

func (a *App) waitForImage(cmd *cobra.Command, c *client.Client, uuid, name string) error {
	wait := &processingWait{w: a.Stderr, tty: a.isTTY(), name: name}
	ticks := int(time.Second / processingTick)
	for i := 0; i < statusPollAttempts; i++ {
		if i > 0 {
			for t := 0; t < ticks; t++ {
				if a.Sleep != nil {
					a.Sleep(processingTick)
				}
				wait.tick()
			}
		}
		st, err := c.GetImageStatus(cmd.Context(), uuid)
		if err != nil {
			wait.fail()
			return err
		}
		if st.Failed {
			wait.fail()
			return fmt.Errorf("Processing failed for %s", uuid)
		}
		if st.Complete {
			wait.succeed()
			return nil
		}
		if !wait.started {
			wait.begin()
		}
	}
	wait.fail()
	return fmt.Errorf("Processing timed out for %s; check later with images status", uuid)
}

type processingWait struct {
	w       io.Writer
	tty     bool
	name    string
	start   time.Time
	frame   int
	width   int
	started bool
}

func (p *processingWait) begin() {
	p.started = true
	p.start = time.Now()
	if p.tty {
		p.draw(false)
		return
	}
	fmt.Fprintf(p.w, "processing %s …\n", p.name)
}

func (p *processingWait) tick() {
	if p.started && p.tty {
		p.draw(false)
	}
}

func (p *processingWait) succeed() {
	if !p.started {
		return
	}
	if p.tty {
		p.draw(true)
		fmt.Fprintln(p.w)
	}
}

func (p *processingWait) fail() {
	if p.started && p.tty {
		fmt.Fprintln(p.w)
	}
}

func (p *processingWait) draw(done bool) {
	var s string
	if done {
		s = fmt.Sprintf("processing %s done", p.name)
	} else {
		s = fmt.Sprintf("processing %s %c %ds", p.name, processingSpinner[p.frame%len(processingSpinner)], int(time.Since(p.start).Seconds()))
		p.frame++
	}
	if len(s) < p.width {
		s += strings.Repeat(" ", p.width-len(s))
	} else {
		p.width = len(s)
	}
	fmt.Fprintf(p.w, "\r%s", s)
}

func (a *App) uploadProgress(name string) func(sent, total int64) {
	tty := a.isTTY()
	lastBucket := 0
	return func(sent, total int64) {
		if total <= 0 {
			return
		}
		pct := float64(sent) / float64(total) * 100
		if tty {
			fmt.Fprintf(a.Stderr, "\r%s %.1f%%", name, pct)
			if sent >= total {
				fmt.Fprintln(a.Stderr)
			}
			return
		}
		bucket := int(pct) / 25
		if bucket > lastBucket || sent >= total {
			lastBucket = bucket
			fmt.Fprintf(a.Stderr, "%s %.1f%%\n", name, pct)
		}
	}
}

func (a *App) isTTY() bool {
	return a.IsTTY != nil && a.IsTTY()
}
