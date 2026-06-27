package settings_service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
)

func TestValidateOCRSettings(t *testing.T) {
	tests := []struct {
		name    string
		input   *models.OCRSettings
		wantErr bool
	}{
		{
			name:    "nil settings returns error",
			input:   nil,
			wantErr: true,
		},
		{
			name: "empty binary gets default",
			input: &models.OCRSettings{
				Binary:          "",
				Audience:        "human",
				DefaultSeverity: "major",
			},
			wantErr: false,
		},
		{
			name: "invalid audience returns error",
			input: &models.OCRSettings{
				Binary:          "ocr",
				Audience:        "invalid",
				DefaultSeverity: "major",
			},
			wantErr: true,
		},
		{
			name: "invalid severity returns error",
			input: &models.OCRSettings{
				Binary:          "ocr",
				Audience:        "human",
				DefaultSeverity: "invalid",
			},
			wantErr: true,
		},
		{
			name: "concurrency out of range returns error",
			input: &models.OCRSettings{
				Binary:          "ocr",
				Audience:        "human",
				DefaultSeverity: "major",
				Concurrency:     100,
			},
			wantErr: true,
		},
		{
			name: "valid settings pass",
			input: &models.OCRSettings{
				Binary:          "ocr",
				Audience:        "human",
				DefaultSeverity: "major",
				Concurrency:     4,
				MaxGitProcs:     8,
				TimeoutMinutes:  10,
				MaxTools:        20,
			},
			wantErr: false,
		},
		{
			name: "invalid extra body JSON returns error",
			input: &models.OCRSettings{
				Binary:          "ocr",
				Audience:        "human",
				DefaultSeverity: "major",
				ExtraBody:       []byte("not json"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOCRSettings(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateOCRSettings() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateToolConfig(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{
			name:    "valid array config",
			raw:     `[{"name":"code_search","definition":{"name":"code_search","parameters":{"type":"object"}}}]`,
			wantErr: false,
		},
		{
			name:    "valid object config",
			raw:     `{"tools":[{"name":"file_read","definition":{"name":"file_read"}}]}`,
			wantErr: false,
		},
		{
			name:    "invalid json",
			raw:     `{`,
			wantErr: true,
		},
		{
			name:    "missing definition",
			raw:     `[{"name":"file_read"}]`,
			wantErr: true,
		},
		{
			name:    "object missing tools",
			raw:     `{"name":"file_read"}`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateToolConfig(json.RawMessage(tt.raw))
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateToolConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDTOToRulePackage(t *testing.T) {
	tests := []struct {
		name    string
		input   rulePackageDTO
		wantErr bool
	}{
		{
			name:    "empty name returns error",
			input:   rulePackageDTO{Rules: []map[string]string{{"path": "**/*.go", "rule": "check errors"}}},
			wantErr: true,
		},
		{
			name:    "empty rules returns error",
			input:   rulePackageDTO{Name: "Go 后端"},
			wantErr: true,
		},
		{
			name:    "missing path returns error",
			input:   rulePackageDTO{Name: "Go 后端", Rules: []map[string]string{{"rule": "check errors"}}},
			wantErr: true,
		},
		{
			name: "valid package converts arrays",
			input: rulePackageDTO{
				Name:        "Go 后端",
				Description: "Go service checks",
				Languages:   []string{"go"},
				Tags:        []string{"backend"},
				Includes:    []string{"**/*.go"},
				Excludes:    []string{"**/*_test.go"},
				ProjectRefs: []string{"github/org/repo"},
				Enabled:     true,
				Rules:       []map[string]string{{"path": "**/*.go", "rule": "check errors"}},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := dtoToRulePackage(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("dtoToRulePackage() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && got.Name != strings.TrimSpace(tt.input.Name) {
				t.Fatalf("dtoToRulePackage() name = %q, want %q", got.Name, strings.TrimSpace(tt.input.Name))
			}
		})
	}
}

func TestDTOToToolPackage(t *testing.T) {
	validConfig := json.RawMessage(`[{"name":"code_search","definition":{"name":"code_search"}}]`)
	tests := []struct {
		name         string
		input        toolPackageDTO
		wantScanType string
		wantErr      bool
	}{
		{
			name:    "empty name returns error",
			input:   toolPackageDTO{Config: validConfig},
			wantErr: true,
		},
		{
			name:    "invalid config returns error",
			input:   toolPackageDTO{Name: "bad", Config: json.RawMessage(`{"name":"file_read"}`)},
			wantErr: true,
		},
		{
			name:    "invalid scan type returns error",
			input:   toolPackageDTO{Name: "bad scan", ScanType: "quick", Config: validConfig},
			wantErr: true,
		},
		{
			name:         "empty scan type defaults all",
			input:        toolPackageDTO{Name: "default scan", Config: validConfig, Enabled: true},
			wantScanType: "all",
			wantErr:      false,
		},
		{
			name:         "valid scan type is preserved",
			input:        toolPackageDTO{Name: "scan only", ScanType: "scan", Config: validConfig, ProjectRefs: []string{"github/org/repo"}},
			wantScanType: "scan",
			wantErr:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := dtoToToolPackage(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("dtoToToolPackage() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && got.ScanType != tt.wantScanType {
				t.Fatalf("dtoToToolPackage() scan type = %q, want %q", got.ScanType, tt.wantScanType)
			}
		})
	}
}

func TestValidateProjectPolicy(t *testing.T) {
	tests := []struct {
		name    string
		input   *models.ProjectPolicy
		wantErr bool
	}{
		{
			name:    "nil policy returns error",
			input:   nil,
			wantErr: true,
		},
		{
			name: "empty provider returns error",
			input: &models.ProjectPolicy{
				Provider: "",
				Repo:     "org/repo",
				FailOn:   "critical",
			},
			wantErr: true,
		},
		{
			name: "empty repo returns error",
			input: &models.ProjectPolicy{
				Provider: "github",
				Repo:     "",
				FailOn:   "critical",
			},
			wantErr: true,
		},
		{
			name: "invalid fail_on returns error",
			input: &models.ProjectPolicy{
				Provider: "github",
				Repo:     "org/repo",
				FailOn:   "invalid",
			},
			wantErr: true,
		},
		{
			name: "valid policy passes",
			input: &models.ProjectPolicy{
				Provider:          "github",
				Repo:              "org/repo",
				FailOn:            "critical",
				OCRConcurrency:    4,
				OCRTimeoutMinutes: 10,
				OCRMaxTools:       20,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateProjectPolicy(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateProjectPolicy() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseUintID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    uint
		wantErr bool
	}{
		{name: "valid id", input: "42", want: 42, wantErr: false},
		{name: "zero id", input: "0", want: 0, wantErr: false},
		{name: "negative returns error", input: "-1", want: 0, wantErr: true},
		{name: "non-numeric returns error", input: "abc", want: 0, wantErr: true},
		{name: "empty returns error", input: "", want: 0, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseUintID(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseUintID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseUintID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDBOCRSettingsToAPI(t *testing.T) {
	input := &models.OCRSettings{
		Enabled:         true,
		Binary:          "ocr",
		Audience:        "human",
		DefaultSeverity: "major",
		Provider:        "openai",
		URL:             "https://api.openai.com",
		AuthToken:       "sk-xxx",
		ModelName:       "gpt-4",
		Concurrency:     4,
	}

	result := dbOCRSettingsToAPI(input)

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !result.Enabled {
		t.Error("expected Enabled to be true")
	}
	if result.Binary == nil || *result.Binary != "ocr" {
		t.Error("expected Binary to be 'ocr'")
	}
	if result.Provider == nil || *result.Provider != "openai" {
		t.Error("expected Provider to be 'openai'")
	}
}

func boolPtr(b bool) *bool { return &b }

func TestDBProjectPolicyToAPI(t *testing.T) {
	input := &models.ProjectPolicy{
		Provider:          "github",
		Repo:              "org/repo",
		FailOn:            "critical",
		OCREnabled:        boolPtr(true),
		OCRConcurrency:    4,
		OCRTimeoutMinutes: 10,
	}

	result := dbProjectPolicyToAPI(input)

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Provider != "github" {
		t.Errorf("expected Provider 'github', got %s", result.Provider)
	}
	if result.Repo != "org/repo" {
		t.Errorf("expected Repo 'org/repo', got %s", result.Repo)
	}
	if result.FailOn != "critical" {
		t.Errorf("expected FailOn 'critical', got %s", result.FailOn)
	}
}

func TestDBGlobalRulesToAPI(t *testing.T) {
	t.Run("nil rules returns empty", func(t *testing.T) {
		result := dbGlobalRulesToAPI(nil)
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if len(result.Rules) != 0 {
			t.Errorf("expected 0 rules, got %d", len(result.Rules))
		}
	})
}

func TestDBReviewerAgentToAPI(t *testing.T) {
	t.Run("nil agent returns nil", func(t *testing.T) {
		result := dbReviewerAgentToAPI(nil)
		if result != nil {
			t.Error("expected nil result")
		}
	})

	t.Run("valid agent converts correctly", func(t *testing.T) {
		input := &models.ReviewerAgent{
			Name:        "security",
			DisplayName: "Security Reviewer",
			Description: "Reviews for security issues",
			Enabled:     true,
			Priority:    1,
		}
		result := dbReviewerAgentToAPI(input)
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if result.Name != "security" {
			t.Errorf("expected Name 'security', got %s", result.Name)
		}
		if !result.Enabled {
			t.Error("expected Enabled to be true")
		}
	})
}

func TestDBCustomProviderToAPI(t *testing.T) {
	t.Run("nil provider returns nil", func(t *testing.T) {
		result := dbCustomProviderToAPI(nil)
		if result != nil {
			t.Error("expected nil result")
		}
	})

	t.Run("API key is masked", func(t *testing.T) {
		input := &models.CustomProvider{
			Name:      "custom",
			Protocol:  "openai",
			APIKey:    "sk-test-" + strings.Repeat("x", 20), //nolint:gosec // test data
			ModelName: "gpt-4",
			Enabled:   true,
		}
		result := dbCustomProviderToAPI(input)
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if result.APIKey == nil || *result.APIKey != "***" {
			t.Error("expected API key to be masked")
		}
	})
}
