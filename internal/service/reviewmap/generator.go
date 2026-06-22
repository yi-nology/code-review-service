package reviewmap

import (
	"fmt"
	"strings"
	"time"

	"github.com/yi-nology/code-review-service/internal/service/types"
)

type Generator struct{}

func NewGenerator() *Generator {
	return &Generator{}
}

func (g *Generator) Generate(sessionID string, diff string, findings []Finding) (*ReviewMap, error) {
	files := g.parseDiffFiles(diff)
	// 兜底：diff 解析出 0 文件时，从 findings 的文件路径提取
	if len(files) == 0 && len(findings) > 0 {
		seen := make(map[string]bool)
		for _, f := range findings {
			if f.Path != "" && !seen[f.Path] {
				seen[f.Path] = true
				files = append(files, File{Path: f.Path, Language: inferLanguage(f.Path)})
			}
		}
	}
	sections := g.groupFilesIntoSections(files)
	g.addFindingsToSections(sections, findings)
	dependencies := g.generateDependencies(files)
	summary := g.generateSummary(sections, findings)

	return &ReviewMap{
		ID:           types.GenerateID(),
		SessionID:    sessionID,
		Title:        fmt.Sprintf("Code Review Map - %d files", len(files)),
		Description:  "Automatically generated code review map",
		Sections:     sections,
		Dependencies: dependencies,
		Summary:      summary,
		FileCount:    len(files),
		SectionCount: len(sections),
		CreatedAt:    time.Now(),
	}, nil
}

func (g *Generator) parseDiffFiles(diff string) []File {
	var files []File
	seen := make(map[string]bool)
	for _, line := range strings.Split(diff, "\n") {
		var path string
		switch {
		case strings.HasPrefix(line, "diff --git"):
			// diff --git a/foo.go b/foo.go
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				path = strings.TrimPrefix(parts[len(parts)-1], "b/")
			}
		case strings.HasPrefix(line, "+++ ") && !strings.HasPrefix(line, "+++ /dev/null"):
			// +++ b/foo.go
			path = strings.TrimSpace(strings.TrimPrefix(line, "+++"))
			path = strings.TrimSpace(strings.TrimPrefix(path, "b/"))
		default:
			continue
		}
		if path == "" || path == "/dev/null" || seen[path] {
			continue
		}
		seen[path] = true
		files = append(files, File{Path: path, Language: inferLanguage(path)})
	}
	return files
}

func (g *Generator) groupFilesIntoSections(files []File) []Section {
	sections := make(map[string]*Section)
	for _, file := range files {
		name := g.inferSection(file.Path)
		if _, ok := sections[name]; !ok {
			sections[name] = &Section{
				ID: types.GenerateID(), Title: name,
				Files: make([]File, 0), Findings: make([]string, 0),
			}
		}
		sections[name].Files = append(sections[name].Files, file)
	}
	result := make([]Section, 0, len(sections))
	for _, s := range sections {
		result = append(result, *s)
	}
	return result
}

func (g *Generator) inferSection(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) > 1 {
		return parts[0]
	}
	return "root"
}

func (g *Generator) addFindingsToSections(sections []Section, findings []Finding) {
	for i, section := range sections {
		for _, finding := range findings {
			for _, file := range section.Files {
				if finding.Path == file.Path {
					sections[i].Findings = append(sections[i].Findings, finding.ID)
					break
				}
			}
		}
	}
}

func (g *Generator) generateDependencies(files []File) []Dependency {
	var deps []Dependency
	seen := make(map[string]bool)
	for i, f1 := range files {
		for j, f2 := range files {
			if i == j {
				continue
			}
			edgeKey := f1.Path + "->" + f2.Path
			reverseKey := f2.Path + "->" + f1.Path
			if seen[edgeKey] || seen[reverseKey] {
				continue
			}
			d1, d2 := getDirectory(f1.Path), getDirectory(f2.Path)
			if d1 == d2 && inferLanguage(f1.Path) == inferLanguage(f2.Path) {
				deps = append(deps, Dependency{From: f1.Path, To: f2.Path, Type: "same_module", Strength: "weak"})
				seen[edgeKey] = true
			} else if d1 != d2 && inferLanguage(f1.Path) == inferLanguage(f2.Path) {
				deps = append(deps, Dependency{From: f1.Path, To: f2.Path, Type: "imports", Strength: "strong"})
				seen[edgeKey] = true
			}
		}
	}
	return deps
}

func (g *Generator) generateSummary(sections []Section, findings []Finding) string {
	totalFiles := 0
	for _, s := range sections {
		totalFiles += len(s.Files)
	}
	return fmt.Sprintf("Review map with %d sections, %d files, and %d findings", len(sections), totalFiles, len(findings))
}

func inferLanguage(path string) string {
	switch {
	case strings.HasSuffix(path, ".go"):
		return "go"
	case strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".jsx"):
		return "javascript"
	case strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".tsx"):
		return "typescript"
	case strings.HasSuffix(path, ".py"):
		return "python"
	case strings.HasSuffix(path, ".java"):
		return "java"
	case strings.HasSuffix(path, ".rs"):
		return "rust"
	case strings.HasSuffix(path, ".vue"):
		return "vue"
	case strings.HasSuffix(path, ".html"):
		return "html"
	case strings.HasSuffix(path, ".css") || strings.HasSuffix(path, ".scss"):
		return "css"
	case strings.HasSuffix(path, ".sql"):
		return "sql"
	case strings.HasSuffix(path, ".md"):
		return "markdown"
	case strings.HasSuffix(path, ".json"):
		return "json"
	case strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml"):
		return "yaml"
	default:
		return "unknown"
	}
}

func getDirectory(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) > 1 {
		return strings.Join(parts[:len(parts)-1], "/")
	}
	return "."
}
