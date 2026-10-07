package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sadmachine/asanamate/internal/asana"
	"github.com/sadmachine/asanamate/internal/config"
	"github.com/sadmachine/asanamate/internal/ticket"
)

func TestListOnlyShowsOnlyTheList(t *testing.T) {
	for name, start := range map[string]func(t *testing.T) *Model{
		"--no-preview": func(t *testing.T) *Model {
			m, _ := testModel(t, config.Config{})
			m.deps.NoPreview = true
			return m
		},
		"layout.reader = false": func(t *testing.T) *Model {
			_, st := testModel(t, config.Config{})
			return New(Deps{Config: config.Config{Layout: config.Layout{Reader: false}}, State: st, StateDir: t.TempDir()})
		},
	} {
		t.Run(name, func(t *testing.T) {
			m := start(t)
			m.Update(tea.WindowSizeMsg{Width: 160, Height: 30})
			m.Update(tasksMsg{tasks: []asana.Task{openTask}})
			m.Update(key("tab"))
			if listW, _, split := m.paneWidths(); listW != 160 || split || m.focusReader {
				t.Fatalf("listW = %d, split = %v, focusReader = %v", listW, split, m.focusReader)
			}
			m.Update(detailMsg{gid: "1", ticket: ticket.Ticket{Task: openTask}})
			body := m.body()
			if m.shownGID != "1" || !strings.Contains(body, "Fix login") || strings.Contains(body, "Loading") {
				t.Fatalf("shown = %q, body = %q", m.shownGID, body)
			}
			if strings.Contains(m.helpView(), "next / previous pane") || strings.Contains(m.helpView(), "views panel") {
				t.Fatal("key help must not advertise the reader in list-only mode")
			}
		})
	}
}

func TestToggleReaderShowsAndHidesTheReader(t *testing.T) {
	m, _ := testModel(t, config.Config{})
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.Update(tasksMsg{tasks: []asana.Task{openTask}})
	m.Update(key("tab"))
	m.Update(key("z"))
	if listW, _, split := m.paneWidths(); listW != 120 || split || m.focusReader {
		t.Fatalf("hidden: listW = %d, split = %v, focusReader = %v", listW, split, m.focusReader)
	}
	if m.Update(key("tab")); m.focusReader {
		t.Fatal("tab must not focus a hidden reader")
	}
	// The ticket arrives while the reader is hidden; showing it draws it.
	m.Update(detailMsg{gid: "1", ticket: ticket.Ticket{Task: openTask}})
	m.Update(key("z"))
	if _, _, split := m.paneWidths(); !split || !strings.Contains(m.reader.View(), "Fix login") {
		t.Fatalf("shown: split = %v, reader = %q", split, m.reader.View())
	}
	m.Update(key("tab"))
	if !m.focusReader {
		t.Fatal("tab must focus the shown reader")
	}
}

func TestNoPreviewIgnoresToggleReader(t *testing.T) {
	m, _ := testModel(t, config.Config{})
	m.deps.NoPreview = true
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.Update(key("z"))
	if _, _, split := m.paneWidths(); split || strings.Contains(m.helpView(), "show / hide reader") {
		t.Fatalf("split = %v; --no-preview must keep the reader off", split)
	}
}

func TestFixedListWidth(t *testing.T) {
	m, _ := testModel(t, config.Config{Layout: config.Layout{Reader: true, ListWidth: 60}})
	m.Update(tea.WindowSizeMsg{Width: 160, Height: 30})
	m.Update(tasksMsg{tasks: []asana.Task{openTask}})
	m.Update(key("="))
	listW, readerW, split := m.paneWidths()
	if listW+panelFrame != 60 || readerW+panelFrame != 160-navW-60 || !split || !m.showNav() {
		t.Fatalf("listW = %d, readerW = %d, split = %v, nav = %v", listW, readerW, split, m.showNav())
	}
	if m.listW != 0 {
		t.Fatalf("= must leave a fixed list alone, fitted %d", m.listW)
	}
	// Too little room beside the list hides the reader, as on narrow terminals.
	m.deps.Config.Layout.ListWidth = 140
	m.Update(tea.WindowSizeMsg{Width: 160, Height: 30})
	if listW, _, split := m.paneWidths(); listW != 160 || split || m.showNav() {
		t.Fatalf("too wide: listW = %d, split = %v, nav = %v", listW, split, m.showNav())
	}
}

func TestSpaceOpensMenuAfterDetailLoads(t *testing.T) {
	m, _ := testModel(t, config.Config{Actions: []config.Action{{Name: "Go", Key: "x", Mode: config.ModeExit, Command: "true"}}})
	m.Update(tasksMsg{tasks: []asana.Task{openTask}})
	_, cmd := m.Update(key(" "))
	if cmd == nil || m.modal != nil || m.menuFor != "1" {
		t.Fatalf("cmd = %v, modal = %v, menuFor = %q", cmd, m.modal, m.menuFor)
	}
	m.Update(detailMsg{gid: "1", ticket: ticket.Ticket{Task: openTask}})
	if m.modal == nil || !strings.HasPrefix(m.modal.title, "Run on: ") || m.menuFor != "" {
		t.Fatalf("modal = %+v, menuFor = %q", m.modal, m.menuFor)
	}
}

func TestSpaceWithCachedDetailOpensMenuImmediately(t *testing.T) {
	m, _ := testModel(t, config.Config{Actions: []config.Action{{Name: "Go", Key: "x", Mode: config.ModeExit, Command: "true"}}})
	m.Update(tasksMsg{tasks: []asana.Task{openTask}})
	m.Update(detailMsg{gid: "1", ticket: ticket.Ticket{Task: openTask}})
	m.Update(key(" "))
	if m.modal == nil || !strings.HasPrefix(m.modal.title, "Run on: ") {
		t.Fatalf("modal = %+v", m.modal)
	}
}

func TestExitOnActionQuitsForBackgroundActions(t *testing.T) {
	for mode, wantExit := range map[string]bool{config.ModeBackground: true, config.ModeForeground: false} {
		t.Run(mode, func(t *testing.T) {
			m, _ := testModel(t, config.Config{Actions: []config.Action{{Name: "Go", Key: "x", Mode: mode, Command: "true"}}})
			m.deps.ExitOnAction = true
			m.Update(tasksMsg{tasks: []asana.Task{openTask}})
			m.Update(detailMsg{gid: "1", ticket: ticket.Ticket{Task: openTask}})
			m.openActionMenu()
			m.pickedAction(0)
			if got := m.ExitCommand() != nil; got != wantExit {
				t.Fatalf("exit = %v, want %v", got, wantExit)
			}
		})
	}
}
