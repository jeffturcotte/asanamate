package dateinput

import (
	"context"
	"testing"
	"time"
)

func TestResolveKeepsExactDatesWithoutDate(t *testing.T) {
	t.Setenv("PATH", "")
	got, err := Resolve(context.Background(), " 2026-10-09\n")
	if err != nil || got != "2026-10-09" {
		t.Fatalf("Resolve = %q, %v", got, err)
	}
	for _, text := range []string{"", "  ", "tomorrow"} {
		if _, err := Resolve(context.Background(), text); err == nil {
			t.Errorf("Resolve(%q) succeeded without date", text)
		}
	}
}

func TestResolveRejectsInvalidDates(t *testing.T) {
	for _, text := range []string{"2026-02-30", "nonsense", "next blursday"} {
		if got, err := Resolve(context.Background(), text); err == nil {
			t.Errorf("Resolve(%q) = %q, want error", text, got)
		}
	}
}

func TestResolveRelativeDates(t *testing.T) {
	if _, err := Resolve(context.Background(), "today"); err != nil {
		t.Skipf("no GNU date: %v", err)
	}
	want := time.Now().AddDate(0, 0, 1).Format(time.DateOnly)
	if got, err := Resolve(context.Background(), "+1 day"); err != nil || got != want {
		t.Fatalf("Resolve(+1 day) = %q, %v; want %q", got, err, want)
	}
}
