// Package tui is the asanamate terminal interface.
package tui

import (
	"cmp"
	"image/color"
	"maps"
	"math/rand/v2"
	"os/exec"
	"slices"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"github.com/sadmachine/asanamate/internal/agents"
	"github.com/sadmachine/asanamate/internal/asana"
	"github.com/sadmachine/asanamate/internal/config"
	"github.com/sadmachine/asanamate/internal/filter"
	"github.com/sadmachine/asanamate/internal/keymap"
	"github.com/sadmachine/asanamate/internal/kitty"
	"github.com/sadmachine/asanamate/internal/state"
	"github.com/sadmachine/asanamate/internal/ticket"
)

const narrowWidth = 100

// maxColW caps a list field column's width; longer values are cut.
const maxColW = 24

// minPaneW is the narrowest a fitted list or its reader gets.
const minPaneW = 30

// minTitleW is the title room a list row keeps before its field columns hide.
const minTitleW = 30

// The reader aims for readerIdealW columns in the default split. A fitted
// list grows into the reader down to readerMinW, then truncates its titles.
const (
	readerIdealW = 120
	readerMinW   = 80
)

// panelFrame is the cells a panel's border takes across and down.
const panelFrame = 2

// listGutter is the blank columns kept right of the list's rows; group
// headers and separators still run the full width.
const listGutter = 1

// Deps are the services the TUI uses.
type Deps struct {
	Config     config.Config
	ConfigPath string // configuration path; empty uses config.Path
	State      *state.State
	Client     *asana.Client
	StateDir   string
	// CacheDir holds the Activity cache; empty keeps it in memory.
	CacheDir string
	Images   bool
	InTmux   bool
	// Symbols is the resolved symbol set: unicode, nerd, or ascii.
	Symbols string
	// ReducedMotion shows static symbols instead of the working spinner.
	ReducedMotion bool
	// NoPreview starts with the reading pane hidden, overriding layout.reader.
	NoPreview bool
	// ExitOnAction runs background actions as exit actions, so asanamate
	// quits first. It suits popups, which close when asanamate exits.
	ExitOnAction bool
	// Palette is the resolved theme. Zero resolves Config's theme for a dark
	// background.
	Palette config.Palette
}

// Model is the Bubble Tea model for asanamate.
type Model struct {
	deps          Deps
	width, height int

	listState
	readerState

	viewProject     *asana.Ref
	activity        *activityView // API activity, cached on disk
	activityOpen    bool
	activityTicket  bool             // Esc returns from a ticket opened through Activity
	palette         config.Palette   // resolved theme, for Markdown
	bindings        []binding        // [keys.main] with their handlers
	accentStyle     lipgloss.Style   // reader headings
	headerStyle     lipgloss.Style   // group headers
	pinnedStyle     lipgloss.Style   // Pinned header
	viewingStyle    lipgloss.Style   // Viewing header
	markerStyle     lipgloss.Style   // selected ticket marker
	now             func() time.Time // today, for due dates
	loading         bool             // tasks are loading; the stale view stays frozen under a modal
	background      bool             // the timer started the loading reload, so it shows no modal
	waiting         string           // loading modal text while a picker's or form's choices load
	refreshInterval time.Duration    // auto-update interval, from saved settings or config
	refreshSeq      uint64           // invalidates timers from earlier session intervals

	focusReader  bool
	focusNav     bool   // the views panel has focus; wide layout only
	navCursor    int    // selected row among the views panel's selectable rows
	help         bool   // the key help is open
	savedView    string // named view associated with the current project's settings
	readerHidden bool   // the reader is hidden for this session
	navHidden    bool   // the views panel is hidden for this session

	projects        []asana.Project
	projectsLoaded  bool                       // projects came from Asana this session, not the state cache
	loadingProjects bool                       // projects are loading
	afterProjects   func() tea.Cmd             // runs once projects load
	linkNames       map[string]string          // names of linked projects outside projects
	projectFields   map[string]map[string]bool // project gid -> its custom field gids
	projectCatalogs map[string][]asana.Ref     // project gid -> its custom field names and gids
	users           []asana.Ref                // workspace users, loaded on first assign

	agents    []agents.Agent
	linked    map[string][]agents.Agent // viewAgents by ticket gid; nil when stale
	agentsErr string
	sym       symbolSet
	frame     int  // spinner frame
	spinning  bool // a spinner tick is scheduled

	// Modals, and the actions, edits, and time entries they collect.
	modal          *picker
	input          *inputBox // free-text modal for input actions
	builder        *actionBuilder
	form           *formModal // shared action and time-entry controls
	notices        []string   // messages that block input until dismissed
	run            *pendingRun
	edit           *pendingEdit
	timeEntry      *pendingTime
	timeSummaries  map[string]timeSummaryState
	timeSummarySeq uint64
	lastAction     int            // index of the last picked action; -1 for none
	menuFor        string         // gid whose menu opens once its details arrive
	menuOpen       func() tea.Cmd // opens that menu
	openGID        string         // history ticket to show once its details arrive

	status  string
	exitCmd *exec.Cmd
}

