package llm

import (
	"context"
	"fmt"
	"sync"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/oekit/logx"
)

// ChatProvider LLM 聊天提供商接口
type ChatProvider interface {
	Name() string
	Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
}

// ChatRequest 聊天请求
type ChatRequest struct {
	Messages    []Message `json:"messages"`
	Model       string    `json:"model"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature"`
}

// Message 聊天消息
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatResponse 聊天响应
type ChatResponse struct {
	Content      string `json:"content"`
	FinishReason string `json:"finish_reason"`
}

// ProviderService 提供商管理服务
type ProviderService struct {
	mu       sync.RWMutex
	registry map[string]ChatProvider
	repo     ProviderRepository
}

// ProviderRepository 提供商仓库接口
type ProviderRepository interface {
	Get(ctx context.Context) (*models.OCRSettings, error)
	Save(ctx context.Context, settings *models.OCRSettings) error
}

// NewProviderService 创建提供商服务
func NewProviderService(repo ProviderRepository) *ProviderService {
	return &ProviderService{
		registry: make(map[string]ChatProvider),
		repo:     repo,
	}
}

// InitProviders 初始化提供商（从配置文件）
func (s *ProviderService) InitProviders(presets []ProviderPreset) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, preset := range presets {
		logx.Infof("init provider: %s (%s)", preset.ID, preset.DisplayName)
	}
}

// RegisterProvider 注册提供商
func (s *ProviderService) RegisterProvider(p ChatProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.registry[p.Name()] = p
}

// UnregisterProvider 注销提供商
func (s *ProviderService) UnregisterProvider(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.registry, name)
}

// GetProvider 获取提供商
func (s *ProviderService) GetProvider(name string) (ChatProvider, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.registry[name]
	return p, ok
}

// GetDefaultProvider 获取默认提供商
func (s *ProviderService) GetDefaultProvider(ctx context.Context) (ChatProvider, error) {
	// 1. 检查数据库中的配置
	settings, err := s.repo.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("读取 OCR 配置失败: %w", err)
	}

	if settings.Provider != "" {
		if p, ok := s.GetProvider(settings.Provider); ok {
			return p, nil
		}
	}

	// 2. 返回错误
	return nil, fmt.Errorf("未配置默认提供商")
}

// ListPresets 获取预设列表
func (s *ProviderService) ListPresets() []ProviderPreset {
	return Presets
}

// GetPreset 获取预设
func (s *ProviderService) GetPreset(id string) (*ProviderPreset, error) {
	preset := GetPresetByID(id)
	if preset == nil {
		return nil, fmt.Errorf("预设不存在: %s", id)
	}
	return preset, nil
}
