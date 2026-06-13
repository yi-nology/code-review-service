package provider

import (
	"reflect"
	"testing"
)

func TestPullRequestFetchRefsGitCodePrefersSourceBranch(t *testing.T) {
	got := pullRequestFetchRefs("gitcode", 10, "feat/api-access-log", "abc123")
	want := []string{
		"feat/api-access-log:refs/heads/pr-10",
		"pull/10/head:refs/heads/pr-10",
		"refs/pull/10/head:refs/heads/pr-10",
		"abc123:refs/heads/pr-10",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("pullRequestFetchRefs() = %#v, want %#v", got, want)
	}
}

func TestPullRequestFetchRefsGitLabUsesMergeRequestRef(t *testing.T) {
	got := pullRequestFetchRefs("gitlab", 7, "feature", "")
	want := []string{
		"merge-requests/7/head:refs/heads/pr-7",
		"feature:refs/heads/pr-7",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("pullRequestFetchRefs() = %#v, want %#v", got, want)
	}
}
