package modules

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/service/runner"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"go.uber.org/fx"
)

var MigrateModule = fx.Options(
	fx.Invoke(MigrateConfigFile),
)

// MigrateConfigFile 在启动时将 ~/.opencodereview/config.json 同步到数据库
func MigrateConfigFile(repos *repository.RepoContainer) {
	reader := runner.NewConfigFileReader(repos.OCRSettings)
	if err := reader.SyncFromFile(context.Background()); err != nil {
		hlog.Warnf("migrate config file failed: %v", err)
	}
}
