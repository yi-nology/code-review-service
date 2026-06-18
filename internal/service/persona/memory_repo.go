package persona

import (
	"fmt"
	"strings"
	"time"

	"github.com/yi-nology/code-review-service/internal/service/types"
)

type MemoryRepository struct {
	personas map[string]*ReviewerPersona
}

func NewMemoryRepository() *MemoryRepository {
	repo := &MemoryRepository{personas: make(map[string]*ReviewerPersona)}
	for _, p := range DefaultPersonas() {
		repo.personas[p.ID] = &p
	}
	return repo
}

func (r *MemoryRepository) Create(persona *ReviewerPersona) error {
	if persona.ID == "" {
		return fmt.Errorf("persona ID is required")
	}
	if _, exists := r.personas[persona.ID]; exists {
		return fmt.Errorf("persona already exists: %s", persona.ID)
	}
	persona.CreatedAt = time.Now()
	persona.UpdatedAt = time.Now()
	r.personas[persona.ID] = persona
	return nil
}

func (r *MemoryRepository) GetByID(id string) (*ReviewerPersona, error) {
	p, exists := r.personas[id]
	if !exists {
		return nil, fmt.Errorf("persona not found: %s", id)
	}
	return p, nil
}

func (r *MemoryRepository) GetByName(name string) (*ReviewerPersona, error) {
	for _, p := range r.personas {
		if p.Name == name {
			return p, nil
		}
	}
	return nil, fmt.Errorf("persona not found: %s", name)
}

func (r *MemoryRepository) List() ([]*ReviewerPersona, error) {
	personas := make([]*ReviewerPersona, 0, len(r.personas))
	for _, p := range r.personas {
		personas = append(personas, p)
	}
	return personas, nil
}

func (r *MemoryRepository) ListByCategory(category string) ([]*ReviewerPersona, error) {
	var personas []*ReviewerPersona
	for _, p := range r.personas {
		if p.Category == category {
			personas = append(personas, p)
		}
	}
	return personas, nil
}

func (r *MemoryRepository) Update(persona *ReviewerPersona) error {
	if persona.ID == "" {
		return fmt.Errorf("persona ID is required")
	}
	if _, exists := r.personas[persona.ID]; !exists {
		return fmt.Errorf("persona not found: %s", persona.ID)
	}
	persona.UpdatedAt = time.Now()
	r.personas[persona.ID] = persona
	return nil
}

func (r *MemoryRepository) Delete(id string) error {
	if _, exists := r.personas[id]; !exists {
		return fmt.Errorf("persona not found: %s", id)
	}
	delete(r.personas, id)
	return nil
}

func (r *MemoryRepository) Search(query string) ([]*ReviewerPersona, error) {
	var results []*ReviewerPersona
	query = strings.ToLower(query)
	for _, p := range r.personas {
		if matchesQuery(p, query) {
			results = append(results, p)
		}
	}
	return results, nil
}

func matchesQuery(p *ReviewerPersona, query string) bool {
	if strings.Contains(strings.ToLower(p.Name), query) {
		return true
	}
	if strings.Contains(strings.ToLower(p.DisplayName), query) {
		return true
	}
	if strings.Contains(strings.ToLower(p.Description), query) {
		return true
	}
	for _, area := range p.FocusAreas {
		if strings.Contains(strings.ToLower(area), query) {
			return true
		}
	}
	for _, tag := range p.Tags {
		if strings.Contains(strings.ToLower(tag), query) {
			return true
		}
	}
	return false
}

type PersonaService struct {
	repo Repository
}

func NewPersonaService(repo Repository) *PersonaService {
	return &PersonaService{repo: repo}
}

func (s *PersonaService) GetPersona(id string) (*ReviewerPersona, error) {
	return s.repo.GetByID(id)
}

func (s *PersonaService) ListPersonas() ([]*ReviewerPersona, error) {
	return s.repo.List()
}

func (s *PersonaService) ListPersonasByCategory(category string) ([]*ReviewerPersona, error) {
	return s.repo.ListByCategory(category)
}

func (s *PersonaService) CreatePersona(config PersonaConfig) (*ReviewerPersona, error) {
	persona := &ReviewerPersona{
		ID: types.GenerateID(), Name: config.Name, DisplayName: config.DisplayName,
		Description: config.Description, FocusAreas: config.FocusAreas,
		PromptTemplate: config.PromptTemplate, Category: config.Category,
		Tags: config.Tags, IsActive: true,
	}
	if err := s.repo.Create(persona); err != nil {
		return nil, err
	}
	return persona, nil
}

func (s *PersonaService) UpdatePersona(id string, config PersonaConfig) (*ReviewerPersona, error) {
	persona, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	persona.Name = config.Name
	persona.DisplayName = config.DisplayName
	persona.Description = config.Description
	persona.FocusAreas = config.FocusAreas
	persona.PromptTemplate = config.PromptTemplate
	persona.Category = config.Category
	persona.Tags = config.Tags
	if err := s.repo.Update(persona); err != nil {
		return nil, err
	}
	return persona, nil
}

func (s *PersonaService) DeletePersona(id string) error {
	return s.repo.Delete(id)
}

func (s *PersonaService) SearchPersonas(query string) ([]*ReviewerPersona, error) {
	return s.repo.Search(query)
}

func (s *PersonaService) GetPersonaPrompt(id string) (string, error) {
	persona, err := s.repo.GetByID(id)
	if err != nil {
		return "", err
	}
	return persona.PromptTemplate, nil
}
