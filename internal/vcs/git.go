package vcs

import (
	"context"
	"fmt"
	"sync"

	"github.com/yi-nology/git-platform-sdk/gitbackend"
)

// Git is an alias for gitbackend.Repository so callers keep a stable, local
// type name while the implementation (and all git operations) live in the SDK.
type Git = gitbackend.Repository

var (
	backendOnce sync.Once
	backendInst gitbackend.GitBackend
	backendErr  error
)

// getBackend returns a cached native git backend, initializing once.
func getBackend() (gitbackend.GitBackend, error) {
	backendOnce.Do(func() {
		backendInst, backendErr = gitbackend.NewNativeGitBackend(gitbackend.Options{})
	})
	return backendInst, backendErr
}

// Clone clones url into dir and returns a Git bound to it.
func Clone(ctx context.Context, dir, url, token string, skipTLS bool) (*Git, error) {
	b, err := getBackend()
	if err != nil {
		return nil, fmt.Errorf("git backend unavailable: %w", err)
	}
	repo, err := gitbackend.CloneRepository(ctx, b, url, dir, gitbackend.NewTokenAuth(token), skipTLS)
	if err != nil {
		return nil, fmt.Errorf("git clone: %w", err)
	}
	return repo, nil
}

// Open wraps an already-present working tree at dir with token auth.
func Open(dir, token string) (*Git, error) {
	b, err := getBackend()
	if err != nil {
		return nil, fmt.Errorf("git backend unavailable: %w", err)
	}
	return gitbackend.OpenRepository(b, dir, gitbackend.NewTokenAuth(token), false), nil
}
