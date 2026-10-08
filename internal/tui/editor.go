package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// editorDoneMsg applies the result on the UI goroutine, including nested editors.
type editorDoneMsg struct {
	text  string
	err   error
	apply func(string, error)
}

// externalEditor suspends the TUI and returns the saved buffer without submitting.
// Editor settings are trusted shell commands, so flags and quoted paths work.
func externalEditor(text string, apply func(string, error)) tea.Cmd {
	done := func(text string, err error) tea.Msg {
		return editorDoneMsg{text: text, err: err, apply: apply}
	}
	dir, err := os.MkdirTemp("", "asanamate-editor-*")
	if err != nil {
		return func() tea.Msg { return done("", fmt.Errorf("create editor buffer: %w", err)) }
	}
	path := filepath.Join(dir, "draft.md")
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		os.RemoveAll(dir)
		return func() tea.Msg { return done("", fmt.Errorf("write editor buffer: %w", err)) }
	}
	editor := strings.TrimSpace(os.Getenv("VISUAL"))
	if editor == "" {
		editor = strings.TrimSpace(os.Getenv("EDITOR"))
	}
	if editor == "" {
		editor = "vim"
	}
	// Pass the buffer separately rather than interpolating a path into shell code.
	cmd := exec.Command("/bin/sh", "-c", "exec "+editor+` "$1"`, "asanamate-editor", path)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		defer os.RemoveAll(dir)
		if err != nil {
			return done("", fmt.Errorf("external editor: %w", err))
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return done("", fmt.Errorf("read editor buffer: %w", err))
		}
		return done(string(data), nil)
	})
}
