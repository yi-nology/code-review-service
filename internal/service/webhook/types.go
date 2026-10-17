// Package webhook 把各平台 webhook payload 归一化为 ChangeEvent。
// Number 是平台 PR/MR 编号，统一为 string（对齐 go-git-platform v0.48 的
// ChangeRequest.Number），仅在与 DB/thrift 等数值边界的交接处转换。
package webhook

type ChangeEvent struct {
	Provider       string
	InstallationID string
	DeliveryID     string
	EventType      string
	Action         string
	RepositoryID   string
	Repository     string
	Number         string
	BaseSHA        string
	StartSHA       string
	HeadSHA        string
	Sender         string
	Author         string
	AuthorName     string
	AuthorAvatar   string
}

type Normalizer interface {
	Normalize(rawBody []byte, eventType string) (*ChangeEvent, error)
	VerifySignature(body []byte, signature string, secret string) bool
	EventType(headerValue string) string
	DeliveryID(headerValue string) string
}
