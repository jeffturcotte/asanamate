package config

import (
	"fmt"
	"strings"
)

// agentsPreset is the released key v0.2.0 replaced with an agents.command.
const agentsPreset = "agents.preset"

// presetCommands maps each agents.preset value to the agents.command that
// replaced it in v0.2.0.
var presetCommands = map[string]string{"ccmux": "asanamate-ccmux"}

// MoveAgentsPreset rewrites a decoded config's agents.preset to the
// agents.command that replaced it. `asanamate config update` applies it, and
// Load names the key when an old config still sets it.
func MoveAgentsPreset(m map[string]any) error {
	v, ok := getPath(m, agentsPreset)
	if !ok {
		return nil
	}
	preset, _ := v.(string)
	if preset == "" {
		deletePath(m, agentsPreset)
		return nil
	}
	command, ok := presetCommands[preset]
	if !ok {
		return fmt.Errorf("%s %q has no replacement; remove it and set agents.command", agentsPreset, preset)
	}
	if c, ok := getPath(m, "agents.command"); ok {
		if c, _ := c.(string); strings.TrimSpace(c) != "" {
			return fmt.Errorf("%s and agents.command are both set; keep only agents.command", agentsPreset)
		}
	}
	deletePath(m, agentsPreset)
	setPath(m, "agents.command", command)
	return nil
}
