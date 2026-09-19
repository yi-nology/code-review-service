package finding

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strconv"
	"strings"
)

// Fingerprint 为单个 Finding 计算跨 review 稳定的指纹。
//
// 设计目标：同一处代码的同一类问题，在多次评审中应产生相同指纹，
// 用于支撑「新增/存量/已修复」生命周期与误报抑制。
//
// 指纹因子（刻意排除会跨 review 漂移的内容）：
//   - path（归一化路径）—— 问题的位置载体，最稳定
//   - category（问题类别）—— 比 LLM 自由文本稳定，代表问题类型
//   - title 签名（规范化后）—— 短标题比 body 全文稳定得多；body 是 LLM 每次生成的长文本
//   - line 桶（line / lineBucketSize）—— 粗粒度分桶，容忍上下文增删导致的几行漂移
//
// 刻意排除：
//   - body 全文 —— LLM 每次生成的描述不同，纳入会导致同一问题判定为不同指纹
//   - 精确 line —— 代码迭代后行号会漂移；用桶容忍小幅变化
//   - severity —— 同一问题在不同模型/配置下严重级别判定可能不同，不应影响"是不是同一个问题"
func Fingerprint(f Finding) string {
	h := sha256.New()
	h.Write([]byte(normalizeFindingPath(f.Path)))
	h.Write([]byte{0})
	h.Write([]byte(strings.ToLower(strings.TrimSpace(f.Category))))
	h.Write([]byte{0})
	h.Write([]byte(titleSignature(f.Title)))
	h.Write([]byte{0})
	h.Write([]byte(lineBucket(f.Line)))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// lineBucketSize 是行号分桶粒度：10 行一桶。
// 同一段代码的同一问题，因上下文增删导致行号小幅漂移时，仍落在同一桶内。
const lineBucketSize = 10

// lineBucket 把行号映射为粗粒度桶字符串。
// line <= 0 视为未定位，桶为 "*"（所有未定位问题共用，因无法区分位置）。
func lineBucket(line int) string {
	if line <= 0 {
		return "*"
	}
	return strconv.Itoa(line / lineBucketSize)
}

// titleSignature 对标题做规范化签名：去首尾空白、转小写、折叠连续空白。
// 这样 "SQL Injection" / "sql  injection " / "SQL injection" 会产生相同签名。
func titleSignature(title string) string {
	s := strings.ToLower(strings.TrimSpace(title))
	// 折叠连续空白（含换行/制表符）为单个空格
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}

// normalizeFindingPath 归一化文件路径，与 writeback.normalizePath 语义一致但独立实现
// （types 包不应反向依赖 writeback 包）。路径跨 review 是最稳定的因子。
func normalizeFindingPath(p string) string {
	p = filepath.ToSlash(p)
	p = filepath.Clean(p)
	if strings.HasPrefix(p, "../") || strings.HasPrefix(p, "/") {
		return ""
	}
	p = strings.TrimPrefix(p, "./")
	return p
}
