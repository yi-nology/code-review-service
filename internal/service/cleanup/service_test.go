package cleanup

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWorkspaceCleaner_RemovesExpiredDirs(t *testing.T) {
	tmpDir := t.TempDir()

	oldDir := filepath.Join(tmpDir, "review-workspace-old")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}

	pastTime := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(oldDir, pastTime, pastTime); err != nil {
		t.Fatal(err)
	}

	freshDir := filepath.Join(tmpDir, "review-workspace-fresh")
	if err := os.MkdirAll(freshDir, 0o755); err != nil {
		t.Fatal(err)
	}

	nonMatchingDir := filepath.Join(tmpDir, "other-dir")
	if err := os.MkdirAll(nonMatchingDir, 0o755); err != nil {
		t.Fatal(err)
	}
	oldNonMatch := time.Now().Add(-48 * time.Hour)
	_ = os.Chtimes(nonMatchingDir, oldNonMatch, oldNonMatch)

	cleaner := NewWorkspaceCleaner(
		1*time.Hour,
	)
	cleaner.baseDir = tmpDir

	cleaner.cleanup()

	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Errorf("expected old workspace to be removed, but it still exists")
	}
	if _, err := os.Stat(freshDir); err != nil {
		t.Errorf("expected fresh workspace to remain, but it was removed: %v", err)
	}
	if _, err := os.Stat(nonMatchingDir); err != nil {
		t.Errorf("expected non-matching dir to remain, but it was removed: %v", err)
	}
}

func TestWorkspaceCleaner_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	cleaner := NewWorkspaceCleaner(1 * time.Hour)
	cleaner.baseDir = tmpDir

	cleaner.cleanup()
}

func TestWorkspaceCleaner_NonExistentDir(t *testing.T) {
	cleaner := NewWorkspaceCleaner(1 * time.Hour)
	cleaner.baseDir = "/nonexistent/path/that/does/not/exist"

	cleaner.cleanup()
}
