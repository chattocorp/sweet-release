import os
from pathlib import Path
import subprocess
import tempfile
import unittest


SCRIPT = Path(__file__).with_name("release-command.sh").resolve()


class ReleaseCommandTests(unittest.TestCase):
    def test_exact_checkout_metadata_retries_and_failures(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            def git(*args):
                return subprocess.check_output(["git", "-C", str(root), *args],
                                               stderr=subprocess.PIPE, text=True).strip()
            git("init")
            git("config", "user.name", "Test")
            git("config", "user.email", "test@example.com")
            git("config", "commit.gpgsign", "false")
            (root / "payload").write_text("release content")
            git("add", ".")
            git("commit", "-m", "Release")
            commit = git("rev-parse", "HEAD")
            (root / "payload").write_text("later content")
            git("commit", "-am", "Later change")
            git("checkout", "--detach", commit)
            env = dict(os.environ, RELEASE_PACKAGE="pkg/a package", RELEASE_VERSION="1.2.3",
                       RELEASE_TAG="api@1.2.3", RELEASE_COMMIT=commit,
                       GH_TOKEN="fixture-token", GH_REPO="example/repo")

            def run(command, **extra):
                return subprocess.run(["bash", str(SCRIPT)], cwd=root,
                                      env=dict(env, RELEASE_COMMAND=command, **extra),
                                      capture_output=True, text=True)

            command = '''
test "$(cat payload)" = 'release content'
test "$RELEASE_PACKAGE" = 'pkg/a package'
test "$RELEASE_VERSION" = '1.2.3'
test "$RELEASE_TAG" = 'api@1.2.3'
test "$GH_TOKEN" = fixture-token
test "$GH_REPO" = example/repo
printf '%s\n' "$RELEASE_COMMIT" >> invocations
'''
            for _ in range(2):
                result = run(command)
                self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual((root / "invocations").read_text().splitlines(), [commit, commit])
            for command in ("false; touch forbidden", "false | true; touch forbidden",
                            "exit 23"):
                self.assertNotEqual(run(command).returncode, 0)
            self.assertNotEqual(run("touch forbidden", RELEASE_COMMIT="0" * 40).returncode, 0)
            self.assertFalse((root / "forbidden").exists())


if __name__ == "__main__":
    unittest.main()
