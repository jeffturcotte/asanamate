package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sadmachine/asanamate/internal/activity"
	"github.com/sadmachine/asanamate/internal/asana"
	"github.com/sadmachine/asanamate/internal/ticket"
)

func activityModel(t *testing.T, width int) *Model {
	t.Helper()
	m := wideModel(t, width)
	m.openActivity()
	entries := []activity.Entry{
		{Task: sideTask, Story: asana.Story{GID: "s1", CreatedBy: &asana.Ref{GID: "me", Name: "Austin"}, ResourceSubtype: "assigned", Text: "Austin assigned to Sam", Assignee: &asana.Ref{Name: "Sam"}}, At: testToday, Mine: true},
		{Task: openTask, Story: asana.Story{GID: "s2", CreatedBy: &asana.Ref{GID: "other", Name: "Sam"}, ResourceSubtype: "due_date_changed", Text: "Sam changed the due date", NewDates: &asana.StoryDates{DueOn: new("2026-10-14")}}, At: testToday.Add(-time.Hour), Reasons: []string{"currently assigned to you"}},
		{Task: doneTask, Story: asana.Story{GID: "s3", ResourceSubtype: "due_today", Text: "due today"}, At: testToday.Add(-2 * time.Hour), Automatic: true, Reasons: []string{"currently assigned to you"}},
	}
	m.Update(activityMsg{seq: m.activity.seq, report: activity.Report{Entries: entries, Scanned: 3, Total: 3, Since: testToday.Add(-7 * 24 * time.Hour), Until: testToday}})
	t.Cleanup(func() { m.closeActivity() })
	return m
}

func TestActivityFiltersAndAutomatedToggle(t *testing.T) {
	m := activityModel(t, 160)
	if len(m.activity.visible) != 2 {
		t.Fatalf("Both = %d", len(m.activity.visible))
	}
	keys(m, "]")
	if m.activity.mode != "Mine" || len(m.activity.visible) != 1 || !m.activity.visible[0].Mine {
		t.Fatalf("Mine = %+v", m.activity.visible)
	}
	keys(m, "]")
	if m.activity.mode != "Related" || len(m.activity.visible) != 1 || m.activity.visible[0].Mine {
		t.Fatalf("Related = %+v", m.activity.visible)
	}
	keys(m, "z")
	if len(m.activity.visible) != 2 || !m.activity.visible[1].Automatic {
		t.Fatalf("automated = %+v", m.activity.visible)
	}
	keys(m, "]")
	if m.activity.mode != "Both" || len(m.activity.visible) != 3 {
		t.Fatalf("Both including automated = %+v", m.activity.visible)
	}
}

func TestActivitySearchDoesNotChangeSavedTicketFilter(t *testing.T) {
	m := activityModel(t, 160)
	before := m.filterInput.Value()
	keys(m, "/", "S", "a", "m", "enter")
	if m.activity.filtering || len(m.activity.visible) != 2 {
		t.Fatalf("search = %q, visible = %d", m.activity.query.Value(), len(m.activity.visible))
	}
	keys(m, "]")
	if len(m.activity.visible) != 1 || m.filterInput.Value() != before {
		t.Fatalf("activity search affected ticket filter")
	}
	if m.deps.State.CurrentSavedViews[""] != "" {
		t.Fatal("Activity must not save a ticket view")
	}
}

func TestActivityTicketRoundTripKeepsEventAndAllowsExistingReader(t *testing.T) {
	m := activityModel(t, 160)
	m.details[sideTask.GID] = ticket.Ticket{Task: sideTask}
	keys(m, "enter")
	if m.activityOpen || !m.activityTicket || !m.focusReader {
		t.Fatal("Enter must open existing ticket reader")
	}
	if m.detailRequests[sideTask.GID] == 0 {
		t.Fatal("cached ticket must refresh on opening from Activity")
	}
	selected, ok := m.selected()
	if !ok || selected.GID != sideTask.GID {
		t.Fatalf("selected = %+v", selected)
	}
	keys(m, "esc")
	if !m.activityOpen || m.activityTicket || m.activity.cursor != 0 {
		t.Fatal("Esc must return to same Activity entry")
	}
	keys(m, "j")
	if m.activity.cursor != 1 {
		t.Fatal("Activity navigation did not resume")
	}
}

