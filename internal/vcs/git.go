package vcs

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type Git struct {
	Dir   string
	Token string
}

func (g *Git) Run(ctx context.Context, args ...string) ([]byte, error) {
	cmd := g.command(ctx, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git %s: %s", args[0], strings.TrimSpace(string(out)))
	}
	return out, nil
}

func (g *Git) Output(ctx context.Context, args ...string) (string, error) {
	out, err := g.Run(ctx, args...)
	return strings.TrimSpace(string(out)), err
}

func (g *Git) Lines(ctx context.Context, args ...string) ([]string, error) {
	out, err := g.Output(ctx, args...)
	if err != nil {
		return nil, err
	}
	return splitNonEmpty(out), nil
}

func (g *Git) RevParse(ctx context.Context, ref string) (string, error) {
	return g.Output(ctx, "rev-parse", ref)
}

func (g *Git) MergeBase(ctx context.Context, a, b string) (string, error) {
	return g.Output(ctx, "merge-base", a, b)
}

func (g *Git) Clone(ctx context.Context, url string) error {
	_, err := g.Run(ctx, "clone", "--quiet", url, g.Dir)
	return err
}

func (g *Git) Fetch(ctx context.Context, ref string) error {
	_, err := g.Run(ctx, "fetch", "--quiet", "origin", ref)
	return err
}

func (g *Git) Checkout(ctx context.Context, ref string) error {
	_, err := g.Run(ctx, "checkout", "--quiet", ref)
	return err
}

func (g *Git) CheckoutDetached(ctx context.Context, ref string) error {
	_, err := g.Run(ctx, "checkout", "--quiet", "--detach", ref)
	return err
}

func (g *Git) CheckoutFiles(ctx context.Context, ref string, files []string) error {
	args := append([]string{"checkout", "--quiet", ref, "--"}, files...)
	_, err := g.Run(ctx, args...)
	return err
}

func (g *Git) Diff(ctx context.Context, baseSHA, headSHA string, paths ...string) ([]byte, error) {
	args := []string{"diff", "--binary", baseSHA, headSHA}
	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}
	return g.Run(ctx, args...)
}

func (g *Git) DiffNameOnly(ctx context.Context, baseSHA, headSHA string) ([]string, error) {
	return g.Lines(ctx, "diff", "--name-only", baseSHA, headSHA)
}

func (g *Git) DeletedFiles(ctx context.Context, baseSHA, headSHA string) ([]string, error) {
	return g.Lines(ctx, "diff", "--diff-filter=D", "--name-only", baseSHA, headSHA)
}

func (g *Git) Apply(ctx context.Context, patch []byte) error {
	cmd := g.command(ctx, "apply", "--index", "--binary", "-")
	cmd.Stdin = bytes.NewReader(patch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git apply: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func (g *Git) CommitWithIdentity(ctx context.Context, name, email, msg string) error {
	_, err := g.Run(ctx,
		"-c", "user.name="+name,
		"-c", "user.email="+email,
		"commit", "--quiet", "-m", msg,
	)
	return err
}

func (g *Git) Add(ctx context.Context, files []string) error {
	args := append([]string{"add", "--"}, files...)
	_, err := g.Run(ctx, args...)
	return err
}

func (g *Git) command(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.Dir
	if g.Token != "" {
		script, err := writeAskPass()
		if err == nil {
			cmd.Env = append(os.Environ(),
				"GIT_ASKPASS="+script,
				"GIT_ASKPASS_TOKEN="+g.Token,
				"GIT_TERMINAL_PROMPT=0",
			)
		}
	}
	return cmd
}

func writeAskPass() (string, error) {
	f, err := os.CreateTemp("", "git-askpass-*.sh")
	if err != nil {
		return "", err
	}
	script := "#!/bin/sh\nexec printf '%s' \"$GIT_ASKPASS_TOKEN\"\n"
	if _, err := f.WriteString(script); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}
	if err := os.Chmod(f.Name(), 0700); err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

func splitNonEmpty(value string) []string {
	lines := strings.Split(value, "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		if line = strings.TrimSpace(line); line != "" {
			result = append(result, line)
		}
	}
	return result
}
