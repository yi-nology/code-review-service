package review

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yi-nology/code-review-service/internal/service/review/domain"
)

func TestBuildSuggestionPatchSingleFinding(t *testing.T) {
	result := BuildSuggestionPatch("r1", []domain.Finding{{
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
	result := BuildSuggestionPatch("r1", []domain.Finding{
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
	result := BuildSuggestionPatch("r1", []domain.Finding{
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
	result := BuildSuggestionPatch("r1", []domain.Finding{
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
	result := BuildSuggestionPatch("r1", []domain.Finding{
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

func TestRunGitApplyCheck(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	run("init")
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\n\nfunc main() {\n\told := call()\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	okPatch := "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -4 +4 @@\n-\told := call()\n+\tnew := call(ctx)\n"
	output, err := runGitApplyCheck(t.Context(), dir, okPatch)
	if err != nil {
		t.Fatalf("runGitApplyCheck() error = %v output=%q", err, output)
	}

	badPatch := "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -4 +4 @@\n-\tmissing := call()\n+\tnew := call(ctx)\n"
	output, err = runGitApplyCheck(t.Context(), dir, badPatch)
	if err == nil {
		t.Fatal("expected invalid patch to fail")
	}
	if output == "" {
		t.Fatal("expected git apply output for invalid patch")
	}
}

func TestApplyPatchCommitCreatesRepairBranch(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init")
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\n\nfunc main() {\n\told := call()\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "a.go")
	run("-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-m", "initial")

	patch := "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -4 +4 @@\n-\told := call()\n+\tnew := call(ctx)\n"
	output, commitSHA, err := applyPatchCommit(t.Context(), dir, "review-service/fix-r1", patch, false, suggestionPatchPushAuth{})
	if err != nil {
		t.Fatalf("applyPatchCommit() error = %v output=%q", err, output)
	}
	if commitSHA == "" {
		t.Fatal("commit SHA is empty")
	}
	if got := run("rev-parse", "--abbrev-ref", "HEAD"); got != "review-service/fix-r1" {
		t.Fatalf("branch = %q", got)
	}
	content, err := os.ReadFile(filepath.Join(dir, "a.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "\tnew := call(ctx)") {
		t.Fatalf("patch not applied:\n%s", content)
	}
}
