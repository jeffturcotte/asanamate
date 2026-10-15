package setup

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/sadmachine/asanamate/internal/config"
)

func TestMergeExtractsEverySetting(t *testing.T) {
	body := "workspace = \"1\"\n" + string(config.DefaultSettings())
	updated, _, settings, err := Merge(body)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{}
	if _, err := toml.Decode(string(config.DefaultSettings()), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(settings, want) {
		t.Fatalf("settings = %v, want %v", settings, want)
	}
	var remaining map[string]any
	if _, err := toml.Decode(updated, &remaining); err != nil {
		t.Fatal(err)
	}
	if changes := config.PendingUpdates(remaining); len(changes) != 0 {
		t.Fatalf("pending after merge: %v", changes)
	}
	cfg := loadString(t, updated, settings)
	if cfg.Theme != config.Default().Theme || cfg.List.Selection != config.Default().List.Selection {
		t.Fatalf("settings changed: %+v", cfg)
	}
}

func TestMergeThemeRenamesBeforeExtraction(t *testing.T) {
	updated, _, settings, err := Merge("workspace = \"1\"\n[theme]\nname = \"light\"\ndark = \"dark\"\nlight = \"light\"\n")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(updated, "[theme]") {
		t.Fatalf("theme table left in config: %s", updated)
	}
	cfg := loadString(t, updated, settings)
	if cfg.Theme.Name != "ansi-light" || cfg.Theme.Auto.Dark != "ansi-dark" || cfg.Theme.Auto.Light != "ansi-light" {
		t.Fatalf("theme = %+v", cfg.Theme)
	}
}

func TestMergeDefaultView(t *testing.T) {
	updated, _, _, err := Merge("workspace = \"1\"\ndefault_filter = \"is:done\"\n[list]\ngroup_by = \"section\"\n[list.sort]\nby = \"due\"\ndirection = \"desc\"\n")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(updated, "[list.sort]") {
		t.Fatalf("old sort table left behind: %s", updated)
	}
	cfg := loadString(t, updated)
	want := config.DefaultView{Filter: "is:done", GroupBy: "section", Sort: config.Sort{By: "due", Direction: "desc"}}
	if cfg.List.DefaultView != want {
		t.Fatalf("view = %+v, want %+v", cfg.List.DefaultView, want)
	}
}

func TestUpdateKeepsExistingSettingsAndBacksUpBothFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	old := []byte("workspace = \"1\"\n[list]\nseparator = true\nrefresh_interval = \"15s\"\n")
	oldSettings := []byte("# My comment\nlist.separator = false\nreader.view = \"markdown\"\n")
	if err := os.WriteFile(path, old, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.SettingsPath(path), oldSettings, 0o600); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := Update(Options{ConfigPath: path, Out: &out}, true); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil || cfg.List.Separator || cfg.Reader.View != "markdown" || cfg.List.RefreshInterval != "15s" {
		t.Fatalf("settings lost: %+v, err = %v", cfg, err)
	}
	for file, want := range map[string][]byte{path: old, config.SettingsPath(path): oldSettings} {
		backups, err := filepath.Glob(file + ".*.bak")
		if err != nil || len(backups) != 1 {
			t.Fatalf("backups = %v, err = %v", backups, err)
		}
		got, err := os.ReadFile(backups[0])
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("backup %s = %q, err = %v", file, got, err)
		}
	}
}

func TestSetupWritesDefaultSettingsAndKeepsExistingSettings(t *testing.T) {
	o, _ := options(t, "2\n-\n")
	if err := Run(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(config.SettingsPath(o.ConfigPath))
	if err != nil || !bytes.Equal(data, config.DefaultSettings()) {
		t.Fatalf("settings = %q, err = %v", data, err)
	}
	if err := config.SaveSetting(o.ConfigPath, "reader.view", "markdown"); err != nil {
		t.Fatal(err)
	}
	o.In = bufio.NewReader(strings.NewReader("y\n2\n-\n"))
	if err := Run(context.Background(), o); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(o.ConfigPath)
	if err != nil || cfg.Reader.View != "markdown" {
		t.Fatalf("setup replaced existing settings: %+v, err = %v", cfg.Reader, err)
	}
}

func TestUpdateRejectsNonTableMoveDestination(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	old := []byte("workspace = \"1\"\n[theme]\ndark = \"dark\"\nauto = \"keep me\"\n")
	if err := os.WriteFile(path, old, 0o600); err != nil {
		t.Fatal(err)
	}
	err := Update(Options{ConfigPath: path, Out: &strings.Builder{}}, true)
	if err == nil || !strings.Contains(err.Error(), "theme.auto is not a table") {
		t.Fatalf("error = %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, old) {
		t.Fatalf("conflicting destination was overwritten: %v", err)
	}
}