// New returns a model that starts on My Tasks with its last used grouping, sort, and filter.
func New(d Deps) *Model {
	in := textinput.New()
	in.Prompt = "/"
	keys, typeFirst = d.Config.Keymap, d.Config.Picker.TypeFirst
	if keys == nil {
		keys = keymap.Default()
	}
	p := d.Palette
	if p.Base == "" {
		p = d.Config.Palette(true)
	}
	applyPalette(p)
	c := p.Colors
	m := &Model{
		deps: d,
		listState: listState{
			filterInput:    in,
			separator:      *cmp.Or(d.State.Display.Separator, &d.Config.List.Separator),
			spacing:        *cmp.Or(d.State.Display.HeaderSpacing, &d.Config.List.Header.Spacing),
			selectionStyle: d.Config.List.Selection.Style,
		},
		readerState: readerState{
			reader:     viewport.New(),
			readerView: cmp.Or(d.State.Display.ReaderView, d.Config.Reader.View),
			renderers:  map[rendererKey]*markdownRenderer{},
			details:    map[string]ticket.Ticket{},
			images:     map[string]*inlineImage{},
			imageSeq:   rand.IntN(imageIDs),
		},
		lastAction:      -1,
		palette:         p,
		accentStyle:     lipStyle(c.Accent),
		headerStyle:     lipStyle(c.Header),
		pinnedStyle:     lipStyle(c.Pinned),
		viewingStyle:    lipStyle(c.Viewing),
		markerStyle:     lipStyle(c.Selection),
		projectFields:   map[string]map[string]bool{},
		projectCatalogs: map[string][]asana.Ref{},
		sym:             newSymbols(d.Symbols, d.Config.Agents.Symbols, d.ReducedMotion),
		loading:         true,
		now:             time.Now,
		readerHidden:    d.NoPreview || !d.Config.Layout.Reader,
	}
	if saved := d.State.Display.SelectionStyle; saved == config.StyleMarker || saved == config.StyleBar {
		m.selectionStyle = saved
	}
	for _, p := range d.State.Projects {
		m.projects = append(m.projects, asana.Project{GID: p.GID, Name: p.Name})
	}
	m.bindings = keyBindings()
	m.activity = newActivityView(d)
	m.restoreView()
	m.refreshInterval, _ = config.ParseRefreshInterval(cmp.Or(d.Config.List.RefreshInterval, config.Default().List.RefreshInterval))
	if saved, err := config.ParseRefreshInterval(d.State.Display.RefreshInterval); err == nil {
		m.refreshInterval = saved
	}
	return m
}

// ExitCommand is the command an exit-mode action left to run after the TUI quits.
func (m *Model) ExitCommand() *exec.Cmd { return m.exitCmd }

