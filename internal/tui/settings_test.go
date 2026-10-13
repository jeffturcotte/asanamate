package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/sadmachine/asanamate/internal/asana"
	"github.com/sadmachine/asanamate/internal/config"
	"github.com/sadmachine/asanamate/internal/state"
)

func TestSettingsTogglesPersist(t *testing.T) {
	m, st := testModel(t, config.Default())
	m.loading = false
	m.Update(key("s"))
	if m.modal == nil || m.modal.title != "Settings" {
		t.Fatal("s did not open settings")
	}
	m.Update(key("s"))
	m.Update(key("h"))
	m.Update(key("d"))
	m.Update(key("v"))
	if m.modal == nil || m.modal.cursor != 2 {
		t.Fatalf("toggle did not reopen settings on its row: %+v", m.modal)
	}
	m.Update(key("esc"))
	if m.modal != nil || !m.separator || !m.spacing || m.focusDim || m.readerView != config.ViewMarkdown {
		t.Fatalf("separator = %v, spacing = %v, dim = %v, reader = %q", m.separator, m.spacing, m.focusDim, m.readerView)
	}
	loaded, err := state.Load(st.Path())
	if err != nil {
		t.Fatal(err)
	}
	next := New(Deps{Config: m.deps.Config, State: loaded})
	if !next.separator || !next.spacing || next.focusDim || next.readerView != config.ViewMarkdown || next.refreshInterval != 30*time.Second {
		t.Fatalf("next session: separator = %v, spacing = %v, dim = %v, reader = %q, interval = %s", next.separator, next.spacing, next.focusDim, next.readerView, next.refreshInterval)
	}
	if !m.deps.Config.Layout.Focus.Dim {
		t.Fatal("dim toggle changed config defaults")
	}
}

func TestSelectionSettingsToggleAndPersist(t *testing.T) {
	for _, initial := range []string{config.StyleMarker, config.StyleBar} {
		t.Run(initial, func(t *testing.T) {
			cfg := config.Default()
			cfg.List.Selection.Style = initial
			cfg.List.Layout = config.LayoutSingle
			m, st := testModel(t, cfg)
			m.Update(tasksMsg{tasks: []asana.Task{{GID: "1", Name: "A"}}})
			m.Update(key("s"))
			want := initial
			for range 2 {
				if want == config.StyleMarker {
					want = config.StyleBar
				} else {
					want = config.StyleMarker
				}
				m.Update(key("l"))
				if m.modal == nil || m.modal.cursor != 4 || m.modal.items[4].Hint != want {
					t.Fatalf("selection toggle did not reopen on its updated row: %+v", m.modal)
				}
				row := strings.Split(m.listView(20, 1), "\n")[0]
				if want == config.StyleMarker {
					if !strings.HasPrefix(row, m.markerStyle.Render(m.sym.cursor+" ")) || m.selectionBar() {
						t.Fatalf("marker did not apply immediately: %q", row)
					}
				} else if strings.Contains(ansi.Strip(row), m.sym.cursor) || !m.selectionBar() {
					t.Fatalf("bar did not apply immediately: %q", row)
				}
				// Saving another visual option must retain the selection style.
				m.Update(key("h"))
				loaded, err := state.Load(st.Path())
				if err != nil {
					t.Fatal(err)
				}
				next := New(Deps{Config: cfg, State: loaded})
				if loaded.Display.SelectionStyle != want || next.selectionStyle != want {
					t.Fatalf("selection style did not persist: saved = %q, next = %q, want %q", loaded.Display.SelectionStyle, next.selectionStyle, want)
				}
				if m.deps.Config.List.Selection.Style != initial {
					t.Fatal("selection toggle changed config defaults")
				}
			}
		})
	}
}

func TestSelectionSettingsFallback(t *testing.T) {
	for _, style := range []string{config.StyleMarker, config.StyleBar} {
		for _, saved := range []string{"", "invalid"} {
			t.Run(style+"/"+saved, func(t *testing.T) {
				cfg := config.Default()
				cfg.List.Selection.Style = style
				_, st := testModel(t, cfg)
				st.Display.SelectionStyle = saved
				m := New(Deps{Config: cfg, State: st})
				if m.selectionStyle != style {
					t.Fatalf("saved %q: selection style = %q, want config default %q", saved, m.selectionStyle, style)
				}
			})
		}
	}
}
