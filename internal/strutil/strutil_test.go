package strutil

import "testing"

func TestTruncate(t *testing.T) {
	tests := []struct {
		name string
		s    string
		max  int
		want string
	}{
		{"short ascii", "hello", 10, "hello"},
		{"exact fit", "hello", 5, "hello"},
		{"ascii cut", "hello world", 8, "hello w…"},
		{"multibyte uncut", "你好世界", 4, "你好世界"},
		{"multibyte cut keeps runes intact", "你好世界啊", 4, "你好世…"},
		{"mixed cut", "ab你好cd", 5, "ab你好…"},
		{"max one", "hello", 1, "…"},
		{"max zero", "hello", 0, ""},
		{"max negative", "hello", -3, ""},
		{"empty", "", 5, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Truncate(tt.s, tt.max)
			if got != tt.want {
				t.Fatalf("Truncate(%q, %d) = %q, want %q", tt.s, tt.max, got, tt.want)
			}
			if n := len([]rune(got)); tt.max > 0 && n > tt.max {
				t.Fatalf("Truncate(%q, %d) has %d runes, exceeds max", tt.s, tt.max, n)
			}
		})
	}
}