func (m *Model) Init() tea.Cmd {
	cmd := tea.Batch(loadTasks(m.deps.Client, m.deps.Config.Workspace, m.viewProject, m.pinnedTasks()), m.startSpinner(), m.scheduleRefresh(), m.startActivity())
	if m.deps.Config.AgentsEnabled() {
		cmd = tea.Batch(cmd, loadAgents(m.deps.Config.Agents))
	}
	return cmd
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case refreshTickMsg:
		return m, m.autoRefresh(msg)
	case activityMsg:
		return m, m.gotActivity(msg)
	case activityTickMsg:
		return m, m.activityTick()
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		// The views panel lists recent projects by name.
		if m.showNav() {
			return m, tea.Batch(m.refreshProjects(), m.requestImageCellSize())
		}
		return m, m.requestImageCellSize()
	case uv.CellSizeEvent:
		return m, m.imageCellSizeChanged(kitty.CellSize{Width: msg.Width, Height: msg.Height})
	case tasksMsg:
		return m, m.gotTasks(msg)
	case detailTickMsg:
		if t, ok := m.selected(); ok && t.GID == msg.gid {
			if _, cached := m.details[msg.gid]; !cached {
				return m, m.loadDetail(msg.gid)
			}
		}
	case detailMsg:
		return m, m.gotDetail(msg)
	case projectsMsg:
		return m, m.gotProjects(msg)
	case linkNamesMsg:
		m.status = ""
		if m.linkNames == nil {
			m.linkNames = map[string]string{}
		}
		maps.Copy(m.linkNames, msg.names)
		return m, m.openLinks()
	case agentsMsg:
		return m, m.gotAgents(msg)
	case spinnerTickMsg:
		if !m.animating() {
			m.spinning = false
			return m, nil
		}
		m.frame++
		return m, scheduleSpinner()
	case agentTickMsg:
		return m, loadAgents(m.deps.Config.Agents)
	case projectFieldsMsg:
		return m, m.gotProjectFields(msg)
	case candidatesMsg:
		if m.waiting != "" {
			m.waiting = ""
			m.openRepoPicker(msg)
		}
	case actionDoneMsg:
		status := actionStatus(msg)
		if msg.err != nil {
			m.showNotice(status)
		} else {
			m.status = status
		}
	case editorDoneMsg:
		msg.apply(msg.text, msg.err)
		return m, nil
	case timeFormMsg:
		m.gotTimeForm(msg)
	case timeDoneMsg:
		return m, m.finishTime(msg)
	case timeSummaryMsg:
		m.gotTimeSummary(msg)
	case sectionsMsg:
		m.openSectionPicker(msg)
	case usersMsg:
		if msg.err == nil {
			m.users = msg.users
		}
		m.openUserPicker(msg.err)
	case editDoneMsg:
		return m, m.editDone(msg)
	case createdMsg:
		return m, m.created(msg)
	case inlineImageMsg:
		return m, m.inlineImageLoaded(msg)
	case imageMsg:
		if msg.err != nil {
			m.status = "image: " + msg.err.Error() + "; opening in browser"
			return m, openURL(msg.url)
		}
		m.status = ""
		viewer := kitty.NewViewer(msg.payload, m.viewerFooter(), len(m.viewImages) > 1, m.deps.InTmux)
		return m, tea.Exec(viewer, func(err error) tea.Msg { return viewerDoneMsg{step: viewer.Step, err: err} })
	case viewerDoneMsg:
		if msg.err != nil || msg.step == 0 {
			m.status = actionStatus(actionDoneMsg{name: "image viewer", err: msg.err})
			return m, nil
		}
		return m, m.viewImage((m.viewIndex + msg.step + len(m.viewImages)) % len(m.viewImages))
	case statusMsg:
		m.status = string(msg)
	case tea.KeyPressMsg:
		return m, m.handleKey(msg)
	}
	// Editors also receive paste events and asynchronous clipboard results.
	if m.activityOpen && m.activity.filtering {
		return m, m.updateActivitySearch(msg)
	}
	if m.builder != nil {
		return m, m.updateActionBuilder(msg)
	}
	if m.input != nil {
		return m, m.updateInput(msg)
	}
	return m, nil
}

// gotTasks applies a task list load for the viewed project.
func (m *Model) gotTasks(msg tasksMsg) tea.Cmd {
	if !sameProject(msg.project, m.viewProject) {
		return nil
	}
	refreshing := m.loading
	m.loading = false
	if msg.err != nil {
		m.status = "loading tasks: " + msg.err.Error()
		return nil
	}
	if msg.catalog != nil && msg.project != nil {
		m.cacheProjectFields(msg.project.GID, msg.catalog)
	}
	prev, _ := m.selected()
	m.tasks, m.pinExtras, m.linked = msg.tasks, msg.pins, nil
	m.syncChanged(msg)
	m.refreshViewing(msg.tasks...)
	m.refreshViewing(msg.pins...)
	m.applyFilter()
	// A reload that drops the selected ticket starts from the top.
	if t, _ := m.selected(); t.GID != prev.GID {
		m.cursor = 0
	}
	m.fitList()
	if msg.warning != "" {
		m.status = msg.warning
	}
	cmd := m.selectionChanged()
	if t, ok := m.selected(); refreshing && ok && t.GID == m.shownGID {
		return tea.Batch(cmd, m.loadDetail(t.GID))
	}
	return cmd
}

// gotDetail stores a ticket's details, then shows the ticket or opens the
// menu that waited for them.
func (m *Model) gotDetail(msg detailMsg) tea.Cmd {
	if msg.seq != 0 && m.detailRequests[msg.gid] != msg.seq {
		return nil
	}
	delete(m.detailRequests, msg.gid)
	t, ok := m.selected()
	isSelected := ok && t.GID == msg.gid
	if msg.err != nil {
		if isSelected || m.openGID == msg.gid {
			m.status = "loading ticket: " + msg.err.Error()
		}
		if m.openGID == msg.gid {
			m.openGID = ""
		}
		if m.menuFor == msg.gid {
			m.menuFor = ""
		}
		return nil
	}
	m.details[msg.gid], m.linked = msg.ticket, nil
	if m.refreshRetained(msg.ticket.Task) {
		m.applyFilter()
	}
	images := tea.Batch(m.loadInlineImages(msg.ticket), m.loadTimeSummary(msg.ticket.Task))
	if m.openGID == msg.gid {
		m.openGID, m.status = "", ""
		return tea.Batch(images, m.showTicket(msg.ticket.Task))
	}
	if isSelected {
		m.renderDetail(m.shownGID == msg.gid)
		if m.menuFor == msg.gid {
			m.menuFor = ""
			return tea.Batch(images, m.menuOpen())
		}
	}
	return images
}

