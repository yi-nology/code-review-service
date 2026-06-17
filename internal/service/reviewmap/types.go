package reviewmap

import (
	"time"

	"github.com/yi-nology/code-review-service/internal/service/types"
)

type ReviewMap struct {
	ID           string       `json:"id"`
	SessionID    string       `json:"session_id"`
	Title        string       `json:"title"`
	Description  string       `json:"description"`
	Sections     []Section    `json:"sections"`
	Dependencies []Dependency `json:"dependencies"`
	Summary      string       `json:"summary"`
	FileCount    int          `json:"file_count"`
	SectionCount int          `json:"section_count"`
	CreatedAt    time.Time    `json:"created_at"`
}

type Section struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Files       []File   `json:"files"`
	Findings    []string `json:"findings"`
	Progress    float64  `json:"progress"`
	Priority    string   `json:"priority"`
}

type File struct {
	Path         string   `json:"path"`
	Language     string   `json:"language"`
	LinesAdded   int      `json:"lines_added"`
	LinesRemoved int      `json:"lines_removed"`
	Findings     []string `json:"findings"`
	Reviewed     bool     `json:"reviewed"`
}

type Dependency struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Type     string `json:"type"`
	Strength string `json:"strength"`
}

type Finding = types.Finding
