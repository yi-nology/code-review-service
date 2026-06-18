package settings_service

import (
	"context"
	"encoding/json"

	"github.com/yi-nology/code-review-service/biz/handler/common"
	modelSettings "github.com/yi-nology/code-review-service/biz/model/settings"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"

	"github.com/oekit/pkg/slicex"

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
	var req modelSettings.UpdateGlobalRulesReq
	if err := c.BindAndValidate(&req); err != nil {
		common.Fail(c, common.BadRequest(err.Error()))
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
		var rules []struct {
			Path string `json:"path"`
			Rule string `json:"rule"`
		}
		if err := json.Unmarshal(r.Rules, &rules); err == nil {
			for _, r := range rules {
				entries = append(entries, &modelSettings.ReviewRule{Path: r.Path, Rule: r.Rule})
			}
		}
	}
	return &modelSettings.GlobalRules{Rules: entries}
}

func apiGlobalRulesToDB(g *modelSettings.GlobalRules) *models.GlobalRules {
	db := &models.GlobalRules{}
	if g == nil {
		return db
	}
	type ruleEntry struct {
		Path string `json:"path"`
		Rule string `json:"rule"`
	}
	entries := slicex.Map(g.Rules, func(r *modelSettings.ReviewRule) ruleEntry {
		return ruleEntry{Path: r.Path, Rule: r.Rule}
	})
	data, _ := json.Marshal(entries)
	db.Rules = datatypes.JSON(data)
	return db
}
