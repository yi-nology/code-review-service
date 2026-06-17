package persona

import (
	"fmt"
	"strings"
	"time"

	"github.com/yi-nology/code-review-service/internal/service/types"
)

// ReviewerPersona represents a reviewer persona
type ReviewerPersona struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	DisplayName  string    `json:"display_name"`
	Description  string    `json:"description"`
	FocusAreas   []string  `json:"focus_areas"`
	PromptTemplate string  `json:"prompt_template"`
	Category     string    `json:"category"`
	Tags         []string  `json:"tags"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// PersonaCategory represents a persona category
type PersonaCategory string

const (
	CategoryGeneralist  PersonaCategory = "generalist"
	CategorySpecialist  PersonaCategory = "specialist"
	CategoryFamous      PersonaCategory = "famous"
	CategoryCustom      PersonaCategory = "custom"
)

// PersonaConfig contains persona configuration
type PersonaConfig struct {
	Name         string   `yaml:"name"`
	DisplayName  string   `yaml:"display_name"`
	Description  string   `yaml:"description"`
	FocusAreas   []string `yaml:"focus_areas"`
	PromptTemplate string `yaml:"prompt_template"`
	Category     string   `yaml:"category"`
	Tags         []string `yaml:"tags"`
}

// DefaultPersonas returns default reviewer personas
func DefaultPersonas() []ReviewerPersona {
	return []ReviewerPersona{
		{
			ID:          "principal",
			Name:        "principal",
			DisplayName: "Principal Engineer",
			Description: "Reviews code for architecture, design patterns, and maintainability",
			FocusAreas:  []string{"architecture", "design_patterns", "maintainability", "scalability"},
			PromptTemplate: `You are a Principal Engineer reviewing this code. Focus on:
- Architecture and design patterns
- Maintainability and scalability
- Code organization and structure
- Long-term technical debt
- System design decisions`,
			Category:    string(CategoryGeneralist),
			Tags:        []string{"architecture", "design"},
			IsActive:    true,
		},
		{
			ID:          "quality",
			Name:        "quality",
			DisplayName: "Quality Engineer",
			Description: "Reviews code for code style, readability, and best practices",
			FocusAreas:  []string{"code_style", "readability", "best_practices", "naming"},
			PromptTemplate: `You are a Quality Engineer reviewing this code. Focus on:
- Code style and formatting
- Readability and clarity
- Naming conventions
- Best practices
- Code duplication`,
			Category:    string(CategoryGeneralist),
			Tags:        []string{"quality", "style"},
			IsActive:    true,
		},
		{
			ID:          "security",
			Name:        "security",
			DisplayName: "Security Engineer",
			Description: "Reviews code for security vulnerabilities and best practices",
			FocusAreas:  []string{"security", "vulnerabilities", "authentication", "authorization"},
			PromptTemplate: `You are a Security Engineer reviewing this code. Focus on:
- Security vulnerabilities
- Authentication and authorization
- Input validation
- Data protection
- Secure coding practices`,
			Category:    string(CategorySpecialist),
			Tags:        []string{"security", "auth"},
			IsActive:    true,
		},
		{
			ID:          "testing",
			Name:        "testing",
			DisplayName: "Testing Engineer",
			Description: "Reviews code for test coverage and quality",
			FocusAreas:  []string{"testing", "coverage", "test_quality", "edge_cases"},
			PromptTemplate: `You are a Testing Engineer reviewing this code. Focus on:
- Test coverage
- Test quality
- Edge cases
- Testability
- Mocking and isolation`,
			Category:    string(CategorySpecialist),
			Tags:        []string{"testing", "quality"},
			IsActive:    true,
		},
		{
			ID:          "performance",
			Name:        "performance",
			DisplayName: "Performance Engineer",
			Description: "Reviews code for performance and optimization",
			FocusAreas:  []string{"performance", "optimization", "scalability", "efficiency"},
			PromptTemplate: `You are a Performance Engineer reviewing this code. Focus on:
