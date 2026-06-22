package workflow

import (
	"testing"
)

func TestExtractChangedFiles(t *testing.T) {
	diff := `diff --git a/foo.go b/foo.go
index 123..456 100644
--- a/foo.go
+++ b/foo.go
@@ -1,3 +1,5 @@
+package main
+
 func Foo() {}
diff --git a/bar/baz_test.go b/bar/baz_test.go
new file mode 100644
--- /dev/null
+++ b/bar/baz_test.go
@@ -0,0 +1,2 @@
+package bar
+
diff --git a/removed.go b/removed.go
deleted file mode 100644
--- a/removed.go
+++ /dev/null`

	files := extractChangedFiles(diff)
	// 期望:foo.go, bar/baz_test.go（removed.go 因 +++ /dev/null 被排除）
	if len(files) != 2 {
		t.Fatalf("expected 2 changed files, got %d: %v", len(files), files)
	}
	want := map[string]bool{"foo.go": true, "bar/baz_test.go": true}
	for _, f := range files {
		if !want[f] {
			t.Errorf("unexpected file: %s", f)
		}
	}
}

func TestExtractChangedFilesEmpty(t *testing.T) {
	if files := extractChangedFiles(""); len(files) != 0 {
		t.Fatalf("expected 0 files for empty diff, got %d", len(files))
	}
	if files := extractChangedFiles("no diff here\njust text"); len(files) != 0 {
		t.Fatalf("expected 0 files for non-diff text, got %d", len(files))
	}
}

func TestDiscoverProjectStandardsEmptyDir(t *testing.T) {
	if m := discoverProjectStandards(""); m != nil {
		t.Fatalf("expected nil for empty dir, got %v", m)
	}
	if m := discoverProjectStandards("/nonexistent/path/that/should/not/exist"); m != nil {
		t.Fatalf("expected nil for nonexistent dir, got %v", m)
	}
}
