package ccmux

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/sadmachine/asanamate/internal/prompt"
	"github.com/sadmachine/asanamate/internal/setup"
)

// Name is the adapter's command name.
const Name = "asanamate-ccmux"

// CodexHome is Codex's config directory: $CODEX_HOME, else ~/.codex.
func CodexHome() (string, error) {
	if dir := os.Getenv("CODEX_HOME"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codex"), nil
}

// CodexHookState is how Codex's hooks.json runs the adapter's hook.
type CodexHookState int

const (
	CodexAbsent CodexHookState = iota // Codex is not installed
	HookMissing                       // no asanamate hook
	HookStale                         // an old path or command, or events without the hook
	HookCurrent                       // every event runs this executable's hook
)

// CodexHook reports the state of the adapter's hook in codexHome/hooks.json
// for executable. It writes nothing.
func CodexHook(codexHome, executable string) (CodexHookState, error) {
	if info, err := os.Stat(codexHome); err != nil || !info.IsDir() {
		return CodexAbsent, nil
	}
	_, _, hooks, err := readCodexHooks(filepath.Join(codexHome, "hooks.json"))
	if err != nil {
		return HookMissing, err
	}
	return codexHookState(hooks, codexHookCommand(executable)), nil
}

// LegacyCodexHook reports whether codexHome/hooks.json still runs
// `asanamate hook codex`, which asanamate forwards to the adapter for one
// release only.
func LegacyCodexHook(codexHome string) (bool, error) {
	_, _, hooks, err := readCodexHooks(filepath.Join(codexHome, "hooks.json"))
	if err != nil {
		return false, err
	}
	legacy := false
	eachCodexHook(hooks, func(_, cmd string) { legacy = legacy || !strings.Contains(cmd, Name) })
	return legacy, nil
}

// OfferCodexHook adds `asanamate-ccmux hook codex` to Codex's hooks.json,
// after asking, when Codex is installed and the hook is missing. When an
// asanamate hook runs another path or command (such as the old
// `asanamate hook codex`) or misses events, it offers to replace it instead.
func OfferCodexHook(in *bufio.Reader, out io.Writer, codexHome, executable string) error {
	if info, err := os.Stat(codexHome); err != nil || !info.IsDir() {
		fmt.Fprintf(out, "Codex is not installed (no %s); no hook needed.\n", codexHome)
		return nil
	}
	path := filepath.Join(codexHome, "hooks.json")
	old, file, hooks, err := readCodexHooks(path)
	if err != nil {
		return err
	}
	command := codexHookCommand(executable)
	question, done := "Add "+Name+"'s Codex hook to "+path+"?", "Added the hook to"
	switch codexHookState(hooks, command) {
	case HookCurrent:
		fmt.Fprintf(out, "The Codex hook in %s is up to date.\n", path)
		return nil
	case HookStale:
		setup.Warn(out, "Warning: asanamate's Codex hook is out of date", codexHookStaleWarning)
		question = "Replace asanamate's Codex hook in " + path + " with " + command + "?"
		done = "Replaced the hook in"
	default:
		setup.Warn(out, "Warning: Codex agent status needs a hook", codexHookWarning)
	}
	install, err := prompt.Confirm(in, out, question)
	if err != nil {
		return err
	}
	if !install {
		fmt.Fprintln(out, "Skipped. Run `"+Name+" setup` to add it later.")
		return nil
	}
	removeCodexHooks(hooks)
	for _, event := range HookEvents() {
		groups, _ := hooks[event].([]any)
		hooks[event] = append(groups, map[string]any{
			"hooks": []any{map[string]any{"type": "command", "command": command}},
		})
	}
	file["hooks"] = hooks
	var data bytes.Buffer
	enc := json.NewEncoder(&data)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(file); err != nil {
		return err
	}
	backup, err := setup.WriteWithBackup(path, old, data.Bytes())
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "%s %s.", done, path)
	if backup != "" {
		fmt.Fprintf(out, " The previous file is saved as %s.", backup)
	}
	fmt.Fprintln(out, "\nCodex asks you to trust the new hooks the next time it starts.")
	return nil
}

