package writeback

import "testing"

func TestParseUnifiedDiffIndexesOldAndNewLines(t *testing.T) {
	diff := `diff --git a/internal/example.go b/internal/example.go
index 1111111..2222222 100644
--- a/internal/example.go
+++ b/internal/example.go
@@ -10,4 +10,5 @@
 context
-old value
+new value
+added value
 tail
`

	index, err := ParseUnifiedDiff(diff)
	if err != nil {
		t.Fatalf("ParseUnifiedDiff() error = %v", err)
	}

	tests := []struct {
		side Side
		line int
		want bool
	}{
		{SideOld, 10, true},
		{SideNew, 10, true},
		{SideOld, 11, true},
		{SideNew, 11, true},
		{SideNew, 12, true},
		{SideOld, 12, true},
		{SideNew, 13, true},
		{SideOld, 13, false},
	}
	for _, tt := range tests {
		if got := index.Contains("internal/example.go", tt.side, tt.line); got != tt.want {
			t.Errorf("Contains(side=%s, line=%d) = %v, want %v", tt.side, tt.line, got, tt.want)
		}
	}
}

func TestDiffIndexContainsRange(t *testing.T) {
	index, err := ParseUnifiedDiff("--- a/a.go\n+++ b/a.go\n@@ -1,2 +1,3 @@\n same\n+added\n tail\n")
	if err != nil {
		t.Fatalf("ParseUnifiedDiff() error = %v", err)
	}

	if !index.ContainsRange("a.go", LineRange{Start: 1, End: 3, Side: SideNew}) {
		t.Fatal("expected new-side range to be present")
	}
	if index.ContainsRange("a.go", LineRange{Start: 1, End: 3, Side: SideOld}) {
		t.Fatal("did not expect old-side range to be present")
	}
}
