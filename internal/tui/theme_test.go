package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sadmachine/asanamate/internal/config"
	"github.com/sadmachine/asanamate/internal/state"
)

func TestThemePickerPreviewsAndPersists(t *testing.T) {
	m, st := testModel(t, config.Default())
	m.loading = false
	m.width, m.height = 120, 40
	m.deps.ConfigPath = filepath.Join(t.TempDir(), "config.toml")
	dir := config.ThemesDir(m.deps.ConfigPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"day":    "base = \"light\"\n[colors]\naccent = \"#123456\"\n",
		"broken": "base = \"blue\"\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name+".toml"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	m.Update(key("s"))
	m.Update(key("t"))
	if m.modal == nil || m.modal.title != "Theme" || m.modal.cursor != 0 || m.modal.items[0].Hint != "current" {
		t.Fatalf("theme picker did not open on the current theme: %+v", m.modal)
	}
	if view := ansi.Strip(m.View().Content); !strings.Contains(view, "Tickets") || !strings.Contains(view, "NORMAL") {
		t.Fatalf("preview missing:\n%s", view)
	}
	m.Update(key("down"))
	m.Update(key("down"))
	m.Update(key("down"))
	if view := ansi.Strip(m.View().Content); !strings.Contains(view, "base") || !strings.Contains(view, `"blue"`) {
		t.Fatalf("broken theme should show its error:\n%s", view)
	}
	m.Update(key("down"))
	m.Update(key("enter"))
	if m.modal != nil || m.deps.Config.Theme.Name != "day" || m.palette.Colors.Accent.FG != "#123456" {
		t.Fatalf("theme not applied: modal = %v, theme = %q, accent = %q", m.modal, m.deps.Config.Theme.Name, m.palette.Colors.Accent.FG)
	}
	if got := m.accentStyle.GetForeground(); got != lipStyle(config.Style{FG: "#123456"}).GetForeground() {
		t.Fatalf("accent style = %v", got)
	}
	loaded, err := state.Load(st.Path())
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Display.Theme != "day" {
		t.Fatalf("saved theme = %q", loaded.Display.Theme)
	}
}

func TestThemePreviewFitsItsSpace(t *testing.T) {
	p := config.Config{}.Palette(true)
	for _, tc := range []struct {
		width, height int
		full          bool
	}{{30, 13, false}, {79, 30, false}, {80, 21, false}, {80, 22, true}, {106, 40, true}} {
		view := themePreview(p, tc.width, tc.height)
		lines := strings.Split(view, "\n")
		if len(lines) > tc.height {
			t.Errorf("%dx%d: %d lines", tc.width, tc.height, len(lines))
		}
		for _, line := range lines {
			if w := ansi.StringWidth(line); w > tc.width {
				t.Errorf("%dx%d: line %d wide: %q", tc.width, tc.height, w, ansi.Strip(line))
			}
		}
		plain := ansi.Strip(view)
		if full := strings.Contains(plain, "Ticket ") && strings.Contains(plain, "enter act"); full != tc.full {
			t.Errorf("%dx%d: full = %v, want %v:\n%s", tc.width, tc.height, full, tc.full, plain)
		}
		if !strings.Contains(plain, "Tickets") || !strings.Contains(plain, "FILTER") {
			t.Errorf("%dx%d: missing ticket list or pills:\n%s", tc.width, tc.height, plain)
		}
	}
}
