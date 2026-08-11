#!/usr/bin/env python3
"""Create and verify exact-run release-readiness attestations."""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import sys
from pathlib import Path
from urllib.parse import urlsplit

from check_release_policy import PolicyError, load_policy


SCHEMA = "mss.io/release-readiness-attestation/v1"
PHASES = ("checkpoint", "feature-freeze", "pre-framework", "pre-root")
PUBLICATION_PHASES = frozenset(("pre-framework", "pre-root"))
REQUIRED_KEYS = frozenset(
    (
        "schema",
        "targetVersion",
        "commit",
        "phase",
        "policySha256",
        "workflowRunId",
        "workflowRunUrl",
        "publicationAuthority",
    )
)
FULL_COMMIT_RE = re.compile(r"^[0-9a-f]{40}$")
SHA256_RE = re.compile(r"^[0-9a-f]{64}$")


class AttestationError(ValueError):
    pass


def policy_sha256(path: Path) -> str:
    try:
        return hashlib.sha256(path.read_bytes()).hexdigest()
    except OSError as exc:
        raise AttestationError(f"cannot read release policy {path}: {exc}") from exc


def _validate_run_url(value: str, run_id: int) -> None:
    parsed = urlsplit(value)
    expected_suffix = f"/actions/runs/{run_id}"
    if (
        parsed.scheme not in {"http", "https"}
        or not parsed.netloc
        or not parsed.path.endswith(expected_suffix)
        or parsed.query
        or parsed.fragment
    ):
        raise AttestationError(
            f"workflowRunUrl must identify workflow run {run_id} without query or fragment"
        )


def _validate_publication_authority(
    policy: dict[str, str | bool], phase: str, publication_authority: bool
) -> None:
    ready = policy["publicationWorkflowsReady"] is True
    if not ready and phase != "checkpoint":
        raise AttestationError(
            "publicationWorkflowsReady=false permits only checkpoint attestations"
        )
    if not ready and publication_authority:
        raise AttestationError(
            "publicationWorkflowsReady=false cannot grant publication authority"
        )
    if publication_authority and phase not in PUBLICATION_PHASES:
        raise AttestationError(
            "publication authority is valid only for pre-framework or pre-root"
        )


def build_attestation(
    *,
    policy_path: Path,
    target_version: str,
    commit: str,
    phase: str,
    workflow_run_id: int,
    workflow_run_url: str,
    publication_authority: bool,
) -> dict[str, object]:
    try:
        policy = load_policy(policy_path)
    except PolicyError as exc:
        raise AttestationError(str(exc)) from exc
    if target_version != policy["nextPublicVersion"]:
        raise AttestationError(
            f"targetVersion {target_version!r} does not match release policy "
            f"{policy['nextPublicVersion']!r}"
        )
    if not FULL_COMMIT_RE.fullmatch(commit):
        raise AttestationError("commit must be a lowercase full commit SHA")
    if phase not in PHASES:
        raise AttestationError(f"unsupported readiness phase: {phase}")
    if isinstance(workflow_run_id, bool) or workflow_run_id <= 0:
        raise AttestationError("workflowRunId must be a positive integer")
    _validate_run_url(workflow_run_url, workflow_run_id)
    if not isinstance(publication_authority, bool):
        raise AttestationError("publicationAuthority must be a boolean")
    _validate_publication_authority(policy, phase, publication_authority)

    return {
        "schema": SCHEMA,
        "targetVersion": target_version,
        "commit": commit,
        "phase": phase,
        "policySha256": policy_sha256(policy_path),
        "workflowRunId": workflow_run_id,
        "workflowRunUrl": workflow_run_url,
        "publicationAuthority": publication_authority,
    }


def _reject_duplicate_keys(pairs: list[tuple[str, object]]) -> dict[str, object]:
    result: dict[str, object] = {}
    for key, value in pairs:
        if key in result:
            raise AttestationError(f"duplicate attestation key: {key}")
        result[key] = value
    return result


def load_attestation(path: Path) -> dict[str, object]:
    try:
        raw = path.read_text(encoding="utf-8")
    except OSError as exc:
        raise AttestationError(f"cannot read readiness attestation {path}: {exc}") from exc
    try:
        value = json.loads(
            raw,
            object_pairs_hook=_reject_duplicate_keys,
            parse_constant=lambda constant: (_ for _ in ()).throw(
                AttestationError(f"invalid JSON constant: {constant}")
            ),
        )
    except (json.JSONDecodeError, UnicodeError) as exc:
        raise AttestationError(f"invalid readiness attestation JSON: {exc}") from exc
    if not isinstance(value, dict):
        raise AttestationError("readiness attestation must be a JSON object")
    return value


