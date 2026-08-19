#!/usr/bin/env python3
"""Validate and inventory the publishable complete Admin Web package."""

from __future__ import annotations

import argparse
import base64
import hashlib
import json
import re
import tarfile
from pathlib import Path, PurePosixPath
from typing import Any, Iterable

from check_portable_paths import check_paths


PACKAGE_NAME = "@mss-boot-io/admin-web"
SCHEMA = "mss.io/admin-web-package-evidence/v1"
SHA_RE = re.compile(r"^[0-9a-f]{40}$")
ADMIN_WEB_PACKAGE_MANAGER = "pnpm@10.34.5"
ADMIN_WEB_RUNTIME_OVERRIDES = {
    "@ant-design/pro-components": "3.1.14-6",
    "@tanstack/react-query": "5.101.4",
    "antd": "6.6.0",
    "axios": "0.33.0",
    "react": "19.2.8",
    "react-dom": "19.2.8",
}
ADMIN_WEB_RUNTIME_DEPENDENCIES = [
    "@ant-design/icons",
    "@ant-design/pro-components",
    "@tanstack/react-query",
    "antd",
    "antd-style",
    "clsx",
    "d",
    "dayjs",
    "react",
    "react-dom",
]
ADMIN_WEB_TOOLING_DEPENDENCIES = [
    "@biomejs/biome",
    "@tailwindcss/postcss",
    "@types/node",
    "@types/react",
    "@types/react-dom",
    "@umijs/max",
    "happy-dom",
    "tailwindcss",
    "typescript",
    "vite",
    "vitest",
]
ADMIN_WEB_BUILD_ONLY_DEPENDENCIES = {
    "image-size": ["0.5.5"],
    "immer": ["8.0.4"],
    "node-fetch": ["1.7.3"],
    "path-to-regexp": ["1.7.0"],
    "vite": ["4.5.2"],
}
ADMIN_WEB_DEPENDENCY_CLASSES = {
    "runtime": ADMIN_WEB_RUNTIME_DEPENDENCIES,
    "tooling": ADMIN_WEB_TOOLING_DEPENDENCIES,
}
VERSION_RE = re.compile(
    r"^(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)"
    r"(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$"
)
BANNED_SEGMENTS = {
    ".git",
    ".github",
    ".cache",
    ".mss",
    ".tmp",
    "__fixtures__",
    "__snapshots__",
    "coverage",
    "fixtures",
    "node_modules",
    "playwright-report",
    "reports",
    "temp",
    "test-results",
    "tmp",
}
BANNED_SUFFIXES = (
    ".log",
    ".pem",
    ".p12",
    ".pfx",
)


class PackageError(ValueError):
    pass


def _read_json_member(archive: tarfile.TarFile, member: tarfile.TarInfo) -> dict[str, Any]:
    if member.size > 4 * 1024 * 1024:
        raise PackageError("package.json exceeds the 4 MiB validation limit")
    stream = archive.extractfile(member)
    if stream is None:
        raise PackageError("package.json is not a regular readable file")
    try:
        value = json.loads(stream.read().decode("utf-8"))
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise PackageError("package.json is not valid UTF-8 JSON") from exc
    if not isinstance(value, dict):
        raise PackageError("package.json must contain one JSON object")
    return value


def _export_targets(value: Any) -> Iterable[str]:
    if isinstance(value, str):
        yield value
    elif isinstance(value, dict):
        for nested in value.values():
            yield from _export_targets(nested)
    elif isinstance(value, list):
        for nested in value:
            yield from _export_targets(nested)


