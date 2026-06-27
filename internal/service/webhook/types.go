package webhook

type ChangeEvent struct {
	Provider       string
	InstallationID string
	DeliveryID     string
	EventType      string
	Action         string
	RepositoryID   string
	Repository     string
	Number         int
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
