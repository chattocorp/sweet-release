package core

import (
	"fmt"
	"regexp"
	"strings"
)

// ChangelogRelease reads one version's body, excluding its heading and history.
// Headings inside fenced code blocks are part of the body.
func ChangelogRelease(data, version string) (string, error) {
	start := -1
	for _, heading := range changelogHeadings(data) {
		if start >= 0 {
			return data[start:heading.start], nil
		}
		if heading.title == version {
			start = heading.end
		}
	}
	if start >= 0 {
		return data[start:], nil
	}
	return "", fmt.Errorf("changelog has no release %q", version)
}

type changelogHeading struct {
	start, end int
	title      string
}

var changelogListMarker = regexp.MustCompile(`^(?:[-+*]|[0-9]{1,9}[.)])[ \t]+`)

func changelogHeadings(data string) []changelogHeading {
	var headings []changelogHeading
	offset, fence := 0, ""
	for _, line := range strings.SplitAfter(data, "\n") {
		trimmed := strings.TrimSpace(line)
		if fence != "" {
			if strings.HasPrefix(trimmed, fence) && strings.Trim(trimmed, fence[:1]) == "" {
				fence = ""
			}
		} else {
			// Bump wraps each note in a bullet. A fence may start on that
			// bullet's first line, including within a nested list in the note.
			for {
				marker := changelogListMarker.FindStringIndex(trimmed)
				if marker == nil {
					break
				}
				trimmed = trimmed[marker[1]:]
			}
			if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
				char := trimmed[:1]
				fence = trimmed[:len(trimmed)-len(strings.TrimLeft(trimmed, char))]
			} else if strings.HasPrefix(line, "## ") {
				headings = append(headings, changelogHeading{offset, offset + len(line), strings.TrimRight(line[3:], "\r\n")})
			}
		}
		offset += len(line)
	}
	return headings
}
