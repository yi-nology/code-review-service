package modules

import (
	"context"
	"time"

	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/yi-nology/code-review-service/internal/ocr"
	"github.com/yi-nology/code-review-service/internal/service/memory"

	"github.com/oekit/logx"
	"github.com/yi-nology/agent-memory/ports"
	"go.uber.org/fx"
)

// MemoryModule 装配跨会话长期记忆（底座 agent-memory）。
//
// 未启用（memory.enabled=false）或装配失败时返回 nil——调用方按 nil 降级，
// 评审行为与启用前完全一致。记忆是副产物，装不上不该让服务起不来。
var MemoryModule = fx.Options(
	fx.Provide(NewMemoryService),
	fx.Provide(NewMemoryMaintainer),
	fx.Invoke(registerMemoryMaintainer),
)

// NewMemoryService 构造记忆服务；未启用或装配失败时返回 nil。
func NewMemoryService(cfg *config.Config, llmProvider ocr.Provider) *memory.Service {
	if cfg == nil || !cfg.Memory.Enabled {
		return nil
	}

	store, err := memory.OpenStore(memory.StoreConfig{
		DSN:  cfg.Memory.DSN,
		Path: cfg.Memory.Path,
	})
	if err != nil {
		logx.Errorf("memory store open failed, long-term memory disabled: %v", err)
		return nil
	}

	namespace := cfg.Memory.Namespace
	if namespace == "" {
		namespace = "review"
	}

	svc, err := memory.New(memory.Config{
		Namespace:    namespace,
		Host:         "review-service",
		RecallLimit:  cfg.Memory.RecallLimit,
		RecallBudget: cfg.Memory.RecallBudget,
	}, memory.Deps{
		Store:      store,
		Redactor:   memory.NewRedactor(),
		Bander:     ports.MemoryBand{},
		LLM:        memory.NewLLMAdapter(llmProvider),
		Escalation: memory.NewRepositoryResolver(),
		Estimator:  ports.StaticEstimator{},
	})
	if err != nil {
		logx.Errorf("memory service assemble failed, long-term memory disabled: %v", err)
		_ = store.Close()
		return nil
	}

	logx.Infof("long-term memory enabled (namespace=%s)", namespace)
	return svc
}

// NewMemoryMaintainer 构造记忆库后台维护器；未启用维护时返回 nil。
func NewMemoryMaintainer(cfg *config.Config, svc *memory.Service) *memory.Maintainer {
	if cfg == nil || !cfg.Memory.Enabled || !cfg.Memory.MaintenanceEnabled || svc == nil {
		return nil
	}

	var opts []memory.MaintainerOption
	if d, err := time.ParseDuration(cfg.Memory.MaintenanceInterval); err == nil && d > 0 {
		opts = append(opts, memory.WithInterval(d))
	} else if cfg.Memory.MaintenanceInterval != "" {
		logx.Warnf("invalid memory maintenance_interval %q, using default: %v", cfg.Memory.MaintenanceInterval, err)
	}
	if d, err := time.ParseDuration(cfg.Memory.RawRetention); err == nil && d > 0 {
		opts = append(opts, memory.WithRawRetention(d))
	} else if cfg.Memory.RawRetention != "" {
		logx.Warnf("invalid memory raw_retention %q, using default: %v", cfg.Memory.RawRetention, err)
	}

	return memory.NewMaintainer(svc, opts...)
}

// registerMemoryMaintainer 挂上维护循环的生命周期钩子。
//
// 维护是可选的：未启用时维护器为 nil，这里直接跳过。
func registerMemoryMaintainer(lc fx.Lifecycle, m *memory.Maintainer) {
	if m == nil {
		return
	}
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			m.StartAsync()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return m.Stop(ctx)
		},
	})
}
