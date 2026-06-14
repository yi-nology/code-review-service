package models

import (
	"gorm.io/gorm"
)

// ProjectAgent represents the association between a project and an agent
type ProjectAgent struct {
	gorm.Model `json:"-"`
	Provider   string `gorm:"type:text;not null;uniqueIndex:idx_project_agent" json:"provider"`
	Repo       string `gorm:"type:text;not null;uniqueIndex:idx_project_agent" json:"repo"`
	AgentID    uint   `gorm:"not null;uniqueIndex:idx_project_agent" json:"agent_id"`
	Enabled    bool   `gorm:"not null;default:true" json:"enabled"`
	Priority   int    `gorm:"not null;default:0" json:"priority"` // Priority within this project

	// Joined fields (not stored in this table)
	AgentName   string `gorm:"-" json:"agent_name,omitempty"`
	DisplayName string `gorm:"-" json:"display_name,omitempty"`
	Background  string `gorm:"-" json:"background,omitempty"`
}

func (ProjectAgent) TableName() string {
	return "project_agents"
}

// ProjectAgentConfig represents the full agent configuration for a project
type ProjectAgentConfig struct {
	Provider string         `json:"provider"`
	Repo     string         `json:"repo"`
	Agents   []ProjectAgent `json:"agents"`
}
