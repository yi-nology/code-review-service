package review

import (
	"strings"
	"testing"
)

func TestBuildSuggestionPatchSingleFinding(t *testing.T) {
	result := BuildSuggestionPatch("r1", []Finding{{
		Path:           "internal/app.go",
		StartLine:      10,
		Title:          "Use context",
		ExistingCode:   "\told := call()",
		SuggestionCode: "\tnew := call(ctx)",
	}}, nil)

	if result.GeneratedCount != 1 || result.SkippedCount != 0 {
		t.Fatalf("counts = generated %d skipped %d", result.GeneratedCount, result.SkippedCount)
	}
	want := "diff --git a/internal/app.go b/internal/app.go\n--- a/internal/app.go\n+++ b/internal/app.go\n@@ -10 +10 @@\n-\told := call()\n+\tnew := call(ctx)\n"
	if result.Patch != want {
		t.Fatalf("patch mismatch\nwant:\n%s\ngot:\n%s", want, result.Patch)
	}
}

func TestBuildSuggestionPatchMultipleFiles(t *testing.T) {
	result := BuildSuggestionPatch("r1", []Finding{
		{Path: "b.go", Line: 3, ExistingCode: "oldB", SuggestionCode: "newB"},
		{Path: "a.go", Line: 1, ExistingCode: "oldA\noldA2", SuggestionCode: "newA"},
	}, nil)

	if result.GeneratedCount != 2 {
		t.Fatalf("generated = %d, want 2", result.GeneratedCount)
	}
	if !strings.Contains(result.Patch, "diff --git a/a.go b/a.go\n") || !strings.Contains(result.Patch, "@@ -1,2 +1 @@\n") {
		t.Fatalf("expected a.go hunk in patch:\n%s", result.Patch)
	}
	if strings.Index(result.Patch, "diff --git a/a.go") > strings.Index(result.Patch, "diff --git a/b.go") {
		t.Fatalf("expected deterministic path ordering:\n%s", result.Patch)
	}
}

func TestBuildSuggestionPatchSkipsInvalidFindings(t *testing.T) {
	result := BuildSuggestionPatch("r1", []Finding{
		{Path: "", Line: 1, ExistingCode: "old", SuggestionCode: "new"},
		{Path: "a.go", Line: 1, ExistingCode: "", SuggestionCode: "new"},
		{Path: "a.go", Line: 1, ExistingCode: "old", SuggestionCode: ""},
		{Path: "a.go", ExistingCode: "old", SuggestionCode: "new"},
	}, nil)

	if result.GeneratedCount != 0 || result.SkippedCount != 4 {
		t.Fatalf("counts = generated %d skipped %d", result.GeneratedCount, result.SkippedCount)
	}
	reasons := strings.Join([]string{
		result.Skipped[0].Reason,
		result.Skipped[1].Reason,
		result.Skipped[2].Reason,
		result.Skipped[3].Reason,
	}, "|")
	for _, want := range []string{"missing path", "missing existing_code", "missing suggestion_code", "missing valid line"} {
		if !strings.Contains(reasons, want) {
			t.Fatalf("missing reason %q in %q", want, reasons)
		}
	}
}

func TestBuildSuggestionPatchSkipsOverlappingRanges(t *testing.T) {
	result := BuildSuggestionPatch("r1", []Finding{
		{Path: "a.go", Line: 10, ExistingCode: "old1\nold2", SuggestionCode: "new1"},
		{Path: "a.go", Line: 11, ExistingCode: "old2", SuggestionCode: "new2"},
	}, nil)

	if result.GeneratedCount != 1 || result.SkippedCount != 1 {
		t.Fatalf("counts = generated %d skipped %d", result.GeneratedCount, result.SkippedCount)
	}
	if result.Skipped[0].Reason != "overlaps another suggestion hunk" {
		t.Fatalf("skip reason = %q", result.Skipped[0].Reason)
	}
}

func TestBuildSuggestionPatchSelectedIndexes(t *testing.T) {
	result := BuildSuggestionPatch("r1", []Finding{
		{Path: "a.go", Line: 1, ExistingCode: "old1", SuggestionCode: "new1"},
		{Path: "b.go", Line: 2, ExistingCode: "old2", SuggestionCode: "new2"},
	}, []int{1})

	if result.GeneratedCount != 1 || result.Items[0].Index != 1 {
		t.Fatalf("items = %+v", result.Items)
	}
	if strings.Contains(result.Patch, "a.go") || !strings.Contains(result.Patch, "b.go") {
		t.Fatalf("unexpected selected patch:\n%s", result.Patch)
	}
}
