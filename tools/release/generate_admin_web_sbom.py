#!/usr/bin/env python3
"""Generate a deterministic SPDX 2.3 SBOM from pnpm's installed dependency graph."""

from __future__ import annotations

import argparse
import hashlib
import json
import re
from pathlib import Path
from typing import Any
from urllib.parse import quote, urlparse


SCHEMA = "mss.io/admin-web-package-evidence/v1"
SPDX_VERSION = "SPDX-2.3"
COMMIT_RE = re.compile(r"^[0-9a-f]{40}$")
CREATED_RE = re.compile(r"^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$")


class SBOMError(ValueError):
    pass


def _read_object(path: Path, label: str) -> dict[str, Any]:
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise SBOMError(f"cannot read {label} JSON: {exc}") from exc
    if not isinstance(value, dict):
        raise SBOMError(f"{label} must contain one JSON object")
    return value


def _spdx_id(name: str, version: str, resolved: str) -> str:
    digest = hashlib.sha256(f"{name}\0{version}\0{resolved}".encode()).hexdigest()[:16]
    safe_name = re.sub(r"[^A-Za-z0-9.-]+", "-", name).strip("-") or "package"
    return f"SPDXRef-Package-{safe_name}-{digest}"


def _purl(name: str, version: str) -> str:
    return f"pkg:npm/{quote(name, safe='/')}@{quote(version, safe='')}"


def _download_location(resolved: Any) -> str:
    if not isinstance(resolved, str):
        return "NOASSERTION"
    parsed = urlparse(resolved)
    if parsed.scheme == "https" and parsed.netloc:
        return resolved
    return "NOASSERTION"


