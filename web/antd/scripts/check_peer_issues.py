#!/usr/bin/env python3
"""Compare pnpm strict-peer diagnostics with the reviewed compatibility policy."""

from __future__ import annotations

import argparse
import json
import pathlib
import sys
from typing import Any, Iterable


ERROR_CODE = "ERR_PNPM_PEER_DEP_ISSUES"


def load_json(path: pathlib.Path) -> dict[str, Any]:
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except OSError as error:
        raise ValueError(f"read {path}: {error}") from error
    except json.JSONDecodeError as error:
        raise ValueError(f"parse {path}: {error}") from error
    if not isinstance(value, dict):
        raise ValueError(f"{path} must contain a JSON object")
    return value


def load_peer_error(path: pathlib.Path) -> dict[str, Any]:
    error_record: dict[str, Any] | None = None
    try:
        lines = path.read_text(encoding="utf-8", errors="replace").splitlines()
    except OSError as error:
        raise ValueError(f"read pnpm report {path}: {error}") from error
    for line_number, raw in enumerate(lines, start=1):
        line = raw.strip()
        if not line:
            continue
        try:
            record = json.loads(line)
        except json.JSONDecodeError:
            continue
        if not isinstance(record, dict):
            continue
        if record.get("code") == ERROR_CODE:
            error_record = record
    if error_record is None:
        raise ValueError(
            f"{path} does not contain pnpm error {ERROR_CODE}; "
            "remove stale peer exceptions when strict resolution becomes clean"
        )
    return error_record


def parent_chain(issue: dict[str, Any]) -> list[str]:
    result: list[str] = []
    parents = issue.get("parents", [])
    if not isinstance(parents, list):
        raise ValueError("peer issue parents must be an array")
    for parent in parents:
        if not isinstance(parent, dict):
            raise ValueError("peer issue parent must be an object")
        name = parent.get("name")
        version = parent.get("version")
        if not isinstance(name, str) or not isinstance(version, str):
            raise ValueError("peer issue parent must contain string name/version")
        result.append(f"{name}@{version}")
    return result


def canonical_bad(issues: dict[str, Any]) -> list[dict[str, Any]]:
    result: list[dict[str, Any]] = []
    bad = issues.get("bad", {})
    if not isinstance(bad, dict):
        raise ValueError("pnpm bad peer issues must be an object")
    for dependency, entries in bad.items():
        if not isinstance(dependency, str) or not isinstance(entries, list):
            raise ValueError("invalid bad peer issue structure")
        for entry in entries:
            if not isinstance(entry, dict):
                raise ValueError("bad peer issue entry must be an object")
            result.append(
                {
                    "dependency": dependency,
                    "foundVersion": entry.get("foundVersion"),
                    "wantedRange": entry.get("wantedRange"),
                    "parents": parent_chain(entry),
                }
            )
    result.sort(
        key=lambda item: (
            str(item["dependency"]),
            str(item["wantedRange"]),
            tuple(item["parents"]),
        )
    )
    return result


def canonical_missing_optional(issues: dict[str, Any]) -> list[str]:
    missing = issues.get("missing", {})
    if not isinstance(missing, dict):
        raise ValueError("pnpm missing peer issues must be an object")
    result: list[str] = []
    for dependency, entries in missing.items():
        if not isinstance(dependency, str) or not isinstance(entries, list):
            raise ValueError("invalid missing peer issue structure")
        for entry in entries:
            if not isinstance(entry, dict):
                raise ValueError("missing peer issue entry must be an object")
            if entry.get("optional") is not True:
                raise ValueError(
                    f"missing required peer {dependency!r} is not an allowed optional adapter"
                )
        result.append(dependency)
    return sorted(result)


def extract_project_issues(record: dict[str, Any]) -> dict[str, Any]:
    projects = record.get("issuesByProjects")
    if not isinstance(projects, dict):
        raise ValueError("pnpm peer error has no issuesByProjects object")
    unexpected_projects = sorted(project for project in projects if project != ".")
    if unexpected_projects:
        raise ValueError(
            "peer policy currently covers only the frontend root project; "
            f"unexpected project entries: {unexpected_projects}"
        )
    issues = projects.get(".")
    if not isinstance(issues, dict):
        raise ValueError("pnpm peer error has no frontend root project entry")
    return issues


def normalize_policy(policy: dict[str, Any]) -> tuple[list[dict[str, Any]], list[str]]:
    if policy.get("version") != 1:
        raise ValueError("peer policy version must equal 1")
    bad = policy.get("allowedBad")
    missing = policy.get("allowedMissingOptional")
    if not isinstance(bad, list) or not isinstance(missing, list):
        raise ValueError("peer policy must contain allowedBad and allowedMissingOptional arrays")
    normalized_bad: list[dict[str, Any]] = []
    for entry in bad:
        if not isinstance(entry, dict):
            raise ValueError("allowedBad entries must be objects")
        parents = entry.get("parents")
        if not isinstance(parents, list) or not all(isinstance(value, str) for value in parents):
            raise ValueError("allowedBad parents must be an array of strings")
        normalized_bad.append(
            {
                "dependency": entry.get("dependency"),
                "foundVersion": entry.get("foundVersion"),
                "wantedRange": entry.get("wantedRange"),
                "parents": list(parents),
            }
        )
    normalized_bad.sort(
        key=lambda item: (
            str(item["dependency"]),
            str(item["wantedRange"]),
            tuple(item["parents"]),
        )
    )
    if not all(isinstance(value, str) for value in missing):
        raise ValueError("allowedMissingOptional entries must be strings")
    return normalized_bad, sorted(set(missing))


def render_difference(label: str, expected: object, actual: object) -> str:
    return (
        f"{label} changed:\n"
        f"expected={json.dumps(expected, indent=2, ensure_ascii=False, sort_keys=True)}\n"
        f"actual={json.dumps(actual, indent=2, ensure_ascii=False, sort_keys=True)}"
    )


def validate(report: pathlib.Path, policy_path: pathlib.Path) -> None:
    policy_bad, policy_missing = normalize_policy(load_json(policy_path))
    issues = extract_project_issues(load_peer_error(report))
    actual_bad = canonical_bad(issues)
    actual_missing = canonical_missing_optional(issues)

    failures: list[str] = []
    if actual_bad != policy_bad:
        failures.append(render_difference("incompatible peer ranges", policy_bad, actual_bad))
    if actual_missing != policy_missing:
        failures.append(
            render_difference("missing optional peer names", policy_missing, actual_missing)
        )
    if failures:
        raise ValueError("\n\n".join(failures))

    print(
        f"peer policy passed: {len(actual_bad)} reviewed range exceptions, "
        f"{len(actual_missing)} reviewed optional adapters"
    )


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--report", required=True, type=pathlib.Path)
    parser.add_argument("--policy", required=True, type=pathlib.Path)
    return parser


def main(argv: Iterable[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    try:
        validate(args.report, args.policy)
    except ValueError as error:
        print(f"pnpm peer policy failure: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
