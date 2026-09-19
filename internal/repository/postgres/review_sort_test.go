package postgres

import "testing"

func TestReviewSortClause(t *testing.T) {
	tests := []struct {
		name    string
		by      string
		order   string
		wantSQL string
	}{
		// 全部分支追加 id tiebreaker：同键行分页间顺序稳定。
		{"score desc", "score", "desc", "score DESC, id DESC"},
		{"score asc", "score", "asc", "score ASC, id ASC"},
		{"score case-insensitive", "SCORE", "ASC", "score ASC, id ASC"},
		{"updated_at desc", "updated_at", "desc", "updated_at DESC, id DESC"},
		{"findings uses jsonb length", "findings", "asc", "COALESCE(jsonb_array_length(response->'findings'), 0) ASC, id ASC"},
		{"findings trimmed", "  findings  ", "desc", "COALESCE(jsonb_array_length(response->'findings'), 0) DESC, id DESC"},
		{"empty by falls back", "", "desc", "updated_at DESC, id DESC"},
		{"empty order defaults desc", "score", "", "score DESC, id DESC"},
		{"injection fragment rejected", "score; DROP TABLE reviews--", "desc", "updated_at DESC, id DESC"},
		{"raw predicate rejected", "1=1", "asc", "updated_at DESC, id DESC"},
		{"arbitrary order ignored", "score", "sideways", "score DESC, id DESC"},
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
