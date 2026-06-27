package review

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

type SuggestionPatchItem struct {
	Index int    `json:"index"`
	Path  string `json:"path"`
	Start int    `json:"start_line"`
	End   int    `json:"end_line"`
	Title string `json:"title,omitempty"`
}

type SuggestionPatchSkipped struct {
	Index  int    `json:"index"`
	Path   string `json:"path,omitempty"`
	Title  string `json:"title,omitempty"`
	Reason string `json:"reason"`
}

type SuggestionPatchPreview struct {
	ReviewID       string                   `json:"review_id"`
	Patch          string                   `json:"patch"`
	Items          []SuggestionPatchItem    `json:"items"`
	Skipped        []SuggestionPatchSkipped `json:"skipped"`
	GeneratedCount int                      `json:"generated_count"`
	SkippedCount   int                      `json:"skipped_count"`
}

type SuggestionPatchCheck struct {
	SuggestionPatchPreview
	Applicable bool   `json:"applicable"`
	Error      string `json:"error,omitempty"`
	Output     string `json:"output,omitempty"`
}

type patchHunk struct {
	item            SuggestionPatchItem
	oldStart        int
	oldLines        []string
	newLines        []string
	overlapInterval interval
}

type interval struct {
	start int
	end   int
}

func BuildSuggestionPatch(reviewID string, findings []Finding, indexes []int) SuggestionPatchPreview {
	selected := indexes
	if len(selected) == 0 {
		selected = make([]int, len(findings))
		for i := range findings {
			selected[i] = i
		}
	}

	hunksByPath := map[string][]patchHunk{}
	occupied := map[string][]interval{}
	result := SuggestionPatchPreview{ReviewID: reviewID}

	for _, idx := range selected {
		if idx < 0 || idx >= len(findings) {
			result.Skipped = append(result.Skipped, SuggestionPatchSkipped{Index: idx, Reason: "finding index out of range"})
			continue
		}
		f := findings[idx]
		start := findingStartLine(f)
		path := strings.TrimSpace(f.Path)
		skip := func(reason string) {
			result.Skipped = append(result.Skipped, SuggestionPatchSkipped{
				Index:  idx,
				Path:   path,
				Title:  f.Title,
				Reason: reason,
			})
		}

		switch {
		case path == "":
			skip("missing path")
			continue
		case strings.TrimSpace(f.ExistingCode) == "":
			skip("missing existing_code")
			continue
		case strings.TrimSpace(f.SuggestionCode) == "":
			skip("missing suggestion_code")
			continue
		case start <= 0:
			skip("missing valid line")
			continue
		}

		oldLines := splitPatchLines(f.ExistingCode)
		newLines := splitPatchLines(f.SuggestionCode)
		if len(oldLines) == 0 {
			skip("missing existing_code")
			continue
		}
		if len(newLines) == 0 {
			skip("missing suggestion_code")
			continue
		}

		span := interval{start: start, end: start + len(oldLines) - 1}
		if overlapsAny(span, occupied[path]) {
			skip("overlaps another suggestion hunk")
			continue
		}
		occupied[path] = append(occupied[path], span)
		end := f.EndLine
		if end <= 0 {
			end = start + len(oldLines) - 1
		}
		hunk := patchHunk{
			item: SuggestionPatchItem{
				Index: idx,
				Path:  path,
				Start: start,
				End:   end,
				Title: f.Title,
			},
			oldStart:        start,
			oldLines:        oldLines,
			newLines:        newLines,
			overlapInterval: span,
		}
		hunksByPath[path] = append(hunksByPath[path], hunk)
		result.Items = append(result.Items, hunk.item)
	}

	result.Patch = renderSuggestionPatch(hunksByPath)
	result.GeneratedCount = len(result.Items)
	result.SkippedCount = len(result.Skipped)
	return result
}

func findingStartLine(f Finding) int {
	if f.StartLine > 0 {
		return f.StartLine
	}
	return f.Line
}

func splitPatchLines(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.TrimRight(value, "\n")
	return strings.Split(value, "\n")
}

func overlapsAny(candidate interval, intervals []interval) bool {
	for _, existing := range intervals {
		if candidate.start <= existing.end && existing.start <= candidate.end {
			return true
		}
	}
	return false
}

func renderSuggestionPatch(hunksByPath map[string][]patchHunk) string {
	if len(hunksByPath) == 0 {
		return ""
	}
	paths := make([]string, 0, len(hunksByPath))
	for path := range hunksByPath {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	var b strings.Builder
	for _, path := range paths {
		hunks := hunksByPath[path]
		sort.SliceStable(hunks, func(i, j int) bool {
			return hunks[i].oldStart < hunks[j].oldStart
		})
		fmt.Fprintf(&b, "diff --git a/%s b/%s\n", path, path)
		fmt.Fprintf(&b, "--- a/%s\n", path)
		fmt.Fprintf(&b, "+++ b/%s\n", path)
		for _, hunk := range hunks {
			fmt.Fprintf(&b, "@@ -%s +%s @@\n", formatRange(hunk.oldStart, len(hunk.oldLines)), formatRange(hunk.oldStart, len(hunk.newLines)))
			for _, line := range hunk.oldLines {
				fmt.Fprintf(&b, "-%s\n", line)
			}
			for _, line := range hunk.newLines {
				fmt.Fprintf(&b, "+%s\n", line)
			}
		}
	}
	return b.String()
}

func formatRange(start, count int) string {
	if count == 1 {
		return fmt.Sprintf("%d", start)
	}
	return fmt.Sprintf("%d,%d", start, count)
}

func (s *Service) CheckSuggestionPatch(ctx context.Context, reviewID string, indexes []int) (*SuggestionPatchCheck, bool, error) {
	record, found, err := s.Get(ctx, reviewID)
	if err != nil || !found {
		return nil, found, err
	}
	preview := BuildSuggestionPatch(record.ID, record.Response.Findings, indexes)
	result := &SuggestionPatchCheck{SuggestionPatchPreview: preview}
	if strings.TrimSpace(preview.Patch) == "" {
		result.Applicable = false
		result.Error = "no patch generated"
		return result, true, nil
	}
	if s.Providers == nil {
		result.Applicable = false
		result.Error = "providers not configured"
		return result, true, nil
	}
	provider, ok := s.Providers.Get(record.Request.Provider)
	if !ok {
		result.Applicable = false
		result.Error = fmt.Sprintf("unsupported provider %q", record.Request.Provider)
		return result, true, nil
	}
	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	workspace, err := provider.Prepare(checkCtx, record.Request)
	if err != nil {
		result.Applicable = false
		result.Error = err.Error()
		return result, true, nil
	}
	if workspace.Dir != "" {
		defer func() { _ = os.RemoveAll(workspace.Dir) }()
	}
	output, err := runGitApplyCheck(checkCtx, workspace.Dir, preview.Patch)
	result.Output = output
	result.Applicable = err == nil
	if err != nil {
		result.Error = err.Error()
	}
	return result, true, nil
}

func runGitApplyCheck(ctx context.Context, dir, patch string) (string, error) {
	if strings.TrimSpace(dir) == "" {
		return "", fmt.Errorf("workspace dir is empty")
	}
	cmd := exec.CommandContext(ctx, "git", "apply", "--check", "--unidiff-zero", "--whitespace=nowarn", "-")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(patch)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return strings.TrimSpace(out.String()), fmt.Errorf("git apply --check failed")
	}
	return strings.TrimSpace(out.String()), nil
}
