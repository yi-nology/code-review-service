package cost

import (
	"github.com/yi-nology/code-review-service/internal/domain/pricing"
)

// Pricing 是模型单价的业务/API 类型。
//
// 类型本体已移到 internal/domain/pricing（领域类型不属于任何一个 service 包，
// 仓储层与 HTTP 层都要用它）。这里保留别名是为了不改动 handler 的绑定代码：
// handler 绑的是 cost.Pricing，别名与原类型完全同一，JSON 字段不变。
type Pricing = pricing.Pricing
