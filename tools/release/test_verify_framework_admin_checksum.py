import importlib.util
import os
import stat
import subprocess
import tempfile
import unittest
from pathlib import Path, PurePosixPath
from unittest import mock


MODULE_PATH = Path(__file__).with_name("verify_framework_admin_checksum.py")
SPEC = importlib.util.spec_from_file_location(
    "verify_framework_admin_checksum", MODULE_PATH
)
assert SPEC and SPEC.loader
CHECKSUM = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(CHECKSUM)


REPOSITORY_ROOT = Path(__file__).resolve().parents[2]
EXPECTED_FRAMEWORK_SUM = "h1:pIrFkBClPs+AkjwekBBcRIPKSH7dhVqJUzCESlFmDJ8="
EXPECTED_FRAMEWORK_GO_MOD_SUM = "h1:qejH+UcGKJRwGtMQisbYCLg7nYf4TEOe/h6fGJ1nK7Q="
EXPECTED_ADMIN_SUM = "h1:8xyIK6biIvYG2FRhBhZFUPLjcL19T5h7d1R462TpEb0="
EXPECTED_ADMIN_GO_MOD_SUM = "h1:v/KJqYYGo5PYW4PNHnctx3ujxQ5yzt8/ZgD/MmUyzxs="


class FrameworkAdminChecksumTest(unittest.TestCase):
    def _git(self, root: Path, *argv: str) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            ["git", *argv],
            cwd=root,
            check=True,
            capture_output=True,
            text=True,
            encoding="utf-8",
        )

    def _init_checksum_repository(self, root: Path) -> None:
        files = {
            ".gitattributes": "* text=auto eol=lf\n",
            "LICENSE": "repository license\n",
            "mss-boot/go.mod": f"module {CHECKSUM.FRAMEWORK_MODULE}\n\ngo 1.26.6\n",
            "mss-boot/runtime.go": "package mssboot\n",
            "admin/go.mod": (
                f"module {CHECKSUM.ADMIN_MODULE}\n\ngo 1.26.6\n\n"
                f"require {CHECKSUM.FRAMEWORK_MODULE} v1.3.3\n"
            ),
            "admin/go.sum": (
                f"{CHECKSUM.FRAMEWORK_MODULE} v1.3.3 h1:old-framework\n"
                f"{CHECKSUM.FRAMEWORK_MODULE} v1.3.3/go.mod h1:old-framework-mod\n"
            ),
            "admin/main.go": "package admin\n",
            "templates/application/go.sum": (
                f"{CHECKSUM.ADMIN_MODULE} v1.3.3 h1:old-admin\n"
                f"{CHECKSUM.ADMIN_MODULE} v1.3.3/go.mod h1:old-admin-mod\n"
                f"{CHECKSUM.FRAMEWORK_MODULE} v1.3.3 h1:old-framework\n"
                f"{CHECKSUM.FRAMEWORK_MODULE} v1.3.3/go.mod h1:old-framework-mod\n"
            ),
            "tools/release/test_verify_framework_admin_checksum.py": (
                'EXPECTED_FRAMEWORK_SUM = "h1:old-framework"\n'
                'EXPECTED_FRAMEWORK_GO_MOD_SUM = "h1:old-framework-mod"\n'
                'EXPECTED_ADMIN_SUM = "h1:old-admin"\n'
                'EXPECTED_ADMIN_GO_MOD_SUM = "h1:old-admin-mod"\n'
            ),
            "go.work": (
                "go 1.26.6\n\n"
                f"replace {CHECKSUM.FRAMEWORK_MODULE} v1.3.3 => ./mss-boot\n"
            ),
        }
        for relative, content in files.items():
            path = root / relative
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(content, encoding="utf-8")
        self._git(root, "init", "-q")
        self._git(root, "config", "user.name", "Checksum Test")
        self._git(root, "config", "user.email", "checksum@example.com")
        self._git(root, "add", ".")
        self._git(root, "commit", "-qm", "fixture")

    def test_final_repository_tree_matches_admin_metadata(self):
        result = CHECKSUM.verify_repository(
            REPOSITORY_ROOT,
            version="v1.3.7",
        )
        self.assertTrue(result["success"])
        self.assertEqual(result["version"], "v1.3.7")
        self.assertGreater(result["candidateFiles"], 0)
        self.assertGreater(result["adminCandidateFiles"], 0)
        self.assertEqual(result["sum"], EXPECTED_FRAMEWORK_SUM)
        self.assertEqual(result["goModSum"], EXPECTED_FRAMEWORK_GO_MOD_SUM)
        self.assertEqual(
            result["adminSum"],
            EXPECTED_ADMIN_SUM,
        )
        self.assertEqual(
            result["adminGoModSum"],
            EXPECTED_ADMIN_GO_MOD_SUM,
        )
        self.assertEqual(result["dependencyMode"], "replace-free-file-proxy")

    def test_nested_module_inherits_repository_root_license(self):
        with tempfile.TemporaryDirectory(
            prefix="mss-admin-license-inheritance-"
        ) as directory:
            root = Path(directory)
            self._init_checksum_repository(root)

            sources = CHECKSUM._git_module_candidate(
                root,
                source_directory="admin",
                label="Admin",
            )
            licenses = [
                source
                for source in sources
                if source.relative == PurePosixPath("LICENSE")
            ]
            self.assertEqual(len(licenses), 1)
            self.assertEqual(licenses[0].content, b"repository license\n")
            self.assertEqual(licenses[0].mode, stat.S_IFREG | 0o644)

    def test_nested_module_license_takes_precedence_over_repository_license(self):
        with tempfile.TemporaryDirectory(
            prefix="mss-admin-own-license-"
        ) as directory:
            root = Path(directory)
            self._init_checksum_repository(root)
            admin_license = root / "admin" / "LICENSE"
            admin_license.write_text("admin license\n", encoding="utf-8")
            self._git(root, "add", "admin/LICENSE")
            self._git(root, "commit", "-qm", "add admin license")

            sources = CHECKSUM._git_module_candidate(
                root,
                source_directory="admin",
                label="Admin",
            )
            licenses = [
                source
                for source in sources
                if source.relative == PurePosixPath("LICENSE")
            ]
            self.assertEqual(len(licenses), 1)
            self.assertEqual(licenses[0].content, b"admin license\n")

    def test_dirty_inherited_repository_license_fails_closed(self):
        with tempfile.TemporaryDirectory(
            prefix="mss-admin-dirty-license-"
        ) as directory:
            root = Path(directory)
            self._init_checksum_repository(root)
            (root / "LICENSE").write_text(
                "uncommitted license drift\n",
                encoding="utf-8",
            )

            with self.assertRaisesRegex(
                CHECKSUM.ChecksumContractError,
                "Admin repository LICENSE candidate drift",
            ):
                CHECKSUM.calculate_admin_candidate_sums(
                    root,
                    version="v1.3.3",
                    go_command=os.environ.get("MSS_TEST_GO", "go"),
                )

    def test_candidate_module_sum_changes_when_source_content_changes(self):
        with tempfile.TemporaryDirectory(
            prefix="mss-framework-checksum-unit-"
        ) as directory:
            root = Path(directory)
            source = root / "source"
            source.mkdir()
            go_mod = source / "go.mod"
            readme = source / "README.md"
            implementation = source / "runtime.go"
            go_mod.write_text(
                f"module {CHECKSUM.FRAMEWORK_MODULE}\n\ngo 1.26.6\n",
                encoding="utf-8",
            )
            readme.write_text("first candidate\n", encoding="utf-8")
            implementation.write_text("package mssboot\n", encoding="utf-8")
            sources = [
                CHECKSUM.GitCandidateFile(
                    PurePosixPath(path.name),
                    path.read_bytes(),
                    0o100644,
                )
                for path in (go_mod, readme, implementation)
            ]

            first_proxy = root / "proxy-first"
            CHECKSUM._write_file_module_proxy(
                first_proxy,
                module=CHECKSUM.FRAMEWORK_MODULE,
                version="v1.3.3",
                sources=sources,
            )
            first_sum, first_go_mod_sum = CHECKSUM._download_candidate_sums(
                proxy_root=first_proxy,
                module=CHECKSUM.FRAMEWORK_MODULE,
                version="v1.3.3",
                go_command="go",
            )

            readme.write_text("second candidate\n", encoding="utf-8")
            sources = [
                CHECKSUM.GitCandidateFile(
                    PurePosixPath(path.name),
                    path.read_bytes(),
                    0o100644,
                )
                for path in (go_mod, readme, implementation)
            ]
            second_proxy = root / "proxy-second"
            CHECKSUM._write_file_module_proxy(
                second_proxy,
                module=CHECKSUM.FRAMEWORK_MODULE,
                version="v1.3.3",
                sources=sources,
            )
            second_sum, second_go_mod_sum = CHECKSUM._download_candidate_sums(
                proxy_root=second_proxy,
                module=CHECKSUM.FRAMEWORK_MODULE,
                version="v1.3.3",
                go_command="go",
            )

            self.assertNotEqual(first_sum, second_sum)
            self.assertEqual(first_go_mod_sum, second_go_mod_sum)

    def test_symlink_go_mod_does_not_hide_regular_files(self):
        sources = [
            CHECKSUM.GitCandidateFile(
                PurePosixPath("go.mod"),
                f"module {CHECKSUM.FRAMEWORK_MODULE}\n".encode(),
                stat.S_IFREG | 0o644,
            ),
            CHECKSUM.GitCandidateFile(
                PurePosixPath("nested/go.mod"),
                b"../go.mod",
                stat.S_IFLNK | 0o777,
            ),
            CHECKSUM.GitCandidateFile(
                PurePosixPath("nested/runtime.go"),
                b"package nested\n",
                stat.S_IFREG | 0o644,
            ),
        ]
        selected = CHECKSUM._module_archive_files(sources)
        selected_paths = {source.relative.as_posix() for source in selected}
        self.assertNotIn("nested/go.mod", selected_paths)
        self.assertIn("nested/runtime.go", selected_paths)

    def test_crlf_worktree_and_lf_checkout_use_identical_git_candidate(self):
        with tempfile.TemporaryDirectory(prefix="mss-checksum-crlf-") as directory:
            root = Path(directory)
            source = root / "mss-boot"
            source.mkdir()
            (source / ".gitattributes").write_text(
                "*.go text\n", encoding="utf-8"
            )
            (source / "go.mod").write_text(
                f"module {CHECKSUM.FRAMEWORK_MODULE}\n\ngo 1.26.6\n",
                encoding="utf-8",
            )
            implementation = source / "runtime.go"
            implementation.write_text("package mssboot\n", encoding="utf-8")
            self._git(root, "init", "-q")
            self._git(root, "config", "user.name", "Checksum Test")
            self._git(root, "config", "user.email", "checksum@example.com")
            self._git(root, "config", "core.autocrlf", "true")
            self._git(root, "add", ".")
            self._git(root, "commit", "-qm", "fixture")

            lf_result = CHECKSUM.calculate_candidate_sums(
                root, version="v1.3.3", go_command=os.environ.get("MSS_TEST_GO", "go")
            )
            implementation.write_bytes(b"package mssboot\r\n")
            self._git(root, "diff", "--quiet", "--ignore-cr-at-eol")
            crlf_result = CHECKSUM.calculate_candidate_sums(
                root, version="v1.3.3", go_command=os.environ.get("MSS_TEST_GO", "go")
            )
            self.assertEqual(crlf_result, lf_result)

    def test_dirty_candidate_fails_closed(self):
        with tempfile.TemporaryDirectory(prefix="mss-checksum-dirty-") as directory:
            root = Path(directory)
            source = root / "mss-boot"
            source.mkdir()
            (source / "go.mod").write_text(
                f"module {CHECKSUM.FRAMEWORK_MODULE}\n\ngo 1.26.6\n",
                encoding="utf-8",
            )
            implementation = source / "runtime.go"
            implementation.write_text("package mssboot\n", encoding="utf-8")
            self._git(root, "init", "-q")
            self._git(root, "config", "user.name", "Checksum Test")
            self._git(root, "config", "user.email", "checksum@example.com")
            self._git(root, "add", ".")
            self._git(root, "commit", "-qm", "fixture")
            implementation.write_text("package mssboot\n// drift\n", encoding="utf-8")

            with self.assertRaisesRegex(
                CHECKSUM.ChecksumContractError,
                "uncommitted Framework candidate drift",
            ):
                CHECKSUM.calculate_candidate_sums(
                    root,
                    version="v1.3.3",
                    go_command=os.environ.get("MSS_TEST_GO", "go"),
                )

    def test_write_is_crlf_safe_idempotent_and_changes_only_approved_files(self):
        with tempfile.TemporaryDirectory(prefix="mss-checksum-write-") as directory:
            root = Path(directory)
            self._init_checksum_repository(root)
            approved = {
                "admin/go.sum",
                "templates/application/go.sum",
                "tools/release/test_verify_framework_admin_checksum.py",
            }
            for relative in approved:
                path = root / relative
                path.write_bytes(path.read_bytes().replace(b"\n", b"\r\n"))
            first = CHECKSUM.refresh_repository_metadata(
                root,
                version="v1.3.3",
                go_command=os.environ.get("MSS_TEST_GO", "go"),
            )
            self.assertEqual(set(first["updatedFiles"]), approved)
            changed = {
                line[3:]
                for line in self._git(root, "status", "--porcelain").stdout.splitlines()
            }
            self.assertEqual(changed, approved)

            second = CHECKSUM.refresh_repository_metadata(
                root,
                version="v1.3.3",
                go_command=os.environ.get("MSS_TEST_GO", "go"),
            )
            self.assertEqual(second["updatedFiles"], [])
            self.assertEqual(second["sum"], first["sum"])
            self.assertEqual(second["adminSum"], first["adminSum"])

            self._git(root, "add", *sorted(approved))
            self._git(root, "commit", "-qm", "refresh checksums")
            verified = CHECKSUM.verify_repository(
                root,
                version="v1.3.3",
                go_command=os.environ.get("MSS_TEST_GO", "go"),
            )
            self.assertEqual(verified["sum"], first["sum"])
            self.assertEqual(verified["adminSum"], first["adminSum"])

    def test_verify_rejects_committed_thin_host_framework_checksum_drift(self):
        for suffix in ("", "/go.mod"):
            with self.subTest(suffix=suffix), tempfile.TemporaryDirectory(
                prefix="mss-checksum-template-drift-"
            ) as directory:
                root = Path(directory)
                self._init_checksum_repository(root)
                refreshed = CHECKSUM.refresh_repository_metadata(
                    root,
                    version="v1.3.3",
                    go_command=os.environ.get("MSS_TEST_GO", "go"),
                )
                self._git(root, "add", *refreshed["updatedFiles"])
                self._git(root, "commit", "-qm", "refresh checksums")

                template = root / "templates/application/go.sum"
                key = f"v1.3.3{suffix}"
                prefix = f"{CHECKSUM.FRAMEWORK_MODULE} {key} "
                lines = template.read_text(encoding="utf-8").splitlines()
                matches = 0
                for index, line in enumerate(lines):
                    if line.startswith(prefix):
                        lines[index] = prefix + "h1:deliberately-wrong"
                        matches += 1
                self.assertEqual(matches, 1)
                template.write_text("\n".join(lines) + "\n", encoding="utf-8")
                self._git(root, "add", template.relative_to(root).as_posix())
                self._git(root, "commit", "-qm", "corrupt thin host framework sum")

                with self.assertRaisesRegex(
                    CHECKSUM.ChecksumContractError,
                    r"Thin Host Framework (?:Module|go\.mod) checksum",
                ):
                    CHECKSUM.verify_repository(
                        root,
                        version="v1.3.3",
                        go_command=os.environ.get("MSS_TEST_GO", "go"),
                    )

    def test_rejects_prerelease_and_version_drift_before_calculation(self):
        with self.assertRaisesRegex(CHECKSUM.ChecksumContractError, "stable"):
            CHECKSUM.verify_repository(
                REPOSITORY_ROOT,
                version="v1.3.3-rc.1",
            )

    def test_rejects_multiline_admin_replace(self):
        with tempfile.TemporaryDirectory(
            prefix="mss-framework-checksum-admin-mod-"
        ) as directory:
            admin = Path(directory)
            go_mod = admin / "go.mod"
            go_mod.write_text(
                f"""module example.com/admin

go 1.26.6

require {CHECKSUM.FRAMEWORK_MODULE} v1.3.3

replace (
	{CHECKSUM.FRAMEWORK_MODULE} v1.3.3 => ../mss-boot
)
""",
                encoding="utf-8",
            )
            with self.assertRaisesRegex(
                CHECKSUM.ChecksumContractError,
                "must not replace",
            ):
                CHECKSUM._required_framework_version(go_mod, go_command="go")

    def test_rejects_versionless_workspace_replace(self):
        with tempfile.TemporaryDirectory(
            prefix="mss-framework-checksum-workspace-"
        ) as directory:
            root = Path(directory)
            (root / "go.work").write_text(
                f"""go 1.26.6

replace {CHECKSUM.FRAMEWORK_MODULE} => ./mss-boot
""",
                encoding="utf-8",
            )
            with self.assertRaisesRegex(
                CHECKSUM.ChecksumContractError,
                "candidate-only",
            ):
                CHECKSUM._verify_workspace_replacement(
                    root,
                    version="v1.3.3",
                    go_command="go",
                )

    def test_workspace_validation_uses_the_explicit_target_when_gowork_is_disabled(self):
        with tempfile.TemporaryDirectory(
            prefix="mss-framework-checksum-explicit-workspace-"
        ) as directory:
            root = Path(directory)
            self._init_checksum_repository(root)
            with mock.patch.dict(os.environ, {"GOWORK": "off"}):
                CHECKSUM._verify_workspace_replacement(
                    root,
                    version="v1.3.3",
                    go_command=os.environ.get("MSS_TEST_GO", "go"),
                )


if __name__ == "__main__":
    unittest.main()
