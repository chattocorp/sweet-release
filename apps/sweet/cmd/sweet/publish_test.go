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
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("requires python3")
	}
	_, source, _, _ := runtime.Caller(0)
	script := filepath.Join(filepath.Dir(source), "../../../../.github/scripts/publish-cli.sh")
	releaseTool := filepath.Join(filepath.Dir(source), "../../../../actions/github-release/release.py")
	for _, prefix := range []string{"sweet@", "apps/sweet/v", "pkg/sweet/v", "pkg/cli/v"} {
		releasePackage := strings.TrimSuffix(prefix, "/v")
		if prefix == "sweet@" {
			releasePackage = "apps/sweet"
		}
		for _, tc := range []struct {
			name, state, failure, version string
			wantFailure                   bool
		}{
			{"missing draft", "missing", "", "0.1.0", true},
			{"retry draft", "draft", "", "0.1.0", false},
			{"published", "published", "", "0.1.0", false},
			{"API failure", "missing", "api", "0.1.0", true},
			{"build failure", "draft", "build", "0.1.0", true},
			{"checksum failure", "draft", "checksum", "0.1.0", true},
			{"wrong binary version", "draft", "binary-version", "0.1.0", true},
			{"upload failure", "draft", "upload", "0.1.0", true},
			{"remote tag moved during build", "draft", "tag-moved", "0.1.0", true},
			{"published during build", "draft", "published-after-build", "0.1.0", true},
			{"API access lost during build", "draft", "api-after-build", "0.1.0", true},
			{"prerelease", "draft", "", "0.2.0-rc.1", false},
		} {
			t.Run(prefix+"/"+tc.name, func(t *testing.T) {
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
				write("config.yml", "builds:\n  - id: sweet\n    dir: apps/sweet\n")
				write("bin/git", `#!/bin/sh
if [ "$1" = ls-remote ]; then
  commit=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
  if [ "$TEST_FAILURE" = tag-moved ] && [ -f "$TEST_BUILT" ]; then
    commit=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
  fi
  printf '%s\trefs/tags/%s\n' "$commit" "$TEST_TAG"
elif [ "$1" = check-ref-format ]; then
  exit 0
else
  echo aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
fi
`)
				write("bin/goreleaser", `#!/bin/sh
echo build >> "$TEST_LOG"
while [ "$1" != --config ]; do shift; done
shift
grep -qx "    dir: $TEST_PACKAGE" "$1" || exit 1
[ "$TEST_FAILURE" != build ] || exit 1
mkdir -p dist/sweet_linux_amd64_v1
if [ "$TEST_FAILURE" = binary-version ]; then SWEET_RELEASE_VERSION=0.0.0; fi
printf '#!/bin/sh\necho "sweet %s"\n' "$SWEET_RELEASE_VERSION" > dist/sweet_linux_amd64_v1/sweet
chmod +x dist/sweet_linux_amd64_v1/sweet
touch dist/test.tar.gz dist/test.zip dist/checksums.txt
touch "$TEST_BUILT"
`)
				write("bin/sha256sum", "#!/bin/sh\n[ \"$TEST_FAILURE\" != checksum ]\n")
				write("bin/gh", `#!/bin/sh
echo "$*" >> "$TEST_LOG"
if [ "$1" = api ]; then
  [ "$TEST_FAILURE" != api ] || exit 1
  if [ -f "$TEST_BUILT" ]; then
    [ "$TEST_FAILURE" != api-after-build ] || exit 1
    if [ "$TEST_FAILURE" = published-after-build ]; then TEST_STATE=published; fi
  fi
  case "$TEST_STATE" in
    draft) printf '{"data":{"repository":{"release":{"isDraft":true,"isPrerelease":false}}}}\n' ;;
    published) printf '{"data":{"repository":{"release":{"isDraft":false,"isPrerelease":false}}}}\n' ;;
    missing) printf '{"data":{"repository":{"release":null}}}\n' ;;
  esac
fi
if [ "$2" = upload ] && [ "$TEST_FAILURE" = upload ]; then exit 1; fi
`)
				cmd := exec.Command("bash", script, prefix+tc.version, "config.yml")
				cmd.Dir = root
				t.Setenv("TEST_PACKAGE", releasePackage)
				t.Setenv("TEST_TAG", prefix+tc.version)
				t.Setenv("SWEET_RELEASE_TOOL", releaseTool)
				t.Setenv("GH_REPO", "example/repo")
				t.Setenv("TEST_BUILT", filepath.Join(root, "built"))
				cmd.Env = append(os.Environ(), "PATH="+filepath.Join(root, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"), "TEST_LOG="+filepath.Join(root, "log"), "TEST_STATE="+tc.state, "TEST_FAILURE="+tc.failure)
				out, err := cmd.CombinedOutput()
				if (err != nil) != tc.wantFailure {
					t.Fatalf("publish: %v: %s", err, out)
				}
				log, _ := os.ReadFile(filepath.Join(root, "log"))
				text := string(log)
				if strings.Contains(text, "release edit") || strings.Contains(text, "release create") {
					t.Fatalf("asset builder changed release lifecycle: %s", text)
				}
				if strings.Contains(text, "build") != (tc.state == "draft") {
					t.Fatalf("must only build for an existing draft: %s", text)
				}
				wantUpload := tc.state == "draft" && (tc.failure == "" || tc.failure == "upload")
				if strings.Contains(text, "release upload") != wantUpload {
					t.Fatalf("unexpected asset upload: %s", text)
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
		out, err := cmd.CombinedOutput()
		if err == nil || !strings.Contains(string(out), "Unsupported release tag") {
			t.Fatalf("tag %s: %v: %s", tag, err, out)
		}
	}
}
