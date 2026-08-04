#!/usr/bin/env python3
"""Validate Go coverprofiles against component and package floors.

The parser intentionally works directly on Go's coverprofile format so the same
policy can be used by independent modules without installing an extra tool.
"""

from __future__ import annotations

import argparse
import json
import pathlib
import sys
from dataclasses import dataclass
from typing import Iterable


@dataclass(frozen=True)
class Coverage:
    statements: int = 0
    covered: int = 0

    @property
    def percent(self) -> float:
        if self.statements == 0:
            return 100.0
        return 100.0 * self.covered / self.statements

    def plus(self, other: "Coverage") -> "Coverage":
        return Coverage(
            statements=self.statements + other.statements,
            covered=self.covered + other.covered,
        )


def parse_profile(path: pathlib.Path) -> tuple[Coverage, dict[str, Coverage]]:
    total = Coverage()
    packages: dict[str, Coverage] = {}
    try:
        lines = path.read_text(encoding="utf-8").splitlines()
    except OSError as error:
        raise ValueError(f"read coverage profile {path}: {error}") from error

    if not lines or not lines[0].startswith("mode:"):
        raise ValueError(f"{path} is not a Go coverprofile")

    for line_number, raw in enumerate(lines[1:], start=2):
        line = raw.strip()
        if not line:
            continue
        try:
            location, statement_count, execution_count = line.rsplit(" ", 2)
            filename, _ = location.split(":", 1)
            statements = int(statement_count)
            executions = int(execution_count)
        except (ValueError, TypeError) as error:
            raise ValueError(
                f"invalid coverprofile row {path}:{line_number}: {raw!r}"
            ) from error
        if statements < 0 or executions < 0:
            raise ValueError(
                f"negative coverprofile value {path}:{line_number}: {raw!r}"
            )
        coverage = Coverage(
            statements=statements,
            covered=statements if executions > 0 else 0,
        )
        total = total.plus(coverage)
        package = filename.rsplit("/", 1)[0] if "/" in filename else "."
        packages[package] = packages.get(package, Coverage()).plus(coverage)

    return total, packages


def load_policy(path: pathlib.Path, component: str) -> dict[str, object]:
    try:
        document = json.loads(path.read_text(encoding="utf-8"))
    except OSError as error:
        raise ValueError(f"read coverage policy {path}: {error}") from error
    except json.JSONDecodeError as error:
        raise ValueError(f"parse coverage policy {path}: {error}") from error

    components = document.get("components")
    if not isinstance(components, dict):
        raise ValueError(f"coverage policy {path} must contain an object 'components'")
    policy = components.get(component)
    if not isinstance(policy, dict):
        raise ValueError(
            f"coverage policy {path} has no component {component!r}"
        )
    return policy


def require_number(value: object, label: str) -> float:
    if isinstance(value, bool) or not isinstance(value, (int, float)):
        raise ValueError(f"{label} must be a number")
    result = float(value)
    if result < 0 or result > 100:
        raise ValueError(f"{label} must be in the range 0..100")
    return result


def matching_coverage(
    packages: dict[str, Coverage], package_pattern: str
) -> Coverage:
    """Return coverage for an exact package or a `...` package subtree."""
    if package_pattern.endswith("/..."):
        prefix = package_pattern[:-4].rstrip("/")
        matched = Coverage()
        for package, coverage in packages.items():
            if package == prefix or package.startswith(prefix + "/"):
                matched = matched.plus(coverage)
        return matched
    return packages.get(package_pattern, Coverage())


def print_result(label: str, coverage: Coverage, floor: float) -> bool:
    passed = coverage.statements > 0 and coverage.percent + 1e-9 >= floor
    status = "PASS" if passed else "FAIL"
    print(
        f"[{status}] {label}: {coverage.percent:.2f}% "
        f"({coverage.covered}/{coverage.statements} statements), floor {floor:.2f}%"
    )
    return passed


def validate(
    total: Coverage,
    packages: dict[str, Coverage],
    component: str,
    policy: dict[str, object],
) -> bool:
    passed = print_result(
        component,
        total,
        require_number(policy.get("minimum"), f"{component}.minimum"),
    )

    package_floors = policy.get("packages", {})
    if not isinstance(package_floors, dict):
        raise ValueError(f"{component}.packages must be an object")
    for package in sorted(package_floors):
        floor = require_number(
            package_floors[package], f"{component}.packages[{package!r}]"
        )
        coverage = matching_coverage(packages, package)
        passed = print_result(package, coverage, floor) and passed
    return passed


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--profile", required=True, type=pathlib.Path)
    parser.add_argument("--policy", required=True, type=pathlib.Path)
    parser.add_argument("--component", required=True)
    parser.add_argument(
        "--summary",
        action="store_true",
        help="print all package coverage values before applying the policy",
    )
    return parser


def main(argv: Iterable[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    try:
        total, packages = parse_profile(args.profile)
        policy = load_policy(args.policy, args.component)
        if args.summary:
            print(f"component {args.component}: {total.percent:.2f}%")
            for package in sorted(packages):
                coverage = packages[package]
                print(
                    f"  {package}: {coverage.percent:.2f}% "
                    f"({coverage.covered}/{coverage.statements})"
                )
        return 0 if validate(total, packages, args.component, policy) else 1
    except ValueError as error:
        print(f"coverage gate configuration error: {error}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
