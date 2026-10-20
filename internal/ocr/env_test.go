package ocr

import (
	"strings"
	"testing"
)

// TestChildEnv_StripsHostSecrets 验证宿主服务自己的凭据不进 OCR 子进程。
//
// OCR 子进程会在工作区里跑 git（可能触发仓库自带 hooks）与 MCP server，
// 工作区内容来自被评审的 PR——服务自己的数据库连接串与 API token 落到那里，
// 等于把凭据交给一段半受控的代码。
func TestChildEnv_StripsHostSecrets(t *testing.T) {
	base := []string{
		"PATH=/usr/bin",
		"DATABASE_URL=postgres://svc:pw@db:5432/review",
		"REVIEW_TOKEN=super-secret",
		"MCP_TOKEN=mcp-secret",
		"ARGUS_BOT_TOKEN=argus-secret",
		"HOME=/root",
	}
	got := childEnv(base)

	for _, kv := range got {
		name, _, _ := strings.Cut(kv, "=")
		if hostSecretEnv[name] {
			t.Errorf("凭据变量 %s 泄漏进子进程环境: %q", name, kv)
		}
	}
	if len(got) != 2 {
		t.Errorf("应只剩 PATH 与 HOME，实际 %d 项: %v", len(got), got)
	}
}

// TestChildEnv_KeepsEverythingElse 验证非凭据变量一律保留。
//
// 刻意不做白名单：OCR 真正需要的变量（SSH_AUTH_SOCK / GIT_SSH_COMMAND /
// NODE_EXTRA_CA_CERTS / 各家 provider 凭证…）挡掉之后是「评审跑不起来」，
// 那是更大的生产风险。
func TestChildEnv_KeepsEverythingElse(t *testing.T) {
	base := []string{
		"PATH=/usr/bin",
		"SSH_AUTH_SOCK=/tmp/ssh-agent.sock",
		"GIT_SSH_COMMAND=ssh -i /root/.ssh/id_ed25519",
		"NODE_EXTRA_CA_CERTS=/etc/ssl/corp.pem",
		"OPENAI_API_KEY=sk-ambient",
	}
	got := childEnv(base)
	if len(got) != len(base) {
		t.Errorf("非凭据变量应全部保留：期望 %d 项，实际 %d 项", len(base), len(got))
	}
}

// TestChildEnv_EmptyBase 验证空输入不炸。
func TestChildEnv_EmptyBase(t *testing.T) {
	if got := childEnv(nil); len(got) != 0 {
		t.Errorf("nil 输入应返回空，got %v", got)
	}
}
