package store

import (
	"os"
	"strings"
)

// DecodePath converts a hyphen-encoded folder name back to a filesystem path.
// Only used as a fallback when projectPath is empty in sessions-index.json.
//
// Encoding rule: each '/' in the original path was replaced with '-'.
// Example: "/mnt/d/mine/proj" → "-mnt-d-mine-proj"
//
// Ambiguity: "-mnt-d-2025-Airflow3" could be "/mnt/d/2025-Airflow3" or
// "/mnt/d/2025/Airflow3". Resolution: validate candidates against the real
// filesystem and return the deepest valid path found.
func DecodePath(folderName string) string {
	if folderName == "" {
		return ""
	}

	// Strip leading '-' which represents the root '/'
	s := folderName
	if strings.HasPrefix(s, "-") {
		s = s[1:]
	}

	parts := strings.Split(s, "-")
	if len(parts) == 0 {
		return "/"
	}

	if best := findValidPath(parts); best != "" {
		return best
	}

	// Fallback: treat every '-' as a path separator
	return "/" + strings.Join(parts, "/")
}

// findValidPath tries all combinations of joining parts as path segments or
// as hyphenated directory names. Returns the deepest path that exists on the
// real filesystem.
func findValidPath(parts []string) string {
	best := ""

	var recurse func(idx int, current string)
	recurse = func(idx int, current string) {
		if idx == len(parts) {
			if _, err := os.Stat(current); err == nil && len(current) > len(best) {
				best = current
			}
			return
		}

		// Option A: treat '-' before this part as a path separator
		recurse(idx+1, current+"/"+parts[idx])

		// Option B: treat '-' before this part as a literal hyphen in the dir name
		recurse(idx+1, current+"-"+parts[idx])
	}

	recurse(1, "/"+parts[0])
	return best
}
