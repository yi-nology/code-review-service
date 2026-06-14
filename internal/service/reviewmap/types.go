package reviewmap

import (
	"fmt"
	"strings"
	"time"
)

// ReviewMap represents a code review map
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

// Section represents a section in the review map
type Section struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Files       []File   `json:"files"`
	Findings    []string `json:"findings"`
	Progress    float64  `json:"progress"`
	Priority    string   `json:"priority"`
}

// File represents a file in a section
type File struct {
	Path        string   `json:"path"`
	Language    string   `json:"language"`
	LinesAdded  int      `json:"lines_added"`
	LinesRemoved int    `json:"lines_removed"`
	Findings    []string `json:"findings"`
	Reviewed    bool     `json:"reviewed"`
}

// Dependency represents a dependency between files
type Dependency struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Type     string `json:"type"` // imports, calls, extends, implements
	Strength string `json:"strength"` // strong, weak
}

// Generator generates code review maps
type Generator struct{}

// NewGenerator creates a new review map generator
func NewGenerator() *Generator {
	return &Generator{}
}

// Generate generates a review map from diff analysis
func (g *Generator) Generate(sessionID string, diff string, findings []Finding) (*ReviewMap, error) {
	// Parse diff to extract files
	files := g.parseDiffFiles(diff)

	// Group files into sections
	sections := g.groupFilesIntoSections(files)

	// Add findings to sections
	g.addFindingsToSections(sections, findings)

	// Generate dependencies
	dependencies := g.generateDependencies(files)

	// Generate summary
	summary := g.generateSummary(sections, findings)

	// Create review map
	reviewMap := &ReviewMap{
		ID:           generateID(),
		SessionID:    sessionID,
		Title:        fmt.Sprintf("Code Review Map - %d files", len(files)),
		Description:  "Automatically generated code review map",
		Sections:     sections,
		Dependencies: dependencies,
		Summary:      summary,
		FileCount:    len(files),
		SectionCount: len(sections),
		CreatedAt:    time.Now(),
	}

	return reviewMap, nil
}

// parseDiffFiles parses diff to extract file information
func (g *Generator) parseDiffFiles(diff string) []File {
	var files []File

	lines := strings.Split(diff, "\n")
	for _, line := range lines {
		// Look for diff headers
		if strings.HasPrefix(line, "diff --git") {
			// Extract file path
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				path := strings.TrimPrefix(parts[3], "b/")
				files = append(files, File{
					Path:     path,
					Language: inferLanguage(path),
				})
			}
		}
	}

	return files
}

// inferLanguage infers the programming language from file path
func inferLanguage(path string) string {
	if strings.HasSuffix(path, ".go") {
		return "go"
	}
	if strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".jsx") {
		return "javascript"
	}
	if strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".tsx") {
		return "typescript"
	}
	if strings.HasSuffix(path, ".py") {
		return "python"
	}
	if strings.HasSuffix(path, ".java") {
		return "java"
	}
	if strings.HasSuffix(path, ".rs") {
		return "rust"
	}
	if strings.HasSuffix(path, ".vue") {
		return "vue"
	}
	if strings.HasSuffix(path, ".html") {
		return "html"
	}
	if strings.HasSuffix(path, ".css") || strings.HasSuffix(path, ".scss") {
		return "css"
	}
	if strings.HasSuffix(path, ".sql") {
		return "sql"
	}
	if strings.HasSuffix(path, ".md") {
		return "markdown"
	}
	if strings.HasSuffix(path, ".json") {
		return "json"
	}
	if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
		return "yaml"
	}
	return "unknown"
}

// groupFilesIntoSections groups files into logical sections
func (g *Generator) groupFilesIntoSections(files []File) []Section {
	sections := make(map[string]*Section)

	for _, file := range files {
		sectionName := g.inferSection(file.Path)
		if _, ok := sections[sectionName]; !ok {
			sections[sectionName] = &Section{
				ID:       generateID(),
				Title:    sectionName,
				Files:    make([]File, 0),
				Findings: make([]string, 0),
			}
		}
		sections[sectionName].Files = append(sections[sectionName].Files, file)
	}

	result := make([]Section, 0, len(sections))
	for _, section := range sections {
		result = append(result, *section)
	}

	return result
}

