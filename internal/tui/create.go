package tui

import (
	"cmp"
	"context"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sadmachine/asanamate/internal/ticket"
	"github.com/sadmachine/asanamate/internal/writeback"
)

// createdMsg reports a created ticket. show is the ticket to select: the new
// one, or the parent of a new subtask.
type createdMsg struct {
	show, what string
	err        error
}

// openCreate asks for the title of a ticket in the viewed project, in the
// selected ticket's section when the list is grouped by section, or else in
// My Tasks.
func (m *Model) openCreate() {
	var nt writeback.NewTicket
	where := "My Tasks"
	if p := m.viewProject; p != nil {
		nt.Project, where = p.GID, ticket.Clean(p.Name)
		if t, ok := m.selected(); ok && strings.EqualFold(m.groupBy, "section") {
			if s := t.SectionIn(p.GID); s != "" {
				nt.Section, where = s, where+" / "+ticket.Clean(s)
			}
		}
	}
	m.openTitle("New ticket in "+where, "ticket", nt)
}

// openCreateSubtask asks for the title of a subtask of the selected ticket.
func (m *Model) openCreateSubtask() {
	t, ok := m.selected()
	if !ok {
		m.status = "no ticket selected"
		return
	}
	m.openTitle("New subtask of "+ticket.Clean(t.Name), "subtask", writeback.NewTicket{Parent: t.GID})
}

// openTitle asks for a one-line title, then creates nt with it. Other fields
// are set afterwards with the edit keys.
func (m *Model) openTitle(title, what string, nt writeback.NewTicket) {
	b := newInputBox(title, "title")
	b.oneLine = true
	b.area.KeyMap.InsertNewline.SetEnabled(false)
	b.onSubmit = func(name string) tea.Cmd {
		m.input = nil
		if name == "" {
			m.status = "title is empty; nothing created"
			return nil
		}
		nt.Name, m.status = name, "create "+what+"…"
		svc := m.writer()
		return request(func(ctx context.Context) tea.Msg {
			t, err := svc.Create(ctx, nt)
			return createdMsg{show: cmp.Or(nt.Parent, t.GID), what: "create " + what, err: err}
		})
	}
	m.input = b
}

// created reloads the list, then selects the ticket to show and focuses the
// reader on it, keeping it under Viewing when the filter hides it.
func (m *Model) created(msg createdMsg) tea.Cmd {
	if msg.err != nil {
		m.status = msg.what + " failed: " + msg.err.Error()
		return nil
	}
	m.status, m.openGID = msg.what+": done", msg.show
	return tea.Batch(m.refresh(), m.loadDetail(msg.show))
}
