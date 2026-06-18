package persona

import (
	"time"
)

type ReviewerPersona struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	DisplayName    string    `json:"display_name"`
	Description    string    `json:"description"`
	FocusAreas     []string  `json:"focus_areas"`
	PromptTemplate string    `json:"prompt_template"`
	Category       string    `json:"category"`
	Tags           []string  `json:"tags"`
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type PersonaCategory string

const (
	CategoryGeneralist PersonaCategory = "generalist"
	CategorySpecialist PersonaCategory = "specialist"
	CategoryFamous     PersonaCategory = "famous"
	CategoryCustom     PersonaCategory = "custom"
)

type PersonaConfig struct {
	Name           string   `yaml:"name"`
	DisplayName    string   `yaml:"display_name"`
	Description    string   `yaml:"description"`
	FocusAreas     []string `yaml:"focus_areas"`
	PromptTemplate string   `yaml:"prompt_template"`
	Category       string   `yaml:"category"`
	Tags           []string `yaml:"tags"`
}

type Repository interface {
	Create(persona *ReviewerPersona) error
	GetByID(id string) (*ReviewerPersona, error)
	GetByName(name string) (*ReviewerPersona, error)
	List() ([]*ReviewerPersona, error)
	ListByCategory(category string) ([]*ReviewerPersona, error)
	Update(persona *ReviewerPersona) error
	Delete(id string) error
	Search(query string) ([]*ReviewerPersona, error)
}
