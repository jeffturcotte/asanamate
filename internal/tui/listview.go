package tui

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/sadmachine/asanamate/internal/agents"
	"github.com/sadmachine/asanamate/internal/asana"
	"github.com/sadmachine/asanamate/internal/config"
	"github.com/sadmachine/asanamate/internal/ticket"
)

// listState is the ticket list: its tasks, the visible rows and their
// layout, the cursor, and the filter being edited.
type listState struct {
	tasks       []asana.Task
	pinExtras   []asana.Task // pinned tickets absent from the normal list
	visible     []asana.Task
	groups      []string    // group label per visible task; nil when ungrouped
	pinnedCount int         // leading visible rows in the Pinned section
	viewingRow  bool        // visible[pinnedCount] is the temporary ticket, under Viewing
	sortBy      config.Sort // ordering within groups; empty By keeps Asana order
	groupBy     string      // list field the list is grouped by; "" for none
	listW       int         // fitted list pane width; 0 for the default split
	cursor      int
	cols        []int // width of each list field column; 0 when empty
	badgeW      int   // width of the widest agent badge
	titleW      int   // width of the widest ticket title
	filterInput textinput.Model
	// filterBefore is the filter when editing started; the filter cancel key restores it.
	filterBefore string
	filtering    bool
	separator    bool        // lines frame each ticket; starts at list.separator
	spacing      bool        // blank lines around group headers; starts at list.header.spacing
	viewing      *asana.Task // edited ticket kept listed after a reload drops it, until the selection moves
}

// listTitle is the list panel's title and its right-hand count, or a
// loading note while tasks load.
func (m *Model) listTitle() (title, count string) {
	count = fmt.Sprintf("%d/%d", len(m.visible), len(m.tasks))
	if m.loading {
		count = "loading"
		if m.sym.spinner != nil {
			count = m.sym.spinner[m.frame%len(m.sym.spinner)] + " " + count
		}
	}
	title = "[1] Tickets · " + m.viewName()
	if len(m.deps.State.SavedViews) > 0 {
		title += " · " + m.savedViewLabel()
	}
	return title, count
}

// skeletonWidths are the placeholder rows' title widths, in percent.
var skeletonWidths = []int{70, 55, 82, 64, 48, 76, 60, 88, 52, 68, 58, 74}

// skeleton fills the list with placeholder rows while the first tasks load;
// a lighter band runs down them with the spinner.
func (m *Model) skeleton(width, height int) string {
	bar, lit := lipgloss.NewStyle().Foreground(borderColor), dimStyle
	colW := min(12, width/4)
	lines := make([]string, min(height, len(skeletonWidths)))
	for i := range lines {
		style := bar
		if m.sym.spinner != nil && i == m.frame%len(lines) {
			style = lit
		}
		titleW := max((width-colW-6)*skeletonWidths[i]/100, 1)
		row := "▆ " + strings.Repeat("▆", titleW)
		lines[i] = style.Render(row + strings.Repeat(" ", max(width-ansi.StringWidth(row)-colW, 1)) + strings.Repeat("▆", max(colW-2, 0)))
	}
	return strings.Join(lines, "\n")
}

// selectionBar reports whether the selected ticket is drawn as a reversed bar
// (the list has focus and selection.style is bar).
func (m *Model) selectionBar() bool {
	return m.deps.Config.List.Selection.Style == config.StyleBar && !m.focusReader
}

// renderRowLine fits line into width with tail right-aligned. A selected line
// is bold, or with selectionBar highlighted across the row up to the tail,
// which styles itself.
func (m *Model) renderRowLine(line, tail string, width int, selected bool) string {
	tailW := ansi.StringWidth(tail)
	if tail != "" && width-tailW-1 < 8 {
		tail, tailW = "", 0
	}
	room := width
	if tail != "" {
		room = width - tailW - 1
	}
	line = ansi.Truncate(line, room, "…")
	switch {
	case selected && m.selectionBar():
		return selectedStyle.Render(ansi.Strip(line)+strings.Repeat(" ", max(width-tailW-ansi.StringWidth(line), 0))) + tail
	case selected:
		line = titleStyle.Render(ansi.Strip(line))
	}
	if tail == "" {
		return line
	}
	return line + strings.Repeat(" ", max(width-ansi.StringWidth(line)-tailW, 1)) + tail
}

