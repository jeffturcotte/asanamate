package config

import (
	"bytes"
	"github.com/BurntSushi/toml"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTheme writes body to themes/<name>.toml next to the config at path.
func writeTheme(t *testing.T, path, name, body string) {
	t.Helper()
	dir := ThemesDir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".toml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestBuiltinThemesLoad(t *testing.T) {
	for _, name := range []string{"ansi-dark", "ansi-light"} {
		th, ok := builtinThemes[name]
		if !ok || th.Base != strings.TrimPrefix(name, "ansi-") || th.Colors.Accent.FG != "4" || len(th.Colors.Authors) != 6 {
			t.Errorf("%s = %+v", name, th)
		}
	}
	if builtinThemes["ansi-light"].Colors.Working.FG != "6" || builtinThemes["ansi-dark"].Colors.Working.FG != "14" {
		t.Error("light theme must use a darker working color")
	}
	if builtinThemes["ansi-dark"].Colors.More.BG != "208" || builtinThemes["ansi-light"].Colors.More.BG != "166" {
		t.Error("themes must give the +N more badge a background")
	}
}

func TestStandardThemesLoadFromFiles(t *testing.T) {
	for _, tc := range []struct{ name, base, accent string }{
		{"one-dark", "dark", "#61afef"},
		{"one-light", "light", "#4078f2"},
		{"monokai", "dark", "#a6e22e"},
		{"tokyo-night", "dark", "#7aa2f7"},
		{"tokyo-day", "light", "#2e7de9"},
		{"sonokai", "dark", "#76cce0"},
		{"sonokai-andromeda", "dark", "#6dcae8"},
		{"sonokai-atlantis", "dark", "#72cce8"},
		{"sonokai-maia", "dark", "#78cee9"},
		{"sonokai-shusia", "dark", "#7accd7"},
		{"sonokai-espresso", "dark", "#81d0c9"},
		{"dracula", "dark", "#bd93f9"},
		{"nord", "dark", "#88c0d0"},
		{"gruvbox-dark", "dark", "#83a598"},
		{"gruvbox-light", "light", "#076678"},
		{"catppuccin-mocha", "dark", "#89b4fa"},
		{"catppuccin-latte", "light", "#1e66f5"},
		{"solarized-dark", "dark", "#268bd2"},
		{"solarized-light", "light", "#268bd2"},
		{"rose-pine", "dark", "#c4a7e7"},
		{"catppuccin-frappe", "dark", "#8caaee"},
		{"catppuccin-macchiato", "dark", "#8aadf4"},
		{"rose-pine-moon", "dark", "#c4a7e7"},
		{"rose-pine-dawn", "light", "#907aa9"},
		{"tokyo-storm", "dark", "#7aa2f7"},
		{"tokyo-moon", "dark", "#82aaff"},
		{"kanagawa-wave", "dark", "#7e9cd8"},
		{"kanagawa-dragon", "dark", "#8ba4b0"},
		{"kanagawa-lotus", "light", "#4d699b"},
		{"everforest-dark", "dark", "#7fbbb3"},
		{"everforest-light", "light", "#3a94c5"},
		{"gruvbox-material-dark", "dark", "#7daea3"},
		{"gruvbox-material-light", "light", "#45707a"},
		{"nightfox", "dark", "#719cd6"},
		{"carbonfox", "dark", "#78a9ff"},
		{"dayfox", "light", "#2848a9"},
		{"github-dark", "dark", "#58a6ff"},
		{"github-light", "light", "#0969da"},
		{"github-dark-high-contrast", "dark", "#71b7ff"},
		{"github-light-high-contrast", "light", "#0349b4"},
		{"ayu-dark", "dark", "#59c2ff"},
		{"ayu-mirage", "dark", "#73d0ff"},
		{"ayu-light", "light", "#22a4e6"},
		{"night-owl", "dark", "#82aaff"},
		{"flexoki-dark", "dark", "#4385be"},
		{"flexoki-light", "light", "#205ea6"},
		{"palenight", "dark", "#82b1ff"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeThemeConfig(t, "workspace = \"1\"\n[theme]\nname = \""+tc.name+"\"\n")
			installThemeFixture(t, path, tc.name)
			cfg, err := Load(path)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := cfg.Themes[tc.name]; !ok {
				t.Fatal("standard themes must load from local files")
			}
			for _, darkBackground := range []bool{false, true} {
				p := cfg.Palette(darkBackground)
				if p.Base != tc.base || p.Colors.Accent.FG != tc.accent {
					t.Fatalf("palette = %+v, want base %s and accent %s", p, tc.base, tc.accent)
				}
				c := p.Colors
				for role, style := range map[string]Style{
					"accent": c.Accent, "border": c.Border, "muted": c.Muted, "highlight": c.Highlight,
					"ok": c.OK, "warn": c.Warn, "error": c.Error, "working": c.Working,
					"header": c.Header, "selection": c.Selection, "pinned": c.Pinned,
					"viewing": c.Viewing, "reacted": c.Reacted, "new": c.New, "more": c.More,
					"mode.normal": c.Mode.Normal, "mode.filter": c.Mode.Filter,
					"mode.read": c.Mode.Read, "mode.edit": c.Mode.Edit,
					"markdown.text": c.Markdown.Text, "markdown.heading": c.Markdown.Heading,
					"markdown.h1": c.Markdown.H1, "markdown.link": c.Markdown.Link,
					"markdown.code": c.Markdown.Code, "markdown.code_block": c.Markdown.CodeBlock,
					"markdown.quote": c.Markdown.Quote, "markdown.rule": c.Markdown.Rule,
				} {
					if !strings.HasPrefix(style.FG, "#") {
						t.Errorf("%s must use the preset palette, got %+v", role, style)
					}
				}
				if len(c.Authors) != 6 {
					t.Fatalf("authors = %+v, want six colors", c.Authors)
				}
				for _, style := range c.Authors {
					if !strings.HasPrefix(style.FG, "#") {
						t.Errorf("author must use the preset palette, got %+v", style)
					}
				}
				if c.Highlight.BG == "" || c.Highlight.Reverse == nil || *c.Highlight.Reverse {
					t.Errorf("highlight must use explicit colors without reverse: %+v", c.Highlight)
				}
				if c.Muted.Faint == nil || *c.Muted.Faint {
					t.Errorf("muted must not dim the preset color: %+v", c.Muted)
				}
			}
		})
	}
}