// gotProjects stores the member projects, caches them in state, and runs
// whatever waited for them.
func (m *Model) gotProjects(msg projectsMsg) tea.Cmd {
	m.loadingProjects = false
	if msg.err != nil {
		m.status = "loading projects: " + msg.err.Error()
		return nil
	}
	m.status = ""
	m.projects, m.projectsLoaded = msg.projects, true
	m.deps.State.Projects = make([]state.Project, len(msg.projects))
	for i, p := range msg.projects {
		m.deps.State.Projects[i] = state.Project{GID: p.GID, Name: p.Name}
	}
	if err := m.deps.State.Save(); err != nil {
		m.status = "saving state: " + err.Error()
	}
	if open := m.afterProjects; open != nil {
		m.afterProjects = nil
		return open()
	}
	return nil
}

// gotAgents applies a scan of running agents and schedules the next.
func (m *Model) gotAgents(msg agentsMsg) tea.Cmd {
	if !m.deps.Config.AgentsEnabled() {
		return nil
	}
	if msg.err != nil {
		if s := "agents: " + msg.err.Error(); s != m.agentsErr {
			m.status, m.agentsErr = s, s
		}
	} else {
		m.agents, m.agentsErr, m.linked = msg.list, "", nil
		badgeW := m.badgeW
		m.applyFilter()
		// Badges widen the rows; refit so titles keep their room.
		if m.badgeW != badgeW {
			m.fitList()
		}
		if t, ok := m.selectedDetail(); ok && t.GID == m.shownGID && m.agentsSection(t.Task) != m.shownAgents {
			m.renderDetail(true)
		}
	}
	return tea.Batch(scheduleAgents(), m.startSpinner())
}

// gotProjectFields stores a project's custom fields for the column picker
// or a run waiting on them.
func (m *Model) gotProjectFields(msg projectFieldsMsg) tea.Cmd {
	if msg.columns && msg.gid != gidOf(m.viewProject) {
		return nil
	}
	if msg.err != nil {
		m.status = "loading project fields: " + msg.err.Error()
		if msg.columns {
			return nil
		}
	}
	m.projectFields[msg.gid], m.linked = msg.fields, nil
	if msg.catalog != nil {
		m.cacheProjectFields(msg.gid, msg.catalog)
	}
	if msg.columns && msg.gid == gidOf(m.viewProject) {
		m.status = ""
		m.openColumnPicker()
	}
	if m.run != nil && m.run.awaitingFields {
		m.run.awaitingFields = false
		return m.resumeRun()
	}
	return nil
}

// editDone reports a finished edit and reloads the list.
func (m *Model) editDone(msg editDoneMsg) tea.Cmd {
	if msg.err != nil {
		m.status = msg.what + " failed: " + msg.err.Error()
		return nil
	}
	m.status = msg.what + ": done"
	if msg.done != "" {
		m.status = msg.done
	}
	// Keep the edited ticket shown even if the reload drops it from the view.
	if t, ok := m.selected(); ok && t.GID == msg.gid {
		m.viewing = &t
	}
	if m.shownGID != msg.gid {
		delete(m.details, msg.gid)
	}
	return m.refresh()
}

func sameProject(a, b *asana.Ref) bool {
	return gidOf(a) == gidOf(b)
}

// gidOf returns the project's gid, or "" for My Tasks (nil).
func gidOf(project *asana.Ref) string {
	if project == nil {
		return ""
	}
	return project.GID
}