// inferSection infers the section name from file path
func (g *Generator) inferSection(path string) string {
	// Extract directory structure
	parts := strings.Split(path, "/")
	if len(parts) > 1 {
		// Use first directory as section
		return parts[0]
	}
	return "root"
}

// addFindingsToSections adds findings to appropriate sections
func (g *Generator) addFindingsToSections(sections []Section, findings []Finding) {
	for i, section := range sections {
		for _, finding := range findings {
			// Check if finding belongs to this section
			for _, file := range section.Files {
				if finding.Path == file.Path {
					sections[i].Findings = append(sections[i].Findings, finding.ID)
					break
				}
			}
		}
	}
}

// generateDependencies generates dependencies between files
func (g *Generator) generateDependencies(files []File) []Dependency {
	var dependencies []Dependency

	// Simple dependency detection based on file patterns
	for i, file1 := range files {
		for j, file2 := range files {
			if i == j {
				continue
			}

			// Check for imports
			if g.hasImport(file1.Path, file2.Path) {
				dependencies = append(dependencies, Dependency{
					From:     file1.Path,
					To:       file2.Path,
					Type:     "imports",
					Strength: "strong",
				})
			}

			// Check for same package
			if g.samePackage(file1.Path, file2.Path) {
				dependencies = append(dependencies, Dependency{
					From:     file1.Path,
					To:       file2.Path,
					Type:     "same_package",
					Strength: "weak",
				})
			}
		}
	}

	return dependencies
}

// hasImport checks if file1 imports file2
func (g *Generator) hasImport(file1, file2 string) bool {
	// Simple heuristic: if files are in the same language and directory
	if inferLanguage(file1) == inferLanguage(file2) {
		dir1 := getDirectory(file1)
		dir2 := getDirectory(file2)
		return dir1 == dir2
	}
	return false
}

// samePackage checks if files are in the same package
func (g *Generator) samePackage(file1, file2 string) bool {
	dir1 := getDirectory(file1)
	dir2 := getDirectory(file2)
	return dir1 == dir2
}

// getDirectory gets the directory of a file
func getDirectory(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) > 1 {
		return strings.Join(parts[:len(parts)-1], "/")
	}
	return "."
}

// generateSummary generates a summary of the review map
func (g *Generator) generateSummary(sections []Section, findings []Finding) string {
	totalFiles := 0
	totalFindings := len(findings)

	for _, section := range sections {
		totalFiles += len(section.Files)
	}

	return fmt.Sprintf("Review map with %d sections, %d files, and %d findings",
		len(sections), totalFiles, totalFindings)
}

// ToMermaid generates a Mermaid diagram of the review map
func (m *ReviewMap) ToMermaid() string {
	var sb strings.Builder

	sb.WriteString("graph TD\n")

	// Add nodes for each section
	for _, section := range m.Sections {
		nodeID := sanitizeID(section.Title)
		sb.WriteString(fmt.Sprintf("    %s[%s]\n", nodeID, section.Title))
	}

	// Add edges for dependencies
	for _, dep := range m.Dependencies {
		fromID := sanitizeID(getDirectory(dep.From))
		toID := sanitizeID(getDirectory(dep.To))
		if fromID != toID {
			sb.WriteString(fmt.Sprintf("    %s --> %s\n", fromID, toID))
		}
	}

	return sb.String()
}

// sanitizeID sanitizes a string for use as a Mermaid node ID
func sanitizeID(s string) string {
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, ".", "_")
	return s
}

// generateID generates a unique ID
func generateID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(6)
}

// randomString generates a random string of specified length
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(result)
}

// Finding represents a finding for review map generation
type Finding struct {
	ID         string   `json:"id"`
	Severity   string   `json:"severity"`
	Category   string   `json:"category"`
	Path       string   `json:"path"`
	Line       int      `json:"line"`
	EndLine    int      `json:"end_line"`
	Title      string   `json:"title"`
	Body       string   `json:"body"`
	Suggestion string   `json:"suggestion"`
	Confidence float64  `json:"confidence"`
	Agents     []string `json:"agents"`
}
