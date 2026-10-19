package tui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/sadmachine/asanamate/internal/config"
	"github.com/sadmachine/asanamate/internal/ticket"
)

// yankValue captures the value at menu opening, independent of later refreshes.
type yankValue struct{ label, text string }

func (v yankValue) copy(m *Model) tea.Cmd {
	if strings.TrimSpace(v.text) == "" {
		return nil
	}
	m.setStatus(statusInfo, v.label+" copied")
	return tea.SetClipboard(v.text)
}

func (m *Model) copySelection() yankValue {
	if m.focusNav {
		items := m.navItems()
		if i := m.navIndex(items); i >= 0 && i < len(items) && items[i].project != nil {
			return yankValue{"Project name", items[i].project.Name}
		}
		return yankValue{}
	}
	if m.focusFeed {
		return yankValue{}
	}
	if !m.focusReader {
		if t, ok := m.selected(); ok {
			return yankValue{"Ticket title", t.Name}
		}
		return yankValue{}
	}
	if m.readerHidden || m.readerView == config.ViewMarkdown {
		return yankValue{}
	}
	t, ok := m.selectedDetail()
	if !ok {
		return yankValue{}
	}
	if c := m.selectedComment(t); c != nil {
		return yankValue{"Selected comment", ticket.HTMLToMarkdown(c.HTMLText)}
	}
	if m.fieldKey == "section:description" || m.fieldKey == "description" {
		return yankValue{"Description", t.Description()}
	}
	for _, fields := range [][]ticket.Field{t.Meta(), t.FieldValues()} {
		for _, f := range fields {
			if f.Key != "" && f.Key == m.fieldKey {
				return yankValue{f.Label, f.Value}
			}
		}
	}
	return yankValue{}
}

func (m *Model) openYankMenu() {
	task, ok := m.selected()
	if !ok {
		if m.focusNav {
			m.yankMenu(ticket.Ticket{}, ticket.Ticket{}, false, m.copySelection())
		}
		return
	}
	t, loaded := m.selectedDetail()
	m.yankMenu(ticket.Ticket{Task: task}, t, loaded, m.copySelection())
}

func (m *Model) yankMenu(base, detail ticket.Ticket, loaded bool, selection yankValue) {
	latest := yankValue{"Latest comment", ""}
	if loaded && len(detail.Comments) > 0 {
		c := detail.Comments[0]
		for _, next := range detail.Comments[1:] {
			a, errA := time.Parse(time.RFC3339Nano, c.CreatedAt)
			b, errB := time.Parse(time.RFC3339Nano, next.CreatedAt)
			if errA == nil && errB == nil && b.After(a) {
				c = next
			}
		}
		latest.text = ticket.HTMLToMarkdown(c.HTMLText)
	}
	if selection.label == "" {
		selection.label = "Current selection"
	}
	values := []yankValue{selection, {"Ticket URL", base.PermalinkURL}, {"Ticket GID", base.GID}, {"Ticket title", base.Name}, latest}
	items := make([]pickItem, len(values))
	for i, v := range values {
		items[i] = pickItem{Label: v.label, Key: []string{"y", "u", "g", "t", "c"}[i], Value: v, Disabled: strings.TrimSpace(v.text) == ""}
		if items[i].Disabled {
			items[i].Hint = "unavailable"
		}
	}
	if !loaded {
		items[4].Hint = "ticket details not loaded"
	}
	p := newPicker(pickValue(func(v yankValue) tea.Cmd { m.modal = nil; return v.copy(m) }), "Yank", items)
	p.keySelect = true
	m.modal = p
}
