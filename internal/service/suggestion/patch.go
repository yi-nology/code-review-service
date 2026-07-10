package suggestion

import (
	"fmt"
	"sort"
	"strings"

	"github.com/yi-nology/code-review-service/internal/service/review/domain"
)

// SuggestionPatchItem 描述一条可生成补丁的 finding。
type SuggestionPatchItem struct {
	Index int    `json:"index"`
	Path  string `json:"path"`
	Start int    `json:"start_line"`
	End   int    `json:"end_line"`
	Title string `json:"title,omitempty"`
}

// SuggestionPatchSkipped 描述一条被跳过的 finding 及原因。
type SuggestionPatchSkipped struct {
	Index  int    `json:"index"`
	Path   string `json:"path,omitempty"`
	Title  string `json:"title,omitempty"`
	Reason string `json:"reason"`
}

// SuggestionPatchPreview 是补丁预览结果（Preview/Download 端点返回）。
type SuggestionPatchPreview struct {
	ReviewID       string                   `json:"review_id"`
	Patch          string                   `json:"patch"`
	Items          []SuggestionPatchItem    `json:"items"`
	Skipped        []SuggestionPatchSkipped `json:"skipped"`
	GeneratedCount int                      `json:"generated_count"`
	SkippedCount   int                      `json:"skipped_count"`
}

// SuggestionPatchCheck 是补丁可用性检查结果（Check 端点返回）。
type SuggestionPatchCheck struct {
	SuggestionPatchPreview
	Applicable bool   `json:"applicable"`
	Error      string `json:"error,omitempty"`
	Output     string `json:"output,omitempty"`
}

// SuggestionPatchApplyOptions 是 Apply 端点的入参。
type SuggestionPatchApplyOptions struct {
	Indexes []int
	Branch  string
	Push    bool
}

// SuggestionPatchApplyResult 是 Apply 端点的返回。
type SuggestionPatchApplyResult struct {
	SuggestionPatchPreview
	Applied   bool   `json:"applied"`
	Pushed    bool   `json:"pushed"`
	Branch    string `json:"branch"`
	CommitSHA string `json:"commit_sha,omitempty"`
	Workspace string `json:"workspace,omitempty"`
	CRNumber  int    `json:"cr_number,omitempty"`
	CRURL     string `json:"cr_url,omitempty"`
	Output    string `json:"output,omitempty"`
	Error     string `json:"error,omitempty"`
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

// BuildSuggestionPatch 根据选中的 finding 索引生成 unified-diff 补丁预览。
// indexes 为空时表示全选。会跳过缺字段 / 行号无效 / 与已有 hunk 重叠的 finding。
func BuildSuggestionPatch(reviewID string, findings []domain.Finding, indexes []int) SuggestionPatchPreview {
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
		rawPath := strings.TrimSpace(f.Path)
		// 安全校验：归一化路径并拒绝路径逃逸（../ 、绝对路径），防止补丁头注入。
		path, pathOK := normalizePatchPath(rawPath)
		skip := func(reason string) {
			result.Skipped = append(result.Skipped, SuggestionPatchSkipped{
				Index:  idx,
				Path:   rawPath,
				Title:  f.Title,
				Reason: reason,
			})
		}

		switch {
		case rawPath == "":
			skip("missing path")
			continue
		case !pathOK:
			skip("path rejected: traversal or absolute path not allowed")
			continue
		case isSensitivePath(path):
			skip("path rejected: sensitive file (CI/build/hooks/secrets) not auto-patchable")
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

func findingStartLine(f domain.Finding) int {
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
