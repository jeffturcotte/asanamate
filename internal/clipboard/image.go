// Package clipboard reads images from the OS clipboard.
package clipboard

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// ErrNoImage means the clipboard holds no PNG image.
var ErrNoImage = errors.New("the clipboard has no image")

// SaveImage writes the clipboard's PNG image to a new temporary file and
// returns its path.
func SaveImage() (string, error) {
	data, err := readPNG()
	if err != nil {
		return "", err
	}
	if http.DetectContentType(data) != "image/png" {
		return "", ErrNoImage
	}
	f, err := os.CreateTemp("", "asanamate-paste-*.png")
	if err != nil {
		return "", err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return "", err
	}
	return f.Name(), f.Close()
}

func readPNG() ([]byte, error) {
	switch {
	case runtime.GOOS == "darwin":
		out, err := run("osascript", "-e", "the clipboard as «class PNGf»")
		if err != nil {
			return nil, err
		}
		return decodeAppleData(out)
	case os.Getenv("WAYLAND_DISPLAY") != "":
		return run("wl-paste", "--type", "image/png")
	default:
		return run("xclip", "-selection", "clipboard", "-t", "image/png", "-o")
	}
}

// run returns a clipboard tool's output. The tools exit non-zero when the
// clipboard holds no PNG.
func run(name string, args ...string) ([]byte, error) {
	out, err := exec.Command(name, args...).Output()
	var exit *exec.ExitError
	switch {
	case errors.As(err, &exit):
		return nil, ErrNoImage
	case err != nil:
		return nil, fmt.Errorf("pasting an image needs %s: %w", name, err)
	}
	return out, nil
}

// decodeAppleData decodes AppleScript's «data PNGf89504E47...» output.
func decodeAppleData(out []byte) ([]byte, error) {
	s, prefixed := strings.CutPrefix(strings.TrimSpace(string(out)), "«data PNGf")
	s, suffixed := strings.CutSuffix(s, "»")
	if !prefixed || !suffixed {
		return nil, ErrNoImage
	}
	return hex.DecodeString(s)
}