def build_sbom(
    dependency_tree: dict[str, Any],
    package_evidence: dict[str, Any],
    *,
    source_repository: str,
    source_commit: str,
    created: str,
) -> dict[str, Any]:
    if package_evidence.get("schema") != SCHEMA:
        raise SBOMError("unsupported package evidence schema")
    if not COMMIT_RE.fullmatch(source_commit):
        raise SBOMError("source commit must be a lowercase full SHA")
    if not CREATED_RE.fullmatch(created):
        raise SBOMError("created must be a UTC timestamp formatted as YYYY-MM-DDTHH:MM:SSZ")
    source = package_evidence.get("source")
    if not isinstance(source, dict) or source.get("repository") != source_repository:
        raise SBOMError("package evidence source repository does not match")
    if source.get("commit") != source_commit:
        raise SBOMError("package evidence source commit does not match")
    package = package_evidence.get("package")
    artifact = package_evidence.get("artifact")
    if not isinstance(package, dict) or not isinstance(artifact, dict):
        raise SBOMError("package evidence is missing package or artifact metadata")
    package_name = package.get("name")
    package_version = package.get("version")
    package_license = package.get("license")
    package_homepage = package.get("homepage")
    package_sha256 = artifact.get("sha256")
    if not isinstance(package_name, str) or not isinstance(package_version, str):
        raise SBOMError("package evidence name and version must be strings")
    if package_license != "MIT":
        raise SBOMError("package evidence license must be MIT")
    if package.get("gitHead") != source_commit:
        raise SBOMError("package evidence gitHead does not match the source commit")
    if package_homepage != "https://docs.mss-boot-io.top/":
        raise SBOMError("package evidence homepage does not match the public documentation")
    if package.get("repository") != {
        "type": "git",
        "url": f"git+https://github.com/{source_repository}.git",
        "directory": "web/antd-v6",
    }:
        raise SBOMError("package evidence repository does not match the source repository")
    if package.get("bugs") != {
        "url": f"https://github.com/{source_repository}/issues"
    }:
        raise SBOMError("package evidence issue tracker does not match the source repository")
    if not isinstance(package_sha256, str) or not re.fullmatch(r"[0-9a-f]{64}", package_sha256):
        raise SBOMError("package evidence SHA-256 is invalid")
    if dependency_tree.get("name") != package_name:
        raise SBOMError("pnpm dependency tree package name does not match the tarball")

    root_id = _spdx_id(package_name, package_version, source_commit)
    packages_by_key: dict[tuple[str, str, str], dict[str, Any]] = {}
    relationships: set[tuple[str, str]] = set()

    def visit(parent_id: str, dependencies: Any) -> None:
        if dependencies is None:
            return
        if not isinstance(dependencies, dict):
            raise SBOMError("pnpm dependency tree dependencies must be objects")
        for dependency_name, value in dependencies.items():
            if not isinstance(dependency_name, str) or not isinstance(value, dict):
                raise SBOMError("pnpm dependency entries must be named objects")
            version = value.get("version")
            if not isinstance(version, str) or not version:
                raise SBOMError(f"pnpm dependency {dependency_name!r} has no version")
            resolved = value.get("resolved") if isinstance(value.get("resolved"), str) else ""
            key = (dependency_name, version, resolved)
            spdx_id = _spdx_id(*key)
            packages_by_key.setdefault(
                key,
                {
                    "SPDXID": spdx_id,
                    "name": dependency_name,
                    "versionInfo": version,
                    "downloadLocation": _download_location(resolved),
                    "filesAnalyzed": False,
                    "licenseConcluded": "NOASSERTION",
                    "licenseDeclared": "NOASSERTION",
                    "copyrightText": "NOASSERTION",
                    "externalRefs": [
                        {
                            "referenceCategory": "PACKAGE-MANAGER",
                            "referenceType": "purl",
                            "referenceLocator": _purl(dependency_name, version),
                        }
                    ],
                },
            )
            relationships.add((parent_id, spdx_id))
            visit(spdx_id, value.get("dependencies"))

    visit(root_id, dependency_tree.get("dependencies"))
    root_package = {
        "SPDXID": root_id,
        "name": package_name,
        "versionInfo": package_version,
        "downloadLocation": "NOASSERTION",
        "filesAnalyzed": False,
        "checksums": [{"algorithm": "SHA256", "checksumValue": package_sha256}],
        "licenseConcluded": package_license,
        "licenseDeclared": package_license,
        "copyrightText": "NOASSERTION",
        "homepage": package_homepage,
        "supplier": "Organization: mss-boot-io",
        "externalRefs": [
            {
                "referenceCategory": "PACKAGE-MANAGER",
                "referenceType": "purl",
                "referenceLocator": _purl(package_name, package_version),
            }
        ],
        "sourceInfo": (
            f"Built from https://github.com/{source_repository}/commit/{source_commit}; "
            f"tarball integrity {artifact.get('integrity', 'NOASSERTION')}"
        ),
    }
    dependency_packages = sorted(
        packages_by_key.values(), key=lambda item: (item["name"], item["versionInfo"], item["SPDXID"])
    )
    relationship_objects = [
        {
            "spdxElementId": "SPDXRef-DOCUMENT",
            "relationshipType": "DESCRIBES",
            "relatedSpdxElement": root_id,
        }
    ]
    relationship_objects.extend(
        {
            "spdxElementId": parent,
            "relationshipType": "DEPENDS_ON",
            "relatedSpdxElement": child,
        }
        for parent, child in sorted(relationships)
    )
    namespace_digest = hashlib.sha256(
        json.dumps(
            {
                "package": package,
                "source": source,
                "dependencies": [item["SPDXID"] for item in dependency_packages],
            },
            sort_keys=True,
            separators=(",", ":"),
        ).encode()
    ).hexdigest()[:24]
    return {
        "spdxVersion": SPDX_VERSION,
        "dataLicense": "CC0-1.0",
        "SPDXID": "SPDXRef-DOCUMENT",
        "name": f"mss-admin-web-{package_version}",
        "documentNamespace": (
            "https://spdx.org/spdxdocs/"
            f"mss-admin-web-{package_version}-{namespace_digest}"
        ),
        "creationInfo": {
            "created": created,
            "creators": [
                "Organization: mss-boot-io",
                "Tool: tools/release/generate_admin_web_sbom.py",
            ],
        },
        "documentComment": (
            f"Source repository {source_repository}; exact commit {source_commit}."
        ),
        "packages": [root_package, *dependency_packages],
        "relationships": relationship_objects,
    }


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--dependency-tree", type=Path, required=True)
    parser.add_argument("--package-evidence", type=Path, required=True)
    parser.add_argument("--source-repository", required=True)
    parser.add_argument("--source-commit", required=True)
    parser.add_argument("--created", required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args(argv)
    try:
        dependency_payload = json.loads(args.dependency_tree.read_text(encoding="utf-8"))
        if not isinstance(dependency_payload, list) or len(dependency_payload) != 1:
            raise SBOMError("pnpm dependency tree must contain exactly one project")
        dependency_tree = dependency_payload[0]
        if not isinstance(dependency_tree, dict):
            raise SBOMError("pnpm dependency tree project must be an object")
        package_evidence = _read_object(args.package_evidence, "package evidence")
        sbom = build_sbom(
            dependency_tree,
            package_evidence,
            source_repository=args.source_repository,
            source_commit=args.source_commit,
            created=args.created,
        )
    except (OSError, json.JSONDecodeError, SBOMError) as exc:
        print(f"Admin Web SBOM rejected: {exc}")
        return 1
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(
        json.dumps(sbom, ensure_ascii=False, indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
    )
    print(f"Admin Web SPDX SBOM generated: {len(sbom['packages'])} packages")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
