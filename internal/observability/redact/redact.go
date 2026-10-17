// Package redact 提供日志 / 错误信息中的敏感数据脱敏工具。
//
// 背景：runner.ExitError 把子进程 stderr 整体写入错误信息，stderr 中常包含
// LLM 提供方回显的 Authorization / Bearer / api_key 等敏感字段。如果把这些
// 信息无脑写到 logx.Infof / logx.Errorf，会泄露到日志收集系统。
//
// 主体委托 agentkit/logredact（审计过的 RE2 线性实现，覆盖 URL 内嵌凭据 /
// 键值对 / PEM 私钥 / Authorization 头 / Bearer / 平台 token 前缀 sk- ghp_ gho_
// github_pat_ xox AKIA / JWT）。本地仅保留 logredact 覆盖不到的补充规则。
//
// 用法：
//
//	if err != nil {
//	    return redact.Errorf(err, "ocr review failed: %s", path)
//	}
//
// 或者把字符串先过 redact.String(s) 再传给 logger。
package redact

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/oagentkit/logredact"
)

// localPatterns 是 logredact 覆盖不到的补充形态，先于 logredact 执行
// （保留键名/关键字前缀的打码风格）：
//   - Authorization 头完整值：logredact 的同名规则只吃到首个空白
//     （"Authorization: Basic xxx" 会漏掉 Basic 后的值）；
//   - api-key 家族的带引号 JSON 键形态（"api_key": "..."，键与分隔符之间
//     有引号，logredact 的键值规则不命中）——LLM 报错体内嵌原始 JSON 是
//     本服务的高频脱敏面；
//   - 散文中裸 "token/bearer <值>"（无键值分隔符；值若为已知平台前缀
//     logredact 兜底，此处兜任意 8+ 位值）；
//   - glpat-：GitLab personal access token（本项目对接 GitLab，clone URL /
//     webhook secret 常含此形态，logredact 平台前缀名单未收录）。
var localPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(authorization\s*[:=]\s*)(?:bearer|basic|token)\s+[^\s"',;}{]+`),
	regexp.MustCompile(`(?i)(api[_-]?key|apikey|access[_-]?key|secret[_-]?key|private[_-]?key)\s*["']?\s*[:=]\s*["']?[^\s"',;}{][^,}\s"';]*`),
	regexp.MustCompile(`(?i)\b(bearer|token)\s+[A-Za-z0-9._\-+/=]{8,}`),
	regexp.MustCompile(`\bglpat-[A-Za-z0-9_\-]{20,}\b`),
}

// String 返回 s 的脱敏版本：先过本地补充规则，再过 agentkit/logredact 核心。
func String(s string) string {
	if s == "" {
		return s
	}
	out := s
	for _, p := range localPatterns {
		out = p.ReplaceAllStringFunc(out, redactMatch)
	}
	return logredact.Redact(out)
}

// redactedError 包装 error，在 Error() 输出时自动脱敏，同时保留 Unwrap 链。
type redactedError struct {
	msg string
	err error
}

func (e *redactedError) Error() string {
	if e.err == nil {
		return String(e.msg)
	}
	return String(e.msg) + ": " + String(e.err.Error())
}

func (e *redactedError) Unwrap() error { return e.err }

// Errorf 包装 fmt.Errorf 同时对底层 err 做脱敏。
// 保留 errors.Is / errors.As 链路（Unwrap 实现）。
func Errorf(err error, format string, args ...any) error {
	return &redactedError{
		msg: fmt.Sprintf(format, args...),
		err: err,
	}
}

func redactMatch(s string) string {
	// 保留前缀（如 "Authorization: "），但替换掉 value
	idx := indexKeySep(s)
	if idx < 0 || idx >= len(s)-1 {
		return "***"
	}
	prefix := s[:idx+1]
	rest := strings.TrimSpace(s[idx+1:])
	// 如果包含 Bearer/Token/Basic 等，保留关键字后替换
	for _, kw := range []string{"Bearer", "bearer", "Basic", "basic", "Token", "token"} {
		if strings.HasPrefix(rest, kw+" ") || rest == kw {
			return prefix + kw + " ***"
		}
	}
	return prefix + "***"
}

// indexKeySep 查找 "key + : / =" 分隔符位置。
func indexKeySep(s string) int {
	for i, c := range s {
		if c == ':' || c == '=' {
			return i
		}
	}
	return -1
}
