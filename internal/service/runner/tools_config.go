package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func appendToolsArgs(args []string, cfg OpenCodeReviewConfig) ([]string, func(), error) {
	if strings.TrimSpace(cfg.ToolsPath) != "" {
		return append(args, "--tools", cfg.ToolsPath), nil, nil
	}
	if strings.TrimSpace(cfg.ToolsConfig) == "" {
		return args, nil, nil
	}
	if !json.Valid([]byte(cfg.ToolsConfig)) {
		return nil, nil, fmt.Errorf("tools_config must be valid JSON")
	}

	f, err := os.CreateTemp("", "ocr-tools-*.json")
	if err != nil {
		return nil, nil, fmt.Errorf("create tools config file: %w", err)
	}
	if _, err := f.WriteString(cfg.ToolsConfig); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return nil, nil, fmt.Errorf("write tools config file: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return nil, nil, fmt.Errorf("close tools config file: %w", err)
	}
	cleanup := func() { _ = os.Remove(f.Name()) }
	return append(args, "--tools", f.Name()), cleanup, nil
}
