package settings_service

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLocateViewerSessionMatchesNearestSession(t *testing.T) {
	home := t.TempDir()
	workspace := "/tmp/review-workspace-123"
	slug := viewerRepoSlug(workspace)
	sessionDir := filepath.Join(home, ".opencodereview", "sessions", slug)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	startedAt := time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC)
	oldSession := `{"cwd":"/tmp/review-workspace-123","sessionId":"old","timestamp":"2026-06-28T09:00:00Z","type":"session_start"}`
	nearSession := `{"cwd":"/tmp/review-workspace-123","sessionId":"near","timestamp":"2026-06-28T10:00:04Z","type":"session_start"}`
	if err := os.WriteFile(filepath.Join(sessionDir, "old.jsonl"), []byte(oldSession+"\n"), 0o644); err != nil {
		t.Fatalf("write old session: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sessionDir, "near.jsonl"), []byte(nearSession+"\n"), 0o644); err != nil {
		t.Fatalf("write near session: %v", err)
	}

	location := locateViewerSession(home, workspace, startedAt)
	if !location.Matched || location.SessionID != "near" {
		t.Fatalf("location = %+v, want matched near session", location)
	}
	if location.Path != "/r/tmp-review-workspace-123/near" {
		t.Fatalf("path = %q", location.Path)
	}
}

func TestLocateViewerSessionFallsBackToRepoPathWhenNoCloseMatch(t *testing.T) {
	home := t.TempDir()
	workspace := "/tmp/review-workspace-123"
	slug := viewerRepoSlug(workspace)
	sessionDir := filepath.Join(home, ".opencodereview", "sessions", slug)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	session := `{"cwd":"/tmp/review-workspace-123","sessionId":"far","timestamp":"2026-06-28T09:00:00Z","type":"session_start"}`
	if err := os.WriteFile(filepath.Join(sessionDir, "far.jsonl"), []byte(session+"\n"), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}

	location := locateViewerSession(home, workspace, time.Date(2026, 6, 28, 11, 0, 0, 0, time.UTC))
	if location.Matched || location.SessionID != "" {
		t.Fatalf("location = %+v, want repo fallback", location)
	}
	if location.Path != "/r/tmp-review-workspace-123" {
		t.Fatalf("path = %q", location.Path)
	}
}

func TestLocateViewerSessionUsesLatestSessionWithoutStartedAt(t *testing.T) {
	home := t.TempDir()
	workspace := "/tmp/review-workspace-123"
	slug := viewerRepoSlug(workspace)
	sessionDir := filepath.Join(home, ".opencodereview", "sessions", slug)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	firstSession := `{"cwd":"/tmp/review-workspace-123","sessionId":"first","timestamp":"2026-06-28T09:00:00Z","type":"session_start"}`
	latestSession := `{"cwd":"/tmp/review-workspace-123","sessionId":"latest","timestamp":"2026-06-28T10:00:00Z","type":"session_start"}`
	if err := os.WriteFile(filepath.Join(sessionDir, "first.jsonl"), []byte(firstSession+"\n"), 0o644); err != nil {
		t.Fatalf("write first session: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sessionDir, "latest.jsonl"), []byte(latestSession+"\n"), 0o644); err != nil {
		t.Fatalf("write latest session: %v", err)
	}

	location := locateViewerSession(home, workspace, time.Time{})
	if !location.Matched || location.SessionID != "latest" {
		t.Fatalf("location = %+v, want latest session", location)
	}
}

func TestLocateViewerSessionGlobalSearchWithoutWorkspaceDir(t *testing.T) {
	home := t.TempDir()
	slugA := "tmp-review-workspace-aaa"
	slugB := "tmp-review-workspace-bbb"
	for _, slug := range []string{slugA, slugB} {
		dir := filepath.Join(home, ".opencodereview", "sessions", slug)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", slug, err)
		}
	}
	earlySession := `{"cwd":"/tmp/review-workspace-aaa","sessionId":"too-early","timestamp":"2026-06-28T09:50:00Z","type":"session_start"}`
	nearSession := `{"cwd":"/tmp/review-workspace-aaa","sessionId":"near-match","timestamp":"2026-06-28T10:05:00Z","type":"session_start"}`
	farSession := `{"cwd":"/tmp/review-workspace-bbb","sessionId":"too-late","timestamp":"2026-06-28T11:30:00Z","type":"session_start"}`
	if err := os.WriteFile(filepath.Join(home, ".opencodereview", "sessions", slugA, "early.jsonl"), []byte(earlySession+"\n"), 0o644); err != nil {
		t.Fatalf("write early session: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, ".opencodereview", "sessions", slugA, "near.jsonl"), []byte(nearSession+"\n"), 0o644); err != nil {
		t.Fatalf("write near session: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, ".opencodereview", "sessions", slugB, "far.jsonl"), []byte(farSession+"\n"), 0o644); err != nil {
		t.Fatalf("write far session: %v", err)
	}

	reviewStartedAt := time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC)
	location := locateViewerSession(home, "", reviewStartedAt)
	if !location.Matched || location.SessionID != "near-match" {
		t.Fatalf("location = %+v, want near-match session", location)
	}
	if location.RepoSlug != slugA {
		t.Fatalf("slug = %q, want %q", location.RepoSlug, slugA)
	}
}
