// cloneurl.go：clone URL 的**出站防护**——判据来自底座 agent-harness/netguard。
//
// # 缺口从哪来
//
// 评审请求的 repo 路径（"owner/repo"）来自 **webhook 载荷**——外部可控。
// 它经 ParseRepoPath 拆成 owner/repo 后，**未经任何校验**就拼进 clone URL：
//
//	base + "/" + owner + "/" + repo + ".git"
//
// 两个后果：
//   - **路径穿越**：owner 含 ".." 时拼出的 URL 指向平台上的另一个路径；
//     含 "?" / "#" 时能截断后缀、把请求改向别的资源。
//   - **host 逃逸**：owner/repo 含 "//" 或 "@" 时（如 owner = "@evil.com"），
//     拼出的 URL 的 host 会变成 evil.com——服务带着 git token 去访问攻击者的服务器，
//     token 直接泄露。这是最严重的一种。
//
// # 为什么用 netguard 而不是自己写正则
//
// host 逃逸的判定要同时看「拼出来的 URL 的 host 是不是预期的那个」，
// 而 host 判定涉及解析、DNS、协议层——自己写正则只能覆盖部分形态
// （如大小写、端口、userinfo、IPv6 字面量），漏一种就是一次 token 泄露。
// netguard 的 CheckURLWith 走的是「解析 + 名字层 + IP 层」三道判定，
// 且判定依据（egress 网段表）在底座单源持有。
//
// # 放行面的取舍
//
// netguard 的零值 Allow{} 是「私网与回环全拒」。本服务**不能**用零值：
// 自建 GitLab 常部署在内网，零值会把正常 clone 全拦掉。
//
// 所以这里用 Allow{Private: true}（放行私网、仍拒回环），
// 把「防回环（SSRF 打本机）」保留住，而「防外网」由**同 host 白名单**承担——
// 拼出来的 URL 的 host 必须与该平台配置的 base URL 的 host 完全一致。
//
// 这是本文件真正的判据：clone 目标只能是管理员配过的那个平台，
// 不可能是别的任何地方。白名单是按 host 比的，不受 DNS rebinding 影响
// （rebinding 影响的是「host 解析到哪个 IP」，而这里约束的是「host 是谁」）。
package vcs

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/oagentkit/guardx/egress"
	"github.com/yi-nology/agent-harness/netguard"
)

// CloneURLGuard 校验 clone URL 的出站目标。
type CloneURLGuard struct {
	// allow 是底座的放行面。放行私网（自建平台常在内网），仍拒回环。
	allow netguard.Allow
}

// NewCloneURLGuard 构造 clone URL 守卫。
//
// 放行面：私网放行、回环拒绝。
// 回环必须拒——回环是 SSRF 打本机服务的入口（cloud metadata、
// 管理员面板、数据库），而 clone 目标没有任何正当理由落在回环上。
func NewCloneURLGuard() *CloneURLGuard {
	return &CloneURLGuard{allow: netguard.Allow{Private: true}}
}

// Validate 校验 clone URL 是否可安全出站。
//
// baseURL 是该平台**已配置**的基地址（如 https://gitlab.example.com），
// cloneURL 是由 owner/repo 拼出的实际目标。
//
// 判据三条，缺一不可：
//  1. cloneURL 的 host 必须与 baseURL 的 host **完全一致**
//     （含端口）。这条挡住 owner/repo 里的 "//"、"@" 造成的 host 逃逸。
//  2. scheme 必须是 http/https（底座口径）。
//  3. host 若写成 IP 字面量，须通过出站校验（拒回环）；
//     域名形态由判据 1 兜住——它只能是配置里那个平台。
//
// 注意这里**不**做 DNS 解析：clone 本来就要连那个 host，
// DNS 失败会由 git 自己报更准确的错。在校验阶段强解析 DNS 的后果是
// 「DNS 抖动 → 全部 clone 失败」，而那不是校验该拦的东西。
func (g *CloneURLGuard) Validate(baseURL, cloneURL string) error {
	base, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return fmt.Errorf("平台基地址非法: %w", err)
	}
	target, err := url.Parse(strings.TrimSpace(cloneURL))
	if err != nil {
		return fmt.Errorf("clone URL 非法: %w", err)
	}

	// ① host 必须一致。
	//
	// 比 Host（含端口）而不是 Hostname（不含端口）：
	// 端口不同意味着不同的服务，目标就不是管理员配的那个了。
	if !sameHost(base, target) {
		return fmt.Errorf("clone 目标 host 不匹配：期望 %q，实际 %q", baseHost(base), targetHost(target))
	}

	// ② scheme 白名单。
	if target.Scheme != "http" && target.Scheme != "https" {
		return fmt.Errorf("clone 目标 scheme %q 不在白名单（仅 http/https）", target.Scheme)
	}

	// ③ 目标不得带 query / fragment。
	//
	// 这条挡的是**保留字符截断**：owner 含 "?" 时，拼出的 URL 解析后
	// query 非空、path 被截短（".../a?b/repo.git" 的 path 只有 "/a"）——
	// 请求的目标资源与管理员配的那个不是同一个。host 白名单拦不住它，
	// 因为 host 确实没变。
	//
	// clone URL 本身不该带 query/fragment（它是 git 仓库地址，不是 API 调用），
	// 所以这条判据不会误伤正常流量。
	if target.RawQuery != "" || target.Fragment != "" {
		return fmt.Errorf("clone 目标含 query 或 fragment（疑似路径截断）：%q", cloneURL)
	}

	// ④ 路径形态校验。
	//
	//  4a. 不得含 "@"：它在路径里虽不改变 host（解析上 host 已定），
	//      但它没有任何正当用途——仓库路径里出现它只可能是构造出来的。
	//      让它进不了 clone URL，就不会有「哪天拼接方式一变，@ 就变成
	//      userinfo 逃逸」的那一天。
	//
	//  4b. 不得含重复斜杠：owner = "//evil.com" 会拼出路径 "///evil.com/repo.git"，
	//      既不是合法仓库路径，也是「构造出来」的信号。
	//      平台侧对重复斜杠的处理各不相同（有的规范化、有的当不同仓库），
	//      与其猜平台的宽容度，不如在入口就拒掉。
	if strings.Contains(target.Path, "@") {
		return fmt.Errorf("clone 目标路径含非法字符 '@'：%q", cloneURL)
	}
	if strings.Contains(target.Path, "//") {
		return fmt.Errorf("clone 目标路径含重复斜杠：%q", cloneURL)
	}

	// 4c. 不得含 "." 或 ".." 路径段：那是路径穿越。
	//
	// host 白名单拦不住它——路径穿越仍指向同一个 host，只是路径变了，
	// 可能落到平台上管理员没打算让你 clone 的另一个仓库。
	for _, seg := range strings.Split(target.Path, "/") {
		if seg == "." || seg == ".." {
			return fmt.Errorf("clone 目标路径含相对路径段（疑似路径穿越）：%q", cloneURL)
		}
	}

	// ④ host 是 IP 字面量时走出站校验（拒回环）。
	//
	// 域名形态不在此判：判据 1 已保证它只能是配置里的那个平台，
	// 而域名背后解析到哪个 IP 由 DNS 与拨号层负责，不是这里能判的。
	if net.ParseIP(target.Hostname()) != nil {
		if err := egress.CheckHostWith(target.Hostname(), g.egressOptions()); err != nil {
			return fmt.Errorf("clone 目标未通过出站校验: %w", err)
		}
	}

	return nil
}

