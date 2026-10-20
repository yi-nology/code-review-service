package ocr

import "strings"

// hostSecretEnv 是**宿主服务自己的凭据**变量名——它们绝不能进 OCR 子进程。
//
// OCR 子进程会在工作区里跑 git（可能触发仓库自带的 hooks）与 MCP server，
// 工作区内容来自被评审的 PR。服务自己的数据库连接串与 API token 落到那里，
// 等于把凭据交给一段半受控的代码。
//
// 为什么是 denylist 而不是白名单：白名单会把 OCR 真正需要的变量一并挡掉
// （SSH_AUTH_SOCK / GIT_SSH_COMMAND / NODE_EXTRA_CA_CERTS / 各家 provider 的
// 凭证…），那是更大的生产风险——挡掉之后是「评审跑不起来」。denylist 只摘掉
// 确定属于本服务、且 OCR 没有任何用途的那几项。
//
// 残留面：环境里若另有 provider 凭证（如 OPENAI_API_KEY），仍会随环境继承
// 下去。要彻底关掉这个面需要换成 oekit/pkg 的环境白名单口径，但 OCR 需要
// stderr 的按行回调（进度走 stderr），当前 oagentkit/runtime/procx 只提供
// stdout 回调——换过去要先给 procx 补 stderr 回调能力。见 docs/OCR_CORE_DESIGN.md。
var hostSecretEnv = map[string]bool{
	"DATABASE_URL":    true, // 含数据库密码
	"REVIEW_TOKEN":    true, // API 鉴权
	"MCP_TOKEN":       true, // MCP 端点鉴权
	"ARGUS_BOT_TOKEN": true, // Argus 机器人凭据
}

// childEnv 构造 OCR 子进程的环境：从 base 里摘掉宿主服务自己的凭据。
//
// base 通常是 os.Environ()——子进程默认**全量继承**宿主环境，而 OCR 的
// 运行范围（工作区 + 仓库 hooks）不完全受控。
func childEnv(base []string) []string {
	if len(hostSecretEnv) == 0 {
		return base
	}
	out := make([]string, 0, len(base))
	for _, kv := range base {
		name, _, _ := strings.Cut(kv, "=")
		if hostSecretEnv[name] {
			continue
		}
		out = append(out, kv)
	}
	return out
}
