package postgres

import "testing"

func TestReviewSortClause(t *testing.T) {
	tests := []struct {
		name    string
		by      string
		order   string
		wantSQL string
	}{
		{"score desc", "score", "desc", "score DESC"},
		{"score asc", "score", "asc", "score ASC"},
		{"score case-insensitive", "SCORE", "ASC", "score ASC"},
		{"updated_at desc", "updated_at", "desc", "updated_at DESC"},
		{"findings uses jsonb length", "findings", "asc", "COALESCE(jsonb_array_length(response->'findings'), 0) ASC"},
		{"findings trimmed", "  findings  ", "desc", "COALESCE(jsonb_array_length(response->'findings'), 0) DESC"},
		{"empty by falls back", "", "desc", "created_at DESC"},
		{"empty order defaults desc", "score", "", "score DESC"},
		{"injection fragment rejected", "score; DROP TABLE reviews--", "desc", "created_at DESC"},
		{"raw predicate rejected", "1=1", "asc", "created_at DESC"},
		{"arbitrary order ignored", "score", "sideways", "score DESC"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reviewSortClause(tt.by, tt.order)
			if got != tt.wantSQL {
				t.Errorf("reviewSortClause(%q, %q) = %q, want %q", tt.by, tt.order, got, tt.wantSQL)
			}
		})
	}
}
