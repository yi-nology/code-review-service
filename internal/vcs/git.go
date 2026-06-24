package vcs

import (
	"context"
	"fmt"

	"github.com/yi-nology/git-platform-sdk/gitbackend"
)

// Git is an alias for gitbackend.Repository so callers keep a stable, local
// type name while the implementation (and all git operations) live in the SDK.
type Git = gitbackend.Repository

// newBackend creates a native git backend. It panics when git is unavailable on
// the host since the service cannot function without it.
func newBackend() gitbackend.GitBackend {
	b, err := gitbackend.NewNativeGitBackend(gitbackend.Options{})
	if err != nil {
		panic(fmt.Sprintf("native git backend unavailable (git not found?): %v", err))
	}
	return b
}

// Clone clones url into dir and returns a Git bound to it.
func Clone(ctx context.Context, dir, url, token string, skipTLS bool) (*Git, error) {
	repo, err := gitbackend.CloneRepository(ctx, newBackend(), url, dir, gitbackend.NewTokenAuth(token), skipTLS)
	if err != nil {
		return nil, fmt.Errorf("git clone: %w", err)
	}
	return repo, nil
}

// Open wraps an already-present working tree at dir with token auth.
func Open(dir, token string) (*Git, error) {
	return gitbackend.OpenRepository(newBackend(), dir, gitbackend.NewTokenAuth(token), false), nil
}
