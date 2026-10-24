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
			// 严格档拒绝一切显式 ".." 段（即便能被抵消掉、文本上不越界）：
			// ".." 的抵消是纯文本语义，看不见符号链接。
			name:    "inner dot-dot rejected",
			in:      []string{"a/../b.go"},
			wantErr: true,
		},
		{
			// Windows 风格分隔符：Linux 上 "\" 只是普通字符，按平台解释会漏判。
			name:    "windows backslash traversal rejected",
			in:      []string{`..\..\etc\passwd`},
			wantErr: true,
		},
		{
			name:    "windows drive path rejected",
			in:      []string{`C:\Windows\System32\config`},
			wantErr: true,
		},
		{
			name:    "unc path rejected",
			in:      []string{`\\host\share\file`},
			wantErr: true,
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
