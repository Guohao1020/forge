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

func TestMetadataPRURL(t *testing.T) {
	// The channel real agent runs actually use: pr_url pinned into issue metadata.
	if got := metadataPRURL([]byte(`{"pr_url":"https://github.com/o/r/pull/7","pr_number":7}`)); got != "https://github.com/o/r/pull/7" {
		t.Fatalf("got %q", got)
	}
	if got := metadataPRURL([]byte(`{"pr_number":7}`)); got != "" {
		t.Fatalf("expected empty when pr_url absent, got %q", got)
	}
	if got := metadataPRURL([]byte(`{}`)); got != "" {
		t.Fatalf("expected empty on empty bag, got %q", got)
	}
	if got := metadataPRURL(nil); got != "" {
		t.Fatalf("expected empty on nil metadata, got %q", got)
	}
	if got := metadataPRURL([]byte(`not json`)); got != "" {
		t.Fatalf("expected empty on bad json, got %q", got)
	}
}
