package tui

import (
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/sadmachine/asanamate/internal/asana"
	"github.com/sadmachine/asanamate/internal/config"
	"github.com/sadmachine/asanamate/internal/ticket"
)

func TestYankSelection(t *testing.T) {
	for _, tt := range []struct {
		name, target, want string
		reader             bool
	}{
		{"list", "", openTask.Name, false},
		{"comment", "comment:c1", "Hello **world**", true},
		{"assignee", "assignee", "Sam", true},
		{"project", "project:p1", "Project / Section", true},
		{"description", "section:description", "Notes", true},
		{"unsupported", commentKey, "", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m, _ := testModel(t, config.Default())
			task := openTask
			task.Assignee = &asana.Ref{Name: "Sam"}
			task.Memberships = []asana.Membership{{Project: asana.Ref{GID: "p1", Name: "Project"}, Section: &asana.Ref{Name: "Section"}}}
			task.HTMLNotes = "<body>Notes</body>"
			m.Update(tasksMsg{tasks: []asana.Task{task}})
			m.Update(detailMsg{gid: task.GID, ticket: ticket.Ticket{Task: task, Comments: []asana.Story{{GID: "c1", HTMLText: "<body>Hello <strong>world</strong></body>"}}}})
			m.focusReader, m.fieldKey = tt.reader, tt.target
			_, cmd := m.Update(key("Y"))
			if tt.want == "" {
				if cmd != nil {
					t.Fatal("unsupported selection copied")
				}
				return
			}
			want := tea.SetClipboard(tt.want)()
			if cmd == nil || !slices.Contains(commandMessages(cmd), want) {
				t.Fatalf("Y did not copy %q", tt.want)
			}
			m.openYankMenu()
			p := m.modal
			result, _ := p.update(key("y"))
			if !result.done {
				t.Fatal("yy did not choose selection")
			}
			cmd = p.onPick(result)
			if cmd == nil || !slices.Contains(commandMessages(cmd), want) {
				t.Fatalf("yy did not copy %q", tt.want)
			}
		})
	}
}

func TestYankMenu(t *testing.T) {
	task := openTask
	task.PermalinkURL = "https://app.asana.com/0/0/1"
	m, _ := testModel(t, config.Default())
	m.Update(tasksMsg{tasks: []asana.Task{task}})
	m.Update(detailMsg{gid: task.GID, ticket: ticket.Ticket{Task: task, Comments: []asana.Story{
		{CreatedAt: "2026-10-01T12:00:00Z", HTMLText: "old"},
		{CreatedAt: "2026-10-02T12:00:00Z", HTMLText: "new"},
	}}})
	for _, tt := range []struct{ key, want string }{{"u", task.PermalinkURL}, {"g", task.GID}, {"t", task.Name}, {"c", "new"}} {
		m.openYankMenu()
		p := m.modal
		result, _ := p.update(key(tt.key))
		if !result.done {
			t.Fatalf("%s unavailable", tt.key)
		}
		cmd := p.onPick(result)
		if cmd == nil || !slices.Contains(commandMessages(cmd), tea.SetClipboard(tt.want)()) {
			t.Fatalf("%s did not copy %q", tt.key, tt.want)
		}
	}
	m.details = nil
	m.focusReader = true
	m.openYankMenu()
	p := m.modal
	for _, k := range []string{"y", "c"} {
		result, _ := p.update(key(k))
		if result.done {
			t.Fatalf("unavailable %s chosen", k)
		}
	}
	p.cursor = 0
	result, _ := p.update(key("enter"))
	if result.done {
		t.Fatal("disabled selection chosen by enter")
	}
	result, _ = p.update(key("esc"))
	if !result.cancelled {
		t.Fatal("escape did not cancel")
	}
}

func TestYankDefaultKeys(t *testing.T) {
	m, _ := testModel(t, config.Default())
	m.Update(tasksMsg{tasks: []asana.Task{openTask}})
	m.Update(detailMsg{gid: openTask.GID, ticket: ticket.Ticket{Task: openTask}})
	m.Update(key("y"))
	if m.modal == nil || m.modal.title != "Yank" {
		t.Fatal("y did not open yank menu")
	}
	m.Update(key("esc"))
	if m.modal != nil {
		t.Fatal("escape left yank menu open")
	}
}

func TestYankSidebarProject(t *testing.T) {
	m, _ := testModel(t, config.Default())
	m.Update(tasksMsg{tasks: []asana.Task{}})
	m.viewProject = &asana.Ref{GID: "p1", Name: "Sidebar project"}
	items := m.navItems()
	n := 0
	for _, it := range items {
		if !it.selectable() {
			continue
		}
		if it.project != nil {
			m.navCursor = n
			break
		}
		n++
	}
	m.focusNav = true
	_, cmd := m.Update(key("Y"))
	if cmd == nil || !slices.Contains(commandMessages(cmd), tea.SetClipboard("Sidebar project")()) {
		t.Fatal("sidebar project not copied")
	}
	m.Update(key("y"))
	if m.modal == nil || m.modal.items[0].Disabled {
		t.Fatal("sidebar project unavailable in yank menu without ticket")
	}
}