func (m *Model) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	k := msg.String()
	if k == "ctrl+c" {
		m.closeActivity()
		return tea.Quit
	}
	if len(m.notices) > 0 {
		if keys.Name("notice", k) == "dismiss" {
			m.notices = m.notices[1:]
		}
		return nil
	}
	if m.waiting != "" {
		switch {
		case keys.Name("main", k) == "quit":
			return tea.Quit
		case keys.Name("picker", k) == "cancel":
			m.waiting, m.run, m.timeEntry = "", nil, nil
		}
		return nil
	}
	if m.builder != nil {
		return m.updateActionBuilder(msg)
	}
	if m.input != nil {
		return m.updateInput(msg)
	}
	if m.form != nil {
		cancelled, cmd := m.form.update(msg)
		if cancelled {
			m.form, m.run, m.timeEntry = nil, nil, nil
		}
		return cmd
	}
	if m.modal != nil {
		return m.updateModal(msg)
	}
	if m.help {
		m.help = false
		return nil
	}
	if m.loading {
		if keys.Name("main", k) == "quit" {
			return tea.Quit
		}
		return nil
	}
	m.status = ""
	if m.activityOpen {
		return m.handleActivityKey(msg)
	}
	if k == "esc" && m.activityTicket && !m.filtering {
		m.activityTicket = false
		m.activityOpen = true
		m.focusList()
		m.layout()
		return nil
	}
	if m.filtering {
		return m.updateFilter(msg)
	}
	if m.focusNav {
		if name := keys.Name("views", k); name != "" {
			return m.updateNav(name)
		}
	}
	if b, ok := m.bindingFor(k); ok {
		return b.run(m, msg)
	}
	if m.focusReader {
		return m.scrollReader(msg)
	}
	return nil
}

func (m *Model) showNotice(message string) {
	m.notices = append(m.notices, message)
	m.status = message
}

func (m *Model) updateModal(msg tea.KeyPressMsg) tea.Cmd {
	p := m.modal
	res, cmd := p.update(msg)
	switch {
	case res.cancelled:
		m.modal, m.run, m.edit, m.timeEntry = nil, nil, nil, nil
	case res.done:
		return tea.Batch(cmd, p.onPick(res))
	}
	return cmd
}

func (m *Model) updateInput(msg tea.Msg) tea.Cmd {
	res, cmd := m.input.update(msg)
	switch {
	case res.cancelled:
		m.input, m.run, m.edit = nil, nil, nil
	case res.done:
		if m.input.onSubmit != nil {
			return tea.Batch(cmd, m.input.onSubmit(res.free))
		}
		return tea.Batch(cmd, m.typedEdit(res.free))
	}
	return cmd
}

func (m *Model) updateFilter(msg tea.KeyPressMsg) tea.Cmd {
	switch bound("filter", msg, true) {
	case "help":
		m.help = true
		return nil
	case "cancel":
		m.filterInput.SetValue(m.filterBefore)
		m.applyFilter()
		m.finishFilter()
		return m.selectionChanged()
	case "done":
		m.finishFilter()
		return nil
	}
	var cmd tea.Cmd
	m.filterInput, cmd = m.filterInput.Update(msg)
	m.applyFilter()
	return tea.Batch(cmd, m.selectionChanged())
}

// finishFilter leaves filter editing with the current filter.
func (m *Model) finishFilter() {
	m.filtering = false
	m.filterInput.Blur()
	m.reader.SetHeight(m.paneHeight())
	m.saveView()
	m.fitList()
}

// reload refetches the tasks, keeping the current view until they land.
func (m *Model) reload() tea.Cmd {
	// Retain the displayed ticket until its replacement arrives. Other cached
	// tickets must be fetched again when selected after a reload.
	for gid := range m.details {
		if gid != m.shownGID {
			delete(m.details, gid)
		}
	}
	m.loading, m.background = true, false
	return tea.Batch(loadTasks(m.deps.Client, m.deps.Config.Workspace, m.viewProject, m.pinnedTasks()), m.startSpinner())
}

// applyFilter recomputes the visible tasks and their groups, keeping the
// selected ticket selected when it is still visible.
func (m *Model) applyFilter() {
	var prevGID string
	if prev, ok := m.selected(); ok {
		prevGID = prev.GID
	}
	pins := m.pinnedTasks()
	var pinned []asana.Task
	for _, gid := range pins {
		for _, tasks := range [][]asana.Task{m.tasks, m.pinExtras} {
			if i := slices.IndexFunc(tasks, func(t asana.Task) bool { return t.GID == gid }); i >= 0 {
				pinned = append(pinned, tasks[i])
				break
			}
		}
	}
	visible := filter.Parse(m.filterInput.Value()).Apply(m.tasks, m.agentStates, m.now())
	visible = slices.DeleteFunc(visible, func(t asana.Task) bool { return slices.Contains(pins, t.GID) })
	m.visible, m.groups = groupTasks(visible, m.groupBy, m.rowContext(), m.now())
	sortTasks(m.visible, m.groups, m.sortBy, m.rowContext())
	m.pinnedCount, m.viewingRow = len(pinned), false
	var viewing []asana.Task
	if t := m.viewing; t != nil && !slices.Contains(pins, t.GID) && !slices.ContainsFunc(visible, func(v asana.Task) bool { return v.GID == t.GID }) {
		viewing = []asana.Task{*t}
		m.viewingRow = true
	}
	if len(pinned)+len(viewing) > 0 {
		if m.groups == nil {
			m.groups = slices.Repeat([]string{ungroupedLabel}, len(m.visible))
		}
		labels := slices.Repeat([]string{pinnedLabel}, len(pinned))
		if m.viewingRow {
			labels = append(labels, viewingLabel)
		}
		m.visible = append(append(pinned, viewing...), m.visible...)
		m.groups = append(labels, m.groups...)
	}
	m.measureColumns()
	if prevGID != "" {
		for i, t := range m.visible {
			if t.GID == prevGID {
				m.cursor = i
				break
			}
		}
	}
	m.moveTo(m.cursor)
}

