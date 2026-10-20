package memory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yi-nology/code-review-service/internal/observability/redact"

	"github.com/yi-nology/agent-memory/ports"
	"github.com/yi-nology/agent-memory/storage/postgres"
	"github.com/yi-nology/agent-memory/storage/sqlite"
	"github.com/yi-nology/agent-memory/store"
)

// StoreConfig 是记忆存储的打开配置。
type StoreConfig struct {
	// DSN Postgres 连接串；非空时走 Postgres。
	DSN string
	// Path SQLite 文件路径（DSN 为空时生效）。父目录须已存在。
	Path string
}

// OpenStore 打开记忆存储。
//
// 默认 SQLite 文件（纯 Go 驱动，无 CGO）：记忆库是独立数据面，
// 与业务库同库会把 11 张记忆表混进业务 schema。配了 DSN 时走 Postgres——
// 多实例部署必须走 Postgres（SQLite 文件在多副本下会各写各的）。
func OpenStore(cfg StoreConfig) (store.Store, error) {
	if cfg.DSN != "" {
		st, err := postgres.Open(cfg.DSN)
		if err != nil {
			return nil, fmt.Errorf("打开记忆库(Postgres): %w", err)
		}
		return st, nil
	}

	path := cfg.Path
	if path == "" {
		return nil, fmt.Errorf("记忆存储未配置：需要 dsn 或 path")
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("创建记忆库目录: %w", err)
		}
	}
	st, err := sqlite.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("打开记忆库(SQLite): %w", err)
	}
	return st, nil
}

// redactorAdapter 把项目的日志脱敏接到记忆的脱敏端口。
//
// 记忆落库是**不可逆**的：日志里脱敏掉的凭据在记忆库里会长期躺着。
// 所以这里复用项目既有的脱敏（AGENTS.md 要求不打印 raw secrets），不另写一套。
type redactorAdapter struct{}

// NewRedactor 构造记忆脱敏器。
func NewRedactor() ports.Redactor { return redactorAdapter{} }

func (redactorAdapter) Redact(_ context.Context, req ports.RedactRequest) (string, error) {
	return redact.String(req.Text), nil
}