def _validate_manifest(
    manifest: dict[str, Any],
    members: set[str],
    expected_name: str,
    expected_version: str,
    source_repository: str,
    source_commit: str,
) -> tuple[list[str], dict[str, str]]:
    if manifest.get("name") != expected_name:
        raise PackageError(
            f"package name {manifest.get('name')!r} does not equal {expected_name!r}"
        )
    if manifest.get("version") != expected_version:
        raise PackageError(
            f"package version {manifest.get('version')!r} does not equal {expected_version!r}"
        )
    if manifest.get("private") is True:
        raise PackageError("publishable Admin Web package must not be private")
    if manifest.get("license") != "MIT":
        raise PackageError("package license must match the repository MIT license")
    expected_repository = {
        "type": "git",
        "url": f"git+https://github.com/{source_repository}.git",
        "directory": "web/antd-v6",
    }
    if manifest.get("repository") != expected_repository:
        raise PackageError("package repository metadata does not identify web/antd-v6")
    if manifest.get("homepage") != "https://docs.mss-boot-io.top/":
        raise PackageError("package homepage must identify the public MSS documentation")
    if manifest.get("bugs") != {
        "url": f"https://github.com/{source_repository}/issues"
    }:
        raise PackageError("package bugs metadata must identify the source issue tracker")
    if manifest.get("gitHead") != source_commit:
        raise PackageError("package gitHead must equal the exact source commit")
    if "package/LICENSE" not in members:
        raise PackageError("package must include its MIT LICENSE file")
    distribution_contract = manifest.get("mssAdminDistribution")
    expected_distribution_contract = {
        "packageManager": ADMIN_WEB_PACKAGE_MANAGER,
        "dependencyClasses": ADMIN_WEB_DEPENDENCY_CLASSES,
        "buildOnlyDependencies": ADMIN_WEB_BUILD_ONLY_DEPENDENCIES,
        "runtimeOverrides": ADMIN_WEB_RUNTIME_OVERRIDES,
    }
    if distribution_contract != expected_distribution_contract:
        raise PackageError(
            "package mssAdminDistribution must retain the exact package manager "
            "and supported dependency security contract"
        )
    package_manager = manifest.get("packageManager")
    if package_manager not in (None, ADMIN_WEB_PACKAGE_MANAGER):
        raise PackageError(
            f"packageManager, when packed, must be {ADMIN_WEB_PACKAGE_MANAGER}"
        )
    engines = manifest.get("engines")
    if not isinstance(engines, dict):
        raise PackageError("package engines contract is missing")
    if engines.get("pnpm") != "10.34.5":
        raise PackageError("package engines.pnpm must equal 10.34.5")
    node_engine = engines.get("node")
    if not isinstance(node_engine, str) or "24" not in node_engine:
        raise PackageError("package engines.node must retain the Node 24 contract")

    files = manifest.get("files")
    if not isinstance(files, list) or not files:
        raise PackageError("package files must be a non-empty explicit allowlist")
    file_allowlist: list[str] = []
    for value in files:
        if not isinstance(value, str) or not value or value.startswith(("/", "!")):
            raise PackageError("package files entries must be non-empty positive relative paths")
        normalized = value.removeprefix("./").rstrip("/")
        path = PurePosixPath(normalized)
        if (
            normalized in {".", "*", "**"}
            or ".." in path.parts
            or "\\" in value
            or normalized.endswith("/**")
        ):
            raise PackageError(f"package files entry is too broad or unsafe: {value!r}")
        file_allowlist.append(normalized)

    exports = manifest.get("exports")
    if not isinstance(exports, dict) or not exports:
        raise PackageError("package exports must be a non-empty explicit object")
    export_targets: dict[str, str] = {}
    for export_name, definition in exports.items():
        if not isinstance(export_name, str) or "*" in export_name:
            raise PackageError("package export names must be explicit and stable")
        targets = sorted(set(_export_targets(definition)))
        if not targets:
            raise PackageError(f"package export {export_name!r} has no target")
        for target in targets:
            if not target.startswith("./") or "*" in target:
                raise PackageError(f"package export target is unsafe: {target!r}")
            member_name = f"package/{target[2:]}"
            if member_name not in members:
                raise PackageError(
                    f"package export {export_name!r} target is absent: {target!r}"
                )
        export_targets[export_name] = ",".join(targets)

    bins = manifest.get("bin")
    if not isinstance(bins, dict) or not isinstance(bins.get("mss-admin-web"), str):
        raise PackageError("package must expose the mss-admin-web CLI")
    bin_target = bins["mss-admin-web"]
    if not bin_target.startswith("./") or "*" in bin_target or ".." in PurePosixPath(bin_target).parts:
        raise PackageError("mss-admin-web bin target must be one explicit relative path")
    if f"package/{bin_target[2:]}" not in members:
        raise PackageError("mss-admin-web bin target is absent from the tarball")

    for required_prefix in ("package/src/", "package/package/", "package/public/"):
        if not any(member.startswith(required_prefix) for member in members):
            raise PackageError(f"complete Admin Web package is missing {required_prefix}")
    return file_allowlist, export_targets


