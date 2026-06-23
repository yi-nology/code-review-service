package settings_service

import (
	"context"
	"encoding/json"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	modelSettings "github.com/yi-nology/code-review-service/biz/model/settings"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"

	"github.com/cloudwego/hertz/pkg/app"
	"gorm.io/datatypes"
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
	dbRules := apiGlobalRulesToDB(req.Rules)
	if err := handlerInst.globalRules.Save(ctx, dbRules); err != nil {
		common.Fail(c, common.BizErr(500, "保存全局规则失败"))
		return
	}
	common.Success(c, &modelSettings.UpdateGlobalRulesResp{Rules: dbGlobalRulesToAPI(dbRules)})
}

func dbGlobalRulesToAPI(r *models.GlobalRules) *modelSettings.GlobalRules {
	if r == nil {
		return &modelSettings.GlobalRules{}
	}
	var entries []*modelSettings.ReviewRule
	if len(r.Rules) > 0 {
		_ = json.Unmarshal(r.Rules, &entries)
	}
	return &modelSettings.GlobalRules{Rules: entries}
}

func apiGlobalRulesToDB(g *modelSettings.GlobalRules) *models.GlobalRules {
	db := &models.GlobalRules{}
	if g == nil || len(g.Rules) == 0 {
		return db
	}
	data, _ := json.Marshal(g.Rules)
	db.Rules = datatypes.JSON(data)
	return db
}