func (m *Model) listView(width, height int) string {
	switch {
	case m.loading && m.tasks == nil:
		return m.skeleton(width, height)
	case len(m.visible) == 0:
		return m.emptyView(width, height)
	}
	itemH, sepH := 1, 0
	if m.deps.Config.List.Layout == config.LayoutMulti {
		itemH = 2
	}
	if m.separator {
		sepH = 1
	}
	// Separators frame every item and neighbours share one; a group header
	// takes the place of the separator above its item. The first row shown
	// always has its group header. With spacing, headers get a blank line
	// below, and above too unless they top the view.
	spacing := 0
	if m.spacing {
		spacing = 1
	}
	header := func(i, start int) bool {
		return m.groups != nil && (i == start || m.groups[i] != m.groups[i-1] || i == m.pinnedCount || m.viewingRow && i == m.pinnedCount+1)
	}
	above := func(i, start int) int {
		if i == start {
			return 0
		}
		return spacing
	}
	rowH := func(i, start int) int {
		if header(i, start) {
			return above(i, start) + itemH + 1 + spacing
		}
		return itemH + sepH
	}
	// Scroll so the cursor's row is the last that fits, then fill below it.
	start, used := m.cursor, sepH+rowH(m.cursor, m.cursor)
	for start > 0 {
		next := used - rowH(start, start) + rowH(start, start-1) + rowH(start-1, start-1)
		if next > height {
			break
		}
		start, used = start-1, next
	}
	end := m.cursor + 1
	for end < len(m.visible) && used+rowH(end, start) <= height {
		used += rowH(end, start)
		end++
	}
	counts := m.groupCounts()
	sep := dimStyle.Render(strings.Repeat("─", width))
	// The marker style keeps a gutter on every row so text doesn't shift.
	var gutter string
	cursor := m.marker()
	if cursor != "" {
		gutter = strings.Repeat(" ", ansi.StringWidth(cursor))
		cursorStyle := m.markerStyle
		if m.focusReader {
			cursorStyle = dimStyle
		}
		cursor = cursorStyle.Render(cursor)
	}
	rowW := max(width-ansi.StringWidth(gutter)-listGutter, 1)
	cols := m.rowCols(rowW)
	var lines []string
	for i := start; i < end; i++ {
		switch {
		case header(i, start):
			for range above(i, start) {
				lines = append(lines, "")
			}
			if label, n := m.retainedSection(i); label != "" {
				style := m.viewingStyle
				if i < m.pinnedCount {
					style = m.pinnedStyle
				}
				lines = append(lines, m.groupHeader(style, label, n, width))
			} else {
				lines = append(lines, m.groupHeader(m.headerStyle, m.groups[i], counts[m.groups[i]], width))
			}
			for range spacing {
				lines = append(lines, "")
			}
		case sepH > 0:
			lines = append(lines, sep)
		}
		row, rowTail := m.listRow(i, cols)
		for j, line := range row {
			tail := ""
			if j == 0 {
				tail = rowTail
			}
			lead := gutter
			if i == m.cursor {
				lead = cursor
			}
			lines = append(lines, lead+m.renderRowLine(line, tail, rowW, i == m.cursor))
		}
	}
	if sepH > 0 {
		lines = append(lines, sep)
	}
	return strings.Join(lines, "\n")
}

// emptyView centers a note on why the list is empty, with the keys that
// change what it shows.
func (m *Model) emptyView(width, height int) string {
	title, why := "Nothing here.", "No tickets in "+m.viewName()
	if len(m.tasks) > 0 {
		title, why = "All clear.", "Nothing matches "+warnStyle.Render(m.filterInput.Value())
	}
	var hints []string
	for _, h := range [][2]string{{"filter", "filter"}, {"projects", "projects"}, {"reload", "reload"}} {
		if k := keyLabel("main", h[0]); k != "" {
			hints = append(hints, m.accentStyle.Render(k)+" "+dimStyle.Render(h[1]))
		}
	}
	block := lipgloss.JoinVertical(lipgloss.Center,
		okStyle.Bold(true).Render(m.sym.done), "", titleStyle.Render(title), dimStyle.Render(why), "",
		strings.Join(hints, "   "))
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, block)
}

// groupCounts returns the number of visible tickets per group label, leaving
// out retained rows so a group sharing their label keeps its own count.
func (m *Model) groupCounts() map[string]int {
	counts := map[string]int{}
	for i, g := range m.groups {
		if label, _ := m.retainedSection(i); label == "" {
			counts[g]++
		}
	}
	return counts
}

// marker is the selected row's left marker with its gap, or "" unless
// selection.style is marker.
func (m *Model) marker() string {
	if m.deps.Config.List.Selection.Style != config.StyleMarker {
		return ""
	}
	return m.sym.cursor + " "
}

