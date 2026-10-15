package config

import (
	"reflect"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

func decode(t *testing.T, body string) map[string]any {
	t.Helper()
	var m map[string]any
	if _, err := toml.Decode(body, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestApplyMovesMovesEveryReleasedKey(t *testing.T) {
	m := decode(t, `workspace = "1"
theme = "light"
accent_color = "2"
[display]
symbols = "nerd"
reduced_motion = false
[repo]
root = "~/projects"
command = "sesh list -z"
branch_field = "Branch Name"
[list]
filter = "is:open project:Work"
layout = "multi"
[list.header]
style = "bar"
color = "3"
[list.pinned]
color = ""
[list.viewing]
color = "5"
[list.selection]
color = "6"
`)
	if err := ApplyMoves(m); err != nil {
		t.Fatal(err)
	}
	want := decode(t, `workspace = "1"
symbols = "nerd"
reduced_motion = false
branch_field = "Branch Name"
[repo_source]
root = "~/projects"
command = "sesh list -z"
[theme]
name = "light"
[colors]
accent = "2"
header = "3"
pinned = "2"
viewing = "5"
selection = "6"
[list]
layout = "multi"
[list.default_view]
filter = "is:open project:Work"
[list.header]
style = "bar"
`)
	if !reflect.DeepEqual(m, want) {
		t.Fatalf("got  %v\nwant %v", m, want)
	}
}

func TestApplyMovesLeavesNewShapeAlone(t *testing.T) {
	m := decode(t, "[theme]\nname = \"dark\"\n[colors]\naccent = \"4\"\n")
	want := decode(t, "[theme]\nname = \"dark\"\n[colors]\naccent = \"4\"\n")
	if err := ApplyMoves(m); err != nil || !reflect.DeepEqual(m, want) {
		t.Fatalf("m = %v, err = %v", m, err)
	}
}

func TestApplyMovesRejectsOldAndNew(t *testing.T) {
	for _, mv := range Moves {
		// TOML itself rejects setting theme as both a scalar and a table.
		if mv.From == "theme" {
			continue
		}
		t.Run(mv.From, func(t *testing.T) {
			m := map[string]any{}
			setPath(m, mv.From, "old")
			setPath(m, mv.To, "new")
			want := mv.From + " and " + mv.To + " are both set"
			if err := ApplyMoves(m); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestLoadNamesMovedKeys(t *testing.T) {
	for body, key := range map[string]string{
		"accent_color = \"2\"\n":                   "accent_color moved to colors.accent",
		"theme = \"dark\"\n":                       "theme moved to theme.name",
		"[list.pinned]\ncolor = \"1\"\n":           "list.pinned.color moved to colors.pinned",
		"[display]\nsymbols = \"nerd\"\n":          "display.symbols moved to symbols",
		"[display]\nreduced_motion = false\n":      "display.reduced_motion moved to reduced_motion",
		"[list]\nfilter = \"is:open\"\n":           "list.filter moved to default_filter",
		"[repo]\nroot = \"~/projects\"\n":          "repo.root moved to repo_source.root",
		"[repo]\ncommand = \"sesh list -z\"\n":     "repo.command moved to repo_source.command",
		"[repo]\nbranch_field = \"Branch Name\"\n": "repo.branch_field moved to branch_field",
	} {
		_, err := Load(writeFile(t, "workspace = \"1\"\n"+body))
		if err == nil || !strings.Contains(err.Error(), key) || !strings.Contains(err.Error(), "asanamate config update") {
			t.Errorf("%q: err = %v", body, err)
		}
	}
}

func TestApplyCommandMovesKeepsFlags(t *testing.T) {
	for old, want := range map[string]string{
		"asanamate time-provider hrvst --task-id 456": "asanamate-hrvst --task-id 456",
		"asanamate time-provider hrvst":               "asanamate-hrvst",
		"asanamate-hrvst --task-id 456":               "asanamate-hrvst --task-id 456",
		"asanamate time-provider hrvstx":              "asanamate time-provider hrvstx",
		"my-provider":                                 "my-provider",
	} {
		m := map[string]any{"time_tracking": map[string]any{"id": "hrvst", "command": old}}
		ApplyCommandMoves(m)
		if got, _ := getPath(m, "time_tracking.command"); got != want {
			t.Errorf("%q became %q, want %q", old, got, want)
		}
	}
}

func TestLoadNamesMovedCommand(t *testing.T) {
	_, err := Load(writeFile(t, "workspace = \"1\"\n[time_tracking]\nid = \"hrvst\"\ncommand = \"asanamate time-provider hrvst --task-id 456\"\n"))
	if err == nil || !strings.Contains(err.Error(), "time_tracking.command: asanamate time-provider hrvst moved to asanamate-hrvst") || !strings.Contains(err.Error(), "asanamate config update") {
		t.Fatalf("err = %v", err)
	}
}
