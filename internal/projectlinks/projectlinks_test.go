package projectlinks

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestCommandProtocol(t *testing.T) {
	path := filepath.Join(t.TempDir(), "request.json")
	provider := Provider{Command: "cat > '" + path + "'; printf '%s' '{\"links\":[{\"type\":\"git\",\"value\":\"https://github.com/acme/site\"},{\"type\":\"other\",\"value\":\"x\"}]}'"}
	links, err := provider.Links(context.Background(), "42")
	if err != nil || len(links) != 2 || links[0] != (Link{Type: Git, Value: "https://github.com/acme/site"}) {
		t.Fatalf("links = %+v, err = %v", links, err)
	}
	request, _ := os.ReadFile(path)
	if string(request) != `{"version":1,"operation":"links","asana":{"project_gid":"42"}}` {
		t.Fatalf("request = %s", request)
	}
	if got := Values(links, Git); !slices.Equal(got, []string{"https://github.com/acme/site"}) {
		t.Fatalf("Values = %v", got)
	}
}

// TestProviderFixture runs testdata/provider.sh, the minimal provider
// docs/extensions.md describes, and checks the requests it received, so a
// protocol change fails here.
func TestProviderFixture(t *testing.T) {
	script, err := filepath.Abs("testdata/provider.sh")
	if err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(t.TempDir(), "requests.jsonl")
	provider := Provider{Command: "/bin/sh '" + script + "' '" + log + "'"}
	links, err := provider.Links(context.Background(), "7")
	if err != nil || !slices.Equal(links, []Link{{Type: Git, Value: "https://github.com/acme/web"}, {Type: HarvestProject, Value: "123"}}) {
		t.Fatalf("links = %+v, err = %v", links, err)
	}
	if links, err := provider.Links(context.Background(), "8"); err != nil || len(links) != 0 {
		t.Fatalf("unknown project links = %+v, err = %v", links, err)
	}
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"version":1,"operation":"links","asana":{"project_gid":"7"}}` + "\n" + `{"version":1,"operation":"links","asana":{"project_gid":"8"}}`
	if got := strings.TrimSpace(string(data)); got != want {
		t.Fatalf("requests:\n%s\nwant:\n%s", got, want)
	}
}

func TestProviderOffAndFailures(t *testing.T) {
	if links, err := (Provider{}).Links(context.Background(), "42"); links != nil || err != nil {
		t.Fatalf("off provider = %v, %v", links, err)
	}
	if _, err := (Provider{Command: "true"}).Links(context.Background(), ""); err == nil {
		t.Fatal("missing project accepted")
	}
	for _, response := range []string{`bad`, `{"links":[{"type":"git"}]}`, `{"links":[{"value":"x"}]}`} {
		if _, err := (Provider{Command: "printf '%s' '" + response + "'"}).Links(context.Background(), "42"); err == nil {
			t.Fatalf("response %s accepted", response)
		}
	}
	_, err := (Provider{Command: "echo unavailable >&2; exit 1"}).Links(context.Background(), "42")
	if err == nil || !strings.Contains(err.Error(), "project links: unavailable") {
		t.Fatalf("error = %v", err)
	}
}
