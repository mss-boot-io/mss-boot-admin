import io
import json
import sys
import tarfile
import tempfile
import unittest
from pathlib import Path


TOOLS_DIR = Path(__file__).resolve().parent
sys.path.insert(0, str(TOOLS_DIR))
import verify_admin_web_package as PACKAGE  # noqa: E402


COMMIT = "a" * 40
REPOSITORY = "mss-boot-io/mss-boot-admin"


def write_tarball(
    path: Path,
    *,
    private: bool = False,
    extra: dict[str, bytes] | None = None,
    metadata: dict[str, object] | None = None,
    include_license: bool = True,
    include_git_head: bool = True,
):
    manifest = {
        "name": PACKAGE.PACKAGE_NAME,
        "version": "1.3.0",
        "private": private,
        "license": "MIT",
        "repository": {
            "type": "git",
            "url": f"git+https://github.com/{REPOSITORY}.git",
            "directory": "web/antd-v6",
        },
        "homepage": "https://docs.mss-boot-io.top/",
        "bugs": {"url": f"https://github.com/{REPOSITORY}/issues"},
        "packageManager": "pnpm@10.34.5",
        "mssAdminDistribution": {
            "packageManager": PACKAGE.ADMIN_WEB_PACKAGE_MANAGER,
            "runtimeOverrides": PACKAGE.ADMIN_WEB_RUNTIME_OVERRIDES,
        },
        "engines": {"node": ">=24.0.0 <25", "pnpm": "10.34.5"},
        "files": [
            "src",
            "package",
            "public",
            "bin",
            "tests/setup.ts",
            "LICENSE",
        ],
        "exports": {
            ".": {"types": "./src/index.ts", "default": "./src/index.ts"},
            "./preset": "./package/preset.js",
        },
        "bin": {"mss-admin-web": "./bin/mss-admin-web.js"},
    }
    if include_git_head:
        manifest["gitHead"] = COMMIT
    files = {
        "package/package.json": json.dumps(manifest).encode(),
        "package/src/index.ts": b"export const adminWeb = true;\n",
        "package/package/preset.js": b"module.exports = {};\n",
        "package/public/logo.svg": b"<svg/>\n",
        "package/bin/mss-admin-web.js": b"#!/usr/bin/env node\n",
        "package/tests/setup.ts": b"export {};\n",
    }
    if include_license:
        files["package/LICENSE"] = b"MIT License\n\nPermission is hereby granted.\n"
    manifest.update(metadata or {})
    files["package/package.json"] = json.dumps(manifest).encode()
    files.update(extra or {})
    with tarfile.open(path, mode="w:gz") as archive:
        for name, content in files.items():
            member = tarfile.TarInfo(name)
            member.size = len(content)
            member.mode = 0o755 if name.endswith("bin/mss-admin-web.js") else 0o644
            member.mtime = 0
            archive.addfile(member, io.BytesIO(content))


