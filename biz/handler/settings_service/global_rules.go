package settings_service

import (
	"context"
	"encoding/json"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	modelSettings "github.com/yi-nology/code-review-service/biz/model/settings"
	"github.com/yi-nology/code-review-service/internal/domain/rules"

	"github.com/cloudwego/hertz/pkg/app"
)

func (h *SettingsHandler) getGlobalRules(ctx context.Context, c *app.RequestContext) {
	rules, err := h.globalRules.Get(ctx)
	if err != nil {
		common.Fail(ctx, c, err)
		return
	}
	common.Success(c, &modelSettings.GetGlobalRulesResp{Rules: dbGlobalRulesToAPI(rules)})
}

// GetGlobalRules 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func GetGlobalRules(ctx context.Context, c *app.RequestContext) { handlerInst.getGlobalRules(ctx, c) }

func (h *SettingsHandler) updateGlobalRules(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelSettings.UpdateGlobalRulesReq](c)
	if !ok {
		return
	}
	dbRules := apiGlobalRulesToDomain(req.Rules)
	if err := h.globalRules.Save(ctx, dbRules); err != nil {
		common.Fail(ctx, c, common.BizErr(500, "保存全局规则失败"))
		return
	}
	common.Success(c, &modelSettings.UpdateGlobalRulesResp{Rules: dbGlobalRulesToAPI(dbRules)})
}

// UpdateGlobalRules 包级入口（biz/router 生成代码引用）；一行委托到方法，测试可直接构造 Handler。
func UpdateGlobalRules(ctx context.Context, c *app.RequestContext) {
	handlerInst.updateGlobalRules(ctx, c)
}

func dbGlobalRulesToAPI(r *rules.Global) *modelSettings.GlobalRules {
	if r == nil {
		return &modelSettings.GlobalRules{}
	}
	var entries []*modelSettings.ReviewRule
	if len(r.Content) > 0 {
		_ = json.Unmarshal(r.Content, &entries)
	}
	return &modelSettings.GlobalRules{Rules: entries}
}

func apiGlobalRulesToDomain(g *modelSettings.GlobalRules) *rules.Global {
	out := &rules.Global{}
	if g == nil || len(g.Rules) == 0 {
		return out
	}
	data, _ := json.Marshal(g.Rules)
	out.Content = data
	return out
}
