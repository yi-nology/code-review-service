package models

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ReviewMap represents a code review map for a session
type ReviewMap struct {
	gorm.Model     `json:"-"`
	MapID          string         `gorm:"type:text;uniqueIndex;not null" json:"map_id"`
	ReviewID       string         `gorm:"type:text;not null;index" json:"review_id"`
	SessionID      string         `gorm:"type:text;not null;index" json:"session_id"`
	Title          string         `gorm:"type:text;not null" json:"title"`
	Description    string         `gorm:"type:text" json:"description"`
	Sections       datatypes.JSON `gorm:"type:jsonb" json:"sections"`
	Dependencies   datatypes.JSON `gorm:"type:jsonb" json:"dependencies"`
	Summary        string         `gorm:"type:text" json:"summary"`
	FileCount      int            `gorm:"not null;default:0" json:"file_count"`
	SectionCount   int            `gorm:"not null;default:0" json:"section_count"`
	MermaidDiagram string         `gorm:"type:text" json:"mermaid_diagram,omitempty"`
	CompletedAt    *time.Time     `json:"completed_at,omitempty"`
}

func (ReviewMap) TableName() string {
	return "review_maps"
}

// SectionData represents a section in the review map (for JSON storage)
type SectionData struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Files       []FileData `json:"files"`
	Findings    []string   `json:"findings"`
	Progress    float64    `json:"progress"`
	Priority    string     `json:"priority"`
}

// FileData represents a file in a section (for JSON storage)
type FileData struct {
	Path         string   `json:"path"`
	Language     string   `json:"language"`
	LinesAdded   int      `json:"lines_added"`
	LinesRemoved int      `json:"lines_removed"`
	Findings     []string `json:"findings"`
	Reviewed     bool     `json:"reviewed"`
}

// DependencyData represents a dependency (for JSON storage)
type DependencyData struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Type     string `json:"type"`
	Strength string `json:"strength"`
}
