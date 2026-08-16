#!/usr/bin/env python3
"""Validate the V5 retirement source inventory and emit local cutover evidence."""

from __future__ import annotations

import argparse
import datetime as dt
import hashlib
import json
import subprocess
import sys
from collections import Counter
from pathlib import Path
from typing import Any


DEFAULT_SOURCE = ".mss/inventories/admin-antd-v5-retirement.json"
DEFAULT_OUTPUT = ".mss/reports/admin-antd-v5-retirement-inventory.json"
PATH_FIELDS = (
    "definingSources",
    "runtimeRegistrations",
    "knownConsumers",
    "replacementPaths",
)
REQUIRED_FIELDS = (
    "id",
    "category",
    "decision",
    "gate",
    *PATH_FIELDS,
    "permissions",
    "configurationKeys",
    "databaseMetadata",
    "replacement",
    "rationale",
)


class InventoryError(ValueError):
    """Raised when the retirement inventory is incomplete or unsafe."""


def repository_root() -> Path:
    return Path(__file__).resolve().parents[2]


def confined_path(root: Path, value: str, *, label: str) -> Path:
    if not isinstance(value, str) or not value.strip():
        raise InventoryError(f"{label} must be a non-empty repository-relative path")
    raw = Path(value)
    if raw.is_absolute() or any(part in {"", ".", ".."} for part in raw.parts):
        raise InventoryError(f"{label} must be a normalized repository-relative path: {value!r}")
    candidate = (root / raw).resolve()
    try:
        candidate.relative_to(root)
    except ValueError as error:
        raise InventoryError(f"{label} escapes the repository root: {value!r}") from error
    return candidate


def load_source(root: Path, source_name: str) -> tuple[dict[str, Any], bytes]:
    source_path = confined_path(root, source_name, label="source")
    try:
        raw = source_path.read_bytes()
    except OSError as error:
        raise InventoryError(f"cannot read inventory source {source_name}: {error}") from error
    try:
        document = json.loads(raw)
    except json.JSONDecodeError as error:
        raise InventoryError(f"inventory source is not valid JSON: {error}") from error
    if not isinstance(document, dict):
        raise InventoryError("inventory source must be a JSON object")
    return document, raw


def require_string(candidate: dict[str, Any], field: str, *, item_id: str) -> str:
    value = candidate.get(field)
    if not isinstance(value, str) or not value.strip():
        raise InventoryError(f"candidate {item_id!r} field {field!r} must be a non-empty string")
    return value.strip()


def require_string_list(candidate: dict[str, Any], field: str, *, item_id: str) -> list[str]:
    value = candidate.get(field)
    if not isinstance(value, list) or any(not isinstance(item, str) or not item.strip() for item in value):
        raise InventoryError(f"candidate {item_id!r} field {field!r} must be a string array")
    if len(value) != len(set(value)):
        raise InventoryError(f"candidate {item_id!r} field {field!r} contains duplicates")
    return value


