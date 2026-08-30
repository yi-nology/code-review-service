package ocr

import (
	"reflect"
	"testing"
)

func TestSanitizeDelegatePaths(t *testing.T) {
	tests := []struct {
		name    string
		in      []string
		want    []string
		wantErr bool
	}{
		{
			name: "empty paths pass through",
			in:   nil,
			want: []string{},
		},
		{
			name: "normal relative paths kept in order",
			in:   []string{"internal/foo.go", "docs/guide.md", "a b/c.go"},
			want: []string{"internal/foo.go", "docs/guide.md", "a b/c.go"},
		},
		{
			name: "empty entries skipped",
			in:   []string{"", "main.go", ""},
			want: []string{"main.go"},
		},
		{
			name:    "leading dash rejected (option injection)",
			in:      []string{"--repo", "/etc"},
			wantErr: true,
		},
		{
			name:    "single dash rejected",
			in:      []string{"-"},
			wantErr: true,
		},
		{
			name:    "absolute path rejected",
			in:      []string{"/etc/passwd"},
			wantErr: true,
		},
		{
			name:    "parent escape rejected",
			in:      []string{"../outside.go"},
			wantErr: true,
		},
		{
			name:    "nested parent escape rejected",
			in:      []string{"a/../../outside.go"},
			wantErr: true,
		},
		{
			name: "inner dot-dot staying inside kept",
			in:   []string{"a/../b.go"},
			want: []string{"a/../b.go"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sanitizeDelegatePaths(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("sanitizeDelegatePaths(%q) expected error, got %v", tt.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("sanitizeDelegatePaths(%q) unexpected error: %v", tt.in, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("sanitizeDelegatePaths(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
