package tui

import (
	"slices"
	"testing"

	"github.com/sadmachine/asanamate/internal/asana"
)

func TestCreateSelectsTheNewTicket(t *testing.T) {
	cases := []struct {
		name, key, title, want, selected string
		setup                            func(m *Model)
	}{
		{"my tasks", "n", "New ticket in My Tasks",
			`POST /tasks {"data":{"assignee":"me","name":"New","workspace":"w"}}`, "9", func(*Model) {}},
		{"project section", "n", "New ticket in Web / Todo",
			`POST /tasks {"data":{"memberships":[{"project":"p1","section":"s1"}],"name":"New","projects":["p1"]}}`, "9",
			func(m *Model) { m.viewProject, m.groupBy = &asana.Ref{GID: "p1", Name: "Web"}, "section" }},
		{"subtask", "N", "New subtask of Fix",
			`POST /tasks/1/subtasks {"data":{"name":"New"}}`, "1", func(*Model) {}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, writes := editModel(t)
			tc.setup(m)
			press(m, tc.key)
			if m.input == nil || m.input.title != tc.title {
				t.Fatalf("input = %+v", m.input)
			}
			m.input.area.SetValue("New")
			send(m, ctrlS)
			if !slices.Equal(*writes, []string{tc.want}) {
				t.Fatalf("writes = %q", *writes)
			}
			if sel, ok := m.selected(); !ok || sel.GID != tc.selected {
				t.Fatalf("selected = %+v, status = %q", sel, m.status)
			}
		})
	}
}

func TestCreateKeepsHiddenTicketInViewing(t *testing.T) {
	m, _ := editModel(t)
	press(m, "n")
	m.input.area.SetValue("New")
	send(m, ctrlS)
	if m.viewing == nil || m.viewing.GID != "9" || !m.viewingRow {
		t.Fatalf("viewing = %+v, row = %v", m.viewing, m.viewingRow)
	}
}

func TestCreateIgnoresEmptyTitle(t *testing.T) {
	m, writes := editModel(t)
	press(m, "n")
	send(m, ctrlS)
	if m.input != nil || len(*writes) != 0 || m.status != "title is empty; nothing created" {
		t.Fatalf("input = %v, writes = %q, status = %q", m.input, *writes, m.status)
	}
}