func TestStandardThemesAutoAndOverrides(t *testing.T) {
	path := writeThemeConfig(t, "workspace = \"1\"\n[theme]\nname = \"auto\"\n[theme.auto]\ndark = \"one-dark\"\nlight = \"one-light\"\n[colors]\npinned = \"#123456\"\n")
	installThemeFixture(t, path, "one-dark")
	installThemeFixture(t, path, "one-light")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		darkBackground bool
		base, accent   string
	}{
		{true, "dark", "#61afef"},
		{false, "light", "#4078f2"},
	} {
		p := cfg.Palette(tc.darkBackground)
		if p.Base != tc.base || p.Colors.Accent.FG != tc.accent {
			t.Fatalf("palette = %+v", p)
		}
		if s := p.Colors.Pinned; s.FG != "#123456" || s.Bold == nil || !*s.Bold {
			t.Fatalf("config must override preset foreground and retain bold: %+v", s)
		}
	}
}

func TestStandardThemeEditsTakeEffect(t *testing.T) {
	path := writeThemeConfig(t, "workspace = \"1\"\n[theme]\nname = \"tokyo-night\"\n")
	installThemeFixture(t, path, "tokyo-night")
	writeTheme(t, path, "tokyo-night", "base = \"light\"\n[colors]\naccent = \"#123456\"\n")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if p := cfg.Palette(true); p.Base != "light" || p.Colors.Accent.FG != "#123456" {
		t.Fatalf("existing custom theme must retain its colors: %+v", p)
	}
	writeTheme(t, path, "tokyo-night", "base = \"invalid\"\n")
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "base must be") {
		t.Fatalf("invalid theme must not silently use a fallback: %v", err)
	}
}

func TestStandardThemeRequiresLocalFile(t *testing.T) {
	path := writeThemeConfig(t, "workspace = \"1\"\n[theme]\nname = \"one-dark\"\n")
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), `theme.name: no theme "one-dark"`) {
		t.Fatalf("standard themes must not be embedded: %v", err)
	}
	if len(builtinThemes) != 2 {
		t.Fatalf("only dark and light should be built in: %d themes", len(builtinThemes))
	}
}

func installThemeFixture(t *testing.T, path, name string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "themes", name+".toml"))
	if err != nil {
		t.Fatal(err)
	}
	writeTheme(t, path, name, string(data))
}

