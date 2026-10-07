package config

import (
	"fmt"
	"strings"
)

// Move is a released key that moved to a new place.
type Move struct{ From, To string }

// Moves lists the keys v0.2.0 moved. `asanamate config update` applies them,
// and Load names them when an old config still uses them.
var Moves = []Move{
	{"theme", "theme.name"},
	{"accent_color", "colors.accent"},
	{"list.header.color", "colors.header"},
	{"list.pinned.color", "colors.pinned"},
	{"list.viewing.color", "colors.viewing"},
	{"list.selection.color", "colors.selection"},
	{"display.symbols", "symbols"},
	{"display.reduced_motion", "reduced_motion"},
	{"list.filter", "default_filter"},
	{"repo.root", "repo_source.root"},
	{"repo.command", "repo_source.command"},
	{"repo.branch_field", "branch_field"},
}

// CommandMove is a released command that moved: a Key value starting with
// From now starts with To, keeping the rest, such as the command's flags.
type CommandMove struct{ Key, From, To string }

// CommandMoves lists the commands v0.2.0 moved. `asanamate config update`
// applies them, and Load names them, like Moves.
var CommandMoves = []CommandMove{
	{"time_tracking.command", "asanamate time-provider hrvst", "asanamate-hrvst"},
}

// commandMovedFrom returns the first command move m still uses and the
// key's rewritten value.
func commandMovedFrom(m map[string]any) (CommandMove, string, bool) {
	for _, mv := range CommandMoves {
		v, _ := getPath(m, mv.Key)
		s, _ := v.(string)
		if rest, ok := strings.CutPrefix(s, mv.From); ok && (rest == "" || rest[0] == ' ') {
			return mv, mv.To + rest, true
		}
	}
	return CommandMove{}, "", false
}

// ApplyCommandMoves rewrites a decoded config's moved commands.
func ApplyCommandMoves(m map[string]any) {
	for {
		mv, v, ok := commandMovedFrom(m)
		if !ok {
			return
		}
		setPath(m, mv.Key, v)
	}
}

// movedFrom returns the first move whose old key m still sets. An old key
// whose value is a table is the new shape (theme), not the old one.
func movedFrom(m map[string]any) (Move, bool) {
	for _, mv := range Moves {
		if v, ok := getPath(m, mv.From); ok && !isTable(v) {
			return mv, true
		}
	}
	return Move{}, false
}

// ApplyMoves rewrites a decoded config's moved keys to their new place and
// drops tables the moves leave empty. In v0.1.0 an empty pinned or viewing
// color meant "use accent_color"; built-in themes now set those roles, so the
// accent value is copied instead.
func ApplyMoves(m map[string]any) error {
	accent := "4"
	if v, ok := m["accent_color"].(string); ok && v != "" {
		accent = v
	}
	for {
		mv, ok := movedFrom(m)
		if !ok {
			return nil
		}
		if _, ok := getPath(m, mv.To); ok {
			return fmt.Errorf("%s and %s are both set; keep only %s", mv.From, mv.To, mv.To)
		}
		v, _ := getPath(m, mv.From)
		if (mv.To == "colors.pinned" || mv.To == "colors.viewing") && v == "" {
			v = accent
		}
		deletePath(m, mv.From)
		setPath(m, mv.To, v)
	}
}

func isTable(v any) bool {
	_, ok := v.(map[string]any)
	return ok
}

func getPath(m map[string]any, dotted string) (any, bool) {
	var v any = m
	for _, k := range strings.Split(dotted, ".") {
		t, ok := v.(map[string]any)
		if !ok {
			return nil, false
		}
		if v, ok = t[k]; !ok {
			return nil, false
		}
	}
	return v, true
}

func setPath(m map[string]any, dotted string, v any) {
	keys := strings.Split(dotted, ".")
	for _, k := range keys[:len(keys)-1] {
		next, ok := m[k].(map[string]any)
		if !ok {
			next = map[string]any{}
			m[k] = next
		}
		m = next
	}
	m[keys[len(keys)-1]] = v
}

// deletePath removes the key and any table the removal leaves empty.
func deletePath(m map[string]any, dotted string) {
	k, rest, nested := strings.Cut(dotted, ".")
	if !nested {
		delete(m, k)
		return
	}
	if t, ok := m[k].(map[string]any); ok {
		deletePath(t, rest)
		if len(t) == 0 {
			delete(m, k)
		}
	}
}
