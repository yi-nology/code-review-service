package runner

import (
	"os"
	"testing"
)

func TestOpenCodeReviewConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     OpenCodeReviewConfig
		wantErr bool
	}{
		{
			name:    "empty binary should fail",
			cfg:     OpenCodeReviewConfig{Binary: ""},
			wantErr: true,
		},
		{
			name:    "valid config",
			cfg:     OpenCodeReviewConfig{Binary: "ocr", TimeoutMinutes: 10, Concurrency: 2},
			wantErr: false,
		},
		{
			name:    "timeout exceeds maximum",
			cfg:     OpenCodeReviewConfig{Binary: "ocr", TimeoutMinutes: 200},
			wantErr: true,
		},
		{
			name:    "concurrency exceeds maximum",
			cfg:     OpenCodeReviewConfig{Binary: "ocr", Concurrency: 64},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestAppendToolsArgsUsesPathFirst(t *testing.T) {
	args, cleanup, err := appendToolsArgs([]string{"review"}, OpenCodeReviewConfig{
		ToolsPath:   "/opt/ocr/tools.json",
		ToolsConfig: `[{"name":"code_search"}]`,
	})
	if err != nil {
		t.Fatalf("appendToolsArgs returned error: %v", err)
	}
	if cleanup != nil {
		t.Fatal("expected no cleanup for explicit tools path")
	}
	if len(args) != 3 || args[1] != "--tools" || args[2] != "/opt/ocr/tools.json" {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestAppendToolsArgsWritesManagedConfig(t *testing.T) {
	args, cleanup, err := appendToolsArgs([]string{"scan"}, OpenCodeReviewConfig{
		ToolsConfig: `[{"name":"code_search"}]`,
	})
	if err != nil {
		t.Fatalf("appendToolsArgs returned error: %v", err)
	}
	if cleanup == nil {
		t.Fatal("expected cleanup for managed tools config")
	}
	defer cleanup()

	if len(args) != 3 || args[1] != "--tools" {
		t.Fatalf("unexpected args: %#v", args)
	}
	data, err := os.ReadFile(args[2])
	if err != nil {
		t.Fatalf("read managed tools config: %v", err)
	}
	if string(data) != `[{"name":"code_search"}]` {
		t.Fatalf("unexpected tools config contents: %s", data)
	}
}

func TestAppendToolsArgsRejectsInvalidConfig(t *testing.T) {
	_, _, err := appendToolsArgs([]string{"review"}, OpenCodeReviewConfig{
		ToolsConfig: `{bad-json`,
	})
	if err == nil {
		t.Fatal("expected invalid JSON error")
	}
}