class AdminWebPackageTest(unittest.TestCase):
    def inspect(self, path: Path):
        return PACKAGE.inspect_package(
            path,
            expected_name=PACKAGE.PACKAGE_NAME,
            expected_version="1.3.0",
            source_repository=REPOSITORY,
            source_commit=COMMIT,
        )

    def test_accepts_explicit_complete_portable_package(self):
        with tempfile.TemporaryDirectory() as directory:
            tarball = Path(directory) / "mss-boot-io-admin-web-1.3.0.tgz"
            write_tarball(tarball)
            evidence = self.inspect(tarball)

        self.assertEqual(evidence["package"]["name"], PACKAGE.PACKAGE_NAME)
        self.assertEqual(evidence["package"]["version"], "1.3.0")
        self.assertEqual(evidence["package"]["license"], "MIT")
        self.assertEqual(
            evidence["package"]["repository"]["directory"], "web/antd-v6"
        )
        self.assertEqual(evidence["source"]["commit"], COMMIT)
        self.assertEqual(
            evidence["package"]["mssAdminDistribution"],
            {
                "packageManager": PACKAGE.ADMIN_WEB_PACKAGE_MANAGER,
                "runtimeOverrides": PACKAGE.ADMIN_WEB_RUNTIME_OVERRIDES,
            },
        )
        self.assertRegex(evidence["artifact"]["sha256"], r"^[0-9a-f]{64}$")
        self.assertTrue(evidence["artifact"]["integrity"].startswith("sha512-"))
        self.assertIn("package/package/preset.js", evidence["artifact"]["members"])
        self.assertIn("package/tests/setup.ts", evidence["artifact"]["members"])
        self.assertIn("package/LICENSE", evidence["artifact"]["members"])

    def test_rejects_missing_or_incorrect_public_metadata(self):
        cases = (
            ({"license": "Apache-2.0"}, True, "repository MIT license"),
            ({"repository": {"type": "git", "url": "https://example.invalid"}}, True, "repository metadata"),
            ({"homepage": "https://example.invalid"}, True, "public MSS documentation"),
            ({"bugs": {"url": "https://example.invalid/issues"}}, True, "source issue tracker"),
            ({}, False, "include its MIT LICENSE"),
        )
        for metadata, include_license, message in cases:
            with self.subTest(metadata=metadata, include_license=include_license):
                with tempfile.TemporaryDirectory() as directory:
                    tarball = Path(directory) / "package.tgz"
                    write_tarball(
                        tarball,
                        metadata=metadata,
                        include_license=include_license,
                    )
                    with self.assertRaisesRegex(PACKAGE.PackageError, message):
                        self.inspect(tarball)

    def test_rejects_missing_or_drifted_distribution_host_contract(self):
        valid_contract = {
            "packageManager": PACKAGE.ADMIN_WEB_PACKAGE_MANAGER,
            "runtimeOverrides": PACKAGE.ADMIN_WEB_RUNTIME_OVERRIDES,
        }
        cases = (
            None,
            {
                **valid_contract,
                "packageManager": "pnpm@10.34.4",
            },
            {
                **valid_contract,
                "runtimeOverrides": {
                    **PACKAGE.ADMIN_WEB_RUNTIME_OVERRIDES,
                    "react": "19.2.7",
                },
            },
            {
                **valid_contract,
                "runtimeOverrides": {
                    name: version
                    for name, version in PACKAGE.ADMIN_WEB_RUNTIME_OVERRIDES.items()
                    if name != "axios"
                },
            },
            {
                **valid_contract,
                "runtimeOverrides": {
                    **PACKAGE.ADMIN_WEB_RUNTIME_OVERRIDES,
                    "unsupported-runtime": "1.0.0",
                },
            },
        )
        for contract in cases:
            with self.subTest(contract=contract):
                with tempfile.TemporaryDirectory() as directory:
                    tarball = Path(directory) / "package.tgz"
                    write_tarball(
                        tarball,
                        metadata={"mssAdminDistribution": contract},
                    )
                    with self.assertRaisesRegex(
                        PACKAGE.PackageError, "exact package manager"
                    ):
                        self.inspect(tarball)

    def test_rejects_private_or_local_package_members(self):
        cases = (
            (True, None, "must not be private"),
            (False, {"package/.env.local": b"SECRET=value\n"}, "environment file"),
            (False, {"package/node_modules/react/index.js": b""}, "banned local"),
            (False, {"package/src/debug.log": b""}, "credential or log"),
            (False, {"package/.umi/umi.ts": b""}, "Umi build state"),
            (
                False,
                {"package/src/generated/routes.ts": b""},
                "reference-app generated code",
            ),
            (
                False,
                {"package/src/runtime.test.ts": b""},
                "repository-only test",
            ),
            (
                False,
                {"package/tests/business-fixture.ts": b""},
                "unsupported test content",
            ),
        )
        for private, extra, message in cases:
            with self.subTest(private=private, extra=extra):
                with tempfile.TemporaryDirectory() as directory:
                    tarball = Path(directory) / "package.tgz"
                    write_tarball(tarball, private=private, extra=extra)
                    with self.assertRaisesRegex(PACKAGE.PackageError, message):
                        self.inspect(tarball)

    def test_evidence_requires_the_exact_embedded_git_head(self):
        with tempfile.TemporaryDirectory() as directory:
            tarball = Path(directory) / "package.tgz"
            write_tarball(tarball)
            evidence = PACKAGE.inspect_package(
                tarball,
                expected_name=PACKAGE.PACKAGE_NAME,
                expected_version="1.3.0",
                source_repository=REPOSITORY,
                source_commit=COMMIT,
            )
            self.assertEqual(evidence["package"]["gitHead"], COMMIT)

            missing = Path(directory) / "missing-git-head.tgz"
            write_tarball(missing, include_git_head=False)
            with self.assertRaisesRegex(PACKAGE.PackageError, "exact source commit"):
                PACKAGE.inspect_package(
                    missing,
                    expected_name=PACKAGE.PACKAGE_NAME,
                    expected_version="1.3.0",
                    source_repository=REPOSITORY,
                    source_commit=COMMIT,
                )

    def test_rejects_wrong_version_and_non_executable_cli(self):
        with tempfile.TemporaryDirectory() as directory:
            tarball = Path(directory) / "package.tgz"
            write_tarball(tarball)
            with self.assertRaisesRegex(PACKAGE.PackageError, "does not equal"):
                PACKAGE.inspect_package(
                    tarball,
                    expected_name=PACKAGE.PACKAGE_NAME,
                    expected_version="1.3.1",
                    source_repository=REPOSITORY,
                    source_commit=COMMIT,
                )

            broken = Path(directory) / "broken.tgz"
            with tarfile.open(tarball, mode="r:gz") as source, tarfile.open(
                broken, mode="w:gz"
            ) as target:
                for member in source.getmembers():
                    payload = source.extractfile(member) if member.isfile() else None
                    if member.name.endswith("bin/mss-admin-web.js"):
                        member.mode = 0o644
                    target.addfile(member, payload)
            with self.assertRaisesRegex(PACKAGE.PackageError, "must be executable"):
                self.inspect(broken)


if __name__ == "__main__":
    unittest.main()
