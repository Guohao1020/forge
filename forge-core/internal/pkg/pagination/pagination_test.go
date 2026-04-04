package pagination

import (
	"testing"
)

func TestNewResult(t *testing.T) {
	tests := []struct {
		name       string
		total      int64
		page       int
		perPage    int
		wantPages  int
	}{
		{"exact division", 100, 1, 20, 5},
		{"remainder", 101, 1, 20, 6},
		{"single page", 5, 1, 20, 1},
		{"empty", 0, 1, 20, 0},
		{"one item", 1, 1, 20, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Params{Page: tt.page, PerPage: tt.perPage, Offset: (tt.page - 1) * tt.perPage}
			r := NewResult(nil, tt.total, p)
			if r.TotalPages != tt.wantPages {
				t.Errorf("TotalPages = %d, want %d", r.TotalPages, tt.wantPages)
			}
			if r.Total != tt.total {
				t.Errorf("Total = %d, want %d", r.Total, tt.total)
			}
		})
	}
}

func TestParamsOffset(t *testing.T) {
	tests := []struct {
		page    int
		perPage int
		offset  int
	}{
		{1, 20, 0},
		{2, 20, 20},
		{3, 10, 20},
		{5, 50, 200},
	}

	for _, tt := range tests {
		p := Params{Page: tt.page, PerPage: tt.perPage, Offset: (tt.page - 1) * tt.perPage}
		if p.Offset != tt.offset {
			t.Errorf("page=%d perPage=%d: offset=%d, want %d", tt.page, tt.perPage, p.Offset, tt.offset)
		}
	}
}
