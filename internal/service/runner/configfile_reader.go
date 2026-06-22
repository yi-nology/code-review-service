package runner

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"gorm.io/datatypes"
)

// ConfigFileReader 读取 ~/.opencodereview/config.json 并同步到数据库
type ConfigFileReader struct {
	ocrRepo repository.OCRSettingsRepository
	filePath string
}

func NewConfigFileReader(ocrRepo repository.OCRSettingsRepository) *ConfigFileReader {
	home, _ := os.UserHomeDir()
	filePath := filepath.Join(home, ".opencodereview", "config.json")
	return &ConfigFileReader{
		ocrRepo: ocrRepo,
		filePath: filePath,
	}
}

type ocrConfigFile struct {
	Provider string      `json:"provider,omitempty"`
	LLM      *ocrLLMConfig `json:"llm,omitempty"`
	Language string      `json:"language,omitempty"`
}

type ocrLLMConfig struct {
	URL          string          `json:"url,omitempty"`
	Model        string          `json:"model,omitempty"`
	AuthToken    string          `json:"auth_token,omitempty"`
	AuthHeader   string          `json:"auth_header,omitempty"`
	UseAnthropic bool            `json:"use_anthropic"`
	ExtraBody    json.RawMessage `json:"extra_body,omitempty"`
}

// SyncFromFile 读取 config.json 并同步到数据库（仅在数据库为空时执行）
func (r *ConfigFileReader) SyncFromFile(ctx context.Context) error {
	// 检查文件是否存在
	data, err := os.ReadFile(r.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			hlog.Infof("config-file reader: %s 不存在，跳过同步", r.filePath)
			return nil
		}
		return err
	}

	// 解析配置文件
	var cfg ocrConfigFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		hlog.Warnf("config-file reader: 解析配置文件失败: %v", err)
		return nil
	}

	// 检查是否有 LLM 配置
	if cfg.LLM == nil || (cfg.LLM.URL == "" && cfg.LLM.Model == "") {
		hlog.Infof("config-file reader: 配置文件中无 LLM 配置，跳过同步")
		return nil
	}

	// 获取当前数据库设置
	existing, err := r.ocrRepo.Get(ctx)
	if err != nil {
		hlog.Warnf("config-file reader: 读取数据库设置失败: %v", err)
		return nil
	}

	// 如果数据库已有 URL 配置，不覆盖
	if existing.URL != "" {
		hlog.Infof("config-file reader: 数据库已有 URL 配置，跳过同步")
		return nil
	}

	// 同步配置到数据库
	existing.Provider = cfg.Provider
	existing.URL = cfg.LLM.URL
	existing.ModelName = cfg.LLM.Model
	existing.AuthToken = cfg.LLM.AuthToken
	if cfg.LLM.AuthHeader != "" {
		existing.AuthHeader = cfg.LLM.AuthHeader
	}
	existing.UseAnthropic = cfg.LLM.UseAnthropic
	if len(cfg.LLM.ExtraBody) > 0 {
		existing.ExtraBody = datatypes.JSON(cfg.LLM.ExtraBody)
	}
	if cfg.Language != "" {
		existing.Language = cfg.Language
	}

	if err := r.ocrRepo.Save(ctx, existing); err != nil {
		hlog.Warnf("config-file reader: 保存到数据库失败: %v", err)
		return err
	}

	hlog.Infof("config-file reader: 已同步 %s 到数据库 (provider=%s, model=%s)", r.filePath, cfg.Provider, cfg.LLM.Model)
	return nil
}
