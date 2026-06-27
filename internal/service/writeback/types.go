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
	Number     int
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
	ReviewID   string
	SummaryID  string
	SummaryURL string
	Comments   []CommentResult
}

type Adapter interface {
	Capabilities(context.Context, ChangeRequest) (Capabilities, error)
	Publish(context.Context, Review) (Result, error)
}
