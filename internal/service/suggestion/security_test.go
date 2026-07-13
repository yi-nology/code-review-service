package suggestion

import (
	"testing"

	"github.com/yi-nology/code-review-service/internal/service/review/domain"
)

// TestNormalizePatchPath_RejectsTraversal 验证路径逃逸防护：
// 补丁中的文件路径禁止穿越到 worktree 之外（../、绝对路径等）。
func TestNormalizePatchPath_RejectsTraversal(t *testing.T) {
	reject := []string{
		"../../../etc/passwd",
		"/etc/cron.d/evil",
		"..",
		"../",
		"src/../../../etc",
		"./../../secret",
		"a/../../b",
	}
	for _, p := range reject {
		t.Run("reject_"+p, func(t *testing.T) {
			if got, ok := normalizePatchPath(p); ok {
				t.Errorf("normalizePatchPath(%q) = %q, want rejection", p, got)
			}
		})
	}
}

// TestNormalizePatchPath_AcceptsValid 验证正常路径通过。
func TestNormalizePatchPath_AcceptsValid(t *testing.T) {
	cases := map[string]string{
		"src/app.go":           "src/app.go",
		"./src/app.go":         "src/app.go",
		"  src/a b.go  ":       "src/a b.go",
		"pkg/internal/util.js": "pkg/internal/util.js",
	}
	for in, want := range cases {
		got, ok := normalizePatchPath(in)
		if !ok {
			t.Errorf("normalizePatchPath(%q) rejected, want accept", in)
			continue
		}
		if got != want {
			t.Errorf("normalizePatchPath(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestIsSensitivePath 验证高风险路径（CI/build/hooks/secrets）被识别。
func TestIsSensitivePath(t *testing.T) {
	sensitive := []string{
		".github/workflows/ci.yml",
		".gitlab-ci.yml",
		"Dockerfile",
		"Makefile",
		".git/hooks/pre-commit",
		".ssh/authorized_keys",
		".env",
	}
	for _, p := range sensitive {
		if !isSensitivePath(p) {
			t.Errorf("isSensitivePath(%q) = false, want true", p)
		}
	}
	if isSensitivePath("src/app.go") {
		t.Error("isSensitivePath(src/app.go) = true, want false")
	}
}

// TestBuildSuggestionPatch_SkipsTraversalAndSensitive 验证补丁生成时
// 拒绝路径逃逸和敏感路径（不进入补丁头，防注入）。
func TestBuildSuggestionPatch_SkipsTraversalAndSensitive(t *testing.T) {
	findings := []domain.Finding{
		{Path: "../../../etc/passwd", StartLine: 1, ExistingCode: "x", SuggestionCode: "y"},      // traversal → skip
		{Path: ".github/workflows/ci.yml", StartLine: 1, ExistingCode: "x", SuggestionCode: "y"}, // sensitive → skip
		{Path: "src/safe.go", StartLine: 1, ExistingCode: "old", SuggestionCode: "new"},          // ok
	}
	result := BuildSuggestionPatch("r1", findings, nil)
	if result.GeneratedCount != 1 {
		t.Fatalf("expected 1 generated (only safe.go), got %d", result.GeneratedCount)
	}
	if result.SkippedCount != 2 {
		t.Fatalf("expected 2 skipped (traversal + sensitive), got %d", result.SkippedCount)
	}
	// 补丁中不应出现恶意路径
	for _, p := range []string{"../../../etc/passwd", "etc/passwd"} {
		if contains(result.Patch, p) {
			t.Errorf("patch must not contain traversal path %q, patch=\n%s", p, result.Patch)
		}
	}
	if !contains(result.Patch, "src/safe.go") {
		t.Errorf("patch should contain safe path, patch=\n%s", result.Patch)
	}
}

// TestIsProtectedBranch 验证主干分支保护（禁止自动推送/合入）。
func TestIsProtectedBranch(t *testing.T) {
	protected := []string{"main", "master", "develop", "dev", "release", "production", "prod", "MAIN", "Master"}
	for _, b := range protected {
		if !IsProtectedBranch(b) {
			t.Errorf("IsProtectedBranch(%q) = false, want true", b)
		}
	}
	safe := []string{"feature/x", "fix/bug-1", "review-service/fix-abc123", "hotfix"}
	for _, b := range safe {
		if IsProtectedBranch(b) {
			t.Errorf("IsProtectedBranch(%q) = true, want false", b)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || indexOf(s, substr) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
