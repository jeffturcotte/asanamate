package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sadmachine/asanamate/internal/asana"
	"github.com/sadmachine/asanamate/internal/writeback"
)

func TestBatchCLI(t *testing.T) {
	const input = `[{"kind":"due","value":"2026-10-20"},{"kind":"complete","value":"true"}]`
	for _, argument := range []string{input, "-"} {
		t.Run(argument, func(t *testing.T) {
			writes := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					io.WriteString(w, `{"data":{"gid":"1","name":"Fix"}}`)
					return
				}
				writes++
				if r.Method != http.MethodPut || r.URL.Path != "/tasks/1" {
					t.Errorf("unexpected write %s %s", r.Method, r.URL.Path)
				}
				io.WriteString(w, `{"data":{}}`)
			}))
			t.Cleanup(srv.Close)
			client := asana.New("token")
			client.BaseURL = srv.URL
			writer := func(yes bool) (writeback.Service, error) {
				if !yes {
					t.Error("--yes not forwarded")
				}
				return writeback.Service{Client: client}, nil
			}
			if err := runWrite("batch", []string{"--yes", "1", argument}, strings.NewReader(input), writer); err != nil || writes != 2 {
				t.Fatalf("err = %v, writes = %d", err, writes)
			}
		})
	}
}

func TestBatchCLIRejectsMalformedInputBeforeWriting(t *testing.T) {
	for _, input := range []string{
		`{"kind":"complete","value":"true"}`,
		`[{"kind":"complete","vale":"true"}]`,
		`[] []`,
		`[{"kind":"move","project":"../other","value":"Done"}]`,
		`[{"kind":"project_remove","target":"not-a-gid"}]`,
	} {
		t.Run(input, func(t *testing.T) {
			// A nil client makes any accidental network call fail the test.
			writer := func(bool) (writeback.Service, error) { return writeback.Service{}, nil }
			if err := runWrite("batch", []string{"1", input}, strings.NewReader(""), writer); err == nil {
				t.Fatal("accepted malformed changes")
			}
		})
	}
}
