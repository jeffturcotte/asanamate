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
	if string(request) != `{"operation":"links","asana":{"project_gid":"42"}}` {
		t.Fatalf("request = %s", request)
	}
	if got := Values(links, Git); !slices.Equal(got, []string{"https://github.com/acme/site"}) {
		t.Fatalf("Values = %v", got)
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
