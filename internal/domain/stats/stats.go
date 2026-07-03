// Package stats 是评审统计查询/结果的领域类型（#6 迁移）。
// 原 models/contributor.go 中的纯 DTO（无 GORM 标签），统一迁入此处。
// ProjectStatsOptions 同时作为 ReviewRepository 与 ContributorStatsRepository 的共享查询类型。
package stats

import "time"

// ContributorStatsOptions 是贡献者统计查询条件。
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

// ProjectStatsOptions 是项目级统计查询条件（Review/Contributor 共享）。
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
