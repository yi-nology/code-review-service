package vcs

import (
	"context"
	"fmt"

	"github.com/yi-nology/git-platform-sdk/gitbackend"
)

type Git struct {
	backend  gitbackend.GitBackend
	dir      string
	auth     gitbackend.AuthConfig
	insecure bool
}

func newBackend() gitbackend.GitBackend {
	b, err := gitbackend.NewGitBackend(gitbackend.Options{Type: "native"})
	if err != nil {
		b = gitbackend.NewGoGitBackend(gitbackend.Options{})
	}
	return b
}

func tokenAuth(token string) gitbackend.AuthConfig {
	if token == "" {
		return gitbackend.AuthConfig{Type: gitbackend.AuthNone}
	}
	return gitbackend.AuthConfig{
		Type:     gitbackend.AuthHTTPBasic,
		Username: "token",
		Password: token,
	}
}

func Clone(ctx context.Context, dir, url, token string, skipTLS bool) (*Git, error) {
	b := newBackend()
	auth := tokenAuth(token)
	err := b.Clone(ctx, gitbackend.CloneOptions{
		URL:             url,
		Path:            dir,
		Auth:            auth,
		InsecureSkipTLS: skipTLS,
	})
	if err != nil {
		return nil, fmt.Errorf("git clone: %w", err)
	}
	return &Git{backend: b, dir: dir, auth: auth, insecure: skipTLS}, nil
}

func Open(dir, token string) (*Git, error) {
	return &Git{
		backend: newBackend(),
		dir:     dir,
		auth:    tokenAuth(token),
	}, nil
}

func (g *Git) Dir() string { return g.dir }

func (g *Git) Fetch(ctx context.Context, refspec string) error {
	_, err := g.backend.Fetch(ctx, gitbackend.FetchOptions{
		RepoPath:        g.dir,
		Remote:          "origin",
		Branches:        []string{refspec},
		Auth:            g.auth,
		InsecureSkipTLS: g.insecure,
	})
	return err
}

func (g *Git) FetchAll(ctx context.Context) error {
	_, err := g.backend.Fetch(ctx, gitbackend.FetchOptions{
		RepoPath:        g.dir,
		Remote:          "origin",
		Tags:            true,
		Auth:            g.auth,
		InsecureSkipTLS: g.insecure,
	})
	if err != nil {
		return err
	}
	branches, branchErr := g.backend.ListRemoteBranches(ctx, g.dir, "origin")
	if branchErr != nil {
		return fmt.Errorf("list remote branches: %w", branchErr)
	}
	for _, branch := range branches {
		_, _ = g.backend.Fetch(ctx, gitbackend.FetchOptions{
			RepoPath:        g.dir,
			Remote:          "origin",
			Branches:        []string{branch},
			Tags:            true,
			Auth:            g.auth,
			InsecureSkipTLS: g.insecure,
		})
	}
	return nil
}

func (g *Git) Checkout(ctx context.Context, ref string) error {
	return g.backend.CheckoutRef(ctx, g.dir, ref)
}

func (g *Git) CheckoutDetached(ctx context.Context, ref string) error {
	return g.backend.CheckoutRef(ctx, g.dir, ref)
}

func (g *Git) RevParse(ctx context.Context, ref string) (string, error) {
	return g.backend.RevParse(ctx, g.dir, ref)
}

func (g *Git) MergeBase(ctx context.Context, a, b string) (string, error) {
	return g.backend.MergeBase(ctx, g.dir, a, b)
}

func (g *Git) Diff(ctx context.Context, baseSHA, headSHA string) ([]byte, error) {
	patch, err := g.backend.Diff(ctx, g.dir, gitbackend.DiffOptions{From: baseSHA, To: headSHA})
	if err != nil {
		return nil, err
	}
	return []byte(patch), nil
}

func (g *Git) DiffNameOnly(ctx context.Context, baseSHA, headSHA string) ([]string, error) {
	return g.backend.DiffNames(ctx, g.dir, baseSHA, headSHA)
}

func (g *Git) DeletedFiles(ctx context.Context, baseSHA, headSHA string) ([]string, error) {
	return g.backend.DeletedFiles(ctx, g.dir, baseSHA, headSHA)
}

func (g *Git) CheckoutFiles(ctx context.Context, ref string, files []string) error {
	return g.backend.CheckoutFiles(ctx, g.dir, ref, files)
}

func (g *Git) CommitWithIdentity(ctx context.Context, name, email, msg string) error {
	return g.backend.CommitWithIdentity(ctx, g.dir, name, email, msg)
}

func (g *Git) Close() error {
	return nil
}
