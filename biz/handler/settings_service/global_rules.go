package settings_service

import (
	"context"
	"encoding/json"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	"github.com/yi-nology/code-review-service/internal/domain/rules"
	modelSettings "github.com/yi-nology/code-review-service/biz/model/settings"

	"github.com/cloudwego/hertz/pkg/app"
)

func GetGlobalRules(ctx context.Context, c *app.RequestContext) {
	rules, err := handlerInst.globalRules.Get(ctx)
	if err != nil {
		common.Fail(c, err)
		return
	}
	common.Success(c, &modelSettings.GetGlobalRulesResp{Rules: dbGlobalRulesToAPI(rules)})
}

func UpdateGlobalRules(ctx context.Context, c *app.RequestContext) {
	req, ok := common.Bind[modelSettings.UpdateGlobalRulesReq](c)
	if !ok {
		return
	}
	dbRules := apiGlobalRulesToDomain(req.Rules)
	if err := handlerInst.globalRules.Save(ctx, dbRules); err != nil {
		common.Fail(c, common.BizErr(500, "保存全局规则失败"))
		return
	}
	common.Success(c, &modelSettings.UpdateGlobalRulesResp{Rules: dbGlobalRulesToAPI(dbRules)})
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
