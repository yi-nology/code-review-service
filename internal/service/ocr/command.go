package ocr

import (
	"context"
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

	err := cmd.Run()
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
