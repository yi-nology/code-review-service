package modules

import (
	"context"
	"time"

	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/service/runner"
	"github.com/oekit/logx"
	"go.uber.org/fx"
)

var MigrateModule = fx.Options(
	fx.Invoke(MigrateConfigFile),
)

// MigrateConfigFile 在 fx 启动时将 ~/.opencodereview/config.json 同步到数据库。
// 绑定到 OnStart 生命周期（使用 lifecycle ctx 而非 context.Background），
// 并施加 10s 超时，避免配置文件同步卡住时阻塞整个应用启动。
func MigrateConfigFile(lc fx.Lifecycle, repos *repository.RepoContainer) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			runMigrate(ctx, repos)
			return nil
		},
	})
}

func runMigrate(ctx context.Context, repos *repository.RepoContainer) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	reader := runner.NewConfigFileReader(repos.OCRSettings)
	if err := reader.SyncFromFile(ctx); err != nil {
		logx.Warnf("migrate config file failed: %v", err)
	}
}
