package timetracking

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sadmachine/asanamate/internal/form"
	"github.com/sadmachine/asanamate/internal/projectlinks"
)

const sampleForm = `{"fields":[{"id":"project_id","label":"Project","type":"select","options":[{"id":"opaque","name":"Project"}],"remember":true},{"id":"hours","label":"Hours","type":"hours"}]}`

func TestCommandProtocol(t *testing.T) {
	path := filepath.Join(t.TempDir(), "request.json")
	script := filepath.Join(t.TempDir(), "provider.sh")
	body := "#!/bin/sh\ncat > '" + path + "'\nprintf '%s' '" + sampleForm + "'\n"
	if err := os.WriteFile(script, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	provider := Provider{Command: script}
	links := []projectlinks.Link{{Type: projectlinks.HarvestProject, Value: "opaque"}}
	spec, err := provider.Form(context.Background(), Asana{TaskGID: "1", ProjectGID: "2"}, links)
	if err != nil || len(spec.Fields) != 2 || spec.Fields[0].Options[0].ID != "opaque" {
		t.Fatalf("form = %+v, err = %v", spec, err)
	}
	request, _ := os.ReadFile(path)
	for _, want := range []string{`"version":1`, `"operation":"form"`, `"project_gid":"2"`, `"links":[{"type":"harvest_project","value":"opaque"}]`} {
		if !strings.Contains(string(request), want) {
			t.Fatalf("request = %s; missing %s", request, want)
		}
	}
	values := map[string]string{"project_id": "opaque", "hours": "1.25"}
	if err := provider.Log(context.Background(), spec, values, Asana{TaskGID: "1", ProjectGID: "2", Title: "Fix", URL: "https://app.asana.com/0/2/1"}); err != nil {
		t.Fatal(err)
	}
	request, _ = os.ReadFile(path)
	for _, want := range []string{`"version":1`, `"operation":"log"`, `"project_id":"opaque"`, `"hours":"1.25"`, `"task_gid":"1"`, `"project_gid":"2"`} {
		if !strings.Contains(string(request), want) {
			t.Fatalf("request = %s; missing %s", request, want)
		}
	}
}

// TestProviderFixture runs every operation through testdata/provider.sh, the
// minimal provider docs/extensions.md describes, and checks the requests it
// received, so a protocol change fails here.
func TestProviderFixture(t *testing.T) {
	script, err := filepath.Abs("testdata/provider.sh")
	if err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(t.TempDir(), "requests.jsonl")
	provider := Provider{Command: "/bin/sh '" + script + "' '" + log + "'"}
	ctx := context.Background()
	asana := Asana{TaskGID: "42", ProjectGID: "7", Title: "Fix login", URL: "https://app.asana.com/0/7/42"}

	spec, err := provider.Form(ctx, asana, []projectlinks.Link{{Type: projectlinks.HarvestProject, Value: "123"}})
	if err != nil || len(spec.Fields) != 2 || spec.Fields[0].Options[0].ID != "123" {
		t.Fatalf("form = %+v, err = %v", spec, err)
	}
	if err := provider.Log(ctx, spec, map[string]string{"project_id": "123", "hours": "1.5"}, asana); err != nil {
		t.Fatal(err)
	}
	summary, err := provider.Summary(ctx, Asana{TaskGID: asana.TaskGID, URL: asana.URL})
	if err != nil || summary.TotalSeconds != 13500 {
		t.Fatalf("summary = %+v, err = %v", summary, err)
	}

	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		`{"version":1,"operation":"form","asana":{"task_gid":"42","project_gid":"7","title":"Fix login","url":"https://app.asana.com/0/7/42"},"links":[{"type":"harvest_project","value":"123"}]}`,
		`{"version":1,"operation":"log","values":{"hours":"1.5","project_id":"123"},"asana":{"task_gid":"42","project_gid":"7","title":"Fix login","url":"https://app.asana.com/0/7/42"}}`,
		`{"version":1,"operation":"summary","asana":{"task_gid":"42","project_gid":"","title":"","url":"https://app.asana.com/0/7/42"}}`,
	}
	got := strings.Split(strings.TrimSpace(string(data)), "\n")
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("requests:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestProviderRejectsBadDataAndFailures(t *testing.T) {
	for _, response := range []string{`{}`, `{"fields":[{"id":"x","label":"X","type":"select","options":[]}]}`, `bad`} {
		provider := Provider{Command: "printf '%s' '" + response + "'"}
		if _, err := provider.Form(context.Background(), Asana{}, nil); err == nil {
			t.Fatalf("response %s should fail", response)
		}
	}
	provider := Provider{Command: "echo unavailable >&2; exit 1"}
	if _, err := provider.Form(context.Background(), Asana{}, nil); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("error = %v", err)
	}
	spec := form.Spec{Fields: []form.Field{{ID: "hours", Label: "Hours", Type: form.Hours}}}
	if err := provider.Log(context.Background(), spec, map[string]string{"hours": "0"}, Asana{}); err == nil {
		t.Fatal("zero hours accepted")
	}
}

func TestSummaryProtocol(t *testing.T) {
	path := filepath.Join(t.TempDir(), "request.json")
	provider := Provider{Command: "cat > '" + path + "'; printf '%s' '{\"total_seconds\":13500}'"}
	summary, err := provider.Summary(context.Background(), Asana{TaskGID: "42", URL: "https://app.asana.com/0/1/42"})
	if err != nil || summary.TotalSeconds != 13500 {
		t.Fatalf("summary = %+v, err = %v", summary, err)
	}
	request, _ := os.ReadFile(path)
	if !strings.Contains(string(request), `"version":1,"operation":"summary"`) || !strings.Contains(string(request), `"task_gid":"42"`) || strings.Contains(string(request), `"values"`) {
		t.Fatalf("request = %s", request)
	}
}

func TestSummaryRejectsInvalidResponses(t *testing.T) {
	for _, response := range []string{`{}`, `null`, `bad`, `{"total_seconds":null}`, `{"total_seconds":-1}`, `{"total_seconds":1.5}`, `{"total_seconds":9223372036854775808}`} {
		provider := Provider{Command: "printf '%s' '" + response + "'"}
		if _, err := provider.Summary(context.Background(), Asana{TaskGID: "42"}); err == nil {
			t.Fatalf("invalid summary accepted: %s", response)
		}
	}
	provider := Provider{Command: "printf '%s' '{\"total_seconds\":0}'"}
	if summary, err := provider.Summary(context.Background(), Asana{TaskGID: "42"}); err != nil || summary.TotalSeconds != 0 {
		t.Fatalf("zero summary = %+v, err = %v", summary, err)
	}
	if _, err := provider.Summary(context.Background(), Asana{}); err == nil {
		t.Fatal("missing ticket accepted")
	}
	provider.Command = "printf '%s' '{\"unsupported\":true}'"
	if _, err := provider.Summary(context.Background(), Asana{TaskGID: "42"}); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("unsupported error = %v", err)
	}
	provider.Command = "echo unknown operation >&2; exit 1"
	if _, err := provider.Summary(context.Background(), Asana{TaskGID: "42"}); err == nil {
		t.Fatal("legacy provider failure treated as zero")
	}
}
