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
	File              string                  `json:"file"`
	OK                bool                    `json:"ok"`
	ImageUUID         string                  `json:"image_uuid,omitempty"`
	URL               string                  `json:"url,omitempty"`
	Duplicate         bool                    `json:"duplicate,omitempty"`
	IsNSFW            *bool                   `json:"is_nsfw,omitempty"`
	IsAIGenerated     *bool                   `json:"is_ai_generated,omitempty"`
	Tags              []string                `json:"tags,omitempty"`
	AvailableVariants []string                `json:"available_variants,omitempty"`
	Variants          *client.ImageVariants   `json:"variants,omitempty"`
	Processing        *client.ImageProcessing `json:"processing,omitempty"`
	Error             string                  `json:"error,omitempty"`
}

func (a *App) uploadCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "upload <files...>",
		Aliases: []string{"up"},
		Short:   "Upload images",
		RunE:    a.uploadRun,
	}
	cmd.Flags().Int64("album", 0, "album id")
	cmd.Flags().Bool("nsfw", false, "set the NSFW flag (true or false)")
	cmd.Flags().Bool("ai-generated", false, "set the AI-generated flag (true or false)")
	cmd.Flags().StringArray("tag", nil, "tag the upload; repeat for each tag")
	cmd.Flags().String("processing", "default", "processing profile (default, original_only, or custom)")
	cmd.Flags().StringArray("derivative", nil, "custom derivative as family/size; repeat for each variant")
	cmd.Flags().Bool("upload-token-form", false, "send the storage token as a multipart field")
	cmd.Flags().Bool("no-wait", false, "skip processing poll")
	cmd.Flags().Bool("copy", false, "copy share URL to clipboard")
	cmd.Flags().Bool("no-copy", false, "do not copy to clipboard")
	return cmd
}

func (a *App) uploadRun(cmd *cobra.Command, args []string) error {
	processing, err := uploadProcessing(cmd)
	if err != nil {
		return err
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
	nsfw, err := changedBool(cmd, "nsfw")
	if err != nil {
		return err
	}
	isAI, err := changedBool(cmd, "ai-generated")
	if err != nil {
		return err
	}
	tags, err := cmd.Flags().GetStringArray("tag")
	if err != nil {
		return err
	}
	if len(tags) > 20 {
		return fmt.Errorf("%w: at most 20 upload tags are allowed", ErrUsage)
	}
	tokenInForm, err := cmd.Flags().GetBool("upload-token-form")
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
	request := client.UploadSessionRequest{Processing: processing, IsNSFW: nsfw, IsAIGenerated: isAI, Tags: tags}
	if albumID != 0 {
		request.AlbumID = &albumID
	}

	results := make([]uploadResult, 0, len(args))
	successCount := 0
	lastURL := ""
	for _, path := range args {
		res := a.uploadOne(cmd, c, vals.BaseURL, path, request, noWait, tokenInForm)
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

func uploadProcessing(cmd *cobra.Command) (*client.UploadProcessingRequest, error) {
	profile, err := cmd.Flags().GetString("processing")
	if err != nil {
		return nil, err
	}
	if profile != "default" && profile != "original_only" && profile != "custom" {
		return nil, fmt.Errorf("%w: --processing must be default, original_only, or custom", ErrUsage)
	}
	values, err := cmd.Flags().GetStringArray("derivative")
	if err != nil {
		return nil, err
	}
	if profile == "custom" && len(values) == 0 {
		return nil, fmt.Errorf("%w: --processing custom requires at least one --derivative", ErrUsage)
	}
	if profile != "custom" && len(values) != 0 {
		return nil, fmt.Errorf("%w: --derivative requires --processing custom", ErrUsage)
	}
	out := &client.UploadProcessingRequest{Profile: profile}
	for _, value := range values {
		family, size, _ := strings.Cut(value, "/")
		if family != "original" && family != "webp" && family != "avif" ||
			size != "original" && size != "medium" && size != "small" {
			return nil, fmt.Errorf("%w: invalid --derivative %q, expected original|webp|avif/original|medium|small", ErrUsage, value)
		}
		if family == "original" && size == "original" {
			return nil, fmt.Errorf("%w: original/original is already included", ErrUsage)
		}
		out.Derivatives = append(out.Derivatives, client.ProcessingDerivative{Family: family, Size: size})
	}
	return out, nil
}

func (a *App) uploadOne(cmd *cobra.Command, c *client.Client, baseURL, path string, req client.UploadSessionRequest, noWait, tokenInForm bool) uploadResult {
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

	req.FileSize = info.Size()
	sess, err := c.CreateUploadSession(cmd.Context(), req)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	if sess.MaxBytes > 0 && info.Size() > sess.MaxBytes {
		res.Error = fmt.Sprintf("file exceeds max_bytes (%d > %d)", info.Size(), sess.MaxBytes)
		return res
	}
	if req.AlbumID != nil && sess.AlbumID == nil {
		fmt.Fprintf(a.Stderr, "album %d was not bound; uploading without album\n", *req.AlbumID)
	}

	f, err := os.Open(path)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	defer f.Close()

	up, err := c.UploadFile(cmd.Context(), sess.UploadURL, sess.Token, filepath.Base(path), f, info.Size(), a.uploadProgress(filepath.Base(path)), tokenInForm)
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
	res.IsNSFW = up.IsNSFW
	res.IsAIGenerated = up.IsAIGenerated
	res.Tags = up.Tags
	res.AvailableVariants = up.AvailableVariants
	res.Variants = up.Variants
	res.Processing = up.Processing

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
