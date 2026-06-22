package ocr

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"syscall"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/service/runner"
)

type ocrCommandResult struct {
	stdout string
	stderr string
}

func runOCRCommand(ctx context.Context, binary string, args []string, env []string) (*ocrCommandResult, error) {
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if len(env) > 0 {
		cmd.Env = env
	}

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return &ocrCommandResult{}, fmt.Errorf("ocr command start failed: %w", err)
	}

	// ctx 超时/取消时杀整个进程组，避免 OCR 起的子进程（如 git/LLM 重试）变孤儿。
	// exec.CommandContext 默认只 kill 主进程，进程组里的孩子会存活。
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			if cmd.Process != nil {
				_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			}
		case <-done:
		}
	}()

	err := cmd.Wait()
	close(done)
	return &ocrCommandResult{
		stdout: stdout.String(),
		stderr: stderr.String(),
	}, err
}

func resolveBinaryFrom(s *models.OCRSettings, fallback runner.OpenCodeReviewConfig) string {
	if s != nil && s.Binary != "" {
		return s.Binary
	}
	if fallback.Binary != "" {
		return fallback.Binary
	}
	return "ocr"
}

// appendLLMEnv 已迁移到 runner.AppendLLMEnv
