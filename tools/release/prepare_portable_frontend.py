#!/usr/bin/env python3
"""Normalize Umi/Dumi static exports and prove the result is portable."""

from __future__ import annotations

import argparse
import re
import shutil
import sys
from pathlib import Path


SCRIPT_DIRECTORY = Path(__file__).resolve().parent
if str(SCRIPT_DIRECTORY) not in sys.path:
    sys.path.insert(0, str(SCRIPT_DIRECTORY))

from check_portable_paths import check_paths  # noqa: E402


DYNAMIC_ROUTE_COMPONENT = re.compile(r"^:[A-Za-z_][A-Za-z0-9_]*$")
REPOSITORY_ROOT = SCRIPT_DIRECTORY.parents[1]


class PreparationError(ValueError):
    pass


def prepare_frontend(
    distribution: Path, *, allowed_root: Path = REPOSITORY_ROOT
) -> list[str]:
    allowed_root = allowed_root.resolve(strict=True)
    distribution = distribution.resolve(strict=True)
    if not distribution.is_dir():
        raise PreparationError(f"static distribution is not a directory: {distribution}")
    try:
        relative_distribution = distribution.relative_to(allowed_root)
    except ValueError as error:
        raise PreparationError(
            f"refusing to modify a distribution outside {allowed_root}: {distribution}"
        ) from error
    if not relative_distribution.parts or distribution.name != "dist":
        raise PreparationError(
            "refusing to modify a path that is not a nested dist directory: "
            f"{distribution}"
        )

    dynamic_directories = sorted(
        (
            path
            for path in distribution.rglob("*")
            if path.is_dir() and DYNAMIC_ROUTE_COMPONENT.fullmatch(path.name)
        ),
        key=lambda path: path.relative_to(distribution).as_posix(),
    )
    removal_plan: list[tuple[Path, str]] = []
    for directory in dynamic_directories:
        members = sorted(
            path.relative_to(directory).as_posix()
            for path in directory.rglob("*")
        )
        index = directory / "index.html"
        if (
            directory.is_symlink()
            or members != ["index.html"]
            or not index.is_file()
            or index.is_symlink()
        ):
            raise PreparationError(
                "dynamic export placeholder contains unexpected members: "
                f"{directory.relative_to(distribution).as_posix()}: {members}"
            )
        relative = directory.relative_to(distribution).as_posix()
        removal_plan.append((directory, relative))

    removed: list[str] = []
    for directory, relative in removal_plan:
        shutil.rmtree(directory)
        removed.append(relative)

    issues = check_paths([distribution])
    if issues:
        details = "; ".join(f"{issue.member}: {issue.reason}" for issue in issues)
        raise PreparationError(f"frontend distribution is not portable: {details}")

    distribution.chmod(0o755)
    for path in distribution.rglob("*"):
        path.chmod(0o755 if path.is_dir() else 0o644)
    return removed


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description=(
            "Remove Umi/Dumi literal dynamic-route HTML placeholders and reject "
            "every remaining cross-platform-incompatible path."
        )
    )
    parser.add_argument("distribution", type=Path)
    parser.add_argument(
        "--allowed-root",
        type=Path,
        default=REPOSITORY_ROOT,
        help="root that must contain the supplied dist directory",
    )
    return parser


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    try:
        removed = prepare_frontend(args.distribution, allowed_root=args.allowed_root)
    except (OSError, PreparationError) as error:
        print(f"portable static preparation failed: {error}", file=sys.stderr)
        return 1
    print(f"Removed {len(removed)} Umi dynamic-route placeholder directorie(s)")
    for path in removed:
        print(f"  {path}")
    print("Portable static distribution check passed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
