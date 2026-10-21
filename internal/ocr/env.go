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
var hostSecretEnv = map[string]bool{
	"DATABASE_URL":    true, // 含数据库密码
	"REVIEW_TOKEN":    true, // API 鉴权
	"MCP_TOKEN":       true, // MCP 端点鉴权
	"ARGUS_BOT_TOKEN": true, // Argus 机器人凭据
}

// providerCredentialEnv 是各家 LLM provider 的凭证变量名。
//
// 它们**只在 OCR 已经显式拿到凭证时**才被摘掉（见 childEnv）：OCR 的凭证
// 走 OCR_LLM_TOKEN 显式下发（见 Config.AppendEnv），此时环境里残留的
// 同类凭证是冗余的——而 OCR 的运行范围（工作区 + 仓库 hooks + MCP server）
// 不完全受控，冗余凭证只会扩大泄露面。
//
// OCR 没拿到显式凭证时保留它们：那种部署正是靠环境里的 key 跑 LLM，摘掉等于
// 让评审跑不起来。
var providerCredentialEnv = map[string]bool{
	"OPENAI_API_KEY":       true,
	"OPENAI_BASE_URL":      true,
	"ANTHROPIC_API_KEY":    true,
	"CLAUDE_API_KEY":       true,
	"DEEPSEEK_API_KEY":     true,
	"MOONSHOT_API_KEY":     true,
	"QIANFAN_API_KEY":      true,
	"ZHIPU_API_KEY":        true,
	"GROQ_API_KEY":         true,
	"GEMINI_API_KEY":       true,
	"GOOGLE_API_KEY":       true,
	"AZURE_OPENAI_API_KEY": true,
	"MINIMAX_API_KEY":      true,
	"MINIMAX_TOKEN":        true,
	"XAI_API_KEY":          true,
}

// childEnv 构造 OCR 子进程的环境：从 base 里摘掉不该给子进程的凭据。
//
// base 通常是 os.Environ()——子进程默认**全量继承**宿主环境，而 OCR 的
// 运行范围（工作区 + 仓库 hooks）不完全受控。
//
// hasExplicitToken 表示 OCR 已经通过 OCR_LLM_TOKEN 拿到了 LLM 凭证；此时环境里
// 残留的 provider 凭证是冗余的，一并摘掉。
func childEnv(base []string, hasExplicitToken bool) []string {
	out := make([]string, 0, len(base))
	for _, kv := range base {
		name, _, _ := strings.Cut(kv, "=")
		if hostSecretEnv[name] {
			continue
		}
		if hasExplicitToken && providerCredentialEnv[name] {
			continue
		}
		out = append(out, kv)
	}
	return out
}
