package review_service

import "testing"

func TestParseFindingIndexes(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		total   int
		want    []int
		wantErr bool
	}{
		{name: "empty means all", raw: "", total: 3, want: nil},
		{name: "valid list", raw: "0, 2,2", total: 3, want: []int{0, 2}},
		{name: "non number", raw: "a", total: 3, wantErr: true},
		{name: "empty part", raw: "0,,1", total: 3, wantErr: true},
		{name: "negative", raw: "-1", total: 3, wantErr: true},
		{name: "out of range", raw: "3", total: 3, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseFindingIndexes(tt.raw, tt.total)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseFindingIndexes() error = %v, wantErr %v", err, tt.wantErr)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("parseFindingIndexes() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("parseFindingIndexes() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}
