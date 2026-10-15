package tui

import (
	"cmp"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sadmachine/asanamate/internal/config"
)

// setPalette draws with p from now on.
func (m *Model) setPalette(p config.Palette) {
	applyPalette(p)
	c := p.Colors
	m.palette = p
	m.accentStyle, m.headerStyle = lipStyle(c.Accent), lipStyle(c.Header)
	m.pinnedStyle, m.viewingStyle = lipStyle(c.Pinned), lipStyle(c.Viewing)
	m.markerStyle = lipStyle(c.Selection)
	m.renderers = map[rendererKey]*markdownRenderer{}
}

// configPath is the config file's path, from Deps or the default location.
func (m *Model) configPath() (string, error) {
	if m.deps.ConfigPath != "" {
		return m.deps.ConfigPath, nil
	}
	return config.Path()
}

// themeChoice is a theme's config, or why its file did not load.
type themeChoice struct {
	cfg config.Config
	err error
}

// openThemes shows every theme with a preview of the highlighted one. It
// asks the terminal for its background, so the auto theme previews right.
func (m *Model) openThemes() tea.Cmd {
	path, err := m.configPath()
	if err != nil {
		m.status = err.Error()
		return nil
	}
	dir := config.ThemesDir(path)
	names, err := config.ThemeNames(dir)
	if err != nil {
		m.status = err.Error()
		return nil
	}
	current := cmp.Or(m.deps.Config.Theme.Name, config.ThemeAuto)
	items := make([]pickItem, len(names))
	for i, name := range names {
		items[i] = pickItem{Label: name, Value: name}
		if name == current {
			items[i].Hint = "current"
		}
	}
	// Theme files load once, when first highlighted.
	choices := map[string]themeChoice{}
	p := newPicker(pickValue(func(name string) tea.Cmd {
		m.modal = nil
		m.setTheme(dir, name)
		return nil
	}), "Theme", items)
	p.cursor = max(slices.Index(names, current), 0)
	p.preview = func(it *pickItem, width, height int) string {
		if it == nil {
			return ""
		}
		name := it.Value.(string)
		choice, ok := choices[name]
		if !ok {
			choice.cfg, choice.err = m.deps.Config.WithTheme(dir, name)
			choices[name] = choice
		}
		if choice.err != nil {
			return errorStyle.Render(ansi.Wrap(choice.err.Error(), width, ""))
		}
		return themePreview(choice.cfg.Palette(!m.lightBackground), width, height)
	}
	m.modal = p
	return tea.RequestBackgroundColor
}

// setTheme switches to the theme name from dir and saves it for later
// sessions.
func (m *Model) setTheme(dir, name string) {
	cfg, err := m.deps.Config.WithTheme(dir, name)
	if err != nil {
		m.status = err.Error()
		return
	}
	m.deps.Config = cfg
	m.setPalette(cfg.Palette(!m.lightBackground))
	m.renderDetail(true)
	m.deps.State.Display.Theme = name
	m.saveDisplay()
}

// themePreview draws a small ticket list and the mode pills in p's colors,
// without touching the active palette.
func themePreview(p config.Palette, width, height int) string {
	c := p.Colors
	accent, muted, border := lipStyle(c.Accent), lipStyle(c.Muted), lipStyle(c.Border)
	tags := cycleStyles(nil, c.TitleTags)
	authors := cycleStyles(nil, c.Authors)
	inner := max(width-2, 10)
	row := func(left, right string) string {
		gap := max(inner-ansi.StringWidth(left)-ansi.StringWidth(right), 1)
		return ansi.Truncate(left+strings.Repeat(" ", gap)+right, inner, "…")
	}
	header := func(name string) string {
		label := lipStyle(c.Header).Render(name)
		return label + " " + border.Render(strings.Repeat("─", max(inner-ansi.StringWidth(label)-1, 0)))
	}
	selected := lipStyle(c.Selection).Render("▌") +
		lipStyle(c.Highlight).Render(row(" [WEB] Fix login", "Mon")[1:])
	list := strings.Join([]string{
		accent.Bold(true).Render("Tickets") + muted.Render(" · 5 open"),
		header("In progress"),
		selected,
		row("  "+tags[min(1, len(tags)-1)].Render("[API]")+" Rate limits", lipStyle(c.New).Render("NEW")),
		row("  "+lipStyle(c.Pinned).Render("Pinned")+" notes", lipStyle(c.Warn).Render("today")),
		row("  Fix sync", lipStyle(c.Error).Render("2d late")),
		header("Done"),
		row("  "+muted.Render("Ship release"), lipStyle(c.OK).Render("done")),
	}, "\n")
	// Drawn as the focused pane, whose border takes the accent.
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
		BorderForeground(accent.GetForeground()).Width(width).Render(list)
	var names []string
	for i, name := range []string{"alice", "bob", "carol", "dave"} {
		names = append(names, authors[i%len(authors)].Render(name))
	}
	a := c.Asana
	var dots []string
	for _, s := range []config.Style{a.Red, a.Orange, a.Yellow, a.Green, a.Teal, a.Blue, a.Purple, a.Pink, a.Gray} {
		dots = append(dots, lipStyle(s).Render("●"))
	}
	lines := []string{
		box,
		pillStyle(c.Mode.Normal).Render(" NORMAL ") + " " + pillStyle(c.Mode.Filter).Render(" FILTER ") + " " +
			pillStyle(c.Mode.Read).Render(" READ ") + " " + pillStyle(c.Mode.Edit).Render(" EDIT "),
		strings.Join(names, " ") + "  " + lipStyle(c.Working).Render("working") + " " + lipStyle(c.Branch).Render("main"),
		strings.Join(dots, " ") + muted.Render("  base: "+p.Base),
	}
	return lipgloss.NewStyle().MaxHeight(height).MaxWidth(width).Render(strings.Join(lines, "\n"))
}
