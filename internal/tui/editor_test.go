package tui

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sadmachine/asanamate/internal/form"
)

// Run the real Bubble Tea terminal handoff, then route its result through Model.
type editorTestModel struct {
	model *Model
	cmd   tea.Cmd
	done  bool
}

func (m *editorTestModel) Init() tea.Cmd  { return m.cmd }
func (m *editorTestModel) View() tea.View { return tea.NewView("") }
func (m *editorTestModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(editorDoneMsg); ok {
		m.model.Update(msg)
		m.done = true
		return m, tea.Quit
	}
	return m, nil
}

func runEditorTest(t *testing.T, model *Model, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		t.Fatal("ctrl+e did not request an editor")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	m := &editorTestModel{model: model, cmd: cmd}
	var output bytes.Buffer
	if _, err := tea.NewProgram(m, tea.WithContext(ctx), tea.WithInput(nil), tea.WithOutput(&output), tea.WithoutRenderer()).Run(); err != nil {
		t.Fatal(err)
	}
	if !m.done {
		t.Fatal("editor did not return a result")
	}
}

func editorScript(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "editor with spaces")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	return "'" + strings.ReplaceAll(path, "'", "'\"'\"'") + "'"
}

func TestExternalEditorSelectionAndPrivateBuffer(t *testing.T) {
	for _, source := range []string{"VISUAL", "EDITOR", "vim"} {
		t.Run(source, func(t *testing.T) {
			t.Setenv("VISUAL", "")
			t.Setenv("EDITOR", "")
			t.Setenv("TMPDIR", t.TempDir())
			command := editorScript(t, `[ "$1" = '--wait' ] && [ "$2" = 'flag value' ] || exit 2
printf 'new draft\nsecond line\n' > "$3"`) + " --wait 'flag value'"
			switch source {
			case "VISUAL":
				t.Setenv("VISUAL", command)
				t.Setenv("EDITOR", "false") // VISUAL wins.
			case "EDITOR":
				t.Setenv("EDITOR", command)
			case "vim":
				dir := t.TempDir()
				if err := os.WriteFile(filepath.Join(dir, "vim"), []byte("#!/bin/sh\nprintf 'new draft\\nsecond line\\n' > \"$1\"\n"), 0o700); err != nil {
					t.Fatal(err)
				}
				t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			}
			b := newInputBox("Comment", "")
			b.area.SetValue("old draft")
			m := &Model{input: b}
			_, cmd := m.Update(tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl})
			paths, err := filepath.Glob(filepath.Join(os.TempDir(), "asanamate-editor-*", "draft.md"))
			if err != nil || len(paths) != 1 {
				t.Fatalf("editor buffers = %v, err = %v", paths, err)
			}
			path := paths[0]
			data, err := os.ReadFile(path)
			if err != nil || string(data) != "old draft" {
				t.Fatalf("initial buffer = %q, err = %v", data, err)
			}
			for _, check := range []struct {
				path string
				mode os.FileMode
			}{{path, 0o600}, {filepath.Dir(path), 0o700}} {
				info, err := os.Stat(check.path)
				if err != nil || info.Mode().Perm() != check.mode {
					t.Fatalf("buffer permissions: %v, err = %v", info, err)
				}
			}
			runEditorTest(t, m, cmd)
			if b.area.Value() != "new draft\nsecond line\n" || b.err != "" || m.input != b {
				t.Fatalf("returned draft = %q, error = %q", b.area.Value(), b.err)
			}
			if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
				t.Fatalf("temporary buffer directory remains: %v", err)
			}
		})
	}
}

func TestExternalEditorPreservesDraft(t *testing.T) {
	for _, tc := range []struct {
		name, body, wantError string
	}{
		{"quit without saving", "exit 0", ""},
		{"failed after writing", `printf 'discarded' > "$1"; exit 7`, "external editor:"},
		{"deleted buffer", `rm "$1"`, "read editor buffer:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("VISUAL", editorScript(t, tc.body))
			t.Setenv("TMPDIR", t.TempDir())
			b := newInputBox("Comment", "")
			b.area.SetValue("keep this\ndraft")
			m := &Model{input: b}
			_, cmd := m.Update(tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl})
			runEditorTest(t, m, cmd)
			if b.area.Value() != "keep this\ndraft" || m.input != b || !strings.Contains(b.err, tc.wantError) || (tc.wantError == "" && b.err != "") {
				t.Fatalf("draft = %q, error = %q", b.area.Value(), b.err)
			}
			paths, _ := filepath.Glob(filepath.Join(os.TempDir(), "asanamate-editor-*"))
			if len(paths) != 0 {
				t.Fatalf("temporary buffers remain: %v", paths)
			}
		})
	}
}

func TestExternalEditorRequiresExplicitSubmit(t *testing.T) {
	for _, nested := range []bool{false, true} {
		t.Run(map[bool]string{false: "comment", true: "action builder"}[nested], func(t *testing.T) {
			t.Setenv("VISUAL", editorScript(t, `printf 'edited draft' > "$1"`))
			b := newInputBox("Text", "")
			var submitted string
			b.onSubmit = func(text string) tea.Cmd { submitted = text; return nil }
			m := &Model{input: b}
			if nested {
				m.input = nil
				m.builder = &actionBuilder{input: b}
			}
			_, cmd := m.Update(tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl})
			runEditorTest(t, m, cmd)
			if submitted != "" || b.area.Value() != "edited draft" {
				t.Fatal("editor submitted automatically or failed to restore draft")
			}
			m.Update(ctrlS)
			if submitted != "edited draft" {
				t.Fatalf("submitted draft = %q", submitted)
			}
		})
	}
}

func TestExternalEditorFormKeepsValuePending(t *testing.T) {
	t.Setenv("VISUAL", editorScript(t, `printf ' 2.5\n' > "$1"`))
	submitted := false
	f := newFormModal("Hours", form.Spec{Fields: []form.Field{{ID: "hours", Label: "Hours", Type: form.Hours}}}, nil,
		func(map[string]string) tea.Cmd { submitted = true; return nil })
	f.values["hours"] = "1.25"
	m := &Model{form: f}
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl})
	runEditorTest(t, m, cmd)
	if submitted || !f.editing || f.input.Value() != "2.5" || f.values["hours"] != "1.25" {
		t.Fatal("editor submitted or committed the field before confirmation")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if f.values["hours"] != "2.5" {
		t.Fatal("enter did not accept edited hours")
	}
	m.Update(ctrlS)
	if !submitted {
		t.Fatal("ctrl+s did not submit the form")
	}
}
