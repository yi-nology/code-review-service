package llm

import (
	"encoding/json"
	"strings"
)

// parseJSONLoose 从可能含 markdown 围栏或前后噪声的 LLM 响应中提取并解析 JSON。
// LLM 即使被要求返回 JSON，有时仍会包 ```json ``` 围栏或附加说明，需容错。
func parseJSONLoose(content string, out any) error {
	s := strings.TrimSpace(content)

	// 去除 ```json ... ``` / ``` ... ``` 围栏
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
		if idx := strings.IndexByte(s, '\n'); idx >= 0 {
			first := strings.TrimSpace(s[:idx])
			if first == "json" || first == "" {
				s = s[idx+1:]
			}
		}
		s = strings.TrimSpace(s)
		s = strings.TrimSuffix(s, "```")
		s = strings.TrimSpace(s)
	}

	// 定位首个 { 或 [ 到末尾对应的闭合符
	start := strings.IndexAny(s, "{[")
	end := strings.LastIndexAny(s, "}]")
	if start < 0 || end <= start {
		return json.Unmarshal([]byte(s), out)
	}
	return json.Unmarshal([]byte(s[start:end+1]), out)
}
