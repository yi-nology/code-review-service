package runner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/yi-nology/code-review-service/internal/service/review/domain"
	"github.com/oekit/concurrency/async"
	"github.com/oekit/logx"
	"github.com/oekit/pkg/convert"
)

func runPreview(ctx context.Context, cfg OpenCodeReviewConfig, workspace domain.Workspace) (*domain.PreviewResult, error) {
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
		"--concurrency", convert.IntToStr(cfg.Concurrency),
		"--max-git-procs", convert.IntToStr(cfg.MaxGitProcs),
		"--timeout", convert.IntToStr(cfg.TimeoutMinutes),
		"--audience", cfg.Audience,
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
	cmd.Stdout = &stdout

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, &BinaryMissingError{Binary: cfg.Binary, Err: err}
	}

	var stderrLines []string
	var stderrMu sync.Mutex
	var stderrWg sync.WaitGroup
	stderrWg.Add(1)
	async.GoSafe(func() {
		defer stderrWg.Done()
		scanner := bufio.NewScanner(stderrPipe)
		scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
		for scanner.Scan() {
			line := scanner.Text()
			stderrMu.Lock()
			stderrLines = append(stderrLines, line)
			stderrMu.Unlock()
		}
		if err := scanner.Err(); err != nil {
			logx.Warnf("ocr stderr scanner error: %v", err)
		}
	})

	err = cmd.Wait()
	stderrWg.Wait()
	if err != nil {
		stderrMu.Lock()
		message := strings.Join(stderrLines, "\n")
		stderrMu.Unlock()
		return nil, classifyExitError(ctx, err, timeout, message)
	}

	output := bytes.TrimSpace(stdout.Bytes())
	if len(output) == 0 {
		return &domain.PreviewResult{Files: []string{}}, nil
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

	return &domain.PreviewResult{Files: files}, nil
}
