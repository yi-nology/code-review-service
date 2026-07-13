package modules

import (
	"context"
	"encoding/json"
	"time"

	"github.com/yi-nology/code-review-service/internal/domain/ocrasset"
	"github.com/yi-nology/code-review-service/internal/repository"

	"github.com/oekit/logx"

	"go.uber.org/fx"
)

// 全局规则迁移模块：把旧 global_rules 单例迁移成一个 scope=global 的内置规则包（「全局默认」）。
// 迁移幂等：仅当存在 global_rules 数据且尚无任何 scope=global 的规则包时执行。
// k8s 多副本并发安全：使用 OCRRulePackageRepository.InsertIfAbsent（ON CONFLICT (name) DO NOTHING），
// 多个副本同时启动只有一个真正执行 INSERT，其余拿到同一 ID 而非报错或重复创建。
// 原 global_rules 表数据保留不动（ResolveRuleFile 仍兜底读取，双轨过渡）。
var MigrateRulesModule = fx.Options(
	fx.Invoke(InitGlobalRulesMigration),
)

// InitGlobalRulesMigration 注册启动期迁移钩子（异步，不阻塞 fx 启动）。
func InitGlobalRulesMigration(lc fx.Lifecycle, repos *repository.RepoContainer) {
	// globalRulesRepo 或 rulePackageRepo 缺一不可；缺则跳过（如未配置全局规则仓库）。
	if repos == nil || repos.GlobalRules == nil || repos.OCRRulePackage == nil {
		return
	}
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			asyncMigrateGlobalRules(context.Background(), repos.GlobalRules, repos.OCRRulePackage)
			return nil
		},
	})
}

// asyncMigrateGlobalRules 执行迁移逻辑，失败只 warn 不影响启动。
func asyncMigrateGlobalRules(ctx context.Context, globalRepo repository.GlobalRulesRepository, pkgRepo repository.OCRRulePackageRepository) {
	migrateCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// 1. 早退优化：已存在 scope=global 的规则包就跳过（避免每次启动都做 INSERT + 重读）。
	//    真正的并发安全由第 4 步 InsertIfAbsent 保证——这一步只是省一次 DB 往返。
	packages, err := pkgRepo.List(migrateCtx)
	if err != nil {
		logx.Warn("global rules migration: failed to list rule packages", "error", err)
		return
	}
	for _, pkg := range packages {
		if pkg.Scope == ocrasset.ScopeGlobal {
			return // 已迁移过（或已有全局规则集），跳过。
		}
	}

	// 2. 读取旧全局规则；无数据则无需迁移。
	globalRules, err := globalRepo.Get(migrateCtx)
	if err != nil {
		logx.Warn("global rules migration: failed to load global rules", "error", err)
		return
	}
	if globalRules == nil || len(globalRules.Content) == 0 {
		return
	}

	// 3. 校验旧全局规则内容可解析为 ruleEntry 数组（与 ResolveRuleFile 一致的裸数组形态）。
	var entries []map[string]string
	if err := json.Unmarshal(globalRules.Content, &entries); err != nil {
		logx.Warn("global rules migration: failed to decode global rules, skipping", "error", err)
		return
	}
	if len(entries) == 0 {
		return
	}

	// 4. 创建迁移后的全局规则集（scope=global, built_in=true）。
	//    Name 带「迁移自全局规则」后缀避免与用户自建包重名冲突。
	tags, _ := json.Marshal([]string{"global", "migrated"})
	migrated := &ocrasset.RulePackage{
		Name:        "全局默认（迁移自全局规则）",
		Description: "由旧「全局规则」自动迁移而来，scope=global，始终合并到每次审查。",
		Tags:        tags,
		Rules:       globalRules.Content,
		Enabled:     true,
		BuiltIn:     true,
		Scope:       ocrasset.ScopeGlobal,
	}
	// 用 InsertIfAbsent 而非 Save：避免 k8s 多副本启动时 unique(name) 冲突产生 warn 日志；
	// 并发的其他副本通过 DoNothing + 重读拿到同一 ID。
	id, err := pkgRepo.InsertIfAbsent(migrateCtx, migrated)
	if err != nil {
		logx.Warn("global rules migration: failed to insert migrated package", "error", err)
		return
	}

	logx.Infof("global rules migration: migrated %d rules into global rule package (id=%d)", len(entries), id)
}
