import importlib.util
import io
import sys
import tarfile
import tempfile
import unittest
import zipfile
from pathlib import Path


SCRIPT_DIRECTORY = Path(__file__).resolve().parent


def load_module(name: str, filename: str):
    spec = importlib.util.spec_from_file_location(name, SCRIPT_DIRECTORY / filename)
    assert spec and spec.loader
    module = importlib.util.module_from_spec(spec)
    sys.modules[name] = module
    spec.loader.exec_module(module)
    return module


PORTABLE = load_module("check_portable_paths", "check_portable_paths.py")
PREPARE = load_module("prepare_portable_frontend", "prepare_portable_frontend.py")


class PortablePathTest(unittest.TestCase):
    def test_directory_rejects_windows_invalid_and_reserved_names(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "departments" / ":id").mkdir(parents=True)
            (root / "departments" / ":id" / "index.html").write_text("route")
            (root / "NUL.txt").write_text("reserved")

            issues = PORTABLE.check_paths([root])

        messages = [f"{issue.member}: {issue.reason}" for issue in issues]
        self.assertTrue(any("departments/:id" in item for item in messages))
        self.assertTrue(any("reserved Windows device" in item for item in messages))

    def test_archives_reject_traversal_case_collisions_and_colons(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            zip_path = root / "bad.zip"
            with zipfile.ZipFile(zip_path, "w") as archive:
                archive.writestr("dist/Menu/index.html", "one")
                archive.writestr("dist/menu/index.html", "two")
                archive.writestr("dist/users/:id/index.html", "three")
                archive.writestr("../escape.txt", "four")

            issues = PORTABLE.check_paths([zip_path])

        reasons = "\n".join(issue.reason for issue in issues)
        self.assertIn("case-insensitive filesystem", reasons)
        self.assertIn("forbidden character", reasons)
        self.assertIn("unsafe component", reasons)

    def test_archives_reject_duplicate_paths_and_special_tar_members(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            zip_path = root / "duplicate.zip"
            with zipfile.ZipFile(zip_path, "w") as archive:
                archive.writestr("dist/index.html", "one")
                archive.writestr("./dist/index.html", "two")

            tar_path = root / "special.tar"
            with tarfile.open(tar_path, "w") as archive:
                fifo = tarfile.TarInfo("dist/release.pipe")
                fifo.type = tarfile.FIFOTYPE
                archive.addfile(fifo, io.BytesIO())

            zip_issues = PORTABLE.check_paths([zip_path])
            tar_issues = PORTABLE.check_paths([tar_path])

        self.assertTrue(
            any("duplicated" in issue.reason for issue in zip_issues), zip_issues
        )
        self.assertTrue(
            any("special files" in issue.reason for issue in tar_issues), tar_issues
        )

    def test_portable_tar_and_directory_pass(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            dist = root / "dist"
            (dist / "assets").mkdir(parents=True)
            (dist / "index.html").write_text("index")
            (dist / "assets" / "umi.12345678.js").write_text("asset")
            tar_path = root / "dist.tar.gz"
            with tarfile.open(tar_path, "w:gz") as archive:
                archive.add(dist, arcname="dist")

            self.assertEqual(PORTABLE.check_paths([dist, tar_path]), [])

    def test_prepare_removes_only_umi_dynamic_placeholders(self):
        with tempfile.TemporaryDirectory() as directory:
            allowed_root = Path(directory)
            dist = allowed_root / "dist"
            dynamic = dist / "departments" / ":id"
            fixed = dist / "departments"
            dynamic.mkdir(parents=True)
            (dynamic / "index.html").write_text("dynamic")
            (fixed / "index.html").write_text("fixed")

            removed = PREPARE.prepare_frontend(dist, allowed_root=allowed_root)

            self.assertEqual(removed, ["departments/:id"])
            self.assertFalse(dynamic.exists())
            self.assertTrue((fixed / "index.html").is_file())
            self.assertEqual(dist.stat().st_mode & 0o777, 0o755)
            self.assertEqual((fixed / "index.html").stat().st_mode & 0o777, 0o644)
            self.assertEqual(PORTABLE.check_paths([dist]), [])

    def test_prepare_fails_closed_on_unexpected_dynamic_content(self):
        with tempfile.TemporaryDirectory() as directory:
            allowed_root = Path(directory)
            dist = allowed_root / "dist"
            dynamic = dist / "users" / ":id"
            dynamic.mkdir(parents=True)
            (dynamic / "index.html").write_text("dynamic")
            (dynamic / "payload.js").write_text("unexpected")

            with self.assertRaisesRegex(
                PREPARE.PreparationError, "unexpected members"
            ):
                PREPARE.prepare_frontend(dist, allowed_root=allowed_root)

    def test_prepare_preflights_every_placeholder_before_deleting(self):
        with tempfile.TemporaryDirectory() as directory:
            allowed_root = Path(directory)
            dist = allowed_root / "dist"
            safe = dist / "departments" / ":id"
            unsafe = dist / "users" / ":id"
            safe.mkdir(parents=True)
            unsafe.mkdir(parents=True)
            (safe / "index.html").write_text("safe")
            (unsafe / "index.html").write_text("unsafe")
            (unsafe / "payload.js").write_text("unexpected")

            with self.assertRaisesRegex(
                PREPARE.PreparationError, "unexpected members"
            ):
                PREPARE.prepare_frontend(dist, allowed_root=allowed_root)

            self.assertTrue((safe / "index.html").is_file())

    def test_prepare_rejects_allowed_root_and_non_dist_directory(self):
        with tempfile.TemporaryDirectory() as directory:
            allowed_root = Path(directory)
            build = allowed_root / "build"
            build.mkdir()

            with self.assertRaisesRegex(PREPARE.PreparationError, "nested dist"):
                PREPARE.prepare_frontend(allowed_root, allowed_root=allowed_root)
            with self.assertRaisesRegex(PREPARE.PreparationError, "nested dist"):
                PREPARE.prepare_frontend(build, allowed_root=allowed_root)


if __name__ == "__main__":
    unittest.main()
