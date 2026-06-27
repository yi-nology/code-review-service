package models

type ListOptions struct {
	Status   string
	Provider string
	Repo     string
	Page     int
	PageSize int
}

type AggregateStats struct {
	Running    int
	GateFailed int
	Findings   int

	// Token 用量聚合（仅统计成功评审），让运营能看到 token 花销（#20）。
	TotalTokens      int64
	InputTokens      int64
	OutputTokens     int64
	CacheReadTokens  int64
	CacheWriteTokens int64
}

type ProjectOCRMetrics struct {
	ReviewCount        int64
	TotalTokens        int64
	InputTokens        int64
	OutputTokens       int64
	CacheReadTokens    int64
	CacheWriteTokens   int64
	TotalReviewedFiles int64
	AvgTokens          float64
	AvgScore           float64
	AvgFindingsDensity float64
	GradeDistribution  map[string]int64
	ToolCalls          []ToolCallStat
}

type ProjectOCRTrendPoint struct {
	Period          string
	ReviewCount     int64
	TotalTokens     int64
	AvgTokens       float64
	AvgElapsedMs    float64
	AvgScore        float64
	FindingsDensity float64
}

type ToolCallStat struct {
	Tool  string `json:"tool"`
	Count int64  `json:"count"`
}
