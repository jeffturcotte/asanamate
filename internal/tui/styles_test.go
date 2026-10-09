package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sadmachine/asanamate/internal/asana"
	"github.com/sadmachine/asanamate/internal/state"
	"github.com/sadmachine/asanamate/internal/ticket"

	"github.com/sadmachine/asanamate/internal/agents"
	"github.com/sadmachine/asanamate/internal/config"
)

func TestLipStyleMapsEveryField(t *testing.T) {
	yes := true
	s := lipStyle(config.Style{FG: "4", BG: "#000000", Bold: &yes, Italic: &yes, Underline: &yes, Faint: &yes, Reverse: &yes})
	if s.GetForeground() != lipgloss.Color("4") || s.GetBackground() != lipgloss.Color("#000000") ||
		!s.GetBold() || !s.GetItalic() || !s.GetUnderline() || !s.GetFaint() || !s.GetReverse() {
		t.Fatalf("style = %v", s)
	}
}

func TestPillReversesOnlyWithoutBackground(t *testing.T) {
	if !pillStyle(config.Style{FG: "6"}).GetReverse() {
		t.Fatal("a pill without bg must be reversed")
	}
	if pillStyle(config.Style{FG: "0", BG: "6"}).GetReverse() {
		t.Fatal("a pill with bg must not be reversed")
	}
	if !pillStyle(config.Style{FG: "6"}).GetBold() {
		t.Fatal("pills are bold unless the style says otherwise")
	}
}

func TestNewAppliesConfiguredColors(t *testing.T) {
	cfg := config.Config{Colors: config.Colors{Accent: config.Style{FG: "#7aa2f7"}, OK: config.Style{FG: "10"}, Mode: config.ModeColors{Read: config.Style{FG: "13"}}}}
	m, _ := testModel(t, cfg)
	m.loading, m.focusReader = false, true
	if m.accentStyle.GetForeground() != lipgloss.Color("#7aa2f7") || !m.accentStyle.GetBold() {
		t.Fatalf("accent = %v", m.accentStyle)
	}
	if m.markerStyle.GetForeground() != lipgloss.Color("#7aa2f7") || m.headerStyle.GetForeground() != lipgloss.Color("#7aa2f7") {
		t.Fatal("marker and header must fall back to accent")
	}
	if okStyle.GetForeground() != lipgloss.Color("10") {
		t.Fatalf("ok = %v", okStyle)
	}
	if got := stateStyles[agents.Completed].GetForeground(); got != lipgloss.Color("10") {
		t.Fatalf("completed agent = %v, want the ok color", got)
	}
	if m.pinnedStyle.GetForeground() != lipgloss.Color("208") {
		t.Fatalf("pinned = %v, want the dark theme's", m.pinnedStyle)
	}
	if _, pill, _ := m.mode(); pill.GetForeground() != lipgloss.Color("13") {
		t.Fatalf("read pill = %v", pill)
	}
}

func TestAdditionalColorsReachTicketSurfaces(t *testing.T) {
	cfg := config.Default()
	cfg.BranchField = "Branch Name"
	cfg.Colors.Branch = config.Style{FG: "#112233"}
	cfg.Colors.TitleTags = []config.Style{{FG: "#445566"}}
	cfg.Colors.Asana.Red = config.Style{FG: "#778899"}
	m, _ := testModel(t, cfg)
	defer applyPalette(config.Config{}.Palette(true))
	project := asana.Ref{GID: "p", Name: "Client", Color: "dark-red"}
	task := asana.Task{GID: "1", Name: "[CLIENT] Work", Memberships: []asana.Membership{{Project: project, Section: &asana.Ref{Name: "QA"}}}, CustomFields: []asana.CustomField{{GID: "b", Name: "Branch Name", DisplayValue: strp("feature/color")}}}
	markerStyle, ok := asanaStyle("dark-red")
	if !ok || markerStyle.GetForeground() != lipgloss.Color("#778899") {
		t.Fatal("project hue override lost")
	}
	for _, symbols := range []symbolSet{unicodeSymbols, asciiSymbols} {
		m.sym = symbols
		want := markerStyle.Render(symbols.icon(iconProjectColor)) + "Client"
		if got := m.projectLabel(project, lipgloss.NewStyle()); got != want {
			t.Fatalf("project label = %q, want %q", got, want)
		}
	}
	m.sym = unicodeSymbols
	if got := m.projectLabel(asana.Ref{Name: "Plain", Color: "none"}, lipgloss.NewStyle()); got != "Plain" {
		t.Fatalf("uncolored project = %q", got)
	}
	m.projects = []asana.Project{{GID: "p", Name: "Client", Color: "dark-red"}}
	m.deps.State.RecentProjects = []string{"p"}
	if got := m.navView(40, 30); !strings.Contains(got, markerStyle.Render(m.sym.icon(iconProjectColor))) {
		t.Fatal("sidebar marker missing")
	}
	for _, column := range []state.Column{{Field: "Branch Name"}, {GID: "b"}} {
		value, style := columnValue(task, column, m.rowContext(), testToday)
		if value != "feature/color" || style.GetForeground() != lipgloss.Color("#112233") {
			t.Fatalf("branch column = %q %v", value, style)
		}
	}
	m.visible = []asana.Task{task}
	m.selectionStyle = config.StyleMarker
	row, _ := m.listRow(0, make([]int, len(m.listColumns())))
	tag := tagStyles[0].Render("[CLIENT]")
	if !strings.Contains(row[0], tag) || !strings.Contains(m.summaryLine(task, 200), branchStyle.Render(m.sym.icon(iconBranch)+"feature/color")) {
		t.Fatal("list tag or header branch missing")
	}
	if got := tagTitle("[client] Work", lipgloss.NewStyle()); !strings.Contains(got, tagStyles[0].Render("[client]")) {
		t.Fatal("tag color not stable")
	}
	if got := tagTitle("Work [CLIENT]", lipgloss.NewStyle()); got != "Work [CLIENT]" {
		t.Fatal("nonleading tag was colored")
	}
	m.fieldLines = map[string]int{}
	body := m.detailsBody(ticket.Ticket{Task: task}, 100, 0)
	if !strings.Contains(ansi.Strip(body), "Client / QA") || !strings.Contains(body, branchStyle.Render("feature/color")) || !strings.Contains(body, markerStyle.Render(m.sym.icon(iconProjectColor))) {
		t.Fatal("Details colors or section text lost")
	}
	m.filterInput.SetValue("is:open")
	m.loading = false
	if !strings.Contains(strings.Join(m.listSegs(), "|"), warnStyle.Render(m.sym.icon(iconFilter)+"is:open")) {
		t.Fatal("applied filter not tinted")
	}
}