func TestActivityNarrowEnterShowsEventThenTicket(t *testing.T) {
	m := activityModel(t, 80)
	keys(m, "j", "enter")
	if !m.activityOpen || !m.activity.details || !strings.Contains(ansi.Strip(m.body()), "New due: 2026-10-14") {
		t.Fatal("narrow Enter must show event details")
	}
	keys(m, "esc")
	if !m.activityOpen || m.activity.details {
		t.Fatal("Esc must return to activity list")
	}
	keys(m, "enter", "enter")
	if m.activityOpen || !m.activityTicket || !m.focusReader {
		t.Fatal("second Enter must open ticket")
	}
}

func TestActivityIgnoresStaleLoadsAndKeepsOldResultsOnError(t *testing.T) {
	m := activityModel(t, 160)
	m.reloadActivity()
	seq := m.activity.seq
	m.Update(activityMsg{seq: seq - 1, report: activity.Report{}})
	if !m.activity.loading || len(m.activity.visible) != 2 {
		t.Fatal("stale response replaced current activity")
	}
	m.Update(activityMsg{seq: seq, err: fmt.Errorf("search unavailable")})
	if m.activity.loading || len(m.activity.visible) != 2 || !strings.Contains(ansi.Strip(m.body()), "search unavailable") {
		t.Fatal("refresh error must preserve results and display error")
	}
	m.closeActivity()
	m.Update(activityMsg{seq: seq, report: activity.Report{}})
	if len(m.activity.visible) != 2 {
		t.Fatal("closed view accepted late response")
	}
}

func TestActivityPeriodChangeDoesNotRelabelOldResults(t *testing.T) {
	m := activityModel(t, 160)
	keys(m, "D", "j", "j", "enter")
	if m.activity.days != 30 || !m.activity.loading || len(m.activity.visible) != 0 || !m.activity.report.Until.IsZero() {
		t.Fatalf("period = %+v", m.activity)
	}
}

func TestActivityNavigationPanelAndProjectSwitch(t *testing.T) {
	m := wideModel(t, 160)
	keys(m, "0", "j", "enter")
	if !m.activityOpen {
		t.Fatal("sidebar Activity did not open")
	}
	m.Update(activityMsg{seq: m.activity.seq, report: activity.Report{}})
	keys(m, "0", "enter")
	if !m.activityOpen {
		t.Fatal("selecting active Activity should keep the view open")
	}
	keys(m, "0", "j", "enter") // Activity, then recent Mobile project.
	if m.activityOpen || m.activityTicket || gidOf(m.viewProject) != "p2" {
		t.Fatalf("project switch = %+v", m.viewProject)
	}
}

func TestActivityFitsTerminalCells(t *testing.T) {
	for _, width := range []int{80, 120, 160} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			m := activityModel(t, width)
			m.Update(tea.WindowSizeMsg{Width: width, Height: 40})
			for _, focus := range []bool{false, true} {
				m.activity.details = focus
				lines := strings.Split(m.View().Content, "\n")
				if len(lines) != 40 {
					t.Fatalf("height = %d, want 40", len(lines))
				}
				for _, line := range lines {
					if ansi.StringWidth(line) > width {
						t.Fatalf("row wider than %d: %q", width, ansi.Strip(line))
					}
				}
			}
		})
	}
}

func TestActivityTextCannotInjectTerminalControls(t *testing.T) {
	m := activityModel(t, 160)
	m.activity.report.Entries[0].Task.Name = "Bad\x1b]52;c;c2VjcmV0\a ticket"
	m.activity.report.Entries[0].Story.Text = "Bad\x1b[2J action"
	m.filterActivity()
	if strings.Contains(m.activity.reader.GetContent(), "\x1b]52;") || strings.Contains(m.activityList(50, 30), "\x1b]52;") || strings.Contains(m.activity.reader.GetContent(), "\x1b[2J") {
		t.Fatal("untrusted activity emitted terminal controls")
	}
}
