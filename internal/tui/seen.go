package tui

import (
	"slices"

	"github.com/sadmachine/asanamate/internal/asana"
)

// syncChanged marks the My Tasks tickets modified since they were last shown,
// and forgets seen tickets the list no longer has. Project views mark none.
func (m *Model) syncChanged(msg tasksMsg) {
	m.changed = nil
	if msg.project != nil {
		return
	}
	listed := append(slices.Clone(msg.tasks), msg.pins...)
	saved := m.deps.State.SyncSeen(listed)
	m.changed = map[string]bool{}
	for _, t := range listed {
		if m.deps.State.Changed(t) {
			m.changed[t.GID] = true
		}
	}
	if saved {
		m.saveSeen()
	}
}

// markSeen records t, shown in the reader, as seen at its current version.
func (m *Model) markSeen(t asana.Task) {
	delete(m.changed, t.GID)
	if m.deps.State.MarkSeen(t) {
		m.saveSeen()
	}
}

func (m *Model) saveSeen() {
	if err := m.deps.State.Save(); err != nil {
		m.status = "saving state: " + err.Error()
	}
}

// newBadge is visible ticket i's NEW badge, reversed on the selected row, or
// "" unless it changed since last shown. The shown ticket is being read, so a
// refresh that changed it is not news.
func (m *Model) newBadge(i int) string {
	t := m.visible[i]
	if !m.changed[t.GID] || t.GID == m.shownGID {
		return ""
	}
	style := newStyle
	if i == m.cursor {
		style = style.Reverse(!style.GetReverse())
	}
	return style.Render(" NEW ")
}
