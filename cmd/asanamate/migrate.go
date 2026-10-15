package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"

	"github.com/sadmachine/asanamate/internal/config"
	"github.com/sadmachine/asanamate/internal/prompt"
	"github.com/sadmachine/asanamate/internal/setup"
	"github.com/sadmachine/asanamate/internal/state"
)

func loadOrMigrate(path string, in io.Reader, out io.Writer, interactive bool) (config.Config, error) {
	cfg, err := config.Load(path)
	var pending *config.UpdateNeeded
	if !interactive || !errors.As(err, &pending) {
		return cfg, err
	}
	fmt.Fprintln(out, "Your config uses settings from an older asanamate:")
	for _, change := range pending.Changes {
		fmt.Fprintln(out, "  - "+change)
	}
	reader := bufio.NewReader(in)
	ok, confirmErr := prompt.Confirm(reader, out, fmt.Sprintf("Update %s now? It keeps your values, resets comments to the current template, and saves old files as <file>.<time>.bak.", path))
	if confirmErr != nil {
		return cfg, confirmErr
	}
	if !ok {
		return cfg, err
	}
	if err := setup.Update(setup.Options{ConfigPath: path, In: reader, Out: out}, true); err != nil {
		return cfg, err
	}
	return config.Load(path)
}

// migrateDisplay silently moves app-owned legacy state. Keep the table until
// every setting and the cleared state have saved, so failures are retryable.
func migrateDisplay(path string, st *state.State) (config.Config, error) {
	d := st.LegacyDisplay
	if d == (state.LegacyDisplay{}) {
		return config.Load(path)
	}
	for _, item := range []struct {
		key   string
		value *bool
	}{
		{"list.separator", d.Separator}, {"list.header.spacing", d.HeaderSpacing}, {"layout.focus.dim", d.FocusDim},
	} {
		if item.value != nil {
			if err := config.SaveSetting(path, item.key, *item.value); err != nil {
				return config.Config{}, err
			}
		}
	}
	theme := d.Theme
	if theme == "dark" || theme == "light" {
		theme = "ansi-" + theme
	}
	for _, item := range []struct{ key, value string }{
		{"list.selection.style", d.SelectionStyle}, {"reader.view", d.ReaderView},
		{"list.refresh_interval", d.RefreshInterval}, {"theme.name", theme},
	} {
		if item.value != "" {
			if err := config.SaveSetting(path, item.key, item.value); err != nil {
				return config.Config{}, err
			}
		}
	}
	st.LegacyDisplay = state.LegacyDisplay{}
	if err := st.Save(); err != nil {
		st.LegacyDisplay = d
		return config.Config{}, err
	}
	return config.Load(path)
}
