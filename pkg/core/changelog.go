package core

import (
	"fmt"
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

func changelogHeadings(data string) []changelogHeading {
	var headings []changelogHeading
	offset, fence := 0, ""
	for _, line := range strings.SplitAfter(data, "\n") {
		trimmed := strings.TrimSpace(line)
		if fence != "" {
			if strings.HasPrefix(trimmed, fence) && strings.Trim(trimmed, fence[:1]) == "" {
				fence = ""
			}
		} else if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			char := trimmed[:1]
			fence = trimmed[:len(trimmed)-len(strings.TrimLeft(trimmed, char))]
		} else if strings.HasPrefix(line, "## ") {
			headings = append(headings, changelogHeading{offset, offset + len(line), strings.TrimRight(line[3:], "\r\n")})
		}
		offset += len(line)
	}
	return headings
}
