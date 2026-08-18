#!/usr/bin/env python3
"""Create the exact-SHA manual decision used by the active release freeze."""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import subprocess
import sys
import tempfile
from pathlib import Path


SCHEMA = "mss.io/release-qualification-decision/v1"
QUALIFICATION_SCHEMA = "mss.io/release-qualification/v1"
RELEASE_FEATURE = ".mss/features/foundation-v1-2-0-release.yaml"
FULL_COMMIT_RE = re.compile(r"^[0-9a-f]{40}$")
PLACEHOLDER_RE = re.compile(r"^(?:none|null|n/a|na|pending|todo|tbd|<.*>)$", re.IGNORECASE)


class QualificationDecisionError(ValueError):
    pass


def _repository_path(root: Path, value: str, *, label: str) -> Path:
    path = (root / value).resolve()
    try:
        path.relative_to(root.resolve())
    except ValueError as exc:
        raise QualificationDecisionError(f"{label} must stay inside the repository") from exc
    return path


def _full_commit(value: str, *, label: str) -> str:
    if not FULL_COMMIT_RE.fullmatch(value):
        raise QualificationDecisionError(f"{label} must be a lowercase full commit SHA")
    return value


def _evidence_reference(value: str, *, label: str) -> str:
    normalized = value.strip()
    if (
        not normalized
        or normalized != value
        or "\n" in normalized
        or "\r" in normalized
        or len(normalized) > 2048
        or PLACEHOLDER_RE.fullmatch(normalized)
    ):
        raise QualificationDecisionError(f"{label} must be a concrete single-line reference")
    return normalized


def _head(root: Path) -> str:
    try:
        result = subprocess.run(
            ["git", "rev-parse", "HEAD"],
            cwd=root,
            check=False,
            capture_output=True,
            text=True,
            encoding="utf-8",
        )
    except OSError as exc:
        raise QualificationDecisionError(f"cannot inspect repository HEAD: {exc}") from exc
    if result.returncode != 0:
        raise QualificationDecisionError("cannot inspect repository HEAD")
    return result.stdout.strip()


def _load_qualification(path: Path, target_version: str) -> tuple[dict[str, object], str]:
    try:
        data = path.read_bytes()
        contract = json.loads(data)
    except (OSError, json.JSONDecodeError) as exc:
        raise QualificationDecisionError("cannot read release qualification contract") from exc
    if not isinstance(contract, dict):
        raise QualificationDecisionError("release qualification contract must be one object")
    if contract.get("schema") != QUALIFICATION_SCHEMA:
        raise QualificationDecisionError("unsupported release qualification schema")
    if contract.get("targetVersion") != target_version:
        raise QualificationDecisionError("qualification target does not match requested version")
    if contract.get("features") != [RELEASE_FEATURE]:
        raise QualificationDecisionError(
            "qualification must select only the active scoped release Feature"
        )
    exclusions = contract.get("excludedFeatures")
    if not isinstance(exclusions, list) or not exclusions:
        raise QualificationDecisionError("qualification carry-forward decisions are required")
    for exclusion in exclusions:
        if not isinstance(exclusion, dict) or not isinstance(exclusion.get("reason"), str):
            raise QualificationDecisionError("qualification exclusion is malformed")
        if not exclusion["reason"].startswith("carry-forward:"):
            raise QualificationDecisionError(
                "every excluded Feature must have an explicit carry-forward decision"
            )
    return contract, hashlib.sha256(data).hexdigest()


def build_decision(
    root: Path,
    *,
    qualification: str,
    target_version: str,
    commit: str,
    phase: str,
    browser_commit: str,
    browser_reference: str,
    blueprint_commit: str,
    blueprint_reference: str,
) -> dict[str, object]:
    root = root.resolve()
    if phase != "feature-freeze":
        raise QualificationDecisionError("manual qualification decision is feature-freeze only")
    commit = _full_commit(commit, label="frozen commit")
    if _head(root) != commit:
        raise QualificationDecisionError("frozen commit does not match repository HEAD")
    if _full_commit(browser_commit, label="browser evidence commit") != commit:
        raise QualificationDecisionError("browser evidence is not bound to the frozen commit")
    if _full_commit(blueprint_commit, label="Blueprint evidence commit") != commit:
        raise QualificationDecisionError("Blueprint evidence is not bound to the frozen commit")
    browser_reference = _evidence_reference(
        browser_reference, label="browser evidence reference"
    )
    blueprint_reference = _evidence_reference(
        blueprint_reference, label="Blueprint evidence reference"
    )
    qualification_path = _repository_path(root, qualification, label="qualification contract")
    _, qualification_digest = _load_qualification(qualification_path, target_version)
    qualification_source = qualification_path.relative_to(root).as_posix()
    return {
        "schema": SCHEMA,
        "targetVersion": target_version,
        "phase": phase,
        "frozenCommit": commit,
        "qualification": {
            "path": qualification_source,
            "sha256": qualification_digest,
        },
        "carryForwardDecision": "accepted",
        "scope": {
            "objectStoreProviderConformanceRequired": False,
            "rustfsRequired": False,
        },
        "evidence": [
            {
                "kind": "codex-in-app-browser",
                "status": "passed",
                "commit": browser_commit,
                "reference": browser_reference,
                "operations": [
                    "navigation",
                    "menu-api-binding",
                    "rbac",
                    "session-management",
                    "settings",
                ],
            },
            {
                "kind": "external-blueprint-upgrade-rehearsal",
                "status": "passed",
                "commit": blueprint_commit,
                "reference": blueprint_reference,
                "assertions": [
                    "customizations-preserved",
                    "second-upgrade-empty",
                    "failed-apply-retains-prior-snapshot",
                ],
            },
        ],
    }


def _write_json_atomic(path: Path, value: dict[str, object]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    payload = json.dumps(value, indent=2, sort_keys=True, ensure_ascii=False) + "\n"
    with tempfile.NamedTemporaryFile(
        mode="w", encoding="utf-8", dir=path.parent, delete=False
    ) as stream:
        stream.write(payload)
        temporary = Path(stream.name)
    temporary.chmod(0o644)
    temporary.replace(path)


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", default=".")
    parser.add_argument("--qualification", default=".mss/release-qualification.json")
    parser.add_argument("--target-version", required=True)
    parser.add_argument("--commit", required=True)
    parser.add_argument("--phase", required=True)
    parser.add_argument("--browser-evidence-commit", required=True)
    parser.add_argument("--browser-evidence-ref", required=True)
    parser.add_argument("--blueprint-evidence-commit", required=True)
    parser.add_argument("--blueprint-evidence-ref", required=True)
    parser.add_argument("--output", required=True)
    return parser


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    root = Path(args.root).resolve()
    output = _repository_path(root, args.output, label="decision output")
    decision = build_decision(
        root,
        qualification=args.qualification,
        target_version=args.target_version,
        commit=args.commit,
        phase=args.phase,
        browser_commit=args.browser_evidence_commit,
        browser_reference=args.browser_evidence_ref,
        blueprint_commit=args.blueprint_evidence_commit,
        blueprint_reference=args.blueprint_evidence_ref,
    )
    _write_json_atomic(output, decision)
    json.dump(
        {
            "success": True,
            "output": output.relative_to(root).as_posix(),
            "commit": decision["frozenCommit"],
        },
        sys.stdout,
        sort_keys=True,
    )
    sys.stdout.write("\n")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except QualificationDecisionError as exc:
        print(f"release qualification decision error: {exc}", file=sys.stderr)
        raise SystemExit(2) from exc
