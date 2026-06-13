package vcs

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/client"
	"github.com/go-git/go-git/v6/plumbing/object"
	xhttp "github.com/go-git/go-git/v6/plumbing/transport/http"
	"github.com/go-git/go-git/v6/utils/merkletrie"
)

type Git struct {
	repo  *git.Repository
	wt    *git.Worktree
	token string
	dir   string
}

func Clone(ctx context.Context, dir, url, token string) (*Git, error) {
	opts := &git.CloneOptions{URL: url}
	if token != "" {
		opts.ClientOptions = tokenClientOpts(token)
	}
	repo, err := git.PlainCloneContext(ctx, dir, opts)
	if err != nil {
		return nil, fmt.Errorf("git clone: %w", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("git worktree: %w", err)
	}
	return &Git{repo: repo, wt: wt, token: token, dir: dir}, nil
}

func Open(dir, token string) (*Git, error) {
	repo, err := git.PlainOpen(dir)
	if err != nil {
		return nil, fmt.Errorf("git open: %w", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("git worktree: %w", err)
	}
	return &Git{repo: repo, wt: wt, token: token, dir: dir}, nil
}

func (g *Git) Dir() string { return g.dir }

func (g *Git) Fetch(ctx context.Context, refspec string) error {
	opts := &git.FetchOptions{
		RefSpecs: []config.RefSpec{config.RefSpec(refspec)},
		Force:    true,
	}
	if g.token != "" {
		opts.ClientOptions = tokenClientOpts(g.token)
	}
	return g.repo.FetchContext(ctx, opts)
}

func (g *Git) Checkout(_ context.Context, ref string) error {
	hash, err := g.resolveRef(ref)
	if err != nil {
		return err
	}
	return g.wt.Checkout(&git.CheckoutOptions{Hash: hash, Force: true})
}

func (g *Git) CheckoutDetached(ctx context.Context, ref string) error {
	return g.Checkout(ctx, ref)
}

func (g *Git) RevParse(_ context.Context, ref string) (string, error) {
	hash, err := g.resolveRef(ref)
	if err != nil {
		return "", err
	}
	return hash.String(), nil
}

func (g *Git) MergeBase(_ context.Context, a, b string) (string, error) {
	commitA, err := g.commit(a)
	if err != nil {
		return "", err
	}
	commitB, err := g.commit(b)
	if err != nil {
		return "", err
	}
	bases, err := commitA.MergeBase(commitB)
	if err != nil {
		return "", fmt.Errorf("merge-base %s %s: %w", a, b, err)
	}
	if len(bases) == 0 {
		return "", nil
	}
	return bases[0].Hash.String(), nil
}

func (g *Git) Diff(_ context.Context, baseSHA, headSHA string) ([]byte, error) {
	commitBase, err := g.commit(baseSHA)
	if err != nil {
		return nil, err
	}
	commitHead, err := g.commit(headSHA)
	if err != nil {
		return nil, err
	}
	patch, err := commitBase.Patch(commitHead)
	if err != nil {
		return nil, fmt.Errorf("diff %s..%s: %w", baseSHA, headSHA, err)
	}
	return []byte(patch.String()), nil
}

func (g *Git) DiffNameOnly(_ context.Context, baseSHA, headSHA string) ([]string, error) {
	changes, err := g.treeChanges(baseSHA, headSHA)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(changes))
	for _, c := range changes {
		result = append(result, changePath(c))
	}
	return result, nil
}

func (g *Git) DeletedFiles(_ context.Context, baseSHA, headSHA string) ([]string, error) {
	changes, err := g.treeChanges(baseSHA, headSHA)
	if err != nil {
		return nil, err
	}
	var result []string
	for _, c := range changes {
		action, err := c.Action()
		if err != nil {
			continue
		}
		if action == merkletrie.Delete {
			result = append(result, c.From.Name)
		}
	}
	return result, nil
}

func (g *Git) CheckoutFiles(_ context.Context, ref string, files []string) error {
	commitObj, err := g.commit(ref)
	if err != nil {
		return err
	}
	tree, err := commitObj.Tree()
	if err != nil {
		return err
	}
	var lastErr error
	for _, file := range files {
		treeFile, err := tree.File(file)
		if err != nil {
			lastErr = fmt.Errorf("file %s not found in tree: %w", file, err)
			continue
		}
		reader, err := treeFile.Blob.Reader()
		if err != nil {
			lastErr = fmt.Errorf("read blob for %s: %w", file, err)
			continue
		}
		fullPath := filepath.Join(g.dir, file)
		_ = os.MkdirAll(filepath.Dir(fullPath), 0o755)
		f, err := os.Create(fullPath)
		if err != nil {
			_ = reader.Close()
			lastErr = fmt.Errorf("create file %s: %w", file, err)
			continue
		}
		_, copyErr := io.Copy(f, reader)
		_ = f.Close()
		_ = reader.Close()
		if copyErr != nil {
			lastErr = fmt.Errorf("write file %s: %w", file, copyErr)
			continue
		}
		if _, err := g.wt.Add(file); err != nil {
			lastErr = fmt.Errorf("git add %s: %w", file, err)
		}
	}
	return lastErr
}

func (g *Git) Add(_ context.Context, files []string) error {
	for _, file := range files {
		if _, err := g.wt.Add(file); err != nil {
			return fmt.Errorf("git add %s: %w", file, err)
		}
	}
	return nil
}

func (g *Git) CommitWithIdentity(_ context.Context, name, email, msg string) error {
	_, err := g.wt.Commit(msg, &git.CommitOptions{
		Author: &object.Signature{
			Name:  name,
			Email: email,
			When:  time.Now(),
		},
		AllowEmptyCommits: true,
	})
	return err
}

func (g *Git) Close() error {
	return g.repo.Close()
}

func (g *Git) resolveRef(ref string) (plumbing.Hash, error) {
	hash, err := g.repo.ResolveRevision(plumbing.Revision(ref))
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("rev-parse %s: %w", ref, err)
	}
	return *hash, nil
}

func (g *Git) commit(ref string) (*object.Commit, error) {
	hash, err := g.resolveRef(ref)
	if err != nil {
		return nil, err
	}
	return g.repo.CommitObject(hash)
}

func (g *Git) treeChanges(baseSHA, headSHA string) (object.Changes, error) {
	commitBase, err := g.commit(baseSHA)
	if err != nil {
		return nil, err
	}
	commitHead, err := g.commit(headSHA)
	if err != nil {
		return nil, err
	}
	treeBase, err := commitBase.Tree()
	if err != nil {
		return nil, err
	}
	treeHead, err := commitHead.Tree()
	if err != nil {
		return nil, err
	}
	return object.DiffTree(treeBase, treeHead)
}

func tokenClientOpts(token string) []client.Option {
	return []client.Option{
		client.WithHTTPAuth(&xhttp.BasicAuth{Username: "token", Password: token}),
	}
}

func changePath(c *object.Change) string {
	if c.To.Name != "" {
		return c.To.Name
	}
	return c.From.Name
}