// listRow returns the lines of visible ticket i's row and the tail
// right-aligned on its first line: the single layout's field columns sized by
// cols and agent badge, or the multi layout's badge.
func (m *Model) listRow(i int, cols []int) (row []string, tail string) {
	t := m.visible[i]
	mark := m.sym.open
	if t.Completed {
		mark = m.sym.done
	}
	name := ticket.OneLine(t.Name)
	linked := m.viewAgents(t)
	bar := i == m.cursor && m.selectionBar()
	badge := m.sym.badge(linked, m.frame, bar)
	switch {
	case len(linked) > 0 && linked[0].State == agents.Waiting && i != m.cursor:
		name = stateStyles[agents.Waiting].Render(name)
	case t.Completed:
		name = dimStyle.Strikethrough(true).Render(name)
	}
	title := dimStyle.Render(mark) + "  " + name
	rc, today := m.rowContext(), m.now()
	if m.deps.Config.List.Layout == config.LayoutMulti {
		var parts []string
		for _, column := range m.listColumns() {
			v, style := columnValue(t, column, rc, today)
			if v == "" {
				continue
			}
			if n := strings.TrimSpace(column.Field); column.GID != "" || (!isDue(n) && !strings.EqualFold(n, initialsField)) {
				style = dimStyle
			}
			parts = append(parts, style.Render(v))
		}
		details := ""
		if len(parts) > 0 {
			details = "   " + strings.Join(parts, dimStyle.Render(" · "))
		}
		return []string{title, details}, badge
	}
	// A highlighted bar runs through the columns, which then lose their colors.
	pad := func(s string, w int) string {
		s += strings.Repeat(" ", max(w-ansi.StringWidth(s), 0))
		if bar && s != "" {
			return selectedStyle.Render(ansi.Strip(s))
		}
		return s
	}
	var cells []string
	for j, column := range m.listColumns() {
		if cols[j] == 0 {
			continue
		}
		v, style := columnValue(t, column, rc, today)
		v = ansi.Truncate(v, cols[j], "…")
		cells = append(cells, pad(style.Render(v), cols[j]))
	}
	if m.badgeW > 0 {
		cells = append(cells, badge+pad("", m.badgeW-ansi.StringWidth(badge)))
	}
	return []string{title}, strings.Join(cells, pad("", 2))
}

// rowCols returns the field column widths for rows width wide, hiding columns
// until titles get minTitleW, or the widest title if shorter: other fields
// from the right, then due. The agent badge always stays.
func (m *Model) rowCols(width int) []int {
	cols := slices.Clone(m.cols)
	columns := m.listColumns()
	// The title line also holds the check mark and the two spaces after it.
	room := min(m.titleW, minTitleW) + ansi.StringWidth(m.sym.open) + 2
	var order, due []int
	for j := len(cols) - 1; j >= 0; j-- {
		if columns[j].GID == "" && isDue(strings.TrimSpace(columns[j].Field)) {
			due = append(due, j)
		} else {
			order = append(order, j)
		}
	}
	for _, j := range append(order, due...) {
		if width-tailWidth(cols, m.badgeW)-1 >= room {
			break
		}
		cols[j] = 0
	}
	return cols
}

// tailWidth is the width of a row tail with these field columns and badge.
func tailWidth(cols []int, badgeW int) int {
	w, n := badgeW, min(badgeW, 1)
	for _, c := range cols {
		if c > 0 {
			w, n = w+c, n+1
		}
	}
	return w + 2*max(n-1, 0)
}

// measureColumns sizes each list field column to its widest visible value,
// up to maxColW, the badge column to the widest agent badge, and titleW to
// the widest title.
func (m *Model) measureColumns() {
	columns := m.listColumns()
	m.cols, m.badgeW, m.titleW = make([]int, len(columns)), 0, 0
	rc, today := m.rowContext(), m.now()
	for _, t := range m.visible {
		m.titleW = max(m.titleW, ansi.StringWidth(ticket.OneLine(t.Name)))
		for j, column := range columns {
			v, _ := columnValue(t, column, rc, today)
			m.cols[j] = min(max(m.cols[j], ansi.StringWidth(v)), maxColW)
		}
		m.badgeW = max(m.badgeW, ansi.StringWidth(m.sym.badge(m.viewAgents(t), 0, false)))
	}
}

// fitList sizes the list pane to its widest row or group header. paneWidths
// caps it to leave the reader readerMinW columns. A fixed layout.list_width
// leaves it alone.
func (m *Model) fitList() {
	if m.fixedListW() > 0 {
		return
	}
	gutter := ansi.StringWidth(m.marker())
	w := 0
	if m.viewingRow {
		w = groupHeaderWidth(viewingLabel, 1)
	}
	if m.pinnedCount > 0 {
		w = max(w, groupHeaderWidth(pinnedLabel, m.pinnedCount))
	}
	for label, n := range m.groupCounts() {
		w = max(w, groupHeaderWidth(label, n))
	}
	for i := range m.visible {
		row, tail := m.listRow(i, m.cols)
		for j, line := range row {
			lineW := gutter + ansi.StringWidth(line)
			if j == 0 && tail != "" {
				lineW += 2 + ansi.StringWidth(tail)
			}
			w = max(w, lineW)
		}
	}
	// The title needs its corners, the spaces around it and the count, and
	// one rule cell between them.
	title, count := m.listTitle()
	m.listW = max(w+listGutter+panelFrame, ansi.StringWidth(title)+ansi.StringWidth(count)+8)
	m.layout()
}
