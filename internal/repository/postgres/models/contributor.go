package models

import "time"

type ContributorStatsOptions struct {
	Provider string
	Repo     string
	Author   string
	From     time.Time
	To       time.Time
	Page     int
	PageSize int
	Sort     string
	Order    string
}

type ProjectStatsOptions struct {
	Provider string
	Repo     string
	From     time.Time
	To       time.Time
	Page     int
	PageSize int
	Sort     string
	Order    string
}

type ContributorSummary struct {
	Author        string    `json:"author"`
	AuthorName    string    `json:"author_name"`
	AuthorAvatar  string    `json:"author_avatar"`
	ReviewCount   int       `json:"review_count"`
	AvgScore      float64   `json:"avg_score"`
	PassRate      float64   `json:"pass_rate"`
	TotalFindings int       `json:"total_findings"`
	CriticalCount int       `json:"critical_count"`
	MajorCount    int       `json:"major_count"`
	MinorCount    int       `json:"minor_count"`
	InfoCount     int       `json:"info_count"`
	LastReviewAt  time.Time `json:"last_review_at"`
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
	Provider      string    `json:"provider"`
	Repo          string    `json:"repo"`
	ReviewCount   int       `json:"review_count"`
	AvgScore      float64   `json:"avg_score"`
	PassRate      float64   `json:"pass_rate"`
	TotalFindings int       `json:"total_findings"`
	CriticalCount int       `json:"critical_count"`
	MajorCount    int       `json:"major_count"`
	MinorCount    int       `json:"minor_count"`
	InfoCount     int       `json:"info_count"`
	LastReviewAt  time.Time `json:"last_review_at"`
}

type ProjectDetail struct {
	ProjectSummary
	MedianScore       float64        `json:"median_score"`
	MaxScore          int            `json:"max_score"`
	MinScore          int            `json:"min_score"`
	GradeDistribution map[string]int `json:"grade_distribution"`
	AvgFindings       float64        `json:"avg_findings"`
	AvgChangedLines   float64        `json:"avg_changed_lines"`
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

type ProjectListResult struct {
	Projects []ProjectSummary `json:"projects"`
	Total    int64            `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}