func TestPaletteLayersAndFallbacks(t *testing.T) {
	path := writeThemeConfig(t, "workspace = \"1\"\n[theme]\nname = \"night\"\n[colors]\naccent = \"#7aa2f7\"\n[colors.mode]\nread = { bg = \"0\" }\n")
	writeTheme(t, path, "night", "base = \"dark\"\n[colors]\nborder = \"#3b4261\"\npinned = { fg = \"#ff9e64\" }\n[colors.markdown]\nlink = { underline = true }\n")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	p := cfg.Palette(true)
	c := p.Colors
	switch {
	case p.Base != "dark":
		t.Errorf("base = %q", p.Base)
	case c.Accent.FG != "#7aa2f7" || c.Accent.Bold == nil || !*c.Accent.Bold:
		t.Errorf("accent = %+v, want config fg over the built-in bold", c.Accent)
	case c.Border.FG != "#3b4261":
		t.Errorf("border = %+v, want the theme file's", c.Border)
	case c.Pinned.FG != "#ff9e64" || !*c.Pinned.Bold:
		t.Errorf("pinned = %+v, want theme fg over built-in bold", c.Pinned)
	case c.Header.FG != "#7aa2f7" || c.Selection.FG != "#7aa2f7" || c.Mode.Normal.FG != "#7aa2f7":
		t.Errorf("header, selection, and mode.normal must fall back to accent: %+v %+v %+v", c.Header, c.Selection, c.Mode.Normal)
	case c.Mode.Filter.FG != "3":
		t.Errorf("mode.filter = %+v, want warn", c.Mode.Filter)
	case c.Mode.Read.FG != "6" || c.Mode.Read.BG != "0":
		t.Errorf("mode.read = %+v", c.Mode.Read)
	case c.Markdown.Link.Underline == nil || !*c.Markdown.Link.Underline:
		t.Errorf("markdown.link = %+v", c.Markdown.Link)
	}
}

func TestPaletteAutoPicksByBackground(t *testing.T) {
	cfg := Config{Theme: Theme{Name: ThemeAuto, Auto: AutoThemes{Dark: "ansi-dark", Light: "ansi-light"}}}
	if cfg.Palette(true).Base != "dark" || cfg.Palette(false).Base != "light" {
		t.Fatal("auto must follow the background")
	}
	if (Config{}).Palette(false).Base != "light" {
		t.Fatal("a zero config must behave like the defaults")
	}
}

