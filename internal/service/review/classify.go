package review

import (
	"github.com/oekit/pkg/stringx"
)

var categoryKeywords = []struct {
	Category string
	Keywords []string
}{
	{"安全性", []string{"sql injection", "xss", "csrf", "vulnerability", "security", "权限", "注入", "漏洞"}},
	{"性能", []string{"performance", "slow", "memory", "性能", "内存"}},
	{"测试", []string{"test", "coverage", "测试", "覆盖率"}},
	{"正确性", []string{"nil", "null", "error", "panic", "错误", "异常", "失效", "空指针"}},
}

func classifyFinding(finding Finding) string {
	text := finding.Title + " " + finding.Body
	for _, entry := range categoryKeywords {
		if stringx.ContainsAny(text, entry.Keywords...) {
			return entry.Category
		}
	}
	return "可维护性"
}
