// 死信判定阈值：review 因重试次数耗尽被标记为死信。
// ⚠️ 必须与后端 internal/repository/postgres/review.go 的 MaxRequeueRetries 保持一致。
// 后端死信 requeue 接口仅对 retry_count >= DEAD_LETTER_THRESHOLD 的 failed 评审生效。
export const DEAD_LETTER_THRESHOLD = 5
