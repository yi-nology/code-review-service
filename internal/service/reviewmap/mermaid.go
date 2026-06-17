package reviewmap

import (
	"fmt"
	"strings"
)

func (m *ReviewMap) ToMermaid() string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

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

	if len(allDirs) <= 1 {
		for filePath := range allFiles {
			nodeID := sanitizeID(filePath)
			parts := strings.Split(filePath, "/")
			sb.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", nodeID, parts[len(parts)-1]))
		}
		seen := make(map[string]bool)
		for _, dep := range m.Dependencies {
			fromID, toID := sanitizeID(dep.From), sanitizeID(dep.To)
			edgeKey := fromID + "->" + toID
			if fromID != toID && !seen[edgeKey] {
				sb.WriteString(fmt.Sprintf("    %s --> %s\n", fromID, toID))
				seen[edgeKey] = true
			}
		}
	} else {
		for dir := range allDirs {
			nodeID := sanitizeID(dir)
			displayName := dir
			parts := strings.Split(dir, "/")
			if len(parts) > 3 {
				displayName = ".../" + strings.Join(parts[len(parts)-2:], "/")
			}
			sb.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", nodeID, displayName))
		}
		seen := make(map[string]bool)
		for _, dep := range m.Dependencies {
			fromDir, toDir := getDirectory(dep.From), getDirectory(dep.To)
			if fromDir == toDir {
				continue
			}
			fromID, toID := sanitizeID(fromDir), sanitizeID(toDir)
			edgeKey := fromID + "->" + toID
			if !seen[edgeKey] {
				sb.WriteString(fmt.Sprintf("    %s --> %s\n", fromID, toID))
				seen[edgeKey] = true
			}
		}
	}

	if !strings.Contains(sb.String(), "-->") {
		sb.WriteString("    note[\"No dependencies detected\"]\n")
	}
	return sb.String()
}

func sanitizeID(s string) string {
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, ".", "_")
	s = strings.ReplaceAll(s, "-", "_")
	return s
}