- Performance bottlenecks
- Optimization opportunities
- Resource usage
- Scalability concerns
- Algorithm efficiency`,
			Category:    string(CategorySpecialist),
			Tags:        []string{"performance", "optimization"},
			IsActive:    true,
		},
		{
			ID:          "martin-fowler",
			Name:        "martin-fowler",
			DisplayName: "Martin Fowler",
			Description: "Reviews code through the lens of Martin Fowler's design principles",
			FocusAreas:  []string{"refactoring", "design_patterns", "enterprise_patterns", "clean_code"},
			PromptTemplate: `You are Martin Fowler reviewing this code. Apply my principles:
- Refactoring: Improve code structure without changing behavior
- Design Patterns: Use appropriate patterns for the problem
- Enterprise Patterns: Apply enterprise integration patterns
- Clean Code: Write code that is easy to understand and maintain
- Continuous Integration: Ensure code is always deployable`,
			Category:    string(CategoryFamous),
			Tags:        []string{"design", "refactoring"},
			IsActive:    true,
		},
		{
			ID:          "kent-beck",
			Name:        "kent-beck",
			DisplayName: "Kent Beck",
			Description: "Reviews code through the lens of Kent Beck's Extreme Programming",
			FocusAreas:  []string{"tdd", "simplicity", "refactoring", "pair_programming"},
			PromptTemplate: `You are Kent Beck reviewing this code. Apply my principles:
- Test-Driven Development: Write tests first
- Simplicity: Do the simplest thing that could possibly work
- Refactoring: Improve code structure continuously
- Communication: Code should communicate intent
- Feedback: Use short feedback loops`,
			Category:    string(CategoryFamous),
			Tags:        []string{"tdd", "simplicity"},
			IsActive:    true,
		},
		{
			ID:          "sandi-metz",
			Name:        "sandi-metz",
			DisplayName: "Sandi Metz",
			Description: "Reviews code through the lens of Sandi Metz's practical object-oriented design",
			FocusAreas:  []string{"object_oriented", "solid", "duck_typing", "testing"},
			PromptTemplate: `You are Sandi Metz reviewing this code. Apply my principles:
