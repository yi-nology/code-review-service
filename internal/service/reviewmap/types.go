package reviewmap

import (
	"fmt"
	"strings"
	"time"

	"github.com/yi-nology/code-review-service/internal/service/types"
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
	seen := make(map[string]bool)

	for i, file1 := range files {
		for j, file2 := range files {
			if i == j {
				continue
			}

			// Create edge key based on file paths
			edgeKey := file1.Path + "->" + file2.Path
			reverseKey := file2.Path + "->" + file1.Path
			
			// Skip if already seen (in either direction)
			if seen[edgeKey] || seen[reverseKey] {
				continue
			}

			dir1 := getDirectory(file1.Path)
			dir2 := getDirectory(file2.Path)

			// Same directory: show file-level dependency
			if dir1 == dir2 {
				if inferLanguage(file1.Path) == inferLanguage(file2.Path) {
					dependencies = append(dependencies, Dependency{
						From:     file1.Path,
						To:       file2.Path,
						Type:     "same_module",
						Strength: "weak",
					})
					seen[edgeKey] = true
				}
			} else {
				// Different directories: show directory-level dependency
				if inferLanguage(file1.Path) == inferLanguage(file2.Path) {
					dependencies = append(dependencies, Dependency{
						From:     file1.Path,
						To:       file2.Path,
						Type:     "imports",
						Strength: "strong",
					})
					seen[edgeKey] = true
				}
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

	// Collect all unique directories and files
	allDirs := make(map[string]bool)
	allFiles := make(map[string]bool)
	
	for _, section := range m.Sections {
		for _, file := range section.Files {
			dir := getDirectory(file.Path)
			if dir != "" && dir != "." {
				allDirs[dir] = true
			}
			allFiles[file.Path] = true
		}
	}

	// Determine if we should show file-level or directory-level
	// If all files are in same directory, show file-level
	// Otherwise show directory-level
	dirCount := len(allDirs)
	
	if dirCount <= 1 {
		// File-level diagram
		for filePath := range allFiles {
			nodeID := sanitizeID(filePath)
			parts := strings.Split(filePath, "/")
			displayName := parts[len(parts)-1]
			sb.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", nodeID, displayName))
		}

		// Add file-level edges
		seen := make(map[string]bool)
		for _, dep := range m.Dependencies {
			fromID := sanitizeID(dep.From)
			toID := sanitizeID(dep.To)
			edgeKey := fromID + "->" + toID
			if fromID != toID && !seen[edgeKey] {
				sb.WriteString(fmt.Sprintf("    %s --> %s\n", fromID, toID))
				seen[edgeKey] = true
			}
		}
	} else {
		// Directory-level diagram
		for dir := range allDirs {
			nodeID := sanitizeID(dir)
			displayName := dir
			parts := strings.Split(dir, "/")
			if len(parts) > 3 {
				displayName = ".../" + strings.Join(parts[len(parts)-2:], "/")
			}
			sb.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", nodeID, displayName))
		}

		// Add directory-level edges (aggregate)
		seen := make(map[string]bool)
		for _, dep := range m.Dependencies {
			fromDir := getDirectory(dep.From)
			toDir := getDirectory(dep.To)
			if fromDir == toDir {
				continue
			}
			fromID := sanitizeID(fromDir)
			toID := sanitizeID(toDir)
			edgeKey := fromID + "->" + toID
			if !seen[edgeKey] {
				sb.WriteString(fmt.Sprintf("    %s --> %s\n", fromID, toID))
				seen[edgeKey] = true
			}
		}
	}

	// If no edges, add a note
	hasEdges := strings.Contains(sb.String(), "-->")
	if !hasEdges {
		sb.WriteString("    note[\"No dependencies detected\"]\n")
	}

	return sb.String()
}

// sanitizeID sanitizes a string for use as a Mermaid node ID
func sanitizeID(s string) string {
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, ".", "_")
	s = strings.ReplaceAll(s, "-", "_")
	return s
}

// Finding is an alias for the canonical types.Finding
type Finding = types.Finding

// generateID generates a unique ID
func generateID() string {
	return types.GenerateID()
}
