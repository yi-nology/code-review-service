package webhook

import (
	"testing"

	// gitlab backend 需 blank import 注册进 SDK registry（生产环境由 main.go
	// 统一引入 backends/all，测试此处自行引入）。
	_ "github.com/yi-nology/git-platform-sdk/backends/gitlab"
)

// GitCode payload 为 GitLab 风格；本测试锁定经 SDK gitlab-family 解析后的
// ChangeEvent 字段与历史手写 normalizer 的行为一致。
func TestGitCodeNormalizeMergeRequest(t *testing.T) {
	raw := []byte(`{
		"object_kind": "merge_request",
		"user": {"username": "alice", "name": "Alice", "avatar_url": "http://a/alice.png"},
		"project": {"id": 42, "path_with_namespace": "owner/repo"},
		"object_attributes": {
			"iid": 7,
			"title": "t",
			"action": "open",
			"source_branch": "feat",
			"target_branch": "main",
			"last_commit": {"id": "head-sha"},
			"diff_refs": {"base_sha": "base-sha", "start_sha": "start-sha", "head_sha": "head-sha"}
		}
	}`)

	ce, err := (&GitCodeNormalizer{}).Normalize(raw, "Merge Request Hook")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if ce == nil {
		t.Fatal("expected event, got nil")
	}
	if ce.Provider != "gitcode" || ce.EventType != "merge_request" || ce.Action != "opened" {
		t.Errorf("unexpected platform/eventType/action: %s/%s/%s", ce.Provider, ce.EventType, ce.Action)
	}
	if ce.Number != "7" {
		t.Errorf("Number = %q, want 7", ce.Number)
	}
	if ce.BaseSHA != "base-sha" || ce.HeadSHA != "head-sha" || ce.StartSHA != "start-sha" {
		t.Errorf("unexpected SHAs: base=%s head=%s start=%s", ce.BaseSHA, ce.HeadSHA, ce.StartSHA)
	}
	if ce.Repository != "owner/repo" {
		t.Errorf("Repository = %q", ce.Repository)
	}
	if ce.RepositoryID != "42" {
		t.Errorf("RepositoryID = %q, want 42", ce.RepositoryID)
	}
	if ce.Sender != "alice" || ce.Author != "alice" {
		t.Errorf("sender=%q author=%q, want alice", ce.Sender, ce.Author)
	}
}

func TestGitCodeNormalizePush(t *testing.T) {
	raw := []byte(`{
		"object_kind": "push",
		"user_username": "bob",
		"project": {"id": 42, "path_with_namespace": "owner/repo"},
		"ref": "refs/heads/main",
		"after": "sha-after"
	}`)

	ce, err := (&GitCodeNormalizer{}).Normalize(raw, "Push Hook")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if ce == nil {
		t.Fatal("expected event, got nil")
	}
	if ce.EventType != "push" {
		t.Errorf("EventType = %q, want push", ce.EventType)
	}
	if ce.HeadSHA != "sha-after" {
		t.Errorf("HeadSHA = %q, want sha-after", ce.HeadSHA)
	}
	if ce.RepositoryID != "42" {
		t.Errorf("RepositoryID = %q, want 42", ce.RepositoryID)
	}
	if ce.Sender != "bob" {
		t.Errorf("Sender = %q, want bob", ce.Sender)
	}
}

// payload 缺 object_kind 时按 eventType header 补写，仍可解析。
func TestGitCodeNormalizeWithoutObjectKind(t *testing.T) {
	raw := []byte(`{
		"user": {"username": "alice", "name": "Alice"},
		"project": {"id": 42, "path_with_namespace": "owner/repo"},
		"object_attributes": {
			"iid": 9,
			"action": "open",
			"last_commit": {"id": "h"},
			"diff_refs": {"base_sha": "b", "start_sha": "s", "head_sha": "h"}
		}
	}`)

	ce, err := (&GitCodeNormalizer{}).Normalize(raw, "Merge Request Hook")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if ce == nil {
		t.Fatal("expected event, got nil")
	}
	if ce.Number != "9" {
		t.Errorf("Number = %q, want 9", ce.Number)
	}
}

func TestGitCodeNormalizeIgnoresUnknownHook(t *testing.T) {
	ce, err := (&GitCodeNormalizer{}).Normalize([]byte(`{}`), "Tag Push Hook")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if ce != nil {
		t.Errorf("expected nil event for tag push, got %+v", ce)
	}
}