- Practical Object-Oriented Design: Design for change
- SOLID Principles: Apply SOLID principles appropriately
- Duck Typing: Use duck typing for flexibility
- Testing: Test behavior, not implementation
- Rules: Follow the rules, then break them when needed`,
			Category:    string(CategoryFamous),
			Tags:        []string{"object_oriented", "solid"},
			IsActive:    true,
		},
	}
}

// Repository defines the interface for persona repository operations
type Repository interface {
	// Create creates a new persona
	Create(persona *ReviewerPersona) error

	// GetByID returns a persona by ID
	GetByID(id string) (*ReviewerPersona, error)

	// GetByName returns a persona by name
	GetByName(name string) (*ReviewerPersona, error)

	// List returns all personas
	List() ([]*ReviewerPersona, error)

	// ListByCategory returns personas by category
	ListByCategory(category string) ([]*ReviewerPersona, error)

	// Update updates a persona
	Update(persona *ReviewerPersona) error

	// Delete deletes a persona
	Delete(id string) error

	// Search searches personas by query
	Search(query string) ([]*ReviewerPersona, error)
}

// MemoryRepository implements Repository with in-memory storage
type MemoryRepository struct {
	personas map[string]*ReviewerPersona
}

// NewMemoryRepository creates a new in-memory persona repository
func NewMemoryRepository() *MemoryRepository {
	repo := &MemoryRepository{
		personas: make(map[string]*ReviewerPersona),
	}

	// Load default personas
	for _, persona := range DefaultPersonas() {
		repo.personas[persona.ID] = &persona
	}

	return repo
}

// Create creates a new persona
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

// GetByID returns a persona by ID
func (r *MemoryRepository) GetByID(id string) (*ReviewerPersona, error) {
	persona, exists := r.personas[id]
	if !exists {
		return nil, fmt.Errorf("persona not found: %s", id)
	}
	return persona, nil
}

// GetByName returns a persona by name
func (r *MemoryRepository) GetByName(name string) (*ReviewerPersona, error) {
	for _, persona := range r.personas {
		if persona.Name == name {
			return persona, nil
		}
	}
	return nil, fmt.Errorf("persona not found: %s", name)
}

// List returns all personas
func (r *MemoryRepository) List() ([]*ReviewerPersona, error) {
	personas := make([]*ReviewerPersona, 0, len(r.personas))
	for _, persona := range r.personas {
		personas = append(personas, persona)
	}
	return personas, nil
}

// ListByCategory returns personas by category
func (r *MemoryRepository) ListByCategory(category string) ([]*ReviewerPersona, error) {
	var personas []*ReviewerPersona
	for _, persona := range r.personas {
		if persona.Category == category {
			personas = append(personas, persona)
		}
	}
	return personas, nil
}

// Update updates a persona
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

// Delete deletes a persona
func (r *MemoryRepository) Delete(id string) error {
	if _, exists := r.personas[id]; !exists {
		return fmt.Errorf("persona not found: %s", id)
	}

	delete(r.personas, id)
	return nil
}

// Search searches personas by query
func (r *MemoryRepository) Search(query string) ([]*ReviewerPersona, error) {
	var results []*ReviewerPersona
	query = strings.ToLower(query)

	for _, persona := range r.personas {
		if matchesQuery(persona, query) {
			results = append(results, persona)
		}
	}

	return results, nil
}

// matchesQuery checks if a persona matches a search query
func matchesQuery(persona *ReviewerPersona, query string) bool {
	// Check name
	if strings.Contains(strings.ToLower(persona.Name), query) {
		return true
	}

	// Check display name
	if strings.Contains(strings.ToLower(persona.DisplayName), query) {
		return true
	}

	// Check description
	if strings.Contains(strings.ToLower(persona.Description), query) {
		return true
	}

	// Check focus areas
	for _, area := range persona.FocusAreas {
		if strings.Contains(strings.ToLower(area), query) {
			return true
		}
	}

	// Check tags
	for _, tag := range persona.Tags {
		if strings.Contains(strings.ToLower(tag), query) {
			return true
		}
	}

	return false
}

// PersonaService manages reviewer personas
type PersonaService struct {
	repo Repository
}

// NewPersonaService creates a new persona service
func NewPersonaService(repo Repository) *PersonaService {
	return &PersonaService{repo: repo}
}

// GetPersona returns a persona by ID
func (s *PersonaService) GetPersona(id string) (*ReviewerPersona, error) {
	return s.repo.GetByID(id)
}

// ListPersonas returns all personas
func (s *PersonaService) ListPersonas() ([]*ReviewerPersona, error) {
	return s.repo.List()
}

// ListPersonasByCategory returns personas by category
func (s *PersonaService) ListPersonasByCategory(category string) ([]*ReviewerPersona, error) {
	return s.repo.ListByCategory(category)
}

// CreatePersona creates a new persona
func (s *PersonaService) CreatePersona(config PersonaConfig) (*ReviewerPersona, error) {
	persona := &ReviewerPersona{
		ID:             generateID(),
		Name:           config.Name,
		DisplayName:    config.DisplayName,
		Description:    config.Description,
		FocusAreas:     config.FocusAreas,
		PromptTemplate: config.PromptTemplate,
		Category:       config.Category,
		Tags:           config.Tags,
		IsActive:       true,
	}

	if err := s.repo.Create(persona); err != nil {
		return nil, err
	}

	return persona, nil
}

// UpdatePersona updates a persona
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

// DeletePersona deletes a persona
func (s *PersonaService) DeletePersona(id string) error {
	return s.repo.Delete(id)
}

// SearchPersonas searches personas by query
func (s *PersonaService) SearchPersonas(query string) ([]*ReviewerPersona, error) {
	return s.repo.Search(query)
}

// GetPersonaPrompt returns the prompt for a persona
func (s *PersonaService) GetPersonaPrompt(id string) (string, error) {
	persona, err := s.repo.GetByID(id)
	if err != nil {
		return "", err
	}

	return persona.PromptTemplate, nil
}

// generateID generates a unique ID
func generateID() string {
	return types.GenerateID()
}
