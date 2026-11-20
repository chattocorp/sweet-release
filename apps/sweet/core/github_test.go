package core

import (
	"errors"
	"strings"
	"testing"
)

// fakeGitHub keeps one release per tag and records calls.
type fakeGitHub struct {
	releases  map[string]*fakeRelease
	remote    []string
	calls     []string
	failCalls map[string]bool
}

type fakeRelease struct {
	title, notes      string
	draft, prerelease bool
	latest            bool
	assets            map[string]int
}

func (f *fakeGitHub) record(call string) error {
	f.calls = append(f.calls, call)
	if f.failCalls[call] {
		delete(f.failCalls, call)
		return errors.New(call + " failed")
	}
	return nil
}

func (f *fakeGitHub) state(tag string) (string, error) {
	release, ok := f.releases[tag]
	switch {
	case !ok:
		return "missing", nil
	case release.draft:
		return "draft", nil
	}
	return "published", nil
}

func (f *fakeGitHub) create(tag, title, notes string, prerelease bool) error {
	if err := f.record("create"); err != nil {
		return err
	}
	f.releases[tag] = &fakeRelease{title: title, notes: notes, draft: true, prerelease: prerelease, assets: map[string]int{}}
	return nil
}

func (f *fakeGitHub) upload(tag string, assets []string) error {
	if err := f.record("upload"); err != nil {
		return err
	}
	for _, asset := range assets {
		f.releases[tag].assets[asset]++
	}
	return nil
}

func (f *fakeGitHub) publish(tag string, prerelease, latest bool) error {
	if err := f.record("publish"); err != nil {
		return err
	}
	release := f.releases[tag]
	release.draft, release.prerelease, release.latest = false, prerelease, latest
	return nil
}

func (f *fakeGitHub) tags(prefix string) ([]string, error) {
	f.calls = append(f.calls, "tags "+prefix)
	return f.remote, nil
}

func githubFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	root, _, write, _ := tagFixture(t)
	for file, text := range files {
		write(file, text)
	}
	return root
}

func TestGitHubReleaseCreateUploadPublish(t *testing.T) {
	root := githubFixture(t, map[string]string{
		".sweet.yml":         "name: App\nversion: 1.2.0\n",
		"CHANGELOG.md":       "# App Changelog\n\n## 1.2.0\n\n- New\n\n## 1.1.0\n\n- Old\n",
		"pkg/lib/.sweet.yml": "version: 0.1.0\n",
	})
	gh := &fakeGitHub{releases: map[string]*fakeRelease{}, remote: []string{"v1.1.0", "v1.2.0", "v1.3.0-beta.0", "vscode@9.0.0"}}
	result, err := publishGitHubRelease(root, gh, GitHubReleaseOptions{Tag: "v1.2.0", Assets: []string{"a.tgz"}})
	if err != nil {
		t.Fatal(err)
	}
	release := gh.releases["v1.2.0"]
	if release.title != "App 1.2.0" || release.notes != "\n- New\n\n" || release.draft || release.prerelease || !release.latest || release.assets["a.tgz"] != 1 {
		t.Fatalf("release: %+v", release)
	}
	if result.Package != "." || result.Version != "1.2.0" || result.State != "missing" || !result.Published || !result.Latest {
		t.Fatalf("result: %+v", result)
	}
	if got := strings.Join(gh.calls, ","); got != "create,upload,tags v,publish" {
		t.Fatalf("calls: %s", got)
	}
}

func TestGitHubReleaseRetries(t *testing.T) {
	root := githubFixture(t, map[string]string{
		".sweet.yml":   "version: 1.0.0\n",
		"CHANGELOG.md": "## 1.0.0\n\nNotes\n",
	})
	gh := &fakeGitHub{releases: map[string]*fakeRelease{}, remote: []string{"v1.0.0"}, failCalls: map[string]bool{"upload": true}}
	opts := GitHubReleaseOptions{Tag: "v1.0.0", Assets: []string{"a", "b"}}
	if _, err := publishGitHubRelease(root, gh, opts); err == nil || !gh.releases["v1.0.0"].draft {
		t.Fatalf("failed upload must leave a draft: %v", err)
	}
	// A retry reuses the draft instead of creating a second release.
	result, err := publishGitHubRelease(root, gh, opts)
	if err != nil || result.State != "draft" || gh.releases["v1.0.0"].draft || gh.releases["v1.0.0"].title != "v1.0.0" {
		t.Fatalf("retry: %+v, %v, %+v", result, err, gh.releases["v1.0.0"])
	}
	// A retry after publication replaces the assets and keeps the release.
	result, err = publishGitHubRelease(root, gh, opts)
	if err != nil || result.State != "published" || !result.Published || gh.releases["v1.0.0"].assets["a"] != 2 {
		t.Fatalf("published retry: %+v, %v", result, err)
	}
	if got := strings.Count(strings.Join(gh.calls, ","), "create"); got != 1 {
		t.Fatalf("created %d releases", got)
	}
}

func TestGitHubReleaseDraft(t *testing.T) {
	root := githubFixture(t, map[string]string{".sweet.yml": "version: 1.0.0\n"})
	gh := &fakeGitHub{releases: map[string]*fakeRelease{}}
	result, err := publishGitHubRelease(root, gh, GitHubReleaseOptions{Tag: "v1.0.0", Draft: true, Title: "Custom", Notes: "Given"})
	release := gh.releases["v1.0.0"]
	if err != nil || result.Published || !release.draft || release.title != "Custom" || release.notes != "Given" {
		t.Fatalf("draft: %+v, %v, %+v", result, err, release)
	}
}

