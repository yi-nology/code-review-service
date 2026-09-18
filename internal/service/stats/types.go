package stats

import "time"

type ContributorSummary struct {
	Author        string     `json:"author"`
	AuthorName    string     `json:"author_name"`
	AuthorAvatar  string     `json:"author_avatar"`
	ReviewCount   int        `json:"review_count"`
	AvgScore      float64    `json:"avg_score"`
	PassRate      float64    `json:"pass_rate"`
	TotalFindings int        `json:"total_findings"`
	CriticalCount int        `json:"critical_count"`
	MajorCount    int        `json:"major_count"`
	MinorCount    int        `json:"minor_count"`
	InfoCount     int        `json:"info_count"`
	LastReviewAt  *time.Time `json:"last_review_at"`
}

type ContributorDetail struct {
	ContributorSummary
	MedianScore       float64        `json:"median_score"`
	MaxScore          int            `json:"max_score"`
	MinScore          int            `json:"min_score"`
	GradeDistribution map[string]int `json:"grade_distribution"`
	AvgFindings       float64        `json:"avg_findings"`
	AvgChangedLines   float64        `json:"avg_changed_lines"`
}

type TrendPoint struct {
	Period      string  `json:"period"`
	AvgScore    float64 `json:"avg_score"`
	ReviewCount int     `json:"review_count"`
}

type CategoryStat struct {
	Category string `json:"category"`
	Count    int    `json:"count"`
}

type ContributorListResult struct {
	Contributors []ContributorSummary `json:"contributors"`
	Total        int64                `json:"total"`
	Page         int                  `json:"page"`
	PageSize     int                  `json:"page_size"`
}

type ProjectSummary struct {
	Provider      string     `json:"provider"`
	Repo          string     `json:"repo"`
	ReviewCount   int        `json:"review_count"`
	AvgScore      float64    `json:"avg_score"`
	PassRate      float64    `json:"pass_rate"`
	TotalFindings int        `json:"total_findings"`
	CriticalCount int        `json:"critical_count"`
	MajorCount    int        `json:"major_count"`
	MinorCount    int        `json:"minor_count"`
	InfoCount     int        `json:"info_count"`
	LastReviewAt  *time.Time `json:"last_review_at"`
}

type ProjectDetail struct {
	ProjectSummary
	MedianScore       float64        `json:"median_score"`
	MaxScore          int            `json:"max_score"`
	MinScore          int            `json:"min_score"`
	GradeDistribution map[string]int `json:"grade_distribution"`
	AvgFindings       float64        `json:"avg_findings"`
	AvgChangedLines   float64        `json:"avg_changed_lines"`
	Findings          []FileFinding  `json:"findings"`
}

type ProjectTrendPoint struct {
	Period        string  `json:"period"`
	AvgScore      float64 `json:"avg_score"`
	ReviewCount   int     `json:"review_count"`
	TotalFindings int     `json:"total_findings"`
}

type FileRiskStat struct {
	Path          string `json:"path"`
	Count         int    `json:"count"`
	CriticalCount int    `json:"critical_count"`
	MajorCount    int    `json:"major_count"`
}

type FileFinding struct {
	ReviewID  string `json:"review_id"`
	Path      string `json:"path"`
	Line      int    `json:"line"`
	Severity  string `json:"severity"`
	Title     string `json:"title"`
	Category  string `json:"category"`
	CreatedAt string `json:"created_at"`
}

type ProjectListResult struct {
	Projects []ProjectSummary `json:"projects"`
	Total    int64            `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

type ProjectOCRMetrics struct {
	ReviewCount        int64            `json:"review_count"`
	TotalTokens        int64            `json:"total_tokens"`
	InputTokens        int64            `json:"input_tokens"`
	OutputTokens       int64            `json:"output_tokens"`
	CacheReadTokens    int64            `json:"cache_read_tokens"`
	CacheWriteTokens   int64            `json:"cache_write_tokens"`
	TotalReviewedFiles int64            `json:"total_reviewed_files"`
	AvgTokens          float64          `json:"avg_tokens"`
	AvgScore           float64          `json:"avg_score"`
	AvgFindingsDensity float64          `json:"avg_findings_density"`
	GradeDistribution  map[string]int64 `json:"grade_distribution"`
	ToolCalls          []ToolCallStat   `json:"tool_calls"`
}

type ProjectOCRTrendPoint struct {
	Period          string  `json:"period"`
	ReviewCount     int64   `json:"review_count"`
	TotalTokens     int64   `json:"total_tokens"`
	AvgTokens       float64 `json:"avg_tokens"`
	AvgElapsedMs    float64 `json:"avg_elapsed_ms"`
	AvgScore        float64 `json:"avg_score"`
	FindingsDensity float64 `json:"findings_density"`
}

type ToolCallStat struct {
	Tool  string `json:"tool"`
	Count int64  `json:"count"`
}
