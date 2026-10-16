// Package conversations defines the generic ticket conversation source and open
// protocol. Native history, title parsing, and resumption belong to external tools.
package conversations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/x/ansi"
	"github.com/sadmachine/asanamate/internal/asana"
	"github.com/sadmachine/asanamate/internal/jsoncmd"
)

// ProtocolVersion changes only when the existing source/open contract breaks.
const ProtocolVersion = 1

// Discovery bounds apply to every source, regardless of its implementation.
const (
	LookupTimeout        = 10 * time.Second
	MaxStdout            = 4 * 1024 * 1024
	MaxStderr            = 64 * 1024
	MaxDiagnosticMessage = 1024 // Unicode code points after display sanitization
)

// Asana identifies one exact selected task, independently of provider identity.
type Asana struct {
	TaskGID string `json:"task_gid"`
}

// Request is sent on discovery stdin, followed by EOF.
type Request struct {
	Version   int    `json:"version"`
	Operation string `json:"operation"`
	Asana     Asana  `json:"asana"`
}

// Record is native saved metadata. Provider and SessionID together identify it.
// Preserve raw metadata for opening; use DisplayText before rendering it.
type Record struct {
	Provider  string `json:"provider"`
	SessionID string `json:"session_id"`
	TaskGID   string `json:"task_gid"`
	Title     string `json:"title"`
	CWD       string `json:"cwd,omitempty"`
	Branch    string `json:"branch,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
	Archived  bool   `json:"archived"`
	Activity  string `json:"activity"`
}

// Validate checks metadata without accessing native history or the filesystem.
// A missing directory remains valid historical context until the opener checks it.
func (r Record) Validate(taskGID string) error {
	if !asana.ValidGID(taskGID) {
		return errors.New("invalid requested task_gid")
	}
	for _, field := range []struct{ name, value string }{{"provider", r.Provider}, {"session_id", r.SessionID}} {
		if strings.TrimSpace(field.value) == "" || strings.ContainsFunc(field.value, unicode.IsControl) {
			return fmt.Errorf("%s must be nonempty and contain no control characters", field.name)
		}
	}
	if !asana.ValidGID(r.TaskGID) || r.TaskGID != taskGID {
		return errors.New("task_gid must be valid and equal the requested task_gid")
	}
	switch r.Activity {
	case "active", "inactive", "unknown":
	default:
		return errors.New("activity must be active, inactive, or unknown")
	}
	if r.CWD != "" && (!filepath.IsAbs(r.CWD) || strings.ContainsRune(r.CWD, '\x00')) {
		return errors.New("cwd must be an absolute path without NUL")
	}
	if r.UpdatedAt != "" {
		if _, err := time.Parse(time.RFC3339Nano, r.UpdatedAt); err != nil {
			return errors.New("updated_at must be an RFC 3339 timestamp")
		}
	}
	return nil
}

// Diagnostic describes provider availability or incomplete discovery. Message is
// bounded and display-safe after List; provider identity remains unmodified.
type Diagnostic struct {
	Provider string `json:"provider"`
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// Result retains diagnostics even when usable conversations are available.
type Result struct {
	Version       int          `json:"version"`
	Conversations []Record     `json:"conversations"`
	Diagnostics   []Diagnostic `json:"diagnostics,omitempty"`
}

// Complete reports whether no provider warned of a partial lookup. An absent
// optional provider is informational when another provider completed discovery.
func (r Result) Complete() bool {
	for _, d := range r.Diagnostics {
		if d.Severity != "info" {
			return false
		}
	}
	return true
}

// OpenRequest is a private file, never interactive stdin. The opener must
// revalidate identity and the ticket link against native history before use.
type OpenRequest struct {
	Version      int    `json:"version"`
	Operation    string `json:"operation"`
	Asana        Asana  `json:"asana"`
	Conversation Record `json:"conversation"`
}

// Source executes trusted configured shell text; request data only enters stdin.
type Source struct{ Command string }

// List performs a fresh, bounded lookup for taskGID. Errors never return truncated
// or partially validated records. Valid partial lookups retain all diagnostics.
func (s Source) List(ctx context.Context, taskGID string) (Result, error) {
	if !asana.ValidGID(taskGID) {
		return Result{}, errors.New("conversations: invalid task_gid")
	}
	if strings.TrimSpace(s.Command) == "" {
		return Result{}, errors.New("conversations.command is not configured")
	}
	ctx, cancel := context.WithTimeout(ctx, LookupTimeout)
	defer cancel()
	out, err := jsoncmd.RunBounded(ctx, "conversations command", s.Command,
		Request{Version: ProtocolVersion, Operation: "list", Asana: Asana{TaskGID: taskGID}},
		jsoncmd.Limits{Stdout: MaxStdout, Stderr: MaxStderr})
	if err != nil {
		return Result{}, fmt.Errorf("conversations: incomplete lookup: %w", err)
	}
	result, err := decode(out, taskGID)
	if err != nil {
		return Result{}, fmt.Errorf("conversations: invalid response: %w", err)
	}
	return result, nil
}

func decode(data []byte, taskGID string) (Result, error) {
	fields, err := object(data, []string{"version", "conversations"}, []string{"diagnostics"})
	if err != nil {
		return Result{}, err
	}
	var version int
	if err := json.Unmarshal(fields["version"], &version); err != nil || version != ProtocolVersion {
		return Result{}, errors.New("unsupported protocol version; expected version 1")
	}
	var rows, diagnostics []json.RawMessage
	if err := json.Unmarshal(fields["conversations"], &rows); err != nil {
		return Result{}, fmt.Errorf("conversations: %w", err)
	}
	if raw, ok := fields["diagnostics"]; ok {
		if err := json.Unmarshal(raw, &diagnostics); err != nil {
			return Result{}, fmt.Errorf("diagnostics: %w", err)
		}
	}
	result := Result{Version: version, Conversations: []Record{}}
	type identity struct{ provider, sessionID string }
	seen := make(map[identity]Record, len(rows))
	for i, raw := range rows {
		fields, err := object(raw, []string{"provider", "session_id", "task_gid", "title", "activity"}, []string{"cwd", "branch", "updated_at", "archived"})
		if err != nil {
			return Result{}, fmt.Errorf("record %d: %w", i, err)
		}
		var r Record
		if err := json.Unmarshal(raw, &r); err != nil {
			return Result{}, fmt.Errorf("record %d: %w", i, err)
		}
		if _, ok := fields["cwd"]; ok && r.CWD == "" {
			return Result{}, fmt.Errorf("record %d: cwd must be absolute when supplied", i)
		}
		if _, ok := fields["updated_at"]; ok && r.UpdatedAt == "" {
			return Result{}, fmt.Errorf("record %d: updated_at must be RFC 3339 when supplied", i)
		}
		if err := r.Validate(taskGID); err != nil {
			return Result{}, fmt.Errorf("record %d: %w", i, err)
		}
		key := identity{r.Provider, r.SessionID}
		if previous, ok := seen[key]; ok {
			if previous != r {
				return Result{}, fmt.Errorf("record %d: conflicting duplicate identity", i)
			}
			continue
		}
		seen[key] = r
		result.Conversations = append(result.Conversations, r)
	}
	for i, raw := range diagnostics {
		if _, err := object(raw, []string{"provider", "code", "severity", "message"}, nil); err != nil {
			return Result{}, fmt.Errorf("diagnostic %d: %w", i, err)
		}
		var d Diagnostic
		if err := json.Unmarshal(raw, &d); err != nil {
			return Result{}, fmt.Errorf("diagnostic %d: %w", i, err)
		}
		if strings.TrimSpace(d.Provider) == "" || strings.ContainsFunc(d.Provider, unicode.IsControl) {
			return Result{}, fmt.Errorf("diagnostic %d: provider must be nonempty without controls", i)
		}
		switch d.Code {
		case "unavailable", "unsupported_version", "permission_denied", "metadata_conflict", "incomplete", "provider_error":
		default:
			return Result{}, fmt.Errorf("diagnostic %d: unsupported code", i)
		}
		switch d.Severity {
		case "info", "warning", "error":
		default:
			return Result{}, fmt.Errorf("diagnostic %d: unsupported severity", i)
		}
		d.Message = DisplayText(d.Message)
		if d.Message == "" {
			return Result{}, fmt.Errorf("diagnostic %d: message must be nonempty", i)
		}
		if runes := []rune(d.Message); len(runes) > MaxDiagnosticMessage {
			d.Message = string(runes[:MaxDiagnosticMessage-1]) + "…"
		}
		result.Diagnostics = append(result.Diagnostics, d)
	}
	return result, nil
}

// object checks field presence separately: encoding/json otherwise treats null
// as a zero value for strings, booleans, and integers. Unknown fields are ignored.
func object(data []byte, required, optional []string) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	if fields == nil {
		return nil, errors.New("expected a JSON object")
	}
	for _, name := range required {
		if _, ok := fields[name]; !ok {
			return nil, fmt.Errorf("missing %s", name)
		}
	}
	for _, names := range [][]string{required, optional} {
		for _, name := range names {
			if value, ok := fields[name]; ok && string(value) == "null" {
				return nil, fmt.Errorf("%s must not be null", name)
			}
		}
	}
	return fields, nil
}

// DisplayText strips terminal escape sequences, controls, and Unicode format
// characters, then makes a single line. Never use it to transform native IDs or
// paths passed to an opener: keep exact selection data in the private snapshot.
func DisplayText(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			return ' '
		}
		return r
	}, ansi.Strip(s))
	return strings.Join(strings.Fields(s), " ")
}