// refreshViewing replaces the viewed ticket with its copy among tasks, if any,
// and reports whether it did.
func (m *Model) refreshViewing(tasks ...asana.Task) bool {
	if m.viewing == nil {
		return false
	}
	for _, t := range tasks {
		if t.GID == m.viewing.GID {
			m.viewing = &t
			return true
		}
	}
	return false
}

// animating reports whether anything shown uses the spinner.
func (m *Model) animating() bool {
	return m.loading || m.waiting != "" || m.anyWorking() || m.activityOpen && m.activity.loading
}

// wait shows text in a loading modal until the reply to cmd clears it;
// cancel closes the modal and drops the reply.
func (m *Model) wait(text string, cmd tea.Cmd) tea.Cmd {
	m.waiting = text
	return tea.Batch(cmd, m.startSpinner())
}

// startSpinner schedules a spinner tick unless one is scheduled or nothing animates.
func (m *Model) startSpinner() tea.Cmd {
	if m.spinning || m.sym.spinner == nil || !m.animating() {
		return nil
	}
	m.spinning = true
	return scheduleSpinner()
}

func (m *Model) anyWorking() bool {
	for _, a := range m.agents {
		if a.State == agents.Working {
			return true
		}
	}
	return false
}

func (m *Model) moveTo(i int) {
	m.cursor = max(min(i, len(m.visible)-1), 0)
}

func (m *Model) selected() (asana.Task, bool) {
	if m.cursor < len(m.visible) {
		return m.visible[m.cursor], true
	}
	return asana.Task{}, false
}

func (m *Model) selectedDetail() (ticket.Ticket, bool) {
	t, ok := m.selected()
	if !ok {
		return ticket.Ticket{}, false
	}
	d, ok := m.details[t.GID]
	return d, ok
}

func (m *Model) selectionChanged() tea.Cmd {
	t, ok := m.selected()
	if m.viewing != nil && (!ok || t.GID != m.viewing.GID) {
		// Moving off the viewed ticket drops it unless the view still lists it.
		m.viewing = nil
		m.applyFilter()
		t, ok = m.selected()
	}
	if !ok {
		m.shownGID = ""
		m.reader.SetContent("")
		return nil
	}
	if t.GID == m.shownGID {
		return nil
	}
	m.fieldKey, m.showEmpty, m.showOthers = "", false, false
	if _, cached := m.details[t.GID]; cached {
		m.renderDetail(false)
		images := m.loadInlineImages(m.details[t.GID])
		if _, cached := m.timeSummaries[t.GID]; !cached {
			return tea.Batch(images, m.loadTimeSummary(m.details[t.GID].Task))
		}
		return images
	}
	m.shownGID = ""
	m.reader.SetContent(dimStyle.Render("Loading " + ticket.Clean(t.Name) + "…"))
	return scheduleDetail(t.GID)
}

func (m *Model) openProjectPicker() {
	items := []pickItem{{Label: "My Tasks", Value: (*asana.Ref)(nil)}}
	byGID := map[string]asana.Project{}
	for _, p := range m.projects {
		byGID[p.GID] = p
	}
	seen := map[string]bool{}
	for _, gid := range m.deps.State.RecentProjects {
		if p, ok := byGID[gid]; ok {
			items = append(items, pickItem{Label: ticket.Clean(p.Name), Hint: "recent", Value: &asana.Ref{GID: p.GID, Name: p.Name}})
			seen[gid] = true
		}
	}
	rest := make([]asana.Project, 0, len(m.projects))
	for _, p := range m.projects {
		if !seen[p.GID] {
			rest = append(rest, p)
		}
	}
	sort.Slice(rest, func(i, j int) bool { return strings.ToLower(rest[i].Name) < strings.ToLower(rest[j].Name) })
	for _, p := range rest {
		items = append(items, pickItem{Label: ticket.Clean(p.Name), Value: &asana.Ref{GID: p.GID, Name: p.Name}})
	}
	m.modal = newPicker(pickValue(m.pickedProject), "Switch project", items)
}

