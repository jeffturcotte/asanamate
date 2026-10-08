package main

import (
	"bytes"
	"context"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/sadmachine/asanamate/internal/projectlinks"
)

// TestMain runs the test binary as asanamate-hub when TestProviderProtocol
// starts it as a provider command.
func TestMain(m *testing.M) {
	if os.Getenv("ASANAMATE_HUB_TEST") != "" {
		os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
	}
	os.Exit(m.Run())
}

// TestProviderProtocol runs asanamate-hub as project_links.command through
// asanamate's provider client, against a fake Hub.
func TestProviderProtocol(t *testing.T) {
	server := hubServer(t)
	t.Setenv("ASANAMATE_HUB_TEST", "1")
	t.Setenv(HubClientIDEnv, "id")
	t.Setenv(HubClientSecretEnv, "secret")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	provider := projectlinks.Provider{Command: "'" + exe + "' --url '" + server.URL + "'"}
	links, err := provider.Links(context.Background(), "42")
	if err != nil || !slices.Equal(projectlinks.Values(links, projectlinks.Git), []string{"https://github.com/acme/site"}) || len(projectlinks.Values(links, projectlinks.HarvestProject)) == 0 {
		t.Fatalf("links = %+v, err = %v", links, err)
	}
}

func TestRunChecksAndRejectsArguments(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"extra"}, strings.NewReader(""), &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "unexpected arguments") {
		t.Fatalf("code = %d, stderr = %s", code, errOut.String())
	}
	if code := run([]string{"-h"}, strings.NewReader(""), &out, &errOut); code != 0 || !strings.Contains(out.String(), "asanamate-hub check") {
		t.Fatalf("help code = %d, stdout = %s", code, out.String())
	}
	t.Setenv(HubClientIDEnv, "id")
	t.Setenv(HubClientSecretEnv, "")
	errOut.Reset()
	if code := run([]string{"check"}, strings.NewReader(""), &out, &errOut); code != 1 || !strings.Contains(errOut.String(), HubClientSecretEnv) {
		t.Fatalf("check code = %d, stderr = %s", code, errOut.String())
	}
}
