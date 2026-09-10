#!/usr/bin/env python3
"""Validate a replaceable Docs deployment against the public site identity."""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path
from typing import NamedTuple


DOCS_VERSION_RE = re.compile(
    r"^v(?P<major>0|[1-9][0-9]*)\."
    r"(?P<minor>0|[1-9][0-9]*)\."
    r"(?P<patch>0|[1-9][0-9]*)"
    r"(?:\+docs\.(?P<revision>[1-9][0-9]*))?$"
)
SHA_RE = re.compile(r"^[0-9a-f]{40}$")
MAX_DOCS_REVISION = 999


class DeploymentStateError(ValueError):
    pass


class DocsIdentity(NamedTuple):
    version: str
    commit: str
    base: tuple[int, int, int]
    revision: int


def parse_version(version: str, commit: str) -> DocsIdentity:
    match = DOCS_VERSION_RE.fullmatch(version)
    if match is None:
        raise DeploymentStateError(
            f"Docs version must use vX.Y.Z or vX.Y.Z+docs.N: {version!r}"
        )
    if SHA_RE.fullmatch(commit) is None:
        raise DeploymentStateError("Docs commit must be one lowercase full SHA")
    revision = int(match.group("revision") or "0")
    if revision > MAX_DOCS_REVISION:
        raise DeploymentStateError(
            f"Docs revision exceeds maximum supported value {MAX_DOCS_REVISION}"
        )
    return DocsIdentity(
        version=version,
        commit=commit,
        base=tuple(int(match.group(name)) for name in ("major", "minor", "patch")),
        revision=revision,
    )


def load_current_identity(path: Path) -> DocsIdentity:
    try:
        payload = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise DeploymentStateError(
            f"cannot read current production Docs identity {path}: {exc}"
        ) from exc
    if not isinstance(payload, dict) or payload.get("application") != "mss-boot-docs":
        raise DeploymentStateError(
            "current production identity must belong to mss-boot-docs"
        )
    version = payload.get("version")
    commit = payload.get("commit")
    if not isinstance(version, str) or not isinstance(commit, str):
        raise DeploymentStateError(
            "current production identity must contain string version and commit"
        )
    return parse_version(version, commit)


def parse_requested_version(version: str, commit: str) -> DocsIdentity:
    identity = parse_version(version, commit)
    if identity.revision != 0:
        raise DeploymentStateError(
            "new Docs deployments must reuse the stable docs/vX.Y.Z identity"
        )
    return identity


def deployment_action(requested: DocsIdentity, current: DocsIdentity) -> str:
    if requested.revision != 0:
        raise DeploymentStateError(
            "new Docs deployments must reuse the stable docs/vX.Y.Z identity"
        )
    if requested.version == current.version and requested.commit == current.commit:
        return "current"

    if requested.base < current.base:
        raise DeploymentStateError(
            f"Docs deployment would roll back product version {current.version} "
            f"to {requested.version}"
        )
    if requested.base > current.base:
        return "deploy"
    return "deploy"


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--current", type=Path, required=True)
    parser.add_argument("--requested-version", required=True)
    parser.add_argument("--requested-commit", required=True)
    args = parser.parse_args(argv)
    try:
        current = load_current_identity(args.current)
        requested = parse_requested_version(
            args.requested_version, args.requested_commit
        )
        action = deployment_action(requested, current)
    except DeploymentStateError as exc:
        print(f"Docs deployment state rejected: {exc}", file=sys.stderr)
        return 1
    print(action)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