func (m *Model) pickedProject(ref *asana.Ref) tea.Cmd {
	m.modal = nil
	m.closeActivity()
	m.viewProject, m.linked, m.viewing = ref, nil, nil
	m.restoreView()
	// The previous rows remain visible while loading the new project. Their
	// widths must already match the new project's column choices.
	m.measureColumns()
	if ref != nil {
		m.deps.State.TouchProject(ref.GID)
		if err := m.deps.State.Save(); err != nil {
			m.status = "saving state: " + err.Error()
		}
	}
	return m.reload()
}

// restoreView applies the viewed project's last used grouping, sort, and filter, else
// the configured group_by, sort, and default_filter.
func (m *Model) restoreView() {
	v, ok := m.deps.State.View(gidOf(m.viewProject))
	if !ok {
		v = state.View{GroupBy: strings.TrimSpace(m.deps.Config.List.GroupBy), Sort: m.deps.Config.List.Sort, Filter: m.deps.Config.DefaultFilter}
	}
	m.groupBy, m.sortBy = v.GroupBy, v.Sort
	m.filterInput.SetValue(v.Filter)
	m.savedView = m.deps.State.CurrentSavedViews[gidOf(m.viewProject)]
	if _, exists := m.deps.State.SavedViews[m.savedView]; !exists {
		m.savedView = ""
	}
}

// saveView remembers the viewed project's grouping, sort, and filter.
func (m *Model) saveView() {
	gid := gidOf(m.viewProject)
	m.deps.State.SetView(gid, m.currentView())
	if m.savedView == "" {
		delete(m.deps.State.CurrentSavedViews, gid)
	} else {
		m.deps.State.CurrentSavedViews[gid] = m.savedView
	}
	if err := m.deps.State.Save(); err != nil {
		m.status = "saving state: " + err.Error()
	}
}

// readerOn reports whether the reader is available: layout.reader sets its
// start, --hide-preview hides it initially, and toggle_reader flips it. On narrow
// terminals it opens full screen instead of beside the list.
func (m *Model) readerOn() bool {
	return !m.readerHidden
}

// readerShown reports whether the reader sits beside the list: it is on and
// the terminal leaves it room. Beside a fixed-width list and the views
// panel, that room is minPaneW columns; otherwise the terminal must be
// narrowWidth wide.
func (m *Model) readerShown() bool {
	if !m.readerOn() {
		return false
	}
	fixed := m.fixedListW()
	if fixed == 0 {
		return m.width >= narrowWidth
	}
	room := m.width - m.navWidth()
	return room-fixed >= minPaneW
}

// fixedListW is the list pane's width from layout.list_width, at least
// minPaneW, or 0 when the list sizes itself.
func (m *Model) fixedListW() int {
	if w := m.deps.Config.Layout.ListWidth; w > 0 {
		return max(w, minPaneW)
	}
	return 0
}

// paneWidths returns the content widths of the list and the reader. Split
// panes sit in panels, each panelFrame columns wider than its content.
func (m *Model) paneWidths() (listW, readerW int, split bool) {
	if !m.readerShown() {
		room := m.width - m.navWidth()
		if m.showNav() {
			return room, room - panelFrame, false
		}
		return room, room, false
	}
	room := m.width - m.navWidth()
	if fixed := m.fixedListW(); fixed > 0 {
		return fixed - panelFrame, room - fixed - panelFrame, true
	}
	outer := min(room*2/5, room-readerIdealW-panelFrame)
	if m.activityOpen {
		outer = room - readerMinW - panelFrame
	}
	if m.listW > 0 && !m.activityOpen {
		outer = min(m.listW, room-readerMinW-panelFrame)
	}
	outer = min(max(outer, minPaneW), room-minPaneW)
	return outer - panelFrame, room - outer - panelFrame, true
}

func (m *Model) bodyHeight() int { return max(m.height-1, 1) }

// paneHeight is the height of the panes' content.
func (m *Model) paneHeight() int {
	if _, _, split := m.paneWidths(); split || m.showNav() {
		return max(m.bodyHeight()-m.filterHintHeight()-panelFrame, 1)
	}
	return max(m.bodyHeight()-m.filterHintHeight(), 1)
}

func (m *Model) layout() {
	if !m.showNav() {
		m.focusNav = false
	}
	_, readerW, _ := m.paneWidths()
	widthChanged := readerW != m.reader.Width()
	if widthChanged {
		m.renderers = map[rendererKey]*markdownRenderer{}
	}
	m.reader.SetWidth(readerW)
	m.reader.SetHeight(m.paneHeight())
	m.filterInput.SetWidth(max(m.width/3, 10))
	if m.activityOpen {
		m.layoutActivity()
	}
	if t, ok := m.selected(); widthChanged && ok && t.GID == m.shownGID {
		m.renderDetail(true)
	}
}

func (m *Model) View() tea.View {
	v := tea.NewView(m.body() + "\n" + m.statusline())
	v.AltScreen = true
	return v
}