def validate_attestation(
    attestation: dict[str, object],
    *,
    policy_path: Path,
    target_version: str,
    commit: str,
    phase: str,
    workflow_run_id: int,
    workflow_run_url: str,
    intent: str,
) -> None:
    try:
        policy = load_policy(policy_path)
    except PolicyError as exc:
        raise AttestationError(str(exc)) from exc

    keys = set(attestation)
    missing = sorted(REQUIRED_KEYS - keys)
    extra = sorted(keys - REQUIRED_KEYS)
    if missing:
        raise AttestationError(f"attestation is missing keys: {', '.join(missing)}")
    if extra:
        raise AttestationError(f"attestation contains unknown keys: {', '.join(extra)}")
    if attestation["schema"] != SCHEMA:
        raise AttestationError("attestation schema does not match the supported schema")
    if target_version != policy["nextPublicVersion"]:
        raise AttestationError(
            f"expected target version {target_version!r} does not match release policy "
            f"{policy['nextPublicVersion']!r}"
        )
    if attestation["targetVersion"] != target_version:
        raise AttestationError("attestation targetVersion does not match the release target")
    if not FULL_COMMIT_RE.fullmatch(commit):
        raise AttestationError("expected commit must be a lowercase full commit SHA")
    if attestation["commit"] != commit:
        raise AttestationError("attestation commit does not match the exact release commit")
    if phase not in PHASES:
        raise AttestationError(f"unsupported required readiness phase: {phase}")
    if attestation["phase"] != phase:
        raise AttestationError("attestation phase does not match the required release phase")

    actual_policy_sha = attestation["policySha256"]
    if not isinstance(actual_policy_sha, str) or not SHA256_RE.fullmatch(actual_policy_sha):
        raise AttestationError("attestation policySha256 must be a lowercase SHA-256 digest")
    if actual_policy_sha != policy_sha256(policy_path):
        raise AttestationError("attestation policySha256 does not match the current policy")

    actual_run_id = attestation["workflowRunId"]
    if isinstance(actual_run_id, bool) or not isinstance(actual_run_id, int):
        raise AttestationError("attestation workflowRunId must be an integer")
    if actual_run_id != workflow_run_id:
        raise AttestationError("attestation workflowRunId does not match the selected run")
    if isinstance(workflow_run_id, bool) or workflow_run_id <= 0:
        raise AttestationError("expected workflow run ID must be a positive integer")
    _validate_run_url(workflow_run_url, workflow_run_id)
    if attestation["workflowRunUrl"] != workflow_run_url:
        raise AttestationError("attestation workflowRunUrl does not match the selected run")

    authority = attestation["publicationAuthority"]
    if not isinstance(authority, bool):
        raise AttestationError("attestation publicationAuthority must be a boolean")
    _validate_publication_authority(policy, phase, authority)
    if intent not in {"qualify", "publish"}:
        raise AttestationError(f"unsupported attestation intent: {intent}")
    if intent == "publish":
        if policy["publicationWorkflowsReady"] is not True:
            raise AttestationError(
                "publicationWorkflowsReady=false cannot authorize publication"
            )
        if authority is not True:
            raise AttestationError("attestation does not grant publication authority")


def _write_attestation(path: Path, attestation: dict[str, object]) -> None:
    try:
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(
            json.dumps(attestation, indent=2, sort_keys=True) + "\n",
            encoding="utf-8",
        )
    except OSError as exc:
        raise AttestationError(f"cannot write readiness attestation {path}: {exc}") from exc


def _positive_run_id(value: str) -> int:
    if not re.fullmatch(r"[1-9][0-9]*", value):
        raise argparse.ArgumentTypeError("must be a positive decimal workflow run ID")
    return int(value)


def _strict_boolean(value: str) -> bool:
    if value == "true":
        return True
    if value == "false":
        return False
    raise argparse.ArgumentTypeError("must be true or false")


def _add_binding_arguments(parser: argparse.ArgumentParser) -> None:
    parser.add_argument("--policy", type=Path, default=Path(".mss/release-policy.yaml"))
    parser.add_argument("--target-version", required=True)
    parser.add_argument("--commit", required=True)
    parser.add_argument("--phase", choices=PHASES, required=True)
    parser.add_argument("--workflow-run-id", type=_positive_run_id, required=True)
    parser.add_argument("--workflow-run-url", required=True)


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser()
    subparsers = parser.add_subparsers(dest="command", required=True)

    create_parser = subparsers.add_parser("create")
    create_parser.add_argument("--output", type=Path, required=True)
    create_parser.add_argument(
        "--publication-authority", type=_strict_boolean, required=True
    )
    _add_binding_arguments(create_parser)

    verify_parser = subparsers.add_parser("verify")
    verify_parser.add_argument("--attestation", type=Path, required=True)
    verify_parser.add_argument("--intent", choices=("qualify", "publish"), required=True)
    _add_binding_arguments(verify_parser)

    args = parser.parse_args(argv)
    try:
        if args.command == "create":
            attestation = build_attestation(
                policy_path=args.policy,
                target_version=args.target_version,
                commit=args.commit,
                phase=args.phase,
                workflow_run_id=args.workflow_run_id,
                workflow_run_url=args.workflow_run_url,
                publication_authority=args.publication_authority,
            )
            _write_attestation(args.output, attestation)
            print(f"wrote exact readiness attestation to {args.output}")
        else:
            validate_attestation(
                load_attestation(args.attestation),
                policy_path=args.policy,
                target_version=args.target_version,
                commit=args.commit,
                phase=args.phase,
                workflow_run_id=args.workflow_run_id,
                workflow_run_url=args.workflow_run_url,
                intent=args.intent,
            )
            print(
                "verified exact readiness attestation for "
                f"{args.target_version} at {args.commit} from run {args.workflow_run_id}"
            )
    except AttestationError as exc:
        print(f"release readiness attestation rejected the request: {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
