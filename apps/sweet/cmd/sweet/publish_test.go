package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCLIPublishing(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("publishing uses Bash")
	}
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("requires jq")
	}
	_, source, _, _ := runtime.Caller(0)
	script := filepath.Join(filepath.Dir(source), "../../../../.github/scripts/publish-cli.sh")
	metadata := filepath.Join(t.TempDir(), "sweet-metadata")
	build := exec.Command("go", "build", "-o", metadata, ".")
	build.Dir = filepath.Dir(source)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build metadata: %v: %s", err, out)
	}
	t.Setenv("TEST_METADATA", metadata)
	for _, releasePackage := range []string{"apps/sweet", "pkg/sweet", "pkg/cli"} {
		for _, tc := range []struct {
			name, state, failure, version string
			wantFailure                   bool
		}{
			{"new", "missing", "", "0.1.0", false},
			{"retry draft", "draft", "", "0.1.0", false},
			{"published", "published", "", "0.1.0", false},
			{"API failure", "missing", "api", "0.1.0", true},
			{"build failure", "missing", "build", "0.1.0", true},
			{"upload failure", "missing", "upload", "0.1.0", true},
			{"prerelease", "missing", "", "0.2.0-rc.1", false},
		} {
			t.Run(releasePackage+"/"+tc.name, func(t *testing.T) {
				root := t.TempDir()
				write := func(name, data string) {
					t.Helper()
					path := filepath.Join(root, name)
					if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, []byte(data), 0o755); err != nil {
						t.Fatal(err)
					}
				}
				write(releasePackage+"/CHANGELOG.md", "# Changelog\n\n## "+tc.version+"\n\n- New feature.\n\n## 0.0.1\n\n- Old feature.\n")
				write(releasePackage+"/.sweet.yml", "version: "+tc.version+"\n")
				write("config.yml", "builds:\n  - id: sweet\n    dir: apps/sweet\n")
				write("bin/git", "#!/bin/sh\necho commit\n")
				write("bin/sweet", "#!/bin/sh\nif [ \"$1\" = changelog ]; then exec \"$TEST_METADATA\" \"$@\"; fi\nprintf '%s\\n' '[{\"path\":\""+releasePackage+"\",\"name\":\"Sweet Release\"}]'\n")
				write("bin/goreleaser", `#!/bin/sh
echo build >> "$TEST_LOG"
while [ "$1" != --config ]; do shift; done
shift
grep -qx "    dir: $TEST_PACKAGE" "$1" || exit 1
[ "$TEST_FAILURE" != build ] || exit 1
mkdir -p dist/sweet_linux_amd64_v1
printf '#!/bin/sh\necho "sweet %s"\n' "$SWEET_RELEASE_VERSION" > dist/sweet_linux_amd64_v1/sweet
chmod +x dist/sweet_linux_amd64_v1/sweet
touch dist/test.tar.gz dist/test.zip dist/checksums.txt
`)
				write("bin/sha256sum", "#!/bin/sh\nexit 0\n")
				write("bin/gh", `#!/bin/sh
echo "$*" >> "$TEST_LOG"
if [ "$1" = api ]; then
  [ "$TEST_FAILURE" != api ] || exit 1
  case "$TEST_STATE" in
    draft) printf '{"tag_name":"%s/v%s","draft":true}\n' "$TEST_PACKAGE" "$TEST_VERSION" ;;
    published) printf '{"tag_name":"%s/v%s","draft":false}\n' "$TEST_PACKAGE" "$TEST_VERSION" ;;
  esac
fi
if [ "$2" = upload ] && [ "$TEST_FAILURE" = upload ]; then exit 1; fi
while [ "$#" -gt 0 ]; do
  if [ "$1" = --notes-file ]; then cp "$2" "$TEST_NOTES"; fi
  shift
done
`)
				cmd := exec.Command("bash", script, releasePackage+"/v"+tc.version, "config.yml")
				cmd.Dir = root
				t.Setenv("SWEET_METADATA", filepath.Join(root, "bin/sweet"))
				t.Setenv("TEST_PACKAGE", releasePackage)
				cmd.Env = append(os.Environ(), "PATH="+filepath.Join(root, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"), "TEST_LOG="+filepath.Join(root, "log"), "TEST_NOTES="+filepath.Join(root, "notes"), "TEST_STATE="+tc.state, "TEST_FAILURE="+tc.failure, "TEST_VERSION="+tc.version)
				out, err := cmd.CombinedOutput()
				if (err != nil) != tc.wantFailure {
					t.Fatalf("publish: %v: %s", err, out)
				}
				log, _ := os.ReadFile(filepath.Join(root, "log"))
				text := string(log)
				published := strings.Contains(text, "release edit")
				if published != (!tc.wantFailure && tc.state != "published") {
					t.Fatalf("unexpected publish: %s", text)
				}
				if tc.state == "published" && strings.Contains(text, "build") {
					t.Fatalf("rebuilt published release: %s", text)
				}
				if tc.state == "draft" && strings.Contains(text, "release create") {
					t.Fatalf("recreated draft: %s", text)
				}
				if published {
					if !strings.Contains(text, "--title Sweet Release "+tc.version) {
						t.Fatalf("wrong title: %s", text)
					}
					notes, _ := os.ReadFile(filepath.Join(root, "notes"))
					if !strings.Contains(string(notes), "New feature") || strings.Contains(string(notes), "Old feature") {
						t.Fatalf("wrong notes: %s", notes)
					}
					if strings.Contains(tc.version, "-rc.") && !strings.Contains(text, "--prerelease=true") {
						t.Fatalf("not a prerelease: %s", text)
					}
				}
			})
		}
	}
}

func TestPublishingRejectsOtherPackages(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("publishing uses Bash")
	}
	_, source, _, _ := runtime.Caller(0)
	script := filepath.Join(filepath.Dir(source), "../../../../.github/scripts/publish-cli.sh")
	for _, tag := range []string{"pkg/core/v0.2.0", "pkg/docs/v1.0.0", "apps/docs/v1.0.0", "v1.0.0"} {
		cmd := exec.Command("bash", script, tag, "unused.yml")
		cmd.Env = append(os.Environ(), "SWEET_METADATA=unused")
		out, err := cmd.CombinedOutput()
		if err == nil || !strings.Contains(string(out), "Unsupported release tag") {
			t.Fatalf("tag %s: %v: %s", tag, err, out)
		}
	}
}
