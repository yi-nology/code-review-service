package ocr

import (
	"context"
	"fmt"
	"strings"

	"github.com/yi-nology/code-review-service/internal/service/review"
)

type ProviderLister struct {
	settings SettingsSource
	resolver *ConfigResolver
}

func NewProviderLister(settings SettingsSource, resolver *ConfigResolver) *ProviderLister {
	return &ProviderLister{settings: settings, resolver: resolver}
}

func (l *ProviderLister) ListLLMProviders(ctx context.Context) ([]review.LLMProvider, error) {
	s, err := l.settings.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("读取 OCR 配置失败: %w", err)
	}

	binary := resolveBinaryFrom(s, l.resolver.fallback)

	result, err := runOCRCommand(ctx, binary, []string{"llm", "providers"}, nil)
	if err != nil {
		msg := strings.TrimSpace(result.stderr)
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("获取 Provider 列表失败: %s", msg)
	}

	output := result.stdout
	var providers []review.LLMProvider
	lines := strings.Split(output, "\n")
	inTable := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "NAME") && strings.Contains(line, "PROTOCOL") {
			inTable = true
			continue
		}
		if inTable && line == "" {
			break
		}
		if inTable && line != "" && !strings.HasPrefix(line, "----") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				p := review.LLMProvider{
					Name:     fields[0],
					Protocol: fields[1],
				}
				if len(fields) >= 3 {
					p.BaseURL = fields[2]
				}
				providers = append(providers, p)
			}
		}
	}

	return providers, nil
}

var _ review.LLMProviderLister = (*ProviderLister)(nil)
