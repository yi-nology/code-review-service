package writeback

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
)

type DiffIndex struct {
	files map[string]map[Side]map[int]struct{}
}

func ParseUnifiedDiff(diff string) (*DiffIndex, error) {
	index := &DiffIndex{files: make(map[string]map[Side]map[int]struct{})}
	scanner := bufio.NewScanner(strings.NewReader(diff))
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)

	var path string
	var oldLine, newLine int
	inHunk := false

	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "+++ "):
			path = cleanDiffPath(strings.TrimPrefix(line, "+++ "))
			inHunk = false
		case strings.HasPrefix(line, "@@ "):
			var err error
			oldLine, newLine, err = parseHunkHeader(line)
			if err != nil {
				return nil, err
			}
			inHunk = path != "" && path != "/dev/null"
		case !inHunk:
			continue
		case strings.HasPrefix(line, "\\ No newline at end of file"):
			continue
		case strings.HasPrefix(line, "+"):
			index.add(path, SideNew, newLine)
			newLine++
		case strings.HasPrefix(line, "-"):
			index.add(path, SideOld, oldLine)
			oldLine++
		default:
			index.add(path, SideOld, oldLine)
			index.add(path, SideNew, newLine)
			oldLine++
			newLine++
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan unified diff: %w", err)
	}
	return index, nil
}

func (d *DiffIndex) Contains(path string, side Side, line int) bool {
	if d == nil || line <= 0 {
		return false
	}
	path = cleanDiffPath(path)
	sides, ok := d.files[path]
	if !ok {
		return false
	}
	lines, ok := sides[side]
	if !ok {
		return false
	}
	_, ok = lines[line]
	return ok
}

func (d *DiffIndex) ContainsRange(path string, lineRange LineRange) bool {
	if lineRange.Start <= 0 || lineRange.End < lineRange.Start {
		return false
	}
	for line := lineRange.Start; line <= lineRange.End; line++ {
		if !d.Contains(path, lineRange.Side, line) {
			return false
		}
	}
	return true
}

func (d *DiffIndex) add(path string, side Side, line int) {
	if line <= 0 {
		return
	}
	if d.files[path] == nil {
		d.files[path] = make(map[Side]map[int]struct{})
	}
	if d.files[path][side] == nil {
		d.files[path][side] = make(map[int]struct{})
	}
	d.files[path][side][line] = struct{}{}
}

func parseHunkHeader(header string) (int, int, error) {
	parts := strings.Fields(header)
	if len(parts) < 3 {
		return 0, 0, fmt.Errorf("invalid unified diff hunk header %q", header)
	}
	oldLine, err := parseRangeStart(parts[1], '-')
	if err != nil {
		return 0, 0, fmt.Errorf("invalid old range in hunk header %q: %w", header, err)
	}
	newLine, err := parseRangeStart(parts[2], '+')
	if err != nil {
		return 0, 0, fmt.Errorf("invalid new range in hunk header %q: %w", header, err)
	}
	return oldLine, newLine, nil
}

func parseRangeStart(value string, prefix byte) (int, error) {
	if len(value) < 2 || value[0] != prefix {
		return 0, fmt.Errorf("expected %q prefix", prefix)
	}
	value = strings.TrimPrefix(value[1:], ",")
	if comma := strings.IndexByte(value, ','); comma >= 0 {
		value = value[:comma]
	}
	return strconv.Atoi(value)
}

func cleanDiffPath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "a/")
	path = strings.TrimPrefix(path, "b/")
	return path
}
