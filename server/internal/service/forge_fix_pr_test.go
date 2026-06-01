package service

import "testing"

func TestParseFixPRURL(t *testing.T) {
	if got := parseFixPRURL([]byte(`{"pr_url":"https://github.com/o/r/pull/1","output":"done"}`)); got != "https://github.com/o/r/pull/1" {
		t.Fatalf("got %q", got)
	}
	if got := parseFixPRURL([]byte(`{"output":"done"}`)); got != "" {
		t.Fatalf("expected empty when absent, got %q", got)
	}
	if got := parseFixPRURL([]byte(`not json`)); got != "" {
		t.Fatalf("expected empty on bad json, got %q", got)
	}
}
