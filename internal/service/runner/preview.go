package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/yi-nology/code-review-service/internal/service/review"
	"github.com/oekit/concurrency/async"
)

func runPreview(ctx context.Context, cfg OpenCodeReviewConfig, workspace review.Workspace) (*review.PreviewResult, error) {
	timeout := time.Duration(cfg.TimeoutMinutes) * time.Minute
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout+30*time.Second)
	defer cancel()

	fromRef := workspace.BaseSHA
	if fromRef == "" {
		fromRef = "HEAD"
	}

	args := []string{
		"review",
		"--preview",
		"--format", "json",
		"--repo", workspace.Dir,
		"--from", fromRef,
		"--to", workspace.HeadSHA,
	}
	rulePath := cfg.RulePath
	if strings.TrimSpace(workspace.RulePath) != "" {
		rulePath = workspace.RulePath
	}
	if strings.TrimSpace(rulePath) != "" {
		args = append(args, "--rule", rulePath)
	}

	cmd := exec.CommandContext(ctx, cfg.Binary, args...)
	cmd.Dir = workspace.Dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	env := os.Environ()
	env = AppendLLMEnv(env, cfg)
	cmd.Env = env

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	done := make(chan struct{})
	async.GoSafe(func() {
		select {
		case <-ctx.Done():
			if cmd.Process != nil {
				_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			}
		case <-done:
		}
	})

	err := cmd.Run()
	close(done)
	if err != nil {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("预览失败: %s", msg)
	}

	output := bytes.TrimSpace(stdout.Bytes())
	if len(output) == 0 {
		return &review.PreviewResult{Files: []string{}}, nil
	}

	var files []string
	if err := json.Unmarshal(output, &files); err != nil {
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		files = make([]string, 0, len(lines))
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				files = append(files, line)
			}
		}
	}

	return &review.PreviewResult{Files: files}, nil
}

// appendLLMEnv 已迁移到 config.go 中的 AppendLLMEnv
