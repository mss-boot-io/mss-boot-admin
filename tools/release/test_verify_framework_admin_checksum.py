import importlib.util
import tempfile
import unittest
from pathlib import Path


MODULE_PATH = Path(__file__).with_name("verify_framework_admin_checksum.py")
SPEC = importlib.util.spec_from_file_location(
    "verify_framework_admin_checksum", MODULE_PATH
)
assert SPEC and SPEC.loader
CHECKSUM = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(CHECKSUM)


REPOSITORY_ROOT = Path(__file__).resolve().parents[2]


class FrameworkAdminChecksumTest(unittest.TestCase):
    def test_final_repository_tree_matches_admin_metadata(self):
        result = CHECKSUM.verify_repository(
            REPOSITORY_ROOT,
            version="v1.3.2",
        )
        self.assertTrue(result["success"])
        self.assertEqual(result["version"], "v1.3.2")
        self.assertGreater(result["candidateFiles"], 0)
        self.assertEqual(result["dependencyMode"], "replace-free-file-proxy")

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
            sources = [go_mod, readme, implementation]

            first_proxy = root / "proxy-first"
            CHECKSUM._write_file_module_proxy(
                first_proxy,
                module=CHECKSUM.FRAMEWORK_MODULE,
                version="v1.3.2",
                source_root=source,
                sources=sources,
            )
            first_sum, first_go_mod_sum = CHECKSUM._download_candidate_sums(
                proxy_root=first_proxy,
                module=CHECKSUM.FRAMEWORK_MODULE,
                version="v1.3.2",
                go_command="go",
            )

            readme.write_text("second candidate\n", encoding="utf-8")
            second_proxy = root / "proxy-second"
            CHECKSUM._write_file_module_proxy(
                second_proxy,
                module=CHECKSUM.FRAMEWORK_MODULE,
                version="v1.3.2",
                source_root=source,
                sources=sources,
            )
            second_sum, second_go_mod_sum = CHECKSUM._download_candidate_sums(
                proxy_root=second_proxy,
                module=CHECKSUM.FRAMEWORK_MODULE,
                version="v1.3.2",
                go_command="go",
            )

            self.assertNotEqual(first_sum, second_sum)
            self.assertEqual(first_go_mod_sum, second_go_mod_sum)

    def test_rejects_prerelease_and_version_drift_before_calculation(self):
        with self.assertRaisesRegex(CHECKSUM.ChecksumContractError, "stable"):
            CHECKSUM.verify_repository(
                REPOSITORY_ROOT,
                version="v1.3.2-rc.1",
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

require {CHECKSUM.FRAMEWORK_MODULE} v1.3.2

replace (
	{CHECKSUM.FRAMEWORK_MODULE} v1.3.2 => ../mss-boot
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
                    version="v1.3.2",
                    go_command="go",
                )


if __name__ == "__main__":
    unittest.main()
