// Package page 定义跨层传递的分页结果。
//
// 背景：仓储接口此前直接返回 ekit 的 gormx.PageResponse（基础设施包类型），
// 泄漏到 service 与 handler 层，导致业务层耦合具体 ORM 工具库。
// 本类型是中立契约；JSON tag 与 gormx.PageResponse 一致（items/total/page/
// page_size/pages），API 契约不变。ekit 类型只允许出现在仓储实现内部，
// 在仓储边界转换一次。
package page

// Page 是泛型分页结果。数据字段名 items 与 handler/crudh 的 REST 契约一致，
// pages 为总页数（向上取整），沿用 gormx 语义。
type Page[T any] struct {
	Items    []T   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Pages    int   `json:"pages"`
}
