import importlib.util
import subprocess
import tempfile
import unittest
from pathlib import Path


MODULE_PATH = Path(__file__).with_name("detect_component_scope.py")
SPEC = importlib.util.spec_from_file_location("detect_component_scope", MODULE_PATH)
assert SPEC and SPEC.loader
SCOPE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(SCOPE)


class ComponentScopeTest(unittest.TestCase):
    def test_classifies_each_owned_directory(self):
        cases = {
            "admin": ["admin/apis/user.go", "admin/service/user.go"],
            "framework": ["mss-boot/pkg/config/config.go"],
            "web": ["web/antd-v6/src/pages/User/index.tsx"],
            "docs": ["docs/.dumirc.ts", "docs/docs/guide/index.md"],
        }
        for expected, paths in cases.items():
            with self.subTest(expected=expected):
                self.assertEqual(SCOPE.component_scope(paths), expected)

    def test_shared_scope_covers_empty_mixed_and_root_changes(self):
        cases = (
            [],
            ["docs/docs/index.md", "admin/main.go"],
            ["tools/release/check_release_policy.py"],
            [".github/workflows/docs.yml"],
        )
        for paths in cases:
            with self.subTest(paths=paths):
                self.assertEqual(SCOPE.component_scope(paths), "shared")

    def test_prefix_lookalikes_are_shared(self):
        self.assertEqual(
            SCOPE.component_scope(["docs-archive/index.md"]), "shared"
        )
        self.assertEqual(
            SCOPE.component_scope(["mss-boot-old/go.mod"]), "shared"
        )

    def test_vulnerability_modules_follow_component_ownership(self):
        self.assertEqual(SCOPE.go_modules_for_scope("admin"), ["admin"])
        self.assertEqual(SCOPE.go_modules_for_scope("framework"), ["mss-boot"])
        self.assertEqual(SCOPE.go_modules_for_scope("docs"), ["none"])
        self.assertEqual(SCOPE.go_modules_for_scope("web"), ["none"])
        self.assertEqual(
            SCOPE.go_modules_for_scope("shared"), SCOPE.ALL_GO_MODULES
        )

    def test_pull_request_diff_uses_merge_base_not_advanced_base_tree(self):
        with tempfile.TemporaryDirectory() as directory:
            repository = Path(directory)
            self._git(repository, "init", "-q")
            self._git(repository, "config", "user.name", "Scope Test")
            self._git(repository, "config", "user.email", "scope@example.invalid")
            (repository / "admin").mkdir()
            (repository / "docs").mkdir()
            (repository / "admin" / "README.md").write_text("base\n")
            (repository / "docs" / "README.md").write_text("base\n")
            self._git(repository, "add", ".")
            self._git(repository, "commit", "-qm", "base")
            base_branch = self._git(repository, "branch", "--show-current")

            self._git(repository, "checkout", "-qb", "docs-change")
            (repository / "docs" / "README.md").write_text("docs\n")
            self._git(repository, "commit", "-qam", "docs")
            head = self._git(repository, "rev-parse", "HEAD")

            self._git(repository, "checkout", "-q", base_branch)
            (repository / "admin" / "README.md").write_text("admin\n")
            self._git(repository, "commit", "-qam", "admin")
            advanced_base = self._git(repository, "rev-parse", "HEAD")

            scope, paths = SCOPE.classify_event(
                repository,
                "pull_request",
                advanced_base,
                head,
            )
            self.assertEqual(scope, "docs")
            self.assertEqual(paths, ["docs/README.md"])

    @staticmethod
    def _git(repository: Path, *args: str) -> str:
        result = subprocess.run(
            ["git", "-C", str(repository), *args],
            check=True,
            capture_output=True,
            text=True,
        )
        return result.stdout.strip()


if __name__ == "__main__":
    unittest.main()
