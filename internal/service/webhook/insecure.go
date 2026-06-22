package webhook

import (
	"os"
	"strconv"

	"github.com/cloudwego/hertz/pkg/common/hlog"
)

// insecureWarned 避免每次请求重复打日志。
var insecureWarned bool

// allowInsecureSignature 控制未配置 webhook secret 时是否放行签名校验。
//
// 默认 false（安全优先：拒绝无 secret 的 webhook，防止伪造）。
// 这是对历史行为（gitea/gitcode/tencent 在 secret 为空时直接放行）的安全收紧。
//
// 受影响的现有部署可通过环境变量 WEBHOOK_ALLOW_INSECURE=true 临时回退，
// 但强烈建议改为在各平台正确配置 webhook secret。
func allowInsecureSignature() bool {
	v, _ := strconv.ParseBool(os.Getenv("WEBHOOK_ALLOW_INSECURE"))
	if v && !insecureWarned {
		hlog.Errorf("[security] WEBHOOK_ALLOW_INSECURE=true: webhook 签名校验对未配置 secret 的平台放行，存在伪造风险，请尽快配置 webhook secret 并移除该环境变量")
		insecureWarned = true
	}
	return v
}
