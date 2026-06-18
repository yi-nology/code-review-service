package ocr

import (
	"context"
	"fmt"
	"strings"

	"github.com/yi-nology/code-review-service/internal/service/review"
)

type Versioner struct {
	settings settingsSource
	resolver *ConfigResolver
}

func NewVersioner(settings settingsSource, resolver *ConfigResolver) *Versioner {
	return &Versioner{settings: settings, resolver: resolver}
}

func (v *Versioner) OCRVersion(ctx context.Context) (*review.OCRVersion, error) {
	s, err := v.settings.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("读取 OCR 配置失败: %w", err)
	}

	binary := resolveBinaryFrom(s, v.resolver.fallback)

	cmdResult, err := runOCRCommand(ctx, binary, []string{"version"}, nil)
	if err != nil {
		msg := strings.TrimSpace(cmdResult.stderr)
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("获取版本失败: %s", msg)
	}

	output := strings.TrimSpace(cmdResult.stdout)
	result := &review.OCRVersion{}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "open-code-review "):
			versionStr := strings.TrimPrefix(line, "open-code-review ")
			parts := strings.SplitN(versionStr, " ", 2)
			if len(parts) >= 1 {
				result.Version = parts[0]
			}
			if len(parts) >= 2 {
				rest := parts[1]
				if strings.HasPrefix(rest, "(") {
					end := strings.Index(rest, ")")
					if end > 0 {
						result.Commit = rest[1:end]
					}
				}
			}
		case strings.Contains(line, "darwin") || strings.Contains(line, "linux") || strings.Contains(line, "windows"):
			result.Platform = line
		case strings.Contains(line, "built at:"):
			result.BuildTime = strings.TrimSpace(strings.TrimPrefix(line, "built at:"))
		}
	}

	return result, nil
}

var _ review.OCRVersioner = (*Versioner)(nil)
