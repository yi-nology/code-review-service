package runner

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/cloudwego/hertz/pkg/common/hlog"
)

type ocrConfigFile struct {
	Provider        string                      `json:"provider,omitempty"`
	Providers       map[string]ocrProviderEntry `json:"providers,omitempty"`
	CustomProviders map[string]ocrProviderEntry `json:"custom_providers,omitempty"`
	LLM             *ocrLLMConfig               `json:"llm,omitempty"`
	Language        string                      `json:"language,omitempty"`
	Telemetry       *ocrTelemetryConfig         `json:"telemetry,omitempty"`
}

type ocrProviderEntry struct {
	APIKey     string          `json:"api_key,omitempty"`
	URL        string          `json:"url,omitempty"`
	Protocol   string          `json:"protocol,omitempty"`
	Model      string          `json:"model,omitempty"`
	AuthHeader string          `json:"auth_header,omitempty"`
	ExtraBody  json.RawMessage `json:"extra_body,omitempty"`
}

type ocrLLMConfig struct {
	URL          string          `json:"url,omitempty"`
	Model        string          `json:"model,omitempty"`
	AuthToken    string          `json:"auth_token,omitempty"`
	AuthHeader   string          `json:"auth_header,omitempty"`
	UseAnthropic bool            `json:"use_anthropic"`
	ExtraBody    json.RawMessage `json:"extra_body,omitempty"`
}

type ocrTelemetryConfig struct {
	Enabled        bool   `json:"enabled,omitempty"`
	Exporter       string `json:"exporter,omitempty"`
	OTLPEndpoint   string `json:"otlp_endpoint,omitempty"`
	ContentLogging bool   `json:"content_logging,omitempty"`
}

type ConfigFileWriter struct {
	ocrRepo  repository.OCRSettingsRepository
	cpRepo   repository.CustomProviderRepository
	filePath string
}

func NewConfigFileWriter(
	ocrRepo repository.OCRSettingsRepository,
	cpRepo repository.CustomProviderRepository,
	filePath string,
) *ConfigFileWriter {
	if filePath == "" {
		home, _ := os.UserHomeDir()
		filePath = filepath.Join(home, ".opencodereview", "config.json")
	}
	return &ConfigFileWriter{
		ocrRepo:  ocrRepo,
		cpRepo:   cpRepo,
		filePath: filePath,
	}
}

func (w *ConfigFileWriter) Sync(ctx context.Context) error {
	s, err := w.ocrRepo.Get(ctx)
	if err != nil {
		hlog.Warnf("config-file writer: read ocr settings: %v", err)
		s = &models.OCRSettings{}
	}

	var customProviders []models.CustomProvider
	if w.cpRepo != nil {
		customProviders, err = w.cpRepo.List(ctx)
		if err != nil {
			hlog.Warnf("config-file writer: list custom providers: %v", err)
		}
	}

	cfg := w.buildConfig(s, customProviders)

	if err := os.MkdirAll(filepath.Dir(w.filePath), 0o750); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(w.filePath, data, 0o600); err != nil {
		return err
	}

	hlog.Infof("config-file writer: synced %s", w.filePath)
	return nil
}

func (w *ConfigFileWriter) buildConfig(s *models.OCRSettings, customProviders []models.CustomProvider) *ocrConfigFile {
	cfg := &ocrConfigFile{
		Language: s.Language,
	}

	if s.Provider != "" {
		cfg.Provider = s.Provider
	}

	cfg.LLM = &ocrLLMConfig{
		URL:          s.URL,
		Model:        s.ModelName,
		AuthToken:    s.AuthToken,
		AuthHeader:   s.AuthHeader,
		UseAnthropic: s.UseAnthropic,
	}
	if len(s.ExtraBody) > 0 {
		cfg.LLM.ExtraBody = json.RawMessage(s.ExtraBody)
	}

	if s.TelemetryEnabled || s.TelemetryExporter != "" {
		cfg.Telemetry = &ocrTelemetryConfig{
			Enabled:        s.TelemetryEnabled,
			Exporter:       s.TelemetryExporter,
			OTLPEndpoint:   s.TelemetryOTLPEndpoint,
			ContentLogging: s.TelemetryContentLogging,
		}
	}

	if len(customProviders) > 0 {
		cfg.CustomProviders = make(map[string]ocrProviderEntry)
		for _, cp := range customProviders {
			if !cp.Enabled {
				continue
			}
			entry := ocrProviderEntry{
				Protocol:   cp.Protocol,
				URL:        cp.URL,
				APIKey:     cp.APIKey,
				Model:      cp.ModelName,
				AuthHeader: cp.AuthHeader,
			}
			if len(cp.ExtraBody) > 0 {
				entry.ExtraBody = json.RawMessage(cp.ExtraBody)
			}
			cfg.CustomProviders[cp.Name] = entry
		}
	}

	return cfg
}