func TestGitHubReleaseLatestAndPrerelease(t *testing.T) {
	yes, no := true, false
	for _, tc := range []struct {
		name       string
		files      map[string]string
		tag        string
		remote     []string
		latest     *bool
		prerelease bool
		want       bool
		fail       bool
	}{
		{name: "highest stable root", files: map[string]string{".sweet.yml": "version: 2.0.0\n", "a/.sweet.yml": "version: 1.0.0\n"}, tag: "v2.0.0", remote: []string{"v1.9.9", "v2.0.0", "v10.0.0-rc.1"}, want: true},
		{name: "older stable root", files: map[string]string{".sweet.yml": "version: 2.0.0\n"}, tag: "v1.5.1", remote: []string{"v1.5.1", "v2.0.0"}},
		{name: "numeric comparison", files: map[string]string{".sweet.yml": "version: 1.10.0\n"}, tag: "v1.10.0", remote: []string{"v1.9.0", "v1.10.0"}, want: true},
		{name: "build metadata", files: map[string]string{".sweet.yml": "version: 1.0.0+b\n"}, tag: "v1.0.0+b", remote: []string{"v1.0.0+a"}, want: true},
		{name: "prerelease root", files: map[string]string{".sweet.yml": "version: 3.0.0-beta.1\n"}, tag: "v3.0.0-beta.1", remote: []string{"v2.0.0"}, prerelease: true},
		{name: "only package", files: map[string]string{"apps/cli/.sweet.yml": "version: 1.0.0\n"}, tag: "cli@1.0.0", remote: []string{"cli@1.0.0"}, want: true},
		{name: "component", files: map[string]string{".sweet.yml": "version: 1.0.0\n", "pkg/lib/.sweet.yml": "version: 1.0.0\n"}, tag: "lib@1.0.0"},
		{name: "forced latest", files: map[string]string{".sweet.yml": "version: 1.0.0\n", "pkg/lib/.sweet.yml": "version: 1.0.0\n"}, tag: "lib@1.0.0", latest: &yes, want: true},
		{name: "forced not latest", files: map[string]string{".sweet.yml": "version: 1.0.0\n"}, tag: "v1.0.0", remote: []string{"v1.0.0"}, latest: &no},
		{name: "forced latest prerelease", files: map[string]string{".sweet.yml": "version: 1.0.0\n"}, tag: "v2.0.0-rc.0", latest: &yes, fail: true},
		{name: "custom prefix", files: map[string]string{"pkg/sweet/.sweet.yml": "version: 1.0.0\ntag-prefix: pkg/sweet/v\n"}, tag: "pkg/sweet/v1.0.0", remote: []string{"pkg/sweet/v1.0.0"}, want: true},
		{name: "unknown tag", files: map[string]string{".sweet.yml": "version: 1.0.0\n"}, tag: "other@1.0.0", fail: true},
		{name: "not a version", files: map[string]string{".sweet.yml": "version: 1.0.0\n"}, tag: "vnext", fail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := githubFixture(t, tc.files)
			gh := &fakeGitHub{releases: map[string]*fakeRelease{}, remote: tc.remote}
			result, err := publishGitHubRelease(root, gh, GitHubReleaseOptions{Tag: tc.tag, Notes: "notes", Latest: tc.latest})
			if (err != nil) != tc.fail {
				t.Fatalf("error: %v", err)
			}
			if tc.fail {
				if len(gh.releases) != 0 {
					t.Fatal("failure created a release")
				}
				return
			}
			release := gh.releases[tc.tag]
			if result.Latest != tc.want || release.latest != tc.want || release.prerelease != tc.prerelease || result.Prerelease != tc.prerelease {
				t.Fatalf("result: %+v, release: %+v", result, release)
			}
		})
	}
}

func TestIsHighestStable(t *testing.T) {
	for _, tc := range []struct {
		version string
		tags    []string
		want    bool
	}{
		{"1.0.0", nil, true},
		{"1.0.0", []string{"v1.0.1"}, false},
		{"1.0.0", []string{"v1.0.1-beta.0", "v0.9.0", "v2.0.0x", "x1.0.0"}, true},
		{"1.0.0-beta.0", nil, false},
		{"99999999999999999999.0.0", []string{"v99999999999999999998.0.0"}, true},
	} {
		if got := isHighestStable(tc.version, "v", tc.tags); got != tc.want {
			t.Errorf("%s with %v = %t", tc.version, tc.tags, got)
		}
	}
}

func TestGitHubReleaseMissingChangelog(t *testing.T) {
	root := githubFixture(t, map[string]string{".sweet.yml": "version: 1.0.0\n", "CHANGELOG.md": "## 0.9.0\n"})
	gh := &fakeGitHub{releases: map[string]*fakeRelease{}}
	if _, err := publishGitHubRelease(root, gh, GitHubReleaseOptions{Tag: "v1.0.0"}); err == nil || !strings.Contains(err.Error(), `no release "1.0.0"`) {
		t.Fatalf("missing notes: %v", err)
	}
	if len(gh.calls) != 0 {
		t.Fatalf("calls: %v", gh.calls)
	}
}
