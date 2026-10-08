// Package jsoncmd runs provider commands that read one JSON request on stdin
// and write one JSON response on stdout.
package jsoncmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// Run executes command with /bin/sh -c and returns its stdout. Data goes
// through stdin, never into user-authored shell text. label prefixes errors,
// such as "time provider".
func Run(ctx context.Context, label, command string, req any) ([]byte, error) {
	input, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)
	cmd.Stdin = bytes.NewReader(input)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if message := strings.TrimSpace(stderr.String()); message != "" {
			return nil, fmt.Errorf("%s: %s: %w", label, message, err)
		}
		return nil, fmt.Errorf("%s: %w", label, err)
	}
	return out, nil
}
