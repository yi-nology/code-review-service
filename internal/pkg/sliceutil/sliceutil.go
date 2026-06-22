package sliceutil

import "strings"

// FirstNonBlank 返回第一个非空字符串
func FirstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// FirstPositive 返回第一个正数
func FirstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

// FirstPositiveOr 返回第一个正数，用于覆盖默认值
func FirstPositiveOr(values ...int) int {
	for _, v := range values {
		if v > 0 {
			return v
		}
	}
	return 0
}
