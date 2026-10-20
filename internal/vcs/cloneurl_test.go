package vcs

import (
	"strings"
	"testing"
)

// TestCloneURLGuardRejectsHostEscape 钉住最严重的那个后果：host 逃逸。
//
// owner 来自 webhook 载荷（外部可控）。若 owner = "@evil.com"，
// 拼出的 URL 是 "https://gitlab.com/@evil.com/repo.git"——
// 解析后 host 变成 evil.com。服务会带着 git token 去访问它，token 直接泄露。
//
// 这条断言钉住「拼出的 host 必须与配置的平台 host 完全一致」。
func TestCloneURLGuardRejectsHostEscape(t *testing.T) {
	g := NewCloneURLGuard()

	cases := []struct {
		name     string
		baseURL  string
		cloneURL string
	}{
		{
			name:     "协议相对逃逸",
			baseURL:  "https://gitlab.example.com",
			cloneURL: "//evil.com/repo.git",
		},
		{
			name:     "完全不同的 host",
			baseURL:  "https://gitlab.example.com",
			cloneURL: "https://evil.com/repo.git",
		},
		{
			name:     "端口不同",
			baseURL:  "https://gitlab.example.com",
			cloneURL: "https://gitlab.example.com:8443/repo.git",
		},
		{
			name:     "scheme 不同",
			baseURL:  "https://gitlab.example.com",
			cloneURL: "http://gitlab.example.com/repo.git",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := g.Validate(tc.baseURL, tc.cloneURL); err == nil {
				t.Fatalf("应拒绝 host 不匹配的 clone 目标：base=%s clone=%s", tc.baseURL, tc.cloneURL)
			}
		})
	}
}

// TestCloneURLGuardRejectsOwnerEscapingHost 钉住 owner/repo 拼进 URL 后的
// **真实风险形态**。
//
// 先说清楚什么**不是**风险：因为 base 固定带 "https://host" 前缀，
// owner 里的 "@" 与 "//" 拼出来仍落在路径段里，host 不会变
// （"https://gitlab.example.com/@evil.com/repo.git" 的 host 仍是 gitlab.example.com）。
//
// 真正的风险是 **URL 保留字符截断**：
//
//	owner = "a?b" → 拼出 ".../a?b/repo.git"，解析后 path 只有 "/a"，
//	               ".git" 与后半段落进了 query——目标路径被改写成别的资源。
//	owner = "a#b" → 同理，fragment 之后的内容全部丢弃。
//
// 这两类由 ValidateRepoPath 拦掉（路径段不得含 "?" / "#"），
// 本用例钉住「从 payload 到出站」这条链上确实有防护。
func TestCloneURLGuardRejectsOwnerEscapingHost(t *testing.T) {
	g := NewCloneURLGuard()
	const base = "https://gitlab.example.com"

	// owner/repo 来自 webhook 载荷，逐个走「校验路径段 → 拼 URL → 出站校验」全链。
	owners := []string{
		"a?b",       // query 截断
		"a#b",       // fragment 截断
		"@evil.com", // userinfo 形态（当前拼接下不可逃逸，但不得放行）
		"//evil.com",
		"../..", // 路径穿越
	}

	for _, owner := range owners {
		// 第一道：路径段校验。
		if err := ValidateRepoPath(owner, "repo"); err == nil {
			t.Fatalf("owner=%q 应被路径段校验拒绝", owner)
			continue
		}
		// 第二道：即便绕过路径段校验，出站校验也不该放行 host 不符的目标。
		cloneURL := base + "/" + owner + "/repo.git"
		if err := g.Validate(base, cloneURL); err == nil {
			t.Fatalf("owner=%q 拼出的 clone 目标应被拒绝：%s", owner, cloneURL)
		}
	}
}