def inspect_package(
    tarball: Path,
    *,
    expected_name: str,
    expected_version: str,
    source_repository: str,
    source_commit: str,
) -> dict[str, Any]:
    if not VERSION_RE.fullmatch(expected_version):
        raise PackageError("expected version must be an unprefixed semantic version")
    if not SHA_RE.fullmatch(source_commit):
        raise PackageError("source commit must be a lowercase full SHA")
    if not re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", source_repository):
        raise PackageError("source repository must use OWNER/REPOSITORY syntax")
    if not tarball.is_file() or not tarfile.is_tarfile(tarball):
        raise PackageError("Admin Web package must be one readable tar archive")

    portability_issues = check_paths([tarball])
    if portability_issues:
        issue = portability_issues[0]
        raise PackageError(f"non-portable package member {issue.member!r}: {issue.reason}")

    with tarfile.open(tarball, mode="r:*") as archive:
        archive_members = archive.getmembers()
        member_names: list[str] = []
        seen_names: set[str] = set()
        package_json_member: tarfile.TarInfo | None = None
        bin_modes: dict[str, int] = {}
        for member in archive_members:
            name = member.name.rstrip("/")
            if not name:
                continue
            if name in seen_names:
                raise PackageError(f"tarball contains a duplicate member: {name!r}")
            seen_names.add(name)
            if not name.startswith("package/"):
                raise PackageError(f"tarball member escapes the package root: {name!r}")
            if member.issym() or member.islnk() or not (member.isfile() or member.isdir()):
                raise PackageError(f"tarball member is not a regular file or directory: {name!r}")
            if member.mode & 0o6002:
                raise PackageError(f"tarball member has unsafe mode {member.mode:o}: {name!r}")
            path = PurePosixPath(name)
            lowered_parts = {part.casefold() for part in path.parts}
            lowered_name = name.casefold()
            if lowered_parts & BANNED_SEGMENTS:
                raise PackageError(f"tarball contains a banned local or generated path: {name!r}")
            if any(part.startswith(".umi") for part in lowered_parts):
                raise PackageError(f"tarball contains Umi build state: {name!r}")
            if lowered_name.startswith("package/src/generated/") or lowered_name.startswith(
                "package/src/pages/generated/"
            ):
                raise PackageError(f"tarball contains reference-app generated code: {name!r}")
            if re.search(r"(?:^|/)[^/]+\.(?:test|spec)\.[^/]+$", lowered_name):
                raise PackageError(f"tarball contains a repository-only test: {name!r}")
            if "tests" in lowered_parts and lowered_name != "package/tests/setup.ts":
                raise PackageError(f"tarball contains unsupported test content: {name!r}")
            if any(part.startswith(".env") for part in lowered_parts):
                raise PackageError(f"tarball contains an environment file: {name!r}")
            if lowered_name.endswith(BANNED_SUFFIXES):
                raise PackageError(f"tarball contains a credential or log file: {name!r}")
            if member.isfile() and not (member.mode & 0o400):
                raise PackageError(f"tarball member is not owner-readable: {name!r}")
            member_names.append(name)
            bin_modes[name] = member.mode
            if name == "package/package.json":
                package_json_member = member

        if package_json_member is None:
            raise PackageError("tarball does not contain package/package.json")
        manifest = _read_json_member(archive, package_json_member)
        member_set = set(member_names)
        file_allowlist, export_targets = _validate_manifest(
            manifest,
            member_set,
            expected_name,
            expected_version,
            source_repository,
            source_commit,
        )
        license_member = archive.getmember("package/LICENSE")
        if license_member.size > 128 * 1024:
            raise PackageError("package LICENSE exceeds the 128 KiB validation limit")
        license_stream = archive.extractfile(license_member)
        if license_stream is None:
            raise PackageError("package LICENSE is not a readable regular file")
        try:
            license_text = license_stream.read().decode("utf-8")
        except UnicodeDecodeError as exc:
            raise PackageError("package LICENSE is not valid UTF-8") from exc
        if not license_text.startswith("MIT License\n"):
            raise PackageError("package LICENSE does not contain the repository MIT text")
        bin_target = manifest["bin"]["mss-admin-web"]
        if not (bin_modes[f"package/{bin_target[2:]}"] & 0o100):
            raise PackageError("mss-admin-web bin target must be executable")

    content = tarball.read_bytes()
    sha256 = hashlib.sha256(content).hexdigest()
    sha512_digest = hashlib.sha512(content).digest()
    return {
        "schema": SCHEMA,
        "package": {
            "name": expected_name,
            "version": expected_version,
            "license": manifest["license"],
            "repository": manifest["repository"],
            "homepage": manifest["homepage"],
            "bugs": manifest["bugs"],
            "gitHead": manifest.get("gitHead"),
            "mssAdminDistribution": manifest["mssAdminDistribution"],
            "files": file_allowlist,
            "exports": export_targets,
            "bin": {"mss-admin-web": manifest["bin"]["mss-admin-web"]},
        },
        "source": {
            "repository": source_repository,
            "commit": source_commit,
        },
        "artifact": {
            "filename": tarball.name,
            "size": len(content),
            "sha256": sha256,
            "integrity": "sha512-" + base64.b64encode(sha512_digest).decode("ascii"),
            "memberCount": len(member_names),
            "members": sorted(member_names),
        },
    }


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--tarball", type=Path, required=True)
    parser.add_argument("--expected-name", default=PACKAGE_NAME)
    parser.add_argument("--expected-version", required=True)
    parser.add_argument("--source-repository", required=True)
    parser.add_argument("--source-commit", required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args(argv)
    try:
        evidence = inspect_package(
            args.tarball,
            expected_name=args.expected_name,
            expected_version=args.expected_version,
            source_repository=args.source_repository,
            source_commit=args.source_commit,
        )
    except PackageError as exc:
        print(f"Admin Web package rejected: {exc}")
        return 1
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(
        json.dumps(evidence, ensure_ascii=False, indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
    )
    print(
        "Admin Web package accepted: "
        f"{evidence['package']['name']}@{evidence['package']['version']} "
        f"({evidence['artifact']['memberCount']} members)"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
