package core

import "testing"

func TestChangelogRelease(t *testing.T) {
	for _, tc := range []struct {
		name, data, version, want string
		fail                      bool
	}{
		{"history", "# Log\n\n## 2.0.0\n\nNew\n\n## 1.0.0\nOld\n", "2.0.0", "\nNew\n\n", false},
		{"older", "## 2.0.0\nNew\n## 1.0.0\nOld", "1.0.0", "Old", false},
		{"CRLF", "## 1.0.0\r\n\r\nNote\r\n", "1.0.0", "\r\nNote\r\n", false},
		{"fences", "```md\n## 1.0.0\n```\n## 1.0.0\n~~~md\n## example\n~~~\nBody\n## 0.1.0\nOld", "1.0.0", "~~~md\n## example\n~~~\nBody\n", false},
		{"exact", "## 1.0.0-beta.1\nNote\n", "1.0.0", "", true},
		{"missing", "# Empty\n", "1.0.0", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ChangelogRelease(tc.data, tc.version)
			if (err != nil) != tc.fail || got != tc.want {
				t.Fatalf("got %q, %v; want %q, failure %v", got, err, tc.want, tc.fail)
			}
		})
	}
}