// egressOptions 把放行面翻译成底座的 egress 判据。
//
// 与 netguard 的 Allow 是同一套口径（私网放行、回环拒绝），
// 但走 egress 而不是 netguard.CheckURLWith——后者会做 DNS 解析，
// 那是 clone 阶段的事，不该在校验阶段强依赖。
func (g *CloneURLGuard) egressOptions() egress.Options {
	return egress.Options{AllowPrivate: g.allow.Private, AllowLoopback: g.allow.Loopback}
}

// sameHost 判断两个 URL 的 host（含端口）是否完全一致。
//
// 大小写不敏感：host 名在 DNS 里不区分大小写，
// 但 scheme 是区分的（HTTP 与 HTTPS 是两个服务）。
func sameHost(a, b *url.URL) bool {
	return strings.EqualFold(a.Scheme, b.Scheme) && a.Host == b.Host
}

// baseHost 返回用于错误信息的 host 展示串。
func baseHost(u *url.URL) string { return u.Scheme + "://" + u.Host }

// targetHost 返回用于错误信息的 host 展示串。
func targetHost(u *url.URL) string { return u.Scheme + "://" + u.Host }

// ValidateRepoPath 校验仓库路径的每个路径段。
//
// 这是 host 白名单之外的**第二道**防线：路径穿越（owner 含 ".."）
// 不会被 host 校验拦下——它仍指向同一个 host，只是路径变了。
//
// # owner 可能是多段
//
// GitLab 的嵌套 group 是常态：`group/subgroup/project` 拆出来的 owner 是
// `group/subgroup`（ParseRepoPath 把除最后一段外的全部前缀留给 owner）。把整个 owner
// 当成一段来校验，会把这类仓库**全部误杀**——现网日志里就有：
//
//	clone 目标校验失败: 仓库路径段 owner 含非法字符："group/subgroup"
//
// 而 BuildCloneURL 拼出的 `base/group/subgroup/project.git` 本身是对的，
// CloneURLGuard 也放行它（守卫只关心 host 与路径形态）——两层自相矛盾，
// 结果是嵌套组仓库一律评审失败。
//
// 因此这里按 "/" 拆开**逐段**校验。防护一条没少，只是判据落到了正确的位置：
//   - ".." / "." 段 → 路径穿越，仍拒；
//   - 空段（"obs//doc"、"/obs"）→ 拼出的路径含重复斜杠，仍拒；
//   - "?" "#" "@" "%" 空格/控制字符/反斜杠 → 仍逐段拒；
//   - owner 里的 "/" 不再算非法字符——它就是嵌套 group 本身。
//
// 判据（逐段）：
//   - 非空
//   - 不含 ".." 或 "."（路径穿越与当前目录）
//   - 不含 URL 保留字符（"?"、"#"、"@"、"%"）与控制字符
//   - 不含空格与反斜杠
func ValidateRepoPath(owner, repo string) error {
	for _, seg := range strings.Split(owner, "/") {
		if err := validatePathSegment("owner", seg); err != nil {
			return err
		}
	}
	return validatePathSegment("repo", repo)
}

// validatePathSegment 校验单个路径段。
func validatePathSegment(kind, seg string) error {
	if strings.TrimSpace(seg) == "" {
		return fmt.Errorf("仓库路径段 %s 为空", kind)
	}
	if seg == "." || seg == ".." {
		return fmt.Errorf("仓库路径段 %s 非法：%q", kind, seg)
	}
	if strings.ContainsAny(seg, "/\\?#@% \t\r\n") {
		return fmt.Errorf("仓库路径段 %s 含非法字符：%q", kind, seg)
	}
	for _, r := range seg {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("仓库路径段 %s 含控制字符：%q", kind, seg)
		}
	}
	return nil
}
