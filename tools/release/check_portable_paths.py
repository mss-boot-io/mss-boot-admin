#!/usr/bin/env python3
"""Reject artifact paths that cannot round-trip across supported filesystems."""

from __future__ import annotations

import argparse
import os
import re
import sys
import tarfile
import unicodedata
import zipfile
from dataclasses import dataclass
from pathlib import Path
from typing import Iterable


INVALID_WINDOWS_CHARACTERS = frozenset('<>:"\\|?*')
WINDOWS_DEVICE_NAME = re.compile(
    r"^(?:CON|PRN|AUX|NUL|CONIN\$|CONOUT\$|COM[1-9¹²³]|LPT[1-9¹²³])(?:\..*)?$",
    re.IGNORECASE,
)


@dataclass(frozen=True)
class PathIssue:
    source: str
    member: str
    reason: str


def _portable_parts(member: str) -> tuple[list[str], list[str]]:
    reasons: list[str] = []
    if not member:
        return [], ["path is empty"]
    if member.startswith(("/", "\\")) or re.match(r"^[A-Za-z]:", member):
        reasons.append("path is absolute")
    if "\\" in member:
        reasons.append("path uses a backslash separator")

    candidate = member.rstrip("/")
    raw_parts = candidate.split("/")
    while raw_parts and raw_parts[0] == ".":
        raw_parts.pop(0)
    if not raw_parts:
        return [], reasons

    for part in raw_parts:
        if part in ("", ".", ".."):
            reasons.append(f"path contains unsafe component {part!r}")
            continue
        if any(ord(character) < 32 or ord(character) == 127 for character in part):
            reasons.append(f"component {part!r} contains a control character")
        invalid = sorted(set(part) & INVALID_WINDOWS_CHARACTERS)
        if invalid:
            reasons.append(
                f"component {part!r} contains forbidden character(s) {''.join(invalid)!r}"
            )
        if part.endswith((" ", ".")):
            reasons.append(f"component {part!r} ends with a space or period")
        if WINDOWS_DEVICE_NAME.fullmatch(part):
            reasons.append(f"component {part!r} is a reserved Windows device name")
        try:
            encoded_part = part.encode("utf-8")
        except UnicodeEncodeError:
            reasons.append(f"component {part!r} is not valid Unicode")
        else:
            if len(encoded_part) > 255:
                reasons.append(f"component {part!r} exceeds 255 UTF-8 bytes")
    return raw_parts, reasons


def _inspect_members(source: str, members: Iterable[str]) -> list[PathIssue]:
    issues: list[PathIssue] = []
    seen: dict[str, str] = {}
    seen_exact: set[str] = set()
    for member in members:
        parts, reasons = _portable_parts(member)
        normalized = "/".join(parts)
        for reason in reasons:
            issues.append(PathIssue(source, member, reason))
        if not normalized:
            continue
        if normalized in seen_exact:
            issues.append(PathIssue(source, member, "path is duplicated in the artifact"))
        else:
            seen_exact.add(normalized)
        collision_key = unicodedata.normalize("NFC", normalized).casefold()
        previous = seen.setdefault(collision_key, normalized)
        if previous != normalized:
            issues.append(
                PathIssue(
                    source,
                    member,
                    f"path collides with {previous!r} on a case-insensitive filesystem",
                )
            )
    return issues


def _directory_members(root: Path) -> tuple[list[str], list[PathIssue]]:
    members: list[str] = []
    issues: list[PathIssue] = []
    for current_root, directory_names, file_names in os.walk(root, followlinks=False):
        current = Path(current_root)
        for name in sorted(directory_names + file_names):
            path = current / name
            relative = path.relative_to(root).as_posix()
            members.append(relative)
            if path.is_symlink():
                issues.append(
                    PathIssue(str(root), relative, "symbolic links are not portable release members")
                )
    return members, issues


def inspect_path(path: Path) -> list[PathIssue]:
    path = path.resolve(strict=True)
    if path.is_dir():
        members, issues = _directory_members(path)
        return issues + _inspect_members(str(path), members)

    if zipfile.is_zipfile(path):
        with zipfile.ZipFile(path) as archive:
            issues = _inspect_members(str(path), (member.filename for member in archive.infolist()))
            for member in archive.infolist():
                unix_mode = (member.external_attr >> 16) & 0o170000
                if unix_mode == 0o120000:
                    issues.append(
                        PathIssue(str(path), member.filename, "ZIP symbolic links are not portable")
                    )
            return issues

    if tarfile.is_tarfile(path):
        with tarfile.open(path, mode="r:*") as archive:
            members = archive.getmembers()
            issues = _inspect_members(str(path), (member.name for member in members))
            for member in members:
                if member.issym() or member.islnk():
                    issues.append(
                        PathIssue(str(path), member.name, "tar links are not portable release members")
                    )
                elif not (member.isfile() or member.isdir()):
                    issues.append(
                        PathIssue(
                            str(path),
                            member.name,
                            "tar special files are not portable release members",
                        )
                    )
            return issues

    return _inspect_members(str(path), [path.name])


def check_paths(paths: Iterable[Path], *, allow_missing: bool = False) -> list[PathIssue]:
    issues: list[PathIssue] = []
    for path in paths:
        if not path.exists():
            if allow_missing:
                continue
            issues.append(PathIssue(str(path), str(path), "input path does not exist"))
            continue
        issues.extend(inspect_path(path))
    return issues


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description=(
            "Validate directories and ZIP/TAR artifacts for Windows, macOS, Linux, "
            "and GitHub artifact path portability."
        )
    )
    parser.add_argument(
        "--allow-missing",
        action="store_true",
        help="Skip missing inputs while still validating every input that exists.",
    )
    parser.add_argument("paths", nargs="+", type=Path)
    return parser


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    issues = check_paths(args.paths, allow_missing=args.allow_missing)
    if issues:
        for issue in issues:
            print(
                f"non-portable artifact path: {issue.source}: {issue.member}: {issue.reason}",
                file=sys.stderr,
            )
        return 1
    print(f"Portable artifact path check passed for {len(args.paths)} input(s)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