def validate_source(root: Path, document: dict[str, Any]) -> list[dict[str, Any]]:
    if document.get("apiVersion") != "mss.io/v1alpha1" or document.get("kind") != "RetirementInventorySource":
        raise InventoryError("inventory source must be mss.io/v1alpha1 RetirementInventorySource")
    metadata = document.get("metadata")
    spec = document.get("spec")
    if not isinstance(metadata, dict) or metadata.get("name") != "admin-antd-v5-retirement":
        raise InventoryError("inventory metadata identity is invalid")
    if not isinstance(spec, dict):
        raise InventoryError("inventory spec must be an object")
    if spec.get("reportPath") != DEFAULT_OUTPUT:
        raise InventoryError(f"inventory reportPath must remain {DEFAULT_OUTPUT}")
    decisions = spec.get("decisions")
    if decisions != ["remove", "retain", "defer"]:
        raise InventoryError("inventory decisions must be exactly remove, retain, defer")
    candidates = spec.get("candidates")
    if not isinstance(candidates, list) or not candidates:
        raise InventoryError("inventory candidates must be a non-empty array")

    normalized: list[dict[str, Any]] = []
    seen_ids: set[str] = set()
    for index, candidate in enumerate(candidates):
        if not isinstance(candidate, dict):
            raise InventoryError(f"candidate at index {index} must be an object")
        missing = [field for field in REQUIRED_FIELDS if field not in candidate]
        if missing:
            raise InventoryError(f"candidate at index {index} is missing fields: {', '.join(missing)}")
        item_id = require_string(candidate, "id", item_id=f"index-{index}")
        if item_id in seen_ids:
            raise InventoryError(f"duplicate candidate id: {item_id}")
        seen_ids.add(item_id)
        decision = require_string(candidate, "decision", item_id=item_id)
        if decision not in decisions:
            raise InventoryError(f"candidate {item_id!r} has unsupported decision {decision!r}")
        for field in ("category", "gate", "replacement", "rationale"):
            require_string(candidate, field, item_id=item_id)
        for field in PATH_FIELDS + ("permissions", "configurationKeys", "databaseMetadata"):
            values = require_string_list(candidate, field, item_id=item_id)
            if field in PATH_FIELDS:
                for value in values:
                    evidence_path = confined_path(root, value, label=f"{item_id}.{field}")
                    if not evidence_path.exists():
                        raise InventoryError(
                            f"candidate {item_id!r} references missing {field} path: {value}"
                        )
        if not candidate["definingSources"]:
            raise InventoryError(f"candidate {item_id!r} must identify at least one defining source")
        if not candidate["replacementPaths"]:
            raise InventoryError(f"candidate {item_id!r} must identify at least one replacement or retained path")
        normalized.append(candidate)
    return sorted(normalized, key=lambda item: item["id"])


def git_output(root: Path, *args: str) -> str:
    result = subprocess.run(
        ["git", *args],
        cwd=root,
        check=True,
        capture_output=True,
        text=True,
    )
    return result.stdout.strip()


def create_report(
    root: Path,
    source_name: str,
    source_raw: bytes,
    candidates: list[dict[str, Any]],
) -> dict[str, Any]:
    counts = Counter(candidate["decision"] for candidate in candidates)
    tracked_status = git_output(root, "status", "--porcelain", "--untracked-files=no")
    return {
        "apiVersion": "mss.io/v1alpha1",
        "kind": "RetirementInventoryReport",
        "metadata": {
            "name": "admin-antd-v5-retirement",
            "generatedAt": dt.datetime.now(dt.timezone.utc).isoformat().replace("+00:00", "Z"),
            "source": source_name,
            "sourceSha256": hashlib.sha256(source_raw).hexdigest(),
            "sourceCommit": git_output(root, "rev-parse", "HEAD"),
            "trackedWorktreeClean": tracked_status == "",
        },
        "summary": {
            "total": len(candidates),
            "remove": counts["remove"],
            "retain": counts["retain"],
            "defer": counts["defer"],
            "readyForDeletion": False,
            "reason": "V5 deletion remains blocked until the V6 default PR is merged and the observation and owner-confirmation gates complete.",
        },
        "candidates": candidates,
    }


def write_report(root: Path, output_name: str, report: dict[str, Any]) -> None:
    if output_name != DEFAULT_OUTPUT:
        raise InventoryError(f"output must remain {DEFAULT_OUTPUT}")
    output_path = confined_path(root, output_name, label="output")
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source", default=DEFAULT_SOURCE)
    parser.add_argument("--output", default=DEFAULT_OUTPUT)
    parser.add_argument(
        "--check",
        action="store_true",
        help="validate source and evidence paths without writing the report",
    )
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv or sys.argv[1:])
    root = repository_root()
    try:
        document, source_raw = load_source(root, args.source)
        candidates = validate_source(root, document)
        report = create_report(root, args.source, source_raw, candidates)
        if not args.check:
            write_report(root, args.output, report)
    except (InventoryError, OSError, subprocess.CalledProcessError) as error:
        print(f"retirement inventory failed: {error}", file=sys.stderr)
        return 1
    summary = report["summary"]
    mode = "validated" if args.check else f"wrote {args.output}"
    print(
        f"{mode}: {summary['total']} candidates "
        f"(remove={summary['remove']}, retain={summary['retain']}, defer={summary['defer']})"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
