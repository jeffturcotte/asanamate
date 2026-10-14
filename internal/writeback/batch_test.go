package writeback

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBatchValidatesAllChangesBeforeWriting(t *testing.T) {
	for _, invalid := range []Change{
		{Kind: "field", Target: "f2", Value: "not a number"},
		{Kind: "field", Target: "f2", Value: "NaN"},
		{Kind: "move", Project: "p1", Value: "missing"},
		{Kind: "assign", Value: "unknown"},
		{Kind: "due", Value: "2026-99-99"},
		{Kind: "complete", Value: "maybe"},
		{Kind: "project_add"},
		{Kind: "unknown"},
	} {
		t.Run(invalid.Kind+invalid.Target, func(t *testing.T) {
			client, writes := fake(t)
			s := Service{Client: client}
			done, err := s.Batch(context.Background(), "1", []Change{{Kind: "complete", Value: "true"}, invalid})
			if err == nil || done != 0 || len(*writes) != 0 {
				t.Fatalf("done = %d, err = %v, writes = %v", done, err, *writes)
			}
		})
	}
}

func TestBatchConfirmsOnceAndSupportsClearing(t *testing.T) {
	client, writes := fake(t)
	confirmed := 0
	s := Service{Client: client, Confirm: func(question string) (bool, error) {
		confirmed++
		if !strings.Contains(question, "Fix login") || !strings.Contains(question, "Unassign") {
			t.Errorf("confirmation = %q", question)
		}
		return true, nil
	}}
	changes := []Change{{Kind: "assign", Value: ""}, {Kind: "due", Value: ""}, {Kind: "field", Target: "f1", Value: ""}}
	done, err := s.Batch(context.Background(), "1", changes)
	if err != nil || done != 3 || confirmed != 1 || len(*writes) != 3 {
		t.Fatalf("done = %d, err = %v, confirmations = %d, writes = %v", done, err, confirmed, *writes)
	}
	if (*writes)[0].body["assignee"] != nil || (*writes)[1].body["due_on"] != nil || (*writes)[2].body["custom_fields"].(map[string]any)["f1"] != nil {
		t.Fatalf("clear values = %+v", *writes)
	}
}

func TestBatchStopsAtFailureAndReportsProgress(t *testing.T) {
	client, _ := fake(t)
	writes := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			io.WriteString(w, taskJSON)
			return
		}
		writes++
		if writes == 2 {
			w.WriteHeader(http.StatusForbidden)
			io.WriteString(w, `{"errors":[{"message":"Denied"}]}`)
			return
		}
		io.WriteString(w, `{"data":{}}`)
	}))
	t.Cleanup(srv.Close)
	client.BaseURL = srv.URL
	s := Service{Client: client}
	done, err := s.Batch(context.Background(), "1", []Change{
		{Kind: "complete", Value: "true"}, {Kind: "due", Value: "2026-10-20"}, {Kind: "assign", Value: ""},
	})
	if done != 1 || err == nil || writes != 2 || !strings.Contains(err.Error(), "1 of 3 changes saved") {
		t.Fatalf("done = %d, err = %v, writes = %d", done, err, writes)
	}
}

func TestBatchDeclinedWritesNothing(t *testing.T) {
	client, writes := fake(t)
	s := Service{Client: client, Confirm: func(string) (bool, error) { return false, nil }}
	done, err := s.Batch(context.Background(), "1", []Change{{Kind: "complete", Value: "true"}})
	if done != 0 || err != ErrDeclined || len(*writes) != 0 {
		t.Fatalf("done = %d, err = %v, writes = %v", done, err, *writes)
	}
}
