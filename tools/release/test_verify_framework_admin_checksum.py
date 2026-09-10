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
EXPECTED_FRAMEWORK_SUM = "h1:5ZA4aTgFSYIzZuSXG8an1HqssdO6PoxrHdNDhHMPvng="
EXPECTED_FRAMEWORK_GO_MOD_SUM = "h1:qejH+UcGKJRwGtMQisbYCLg7nYf4TEOe/h6fGJ1nK7Q="
EXPECTED_ADMIN_SUM = "h1:sQl2Q58DVYF1NTikN8OhUcnbyF949TtU1hbyyLaTSQU="
EXPECTED_ADMIN_GO_MOD_SUM = "h1:v/KJqYYGo5PYW4PNHnctx3ujxQ5yzt8/ZgD/MmUyzxs="
EXPECTED_RELEASE_COMMIT = "77b53d41092741eac62fa6418c0bdbf87413c7cd"


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

    def _publish_checksum_fixture(
        self,
        root: Path,
        *,
        admin_tag: str = "release",
    ) -> str:
        refreshed = CHECKSUM.refresh_repository_metadata(
            root,
            version="v1.3.3",
            go_command=os.environ.get("MSS_TEST_GO", "go"),
        )
        self._git(root, "add", *refreshed["updatedFiles"])
        self._git(root, "commit", "-qm", "freeze release metadata")
        release_commit = self._git(root, "rev-parse", "HEAD").stdout.strip()

        policy_text = (
            (REPOSITORY_ROOT / ".mss" / "release-policy.yaml")
            .read_text(encoding="utf-8")
            .replace(EXPECTED_RELEASE_COMMIT, release_commit)
            .replace("v1.3.7", "v1.3.3")
        )
        policy_path = root / ".mss" / "release-policy.yaml"
        policy_path.parent.mkdir(parents=True)
        policy_path.write_text(policy_text, encoding="utf-8")
        self._git(root, "add", policy_path.relative_to(root).as_posix())
        self._git(root, "commit", "-qm", "record current stable identity")
        policy_commit = self._git(root, "rev-parse", "HEAD").stdout.strip()

        self._git(root, "tag", "v1.3.3", release_commit)
        self._git(root, "tag", "mss-boot/v1.3.3", release_commit)
        if admin_tag == "release":
            self._git(root, "tag", "admin/v1.3.3", release_commit)
        elif admin_tag == "policy":
            self._git(root, "tag", "admin/v1.3.3", policy_commit)
        elif admin_tag != "missing":
            self.fail(f"unsupported admin tag fixture mode: {admin_tag}")
        return release_commit

    def test_published_current_stable_tree_matches_adopter_metadata(self):
        result = CHECKSUM.verify_current_stable_repository(
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
        self.assertEqual(result["sourceMode"], "current-stable")
        self.assertEqual(result["sourceCommit"], EXPECTED_RELEASE_COMMIT)
        self.assertEqual(
            result["sourceTags"],
            {
                "root": "v1.3.7",
                "framework": "mss-boot/v1.3.7",
                "admin": "admin/v1.3.7",
            },
        )

    def test_current_stable_uses_release_tree_after_later_component_docs_change(self):
        with tempfile.TemporaryDirectory(
            prefix="mss-published-stable-checksum-"
        ) as directory:
            root = Path(directory)
            self._init_checksum_repository(root)
            release_commit = self._publish_checksum_fixture(root)

            (root / "mss-boot" / "README.md").write_text(
                "post-release framework documentation\n",
                encoding="utf-8",
            )
            (root / "admin" / "README.md").write_text(
                "post-release admin documentation\n",
                encoding="utf-8",
            )
            self._git(root, "add", "mss-boot/README.md", "admin/README.md")
            self._git(root, "commit", "-qm", "update post-release docs")

            stable = CHECKSUM.verify_current_stable_repository(
                root,
                version="v1.3.3",
                go_command=os.environ.get("MSS_TEST_GO", "go"),
            )
            self.assertEqual(stable["sourceMode"], "current-stable")
            self.assertEqual(stable["sourceCommit"], release_commit)
            self.assertNotEqual(stable["headCommit"], release_commit)
            self.assertNotIn("docs", stable["sourceTags"])

            with self.assertRaisesRegex(
                CHECKSUM.ChecksumContractError,
                "final candidate tree",
            ):
                CHECKSUM.verify_repository(
                    root,
                    version="v1.3.3",
                    go_command=os.environ.get("MSS_TEST_GO", "go"),
                )

    def test_current_stable_rejects_missing_or_mismatched_component_tag(self):
        for admin_tag, error in (
            ("missing", "admin tag is missing"),
            ("policy", "admin tag .* does not resolve"),
        ):
            with self.subTest(admin_tag=admin_tag), tempfile.TemporaryDirectory(
                prefix="mss-published-tag-contract-"
            ) as directory:
                root = Path(directory)
                self._init_checksum_repository(root)
                self._publish_checksum_fixture(root, admin_tag=admin_tag)
                with self.assertRaisesRegex(
                    CHECKSUM.ChecksumContractError,
                    error,
                ):
                    CHECKSUM.verify_current_stable_repository(
                        root,
                        version="v1.3.3",
                        go_command=os.environ.get("MSS_TEST_GO", "go"),
                    )

    def test_current_stable_rejects_version_outside_policy_identity(self):
        with tempfile.TemporaryDirectory(
            prefix="mss-published-version-contract-"
        ) as directory:
            root = Path(directory)
            self._init_checksum_repository(root)
            self._publish_checksum_fixture(root)
            with self.assertRaisesRegex(
                CHECKSUM.ChecksumContractError,
                "requires policy version v1.3.3",
            ):
                CHECKSUM.verify_current_stable_repository(
                    root,
                    version="v1.3.4",
                    go_command=os.environ.get("MSS_TEST_GO", "go"),
                )

    def test_published_source_commit_must_be_exact_and_ancestral(self):
        with tempfile.TemporaryDirectory(
            prefix="mss-published-source-identity-"
        ) as directory:
            root = Path(directory)
            self._init_checksum_repository(root)
            head = self._git(root, "rev-parse", "HEAD").stdout.strip()
            tree = self._git(root, "rev-parse", "HEAD^{tree}").stdout.strip()
            unrelated = self._git(
                root,
                "commit-tree",
                tree,
                "-m",
                "unrelated source",
            ).stdout.strip()

            for source_commit, error in (
                (head[:12], "full lowercase"),
                ("f" * 40, "does not resolve exactly"),
                (unrelated, "must be an ancestor"),
            ):
                with self.subTest(source_commit=source_commit):
                    with self.assertRaisesRegex(
                        CHECKSUM.ChecksumContractError,
                        error,
                    ):
                        CHECKSUM._resolve_published_source_commit(
                            root,
                            source_commit=source_commit,
                        )

    def test_current_stable_write_modes_fail_before_refresh(self):
        for source_mode in ("candidate", "current-stable"):
            with self.subTest(source_mode=source_mode), mock.patch.object(
                CHECKSUM,
                "refresh_repository_metadata",
            ) as refresh, mock.patch("builtins.print"):
                exit_code = CHECKSUM.main(
                    [
                        "--root",
                        str(REPOSITORY_ROOT),
                        "--version",
                        "v1.3.7",
                        "--source-mode",
                        source_mode,
                        "--write",
                    ]
                )
                self.assertEqual(exit_code, 1)
                refresh.assert_not_called()

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

    def test_candidate_verification_rejects_head_change_before_reporting(self):
        with tempfile.TemporaryDirectory(prefix="mss-checksum-head-race-") as directory:
            root = Path(directory)
            self._init_checksum_repository(root)
            refreshed = CHECKSUM.refresh_repository_metadata(
                root,
                version="v1.3.3",
                go_command=os.environ.get("MSS_TEST_GO", "go"),
            )
            self._git(root, "add", *refreshed["updatedFiles"])
            self._git(root, "commit", "-qm", "refresh checksums")
            head = self._git(root, "rev-parse", "HEAD").stdout.strip()

            with mock.patch.object(
                CHECKSUM,
                "_head_commit",
                side_effect=(head, "f" * 40),
            ), self.assertRaisesRegex(
                CHECKSUM.ChecksumContractError,
                "HEAD changed while .* evidence",
            ):
                CHECKSUM.verify_repository(
                    root,
                    version="v1.3.3",
                    go_command=os.environ.get("MSS_TEST_GO", "go"),
                )

    def test_checksum_refresh_rejects_head_change_before_writing(self):
        with tempfile.TemporaryDirectory(prefix="mss-checksum-write-race-") as directory:
            root = Path(directory)
            self._init_checksum_repository(root)
            head = self._git(root, "rev-parse", "HEAD").stdout.strip()

            with mock.patch.object(
                CHECKSUM,
                "_head_commit",
                side_effect=(head, "f" * 40),
            ), mock.patch.object(
                CHECKSUM,
                "_write_if_changed",
            ) as write, self.assertRaisesRegex(
                CHECKSUM.ChecksumContractError,
                "HEAD changed before refreshed checksum metadata could be written",
            ):
                CHECKSUM.refresh_repository_metadata(
                    root,
                    version="v1.3.3",
                    go_command=os.environ.get("MSS_TEST_GO", "go"),
                )
            write.assert_not_called()

    def test_checksum_refresh_rejects_output_tampering_before_success(self):
        with tempfile.TemporaryDirectory(prefix="mss-checksum-output-race-") as directory:
            root = Path(directory)
            self._init_checksum_repository(root)
            real_write = CHECKSUM._write_if_changed
            writes = 0

            def write_then_tamper(path: Path, content: bytes) -> bool:
                nonlocal writes
                changed = real_write(path, content)
                writes += 1
                if writes == 3:
                    (root / "admin" / "go.sum").write_text(
                        "concurrent output tampering\n",
                        encoding="utf-8",
                    )
                return changed

            with mock.patch.object(
                CHECKSUM,
                "_write_if_changed",
                side_effect=write_then_tamper,
            ), self.assertRaisesRegex(
                CHECKSUM.ChecksumContractError,
                "metadata changed before success: admin/go.sum",
            ):
                CHECKSUM.refresh_repository_metadata(
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