// TestCloneURLGuardAcceptsLegitimateClone 合法 clone 必须放行。
//
// 守卫拦错正常流量比不拦更糟：评审跑不起来，所有人以为服务坏了。
func TestCloneURLGuardAcceptsLegitimateClone(t *testing.T) {
	g := NewCloneURLGuard()

	cases := []struct {
		name     string
		baseURL  string
		cloneURL string
	}{
		{
			name:     "常规路径",
			baseURL:  "https://gitlab.example.com",
			cloneURL: "https://gitlab.example.com/acme/app.git",
		},
		{
			name:     "带 API 前缀的基地址",
			baseURL:  "https://gitlab.example.com/api/v4",
			cloneURL: "https://gitlab.example.com/acme/app.git",
		},
		{
			name:     "嵌套 group",
			baseURL:  "https://gitlab.example.com",
			cloneURL: "https://gitlab.example.com/group/sub/app.git",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := g.Validate(tc.baseURL, tc.cloneURL); err != nil {
				t.Fatalf("合法 clone 目标应放行：base=%s clone=%s err=%v", tc.baseURL, tc.cloneURL, err)
			}
		})
	}
}

// TestCloneURLGuardRejectsLoopback 回环必须被拒——
// 它是 SSRF 打本机服务（管理面板、数据库、cloud metadata）的入口，
// 而 clone 目标没有任何正当理由落在回环上。
func TestCloneURLGuardRejectsLoopback(t *testing.T) {
	g := NewCloneURLGuard()

	err := g.Validate("http://127.0.0.1:8080", "http://127.0.0.1:8080/acme/app.git")
	if err == nil {
		t.Fatal("回环 clone 目标应被拒绝")
	}
	if !strings.Contains(err.Error(), "出站") {
		t.Fatalf("错误信息应说明是出站校验拒绝，实际 %q", err.Error())
	}
}

// TestValidateRepoPathRejectsTraversal 路径穿越必须被拒。
//
// host 白名单拦不住路径穿越——它仍指向同一个 host，只是路径变了。
// owner = ".." 时拼出的路径会指到平台上的上一层。
func TestValidateRepoPathRejectsTraversal(t *testing.T) {
	cases := []struct {
		name  string
		owner string
		repo  string
	}{
		{name: "owner 为 ..", owner: "..", repo: "app"},
		{name: "repo 为 ..", owner: "acme", repo: ".."},
		{name: "owner 为 .", owner: ".", repo: "app"},
		{name: "owner 含斜杠", owner: "a/b", repo: "app"},
		{name: "repo 含斜杠", owner: "acme", repo: "a/b"},
		{name: "owner 含问号", owner: "acme?", repo: "app"},
		{name: "repo 含井号", owner: "acme", repo: "app#x"},
		{name: "owner 含 at", owner: "a@b", repo: "app"},
		{name: "repo 含百分号", owner: "acme", repo: "app%2f"},
		{name: "owner 为空", owner: "", repo: "app"},
		{name: "repo 为空", owner: "acme", repo: ""},
		{name: "owner 含空格", owner: "acme org", repo: "app"},
		{name: "repo 含反斜杠", owner: "acme", repo: `app\x`},
		{name: "repo 含控制字符", owner: "acme", repo: "app\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateRepoPath(tc.owner, tc.repo); err == nil {
				t.Fatalf("应拒绝非法路径段：owner=%q repo=%q", tc.owner, tc.repo)
			}
		})
	}
}

// TestValidateRepoPathAcceptsLegitimate 合法路径必须放行。
func TestValidateRepoPathAcceptsLegitimate(t *testing.T) {
	cases := []struct {
		owner string
		repo  string
	}{
		{owner: "acme", repo: "app"},
		{owner: "acme", repo: "app.git"},
		{owner: "my-group", repo: "my-project"},
		{owner: "acme", repo: "app_name"},
		{owner: "团队", repo: "应用"},
	}

	for _, tc := range cases {
		if err := ValidateRepoPath(tc.owner, tc.repo); err != nil {
			t.Fatalf("合法路径应放行：owner=%q repo=%q err=%v", tc.owner, tc.repo, err)
		}
	}
}