func (m *Model) body() string {
	h := m.bodyHeight()
	hints := m.filterHints()
	panes := m.panes(max(h-len(hints), 1))
	if len(hints) > 0 {
		panes += "\n" + strings.Join(hints, "\n")
	}
	w, mh := max(min(m.width-4, 80), 10), max(h-2, 3)
	var content string
	style := modalStyle.BorderForeground(m.accentStyle.GetForeground())
	switch {
	case len(m.notices) > 0:
		content = m.accentStyle.Render("Notice") + "\n\n" + ansi.Wrap(ticket.OneLine(m.notices[0]), w, "") +
			"\n\n" + hintLine(m.accentStyle, [2]string{keyLabel("notice", "dismiss"), "dismiss"})
		style = modalStyle.BorderForeground(warnStyle.GetForeground())
	case m.builder != nil:
		content = m.builder.view(w, mh, m.accentStyle)
	case m.input != nil:
		if m.input.area.DynamicHeight {
			w = max(min(m.width-4, 100), 10)
		}
		content = m.input.view(w, mh, m.accentStyle)
	case m.form != nil:
		content = m.form.view(w, mh, m.accentStyle)
	case m.modal != nil:
		content = m.modal.view(w, mh, m.accentStyle)
	case m.help:
		if m.activityOpen {
			content = m.activityHelp()
		} else if m.filtering {
			content = m.filterHelpView()
		} else {
			content = m.helpView()
		}
		style = style.Padding(0, 2)
	case m.waiting != "" || (m.loading && m.tasks != nil && !m.background):
		content = cmp.Or(m.waiting, "Loading tasks…")
		if m.sym.spinner != nil {
			content = m.sym.spinner[m.frame%len(m.sym.spinner)] + " " + content
		}
		style = loadingStyle
	default:
		return panes
	}
	// Float the modal over the panes, faded so the modal stands out.
	box := style.Render(content)
	x := max((m.width-lipgloss.Width(box))/2, 0)
	y := max((h-lipgloss.Height(box))/2, 0)
	canvas := lipgloss.NewCanvas(m.width, h).Compose(lipgloss.NewCompositor(
		lipgloss.NewLayer(dimStyle.Render(ansi.Strip(panes))),
		lipgloss.NewLayer(box).X(x).Y(y).Z(1),
	))
	// Canvas trims trailing blanks; restore the full body height.
	return lipgloss.NewStyle().Height(h).Render(canvas.Render())
}

func (m *Model) panes(h int) string {
	if m.activityOpen {
		return m.activityPanes(h)
	}
	listW, readerW, split := m.paneWidths()
	inner := m.paneHeight()
	var panes string
	switch {
	case split:
		title, count := m.listTitle()
		list := m.panel(title, count, m.listView(listW, inner), listW+panelFrame, h, m.listHighlight())
		reader := m.panel("[2] Ticket", m.readerView, m.reader.View(), readerW+panelFrame, h, m.readerHighlight())
		panes = lipgloss.JoinHorizontal(lipgloss.Top, list, reader)
	case m.focusReader:
		panes = m.reader.View()
		if m.showNav() {
			panes = m.panel("[2] Ticket", m.readerView, panes, readerW+panelFrame, h, m.readerHighlight())
		}
	case m.showNav():
		title, count := m.listTitle()
		panes = m.panel(title, count, m.listView(listW-panelFrame, inner), listW, h, m.listHighlight())
	default:
		panes = lipgloss.NewStyle().Width(listW).Height(h).Render(m.listView(listW, h))
	}
	if m.showNav() {
		nav := m.panel("[0] Views", "", m.navView(navW-panelFrame, inner), navW, h, m.focusColor(m.focusNav))
		panes = lipgloss.JoinHorizontal(lipgloss.Top, nav, panes)
	}
	return panes
}

// focusColor is the accent color for a focused panel, or nil.
func (m *Model) focusColor(focused bool) color.Color {
	if focused {
		return m.accentStyle.GetForeground()
	}
	return nil
}

// listHighlight marks the list panel in the warning color while the timer
// refreshes it, since that refresh shows no modal.
func (m *Model) listHighlight() color.Color {
	if m.loading && m.background {
		return warnStyle.GetForeground()
	}
	return m.focusColor(!m.focusReader && !m.focusNav)
}

// readerHighlight marks the selected ticket while its details load, including
// the list request that precedes a refresh of the displayed ticket.
func (m *Model) readerHighlight() color.Color {
	if t, ok := m.selected(); ok && (m.detailRequests[t.GID] != 0 ||
		m.loading && m.background && t.GID == m.shownGID) {
		return warnStyle.GetForeground()
	}
	return m.focusColor(m.focusReader)
}