func TestThemeFileErrors(t *testing.T) {
	for name, tc := range map[string]struct{ config, theme, want string }{
		"missing file": {"[theme]\nname = \"nope\"\n", "", `theme.name: no theme "nope"`},
		"bad name":     {"[theme]\nname = \"Night Owl\"\n", "", "theme.name must be"},
		"auto as dark": {"[theme.auto]\ndark = \"auto\"\n", "", `theme.auto.dark can't be "auto"`},
		"bad base":     {"[theme]\nname = \"x\"\n", "base = \"blue\"\n", "base must be"},
		"missing base": {"[theme]\nname = \"x\"\n", "[colors]\naccent = \"1\"\n", "base must be"},
		"unknown key":  {"[theme]\nname = \"x\"\n", "base = \"dark\"\n[colors]\naccnt = \"1\"\n", "unknown keys"},
		"bad color":    {"[theme]\nname = \"x\"\n", "base = \"dark\"\n[colors.mode]\nread = \"blue\"\n", "colors.mode.read"},
		"bad author":   {"[theme]\nname = \"x\"\n", "base = \"dark\"\n[colors]\nauthors = [\"1\", \"x\"]\n", "colors.authors[1]"},
	} {
		t.Run(name, func(t *testing.T) {
			path := writeThemeConfig(t, "workspace = \"1\"\n"+tc.config)
			if tc.theme != "" {
				writeTheme(t, path, "x", tc.theme)
			}
			_, err := Load(path)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestThemeFileCannotShadowBuiltin(t *testing.T) {
	path := writeThemeConfig(t, "workspace = \"1\"\n")
	writeTheme(t, path, "dark", "base = \"dark\"\n")
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), `"dark" is a built-in theme; rename the file`) {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadValidatesBothAutoThemes(t *testing.T) {
	path := writeThemeConfig(t, "workspace = \"1\"\n[theme]\nname = \"auto\"\n[theme.auto]\nlight = \"day\"\n")
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), `theme.auto.light: no theme "day"`) {
		t.Fatalf("err = %v", err)
	}
}

func TestConfigColorsAreValidated(t *testing.T) {
	_, err := Load(writeThemeConfig(t, "workspace = \"1\"\n[colors]\nheader = { fg = \"256\" }\n"))
	if err == nil || !strings.Contains(err.Error(), "colors.header") {
		t.Fatalf("err = %v", err)
	}
}

func TestAdditionalColorRoles(t *testing.T) {
	cfg := Default()
	fallback := cfg.Palette(true).Colors
	if fallback.Branch.FG != fallback.Viewing.FG || fallback.Branch.Bold != nil || fallback.Asana.Orange.FG != fallback.Pinned.FG || fallback.Asana.Red.FG != fallback.Error.FG || len(fallback.TitleTags) != len(fallback.Authors) {
		t.Fatalf("unexpected fallback roles: %+v", fallback)
	}
	cfg, err := Load(writeThemeConfig(t, `workspace = "1"
[colors]
branch = "#112233"
title_tags = ["#445566"]
[colors.asana]
red = "#778899"
`))
	if err != nil {
		t.Fatal(err)
	}
	colors := cfg.Palette(true).Colors
	if colors.Branch.FG != "#112233" || len(colors.TitleTags) != 1 || colors.TitleTags[0].FG != "#445566" || colors.Asana.Red.FG != "#778899" {
		t.Fatalf("overrides not applied: %+v", colors)
	}
	theme, err := parseTheme("custom.toml", []byte(`base = "dark"
[colors]
branch = "#112233"
title_tags = []
[colors.asana]
blue = "#445566"
`))
	if err != nil {
		t.Fatal(err)
	}
	cfg.Colors = Colors{}
	cfg.Theme.Name = "custom"
	cfg.Themes = map[string]ThemeFile{"custom": {Base: "dark", Colors: theme.Colors.Over(builtinThemes["ansi-dark"].Colors)}}
	colors = cfg.Palette(true).Colors
	if len(colors.TitleTags) != 0 || colors.TitleTags == nil || colors.Asana.Blue.FG != "#445566" {
		t.Fatal("theme override or empty tag palette lost")
	}
	for _, tc := range []struct{ body, key string }{
		{"[colors]\nbranch = \"bad\"", "colors.branch"},
		{"[colors]\ntitle_tags = [\"bad\"]", "colors.title_tags[0]"},
		{"[colors.asana]\nred = \"bad\"", "colors.asana.red"},
	} {
		_, err := Load(writeThemeConfig(t, "workspace = \"1\"\n"+tc.body))
		if err == nil || !strings.Contains(err.Error(), tc.key) {
			t.Fatalf("validation for %s: %v", tc.key, err)
		}
	}
}

func TestThemeNamesListsBuiltinsThenFiles(t *testing.T) {
	path := writeThemeConfig(t, "workspace = \"1\"\n")
	if names, err := ThemeNames(ThemesDir(path)); err != nil || strings.Join(names, ",") != "auto,ansi-dark,ansi-light" {
		t.Fatalf("missing dir: names = %v, err = %v", names, err)
	}
	writeTheme(t, path, "nord", "base = \"dark\"\n")
	writeTheme(t, path, "dracula", "base = \"dark\"\n")
	writeTheme(t, path, "Bad Name", "base = \"dark\"\n")
	for _, extra := range []string{"example.toml.example", "nord.toml.2026-10-14--11-42-21.bak"} {
		if err := os.WriteFile(filepath.Join(ThemesDir(path), extra), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	names, err := ThemeNames(ThemesDir(path))
	if err != nil || strings.Join(names, ",") != "auto,ansi-dark,ansi-light,dracula,nord" {
		t.Fatalf("names = %v, err = %v", names, err)
	}
}

func TestWithThemeSwitchesTheme(t *testing.T) {
	path := writeThemeConfig(t, "workspace = \"1\"\n[theme]\nname = \"ansi-dark\"\n[colors]\nok = \"9\"\n")
	writeTheme(t, path, "day", "base = \"light\"\n[colors]\naccent = \"#123456\"\n")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	day, err := cfg.WithTheme(ThemesDir(path), "day")
	if err != nil {
		t.Fatal(err)
	}
	p := day.Palette(true)
	if day.Theme.Name != "day" || p.Base != "light" || p.Colors.Accent.FG != "#123456" || p.Colors.OK.FG != "9" {
		t.Fatalf("theme = %+v, palette = %+v", day.Theme, p)
	}
	if cfg.Theme.Name != "ansi-dark" {
		t.Fatal("WithTheme changed the original config")
	}
	if _, err := cfg.WithTheme(ThemesDir(path), "gone"); err == nil || !strings.Contains(err.Error(), `no theme "gone"`) {
		t.Fatalf("err = %v", err)
	}
}

// writeThemeConfig separates a theme fixture from config-only color overrides.
func writeThemeConfig(t *testing.T, body string) string {
	t.Helper()
	var raw map[string]any
	if _, err := toml.Decode(body, &raw); err != nil {
		t.Fatal(err)
	}
	theme, hasTheme := raw["theme"]
	delete(raw, "theme")
	var configBody, settings bytes.Buffer
	if err := toml.NewEncoder(&configBody).Encode(raw); err != nil {
		t.Fatal(err)
	}
	if hasTheme {
		if err := toml.NewEncoder(&settings).Encode(map[string]any{"theme": theme}); err != nil {
			t.Fatal(err)
		}
	}
	return writeSettingsFixture(t, configBody.String(), settings.String())
}
