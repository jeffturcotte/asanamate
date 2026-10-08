package projectlinks

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// hubFind holds a fuzzy match, whose Asana link only contains the GID, and
// the exact match with an expired, a current, and an unread link.
const hubFind = `{"data":[
{"project":{"links":[{"type":"Asana Project","value":"420"},{"type":"Git Repository","value":"https://github.com/acme/wrong"}],"contract_links":[]},"score":0.9},
{"project":{"links":[
  {"type":"Git Repository","value":"https://github.com/acme/site"},
  {"type":"Harvest Project","value":"111","end_date":"2026-01-31"},
  {"type":"Asana Project","value":"42"}],
 "contract_links":[
  {"type":"Harvest Project","value":"222","start_date":"2026-02-01","end_date":"2026-12-31"},
  {"type":"Parallax Project","value":"p"}]},"score":1.0}]}`

func hubServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/token":
			r.ParseForm()
			if r.Method != http.MethodPost || r.Form.Get("grant_type") != "client_credentials" || r.Form.Get("client_id") != "id" || r.Form.Get("client_secret") != "secret" {
				http.Error(w, `{"error":"invalid_client"}`, http.StatusUnauthorized)
				return
			}
			fmt.Fprint(w, `{"token_type":"Bearer","access_token":"token"}`)
		case "/api/projects/find":
			q := r.URL.Query()
			if r.Header.Get("Authorization") != "Bearer token" || r.Header.Get("Accept") != "application/json" || q.Get("type") != "Asana Project" || q.Get("term") != "42" {
				http.Error(w, `{"message":"Unauthenticated."}`, http.StatusUnauthorized)
				return
			}
			fmt.Fprint(w, hubFind)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

var hubToday = time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)

func TestHubReturnsCurrentLinksOfExactMatches(t *testing.T) {
	server := hubServer(t)
	var out bytes.Buffer
	err := runHub(context.Background(), strings.NewReader(`{"operation":"links","asana":{"project_gid":"42"}}`), &out, server.URL+"/", "id", "secret", hubToday)
	want := `{"links":[{"type":"harvest_project","value":"222"},{"type":"git","value":"https://github.com/acme/site"}]}`
	if err != nil || strings.TrimSpace(out.String()) != want {
		t.Fatalf("response = %s, err = %v", out.String(), err)
	}
	out.Reset()
	if err := runHub(context.Background(), strings.NewReader(`{"operation":"links","asana":{"project_gid":"42"}}`), &out, server.URL, "id", "secret", time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil || !strings.Contains(out.String(), `"value":"111"`) || strings.Contains(out.String(), `"value":"222"`) {
		t.Fatalf("dated links before the contract = %s, err = %v", out.String(), err)
	}
}

func TestHubRejectsBadRequestsAndCredentials(t *testing.T) {
	server := hubServer(t)
	for _, tc := range []struct{ request, id, secret, want string }{
		{`{"operation":"links","asana":{"project_gid":"4x"}}`, "id", "secret", "numeric Asana project GID"},
		{`{"operation":"summary","asana":{"project_gid":"42"}}`, "id", "secret", "unknown project links operation"},
		{`{"operation":"links","asana":{"project_gid":"42"}}`, "id", "wrong", "401"},
		{`{"operation":"links","asana":{"project_gid":"42"}}`, "id", "", HubClientSecretEnv},
	} {
		t.Setenv(HubClientIDEnv, tc.id)
		t.Setenv(HubClientSecretEnv, tc.secret)
		var out bytes.Buffer
		err := runHub(context.Background(), strings.NewReader(tc.request), &out, server.URL, tc.id, tc.secret, hubToday)
		if err == nil || !strings.Contains(err.Error(), tc.want) || out.Len() != 0 {
			t.Errorf("request %s: out = %s, err = %v, want %q", tc.request, out.String(), err, tc.want)
		}
	}
}

func TestCheckHubNamesMissingCredentials(t *testing.T) {
	t.Setenv(HubClientIDEnv, "id")
	t.Setenv(HubClientSecretEnv, "")
	if err := CheckHub(); err == nil || !strings.Contains(err.Error(), HubClientSecretEnv) || strings.Contains(err.Error(), HubClientIDEnv) {
		t.Fatalf("CheckHub = %v", err)
	}
	t.Setenv(HubClientSecretEnv, "secret")
	if err := CheckHub(); err != nil {
		t.Fatal(err)
	}
}
