#!/usr/bin/env python3
"""Classify a GitHub change into one independently owned component scope."""

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
from pathlib import Path, PurePosixPath


SHA_RE = re.compile(r"^[0-9a-f]{40}$")
ZERO_SHA = "0" * 40
ROOT_SCOPES = {
    "admin": "admin",
    "docs": "docs",
    "mss-boot": "framework",
    "web": "web",
}
ALL_GO_MODULES = [
    "admin",
    "mss-boot",
    "cmd/tools/pr",
    "compose/consul",
    "compose/kafka",
]


class ScopeError(ValueError):
    pass


def component_scope(paths: list[str]) -> str:
    normalized = [PurePosixPath(path) for path in paths if path]
    if not normalized:
        return "shared"
    scopes = {
        ROOT_SCOPES.get(path.parts[0], "shared")
        for path in normalized
        if path.parts
    }
    if len(scopes) == 1:
        return scopes.pop()
    return "shared"


def go_modules_for_scope(scope: str) -> list[str]:
    if scope == "admin":
        return ["admin"]
    if scope == "framework":
        return ["mss-boot"]
    if scope in {"docs", "web"}:
        return ["none"]
    return ALL_GO_MODULES


def changed_paths(
    repository: Path, base: str, head: str, *, merge_base: bool = False
) -> list[str]:
    for label, value in (("base", base), ("head", head)):
        if not SHA_RE.fullmatch(value):
            raise ScopeError(f"{label} must be a lowercase full commit SHA")
    if base == ZERO_SHA:
        return []

    comparison = [f"{base}...{head}"] if merge_base else [base, head]
    result = subprocess.run(
        [
            "git",
            "-C",
            str(repository),
            "diff",
            "--name-only",
            "-z",
            *comparison,
            "--",
        ],
        check=False,
        capture_output=True,
    )
    if result.returncode != 0:
        detail = result.stderr.decode("utf-8", errors="replace").strip()
        raise ScopeError(f"git diff failed: {detail or 'no diagnostic output'}")
    return sorted(
        path
        for path in result.stdout.decode("utf-8").split("\0")
        if path
    )


def classify_event(
    repository: Path, event_name: str, base: str, head: str
) -> tuple[str, list[str]]:
    if event_name not in {"pull_request", "push"} or base == ZERO_SHA:
        return "shared", []
    paths = changed_paths(
        repository,
        base,
        head,
        merge_base=event_name == "pull_request",
    )
    return component_scope(paths), paths


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--repository", type=Path, default=Path("."))
    parser.add_argument("--event-name", required=True)
    parser.add_argument("--base", default=ZERO_SHA)
    parser.add_argument("--head", default=ZERO_SHA)
    parser.add_argument("--github-output", type=Path)
    args = parser.parse_args(argv)

    try:
        scope, paths = classify_event(
            args.repository, args.event_name, args.base, args.head
        )
    except ScopeError as exc:
        print(f"component scope rejected: {exc}", file=sys.stderr)
        return 1

    go_modules = go_modules_for_scope(scope)
    payload = {
        "scope": scope,
        "changedCount": len(paths),
        "goModules": go_modules,
        "paths": paths,
    }
    print(json.dumps(payload, ensure_ascii=False, sort_keys=True))
    if args.github_output is not None:
        with args.github_output.open("a", encoding="utf-8") as output:
            output.write(f"scope={scope}\n")
            output.write(f"changed_count={len(paths)}\n")
            output.write(f"go_modules={json.dumps(go_modules, separators=(',', ':'))}\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
