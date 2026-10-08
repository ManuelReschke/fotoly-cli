package cli

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func addAlbumMetaFlags(cmd *cobra.Command) {
	cmd.Flags().String("title", "", "album title")
	cmd.Flags().String("description", "", "album description")
	cmd.Flags().Bool("public", false, "make the album public")
	cmd.Flags().Bool("private", false, "make the album private")
	cmd.Flags().Bool("nsfw", false, "mark the album NSFW")
	cmd.Flags().Bool("sfw", false, "mark the album SFW")
	cmd.Flags().String("password", "", "share password; an empty value clears it on edit")
	cmd.Flags().String("sort", "", "image order inside the album: desc or asc")
}

func albumMetaChanged(cmd *cobra.Command) bool {
	for _, name := range []string{"title", "description", "public", "private", "nsfw", "sfw", "password", "sort"} {
		if cmd.Flags().Changed(name) {
			return true
		}
	}
	return false
}

func changedString(cmd *cobra.Command, name string) (*string, error) {
	if !cmd.Flags().Changed(name) {
		return nil, nil
	}
	value, err := cmd.Flags().GetString(name)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func changedSort(cmd *cobra.Command) (*string, error) {
	if !cmd.Flags().Changed("sort") {
		return nil, nil
	}
	value, err := cmd.Flags().GetString("sort")
	if err != nil {
		return nil, err
	}
	if value != "desc" && value != "asc" {
		return nil, fmt.Errorf("%w: --sort must be desc or asc", ErrUsage)
	}
	return &value, nil
}

func optionalVisibility(cmd *cobra.Command) (isPublic, isNSFW *bool, err error) {
	if err := rejectFalseFlag(cmd, "public", "private"); err != nil {
		return nil, nil, err
	}
	if err := rejectFalseFlag(cmd, "private", "public"); err != nil {
		return nil, nil, err
	}
	if err := rejectFalseFlag(cmd, "nsfw", "sfw"); err != nil {
		return nil, nil, err
	}
	if err := rejectFalseFlag(cmd, "sfw", "nsfw"); err != nil {
		return nil, nil, err
	}
	public, private, err := exclusiveBools(cmd, "public", "private")
	if err != nil {
		return nil, nil, err
	}
	nsfw, sfw, err := exclusiveBools(cmd, "nsfw", "sfw")
	if err != nil {
		return nil, nil, err
	}
	if public {
		isPublic = boolPtr(true)
	} else if private {
		isPublic = boolPtr(false)
	}
	if nsfw {
		isNSFW = boolPtr(true)
	} else if sfw {
		isNSFW = boolPtr(false)
	}
	return isPublic, isNSFW, nil
}

func rejectFalseFlag(cmd *cobra.Command, name, instead string) error {
	if !cmd.Flags().Changed(name) {
		return nil
	}
	value, err := cmd.Flags().GetBool(name)
	if err != nil {
		return err
	}
	if !value {
		return fmt.Errorf("%w: --%s=false is not a change; use --%s", ErrUsage, name, instead)
	}
	return nil
}

func exclusiveBools(cmd *cobra.Command, yesName, noName string) (bool, bool, error) {
	yes, err := cmd.Flags().GetBool(yesName)
	if err != nil {
		return false, false, err
	}
	no, err := cmd.Flags().GetBool(noName)
	if err != nil {
		return false, false, err
	}
	if yes && no {
		return false, false, fmt.Errorf("%w: --%s and --%s are mutually exclusive", ErrUsage, yesName, noName)
	}
	return yes, no, nil
}

func parsePositiveID(raw, name string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || id < 1 {
		return 0, fmt.Errorf("%w: %s must be a positive integer", ErrUsage, name)
	}
	return id, nil
}

func parseIDList(raw string) ([]int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []int64{}, nil
	}
	parts := strings.Split(raw, ",")
	out := make([]int64, 0, len(parts))
	for _, part := range parts {
		id, err := parsePositiveID(part, "category id")
		if err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}

func parseUUIDs(args []string) ([]string, error) {
	out := make([]string, len(args))
	for i, raw := range args {
		id := strings.TrimSpace(raw)
		if id == "" || len(id) > 36 {
			return nil, fmt.Errorf("%w: image uuid must be 1 to 36 characters", ErrUsage)
		}
		out[i] = id
	}
	return out, nil
}

// confirmDestructive returns false when a TTY user aborts. The caller returns nil in that case.
func (a *App) confirmDestructive(cmd *cobra.Command, prompt, nonTTY string) (bool, error) {
	yes, err := cmd.Flags().GetBool("yes")
	if err != nil {
		return false, err
	}
	if yes {
		return true, nil
	}
	if a.IsTTY != nil && a.IsTTY() && a.Prompter != nil {
		ok, err := a.Prompter.Confirm(prompt)
		if err != nil {
			return false, err
		}
		if !ok {
			fmt.Fprintln(a.Stdout, "Aborted.")
			return false, nil
		}
		return true, nil
	}
	return false, errors.New(nonTTY)
}
