// Package dateinput turns typed dates into YYYY-MM-DD. Exact dates parse
// here; anything else, such as "+1 day" or "next fri", goes to GNU date.
package dateinput

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Resolve returns text as a local YYYY-MM-DD date. Text that is not already
// one is passed to `date -d` (`gdate` when installed, since macOS date has no
// -d), which does the validation.
func Resolve(ctx context.Context, text string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", errors.New("enter a date")
	}
	if _, err := time.Parse(time.DateOnly, text); err == nil {
		return text, nil
	}
	bin := "date"
	if _, err := exec.LookPath("gdate"); err == nil {
		bin = "gdate"
	}
	cmd := exec.CommandContext(ctx, bin, "-d", text, "+%F")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if bin == "date" && runtime.GOOS == "darwin" {
			return "", fmt.Errorf("date %q: use YYYY-MM-DD, or run `brew install coreutils` for relative dates", text)
		}
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			err = errors.New(msg)
		}
		return "", fmt.Errorf("date %q: %w", text, err)
	}
	date := strings.TrimSpace(string(out))
	if _, err := time.Parse(time.DateOnly, date); err != nil {
		return "", fmt.Errorf("date %q: unexpected output %q", text, date)
	}
	return date, nil
}
