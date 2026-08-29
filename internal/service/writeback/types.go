package writeback

import "context"

type Side string

const (
	SideOld Side = "old"
	SideNew Side = "new"
)

type Event string

const (
	EventComment        Event = "comment"
	EventReview         Event = "review"
	EventApprove        Event = "approve"
	EventRequestChanges Event = "request_changes"
)

type ChangeRequest struct {
	Provider   string
	Repository string
	Number     string
	BaseSHA    string
	HeadSHA    string
}

type LineRange struct {
	Start int
	End   int
	Side  Side
}

type Comment struct {
	Key        string
	Path       string
	Body       string
	Range      LineRange
	Suggestion string
}

type Review struct {
	ChangeRequest ChangeRequest
	Summary       string
	Event         Event
	Comments      []Comment
}

type Capabilities struct {
	BatchReview       bool
	MultiLineComments bool
	Suggestions       bool
	UpdateSummary     bool
}

type CommentResult struct {
	Key        string
	ExternalID string
	URL        string
	Inline     bool
	Error      string
}

type Result struct {
	ReviewID    string
	SummaryID   string
	SummaryURL  string
	Comments    []CommentResult
	Stale       bool
	ExpectedSHA string
	ActualSHA   string
}

type PublishStatusItem struct {
	FindingKey string `json:"finding_key"`
	Kind       string `json:"kind"`
	Status     string `json:"status"`
	Attempts   int    `json:"attempts"`
	ExternalID string `json:"external_id,omitempty"`
	URL        string `json:"url,omitempty"`
	LastError  string `json:"last_error,omitempty"`
}

type PublishStatus struct {
	ReviewID   string              `json:"review_id"`
	Total      int                 `json:"total"`
	Published  int                 `json:"published"`
	Failed     int                 `json:"failed"`
	Downgraded int                 `json:"downgraded"`
	Pending    int                 `json:"pending"`
	Items      []PublishStatusItem `json:"items"`
}

type Adapter interface {
	Capabilities(context.Context, ChangeRequest) (Capabilities, error)
	Publish(context.Context, Review) (Result, error)
}
