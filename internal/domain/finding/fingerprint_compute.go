package finding

// ComputeFingerprints 为一组 findings 批量计算并填充 Fingerprint 字段。
//
// 应在 findings 最终组装完成（聚合/去重之后、写入 Response 之前）调用一次，
// 确保同一 review 内指纹已就绪，且所有持久化/回写/反馈路径拿到一致的指纹。
// 已有指纹的 finding 会被覆盖（统一以算法为准，避免外部注入不一致值）。
func ComputeFingerprints(findings []Finding) {
	for i := range findings {
		findings[i].Fingerprint = Fingerprint(findings[i])
	}
}