// readCodexHooks reads the hooks.json at path: its bytes, nil when missing,
// the whole file, and its "hooks" object.
func readCodexHooks(path string) ([]byte, map[string]any, map[string]any, error) {
	old, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil, err
	}
	file := map[string]any{}
	if len(old) > 0 {
		if err := json.Unmarshal(old, &file); err != nil {
			return nil, nil, nil, fmt.Errorf("reading %s: %w", path, err)
		}
	}
	hooks, ok := file["hooks"].(map[string]any)
	if file["hooks"] != nil && !ok {
		return nil, nil, nil, fmt.Errorf("%s: \"hooks\" is not an object", path)
	}
	if hooks == nil {
		hooks = map[string]any{}
	}
	return old, file, hooks, nil
}

// codexHookCommand is the hook command that runs executable.
func codexHookCommand(executable string) string {
	return shellQuote(executable) + " hook codex"
}

const codexHookWarning = `  Without the Codex hook, Codex agent status will be
  inaccurate or wrong: ccmux can show a busy Codex session as idle.
  The hook records each session's real state.
`

const codexHookStaleWarning = `  An asanamate hook in Codex runs another path or the old
  ` + "`asanamate hook codex`" + `, or some hook events lack it, as after moving
  or reinstalling ` + Name + `. Codex agent status stays wrong until
  it is replaced.
`

// eachCodexHook calls fn with the event and command of every asanamate hook.
func eachCodexHook(hooks map[string]any, fn func(event, cmd string)) {
	for event, groups := range hooks {
		groups, _ := groups.([]any)
		for _, g := range groups {
			g, _ := g.(map[string]any)
			list, _ := g["hooks"].([]any)
			for _, h := range list {
				if cmd, ok := codexHookOf(h); ok {
					fn(event, cmd)
				}
			}
		}
	}
}

// codexHookState reports whether hooks run command for every hook event,
// and no other asanamate hook.
func codexHookState(hooks map[string]any, command string) CodexHookState {
	found, current := 0, map[string]bool{}
	eachCodexHook(hooks, func(event, cmd string) {
		found++
		current[event] = current[event] || cmd == command
	})
	if found == 0 {
		return HookMissing
	}
	events := HookEvents()
	for _, event := range events {
		if !current[event] {
			return HookStale
		}
	}
	if found != len(events) {
		return HookStale
	}
	return HookCurrent
}

// removeCodexHooks deletes every asanamate hook from hooks, then the groups
// and events it leaves empty. Other hooks stay as they are.
func removeCodexHooks(hooks map[string]any) {
	for event, groups := range hooks {
		groups, ok := groups.([]any)
		if !ok {
			continue
		}
		var kept []any
		for _, g := range groups {
			gm, ok := g.(map[string]any)
			list, isList := gm["hooks"].([]any)
			if !ok || !isList {
				kept = append(kept, g)
				continue
			}
			var rest []any
			for _, h := range list {
				if _, ours := codexHookOf(h); !ours {
					rest = append(rest, h)
				}
			}
			if len(rest) == len(list) {
				kept = append(kept, g)
			} else if len(rest) > 0 {
				gm["hooks"] = rest
				kept = append(kept, gm)
			}
		}
		if len(kept) == 0 {
			delete(hooks, event)
		} else {
			hooks[event] = kept
		}
	}
}

// codexHookOf returns h's command when h runs `asanamate hook codex` or
// `asanamate-ccmux hook codex`.
func codexHookOf(h any) (string, bool) {
	hm, _ := h.(map[string]any)
	cmd, _ := hm["command"].(string)
	return cmd, strings.Contains(cmd, "asanamate") && strings.HasSuffix(cmd, " hook codex")
}

// shellQuote quotes s for /bin/sh.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
