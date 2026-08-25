#!/usr/bin/env python3
"""Run checked-in, phase-scoped Feature command evidence without a shell."""

from __future__ import annotations

import argparse
import dataclasses
import hashlib
import json
import os
import re
import shlex
import subprocess
import sys
import tempfile
import time
from datetime import datetime, timezone
from pathlib import Path
from typing import Iterable

from check_release_policy import PolicyError, load_policy


SCHEMA = "mss.io/release-phase-command-evidence/v1"
QUALIFICATION_SCHEMA = "mss.io/release-qualification/v1"
PHASES = ("checkpoint", "feature-freeze", "pre-framework", "pre-root", "post-publication")
FULL_COMMIT_RE = re.compile(r"^[0-9a-f]{40}$")
SAFE_ID_RE = re.compile(r"[^a-zA-Z0-9_.-]+")
ENVIRONMENT_NAME_RE = re.compile(r"^(?:GOWORK|GOTOOLCHAIN|GOFLAGS|MSS_[A-Z0-9_]+)$")
ALLOWED_EXECUTABLES = frozenset(("bash", "corepack", "go", "make", "node", "pnpm", "python3"))
ALLOWED_BASH_SCRIPTS = frozenset(
    (
        "tools/compatibility/test-admin-external-consumer.sh",
        "tools/compatibility/test-thin-host-external-consumer.sh",
        "tools/release/verify_readiness_run.sh",
    )
)
ALLOWED_BASH_INVOCATIONS = {
    "tools/install/test-install-mss.sh": frozenset(((),)),
    "tools/compatibility/test-standalone-mss-consumer.sh": frozenset(
        (
            (),
            ("--lifecycle",),
            ("--upgrade",),
            ("--public-packages", "--lifecycle", "--upgrade"),
        )
    ),
}
SHELL_CONTROL_TOKENS = frozenset(("&&", "||", ";", "|", ">", ">>", "<", "<<"))


class PhaseEvidenceError(ValueError):
    pass


class UnsupportedCommandError(PhaseEvidenceError):
    pass


@dataclasses.dataclass(frozen=True)
class Reference:
    feature: str
    acceptance: str
    evidence_index: int
    required: bool


@dataclasses.dataclass
class CommandStep:
    identifier: str
    argv: list[str]
    references: list[Reference]
    working_directory: str = "."
    environment: dict[str, str] = dataclasses.field(default_factory=dict)


@dataclasses.dataclass(frozen=True)
class ParsedCommand:
    argv: list[str]
    working_directory: str
    environment: dict[str, str]


def _json_object(value: str, *, source: str) -> dict[str, object]:
    try:
        parsed = json.loads(value)
    except json.JSONDecodeError as exc:
        raise PhaseEvidenceError(f"{source} did not return valid JSON") from exc
    if not isinstance(parsed, dict):
        raise PhaseEvidenceError(f"{source} must return one JSON object")
    return parsed


def _run_checked(
    argv: list[str], *, cwd: Path, env: dict[str, str] | None = None
) -> subprocess.CompletedProcess[str]:
    try:
        result = subprocess.run(
            argv,
            cwd=cwd,
            env=env,
            check=False,
            capture_output=True,
            text=True,
            encoding="utf-8",
        )
    except OSError as exc:
        raise PhaseEvidenceError(f"cannot execute {argv[0]!r}: {exc}") from exc
    if result.returncode != 0:
        raise PhaseEvidenceError(
            f"command {argv[0]!r} failed while loading release contracts "
            f"(exit {result.returncode})"
        )
    return result


def load_qualification(root: Path, qualification_path: Path, target_version: str) -> list[Path]:
    path = (root / qualification_path).resolve()
    try:
        path.relative_to(root.resolve())
    except ValueError as exc:
        raise PhaseEvidenceError("qualification contract must stay inside the repository") from exc
    try:
        contract = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise PhaseEvidenceError(f"cannot read qualification contract {qualification_path}") from exc
    if not isinstance(contract, dict) or set(contract) != {
        "schema",
        "targetVersion",
        "features",
        "excludedFeatures",
    }:
        raise PhaseEvidenceError("qualification contract has unsupported keys")
    if contract["schema"] != QUALIFICATION_SCHEMA:
        raise PhaseEvidenceError("unsupported qualification contract schema")
    if contract["targetVersion"] != target_version:
        raise PhaseEvidenceError("qualification target does not match requested version")
    features = contract["features"]
    excluded = contract["excludedFeatures"]
    if not isinstance(features, list) or not isinstance(excluded, list):
        raise PhaseEvidenceError("qualification features and exclusions must be arrays")

    selected: list[Path] = []
    declared: set[str] = set()
    for value in features:
        if not isinstance(value, str) or not value.startswith(".mss/features/") or not value.endswith(".yaml"):
            raise PhaseEvidenceError("qualification feature paths must name .mss/features YAML files")
        if value in declared:
            raise PhaseEvidenceError(f"duplicate qualification feature: {value}")
        declared.add(value)
        feature_path = (root / value).resolve()
        try:
            feature_path.relative_to((root / ".mss" / "features").resolve())
        except ValueError as exc:
            raise PhaseEvidenceError("qualification feature escapes .mss/features") from exc
        if not feature_path.is_file():
            raise PhaseEvidenceError(f"qualification feature does not exist: {value}")
        selected.append(feature_path)
    for item in excluded:
        if (
            not isinstance(item, dict)
            or set(item) != {"path", "reason"}
            or not isinstance(item["path"], str)
            or not isinstance(item["reason"], str)
            or not item["reason"].strip()
        ):
            raise PhaseEvidenceError("qualification exclusions require exact path and reason")
        value = item["path"]
        if value in declared:
            raise PhaseEvidenceError(f"duplicate qualification feature or exclusion: {value}")
        declared.add(value)

    actual = {
        feature.relative_to(root).as_posix()
        for feature in (root / ".mss" / "features").glob("*.yaml")
    }
    missing = sorted(actual - declared)
    extra = sorted(declared - actual)
    if missing or extra:
        details = []
        if missing:
            details.append(f"undeclared features: {', '.join(missing)}")
        if extra:
            details.append(f"unknown features: {', '.join(extra)}")
        raise PhaseEvidenceError("; ".join(details))
    return selected


def discover_feature_plans(root: Path, feature_paths: list[Path], go_binary: str) -> list[dict[str, object]]:

    plans: list[dict[str, object]] = []
    for feature_path in feature_paths:
        relative = feature_path.relative_to(root).as_posix()
        try:
            result = _run_checked(
                [
                    go_binary,
                    "run",
                    "./cmd/mss",
                    "feature",
                    "plan",
                    relative,
                    "--format",
                    "json",
                ],
                cwd=root,
            )
        except PhaseEvidenceError as exc:
            raise PhaseEvidenceError(f"cannot load feature plan {relative}: {exc}") from exc
        plan = _json_object(result.stdout, source=relative)
        if plan.get("success") is not True:
            raise PhaseEvidenceError(f"feature plan is not successful: {relative}")
        plans.append(plan)
    if not plans:
        raise PhaseEvidenceError("no Feature contracts were found")
    return plans


def parse_command(value: str) -> ParsedCommand:
    if not isinstance(value, str) or not value.strip():
        raise PhaseEvidenceError("command evidence must be a non-empty string")
    if "\x00" in value or "\n" in value or "\r" in value:
        raise PhaseEvidenceError("command evidence must be a single logical line")
    try:
        argv = shlex.split(value, posix=True)
    except ValueError as exc:
        raise PhaseEvidenceError(f"invalid command evidence: {exc}") from exc
    if not argv:
        raise PhaseEvidenceError("command evidence has no arguments")
    working_directory = "."
    if argv[0] == "cd":
        if len(argv) < 4 or argv[2] != "&&":
            raise PhaseEvidenceError("cd evidence must use the exact `cd <directory> && command` form")
        candidate = Path(argv[1])
        if candidate.is_absolute() or "\\" in argv[1] or ".." in candidate.parts:
            raise PhaseEvidenceError("command working directory must be repository-relative")
        working_directory = candidate.as_posix()
        argv = argv[3:]

    environment: dict[str, str] = {}
    while argv and "=" in argv[0] and not argv[0].startswith("="):
        name, raw_value = argv[0].split("=", 1)
        if not ENVIRONMENT_NAME_RE.fullmatch(name):
            break
        if not raw_value or "$" in raw_value or "`" in raw_value:
            raise PhaseEvidenceError("command environment values must be non-empty literals")
        if name in environment:
            raise PhaseEvidenceError(f"duplicate command environment variable: {name}")
        environment[name] = raw_value
        argv = argv[1:]
    if not argv:
        raise PhaseEvidenceError("command evidence has no executable")
    if argv[0] not in ALLOWED_EXECUTABLES:
        raise UnsupportedCommandError(f"command executable is not allowlisted: {argv[0]!r}")
    if any(token in SHELL_CONTROL_TOKENS for token in argv):
        raise PhaseEvidenceError("shell control operators are not allowed in command evidence")
    if any("$(" in token or "`" in token for token in argv):
        raise PhaseEvidenceError("shell expansion is not allowed in command evidence")
    if argv[0] == "bash":
        if len(argv) < 2 or (
            argv[1] not in ALLOWED_BASH_SCRIPTS
            and argv[1] not in ALLOWED_BASH_INVOCATIONS
        ):
            raise PhaseEvidenceError(
                "bash evidence may invoke only an explicitly allowlisted qualification script"
            )
        allowed_arguments = ALLOWED_BASH_INVOCATIONS.get(argv[1])
        if allowed_arguments is not None and tuple(argv[2:]) not in allowed_arguments:
            raise PhaseEvidenceError(
                f"bash evidence uses unsupported arguments for {argv[1]!r}"
            )
    return ParsedCommand(argv, working_directory, environment)


def is_nonexact_named_go_test(command: ParsedCommand) -> bool:
    argv = command.argv
    return argv[0] == "go" and len(argv) > 1 and argv[1] == "test" and "-run" in argv


def _feature_name(plan: dict[str, object]) -> str:
    feature = plan.get("feature")
    if not isinstance(feature, dict) or not isinstance(feature.get("name"), str):
        raise PhaseEvidenceError("feature plan is missing feature.name")
    return feature["name"]


def collect_phase_commands(
    plans: Iterable[dict[str, object]], *, phase: str, include_optional: bool = False
) -> tuple[list[CommandStep], list[dict[str, object]]]:
    if phase not in PHASES:
        raise PhaseEvidenceError(f"unsupported release phase: {phase}")
    by_command: dict[tuple[str, ...], CommandStep] = {}
    review_items: list[dict[str, object]] = []

    for plan in plans:
        feature_name = _feature_name(plan)
        acceptance = plan.get("acceptance")
        if not isinstance(acceptance, list):
            raise PhaseEvidenceError(f"feature {feature_name!r} is missing acceptance")
        for criterion in acceptance:
            if not isinstance(criterion, dict) or criterion.get("phase") != phase:
                continue
            required = criterion.get("required") is True
            if not required and not include_optional:
                continue
            acceptance_id = criterion.get("id")
            evidence = criterion.get("evidence")
            if not isinstance(acceptance_id, str) or not isinstance(evidence, list):
                raise PhaseEvidenceError(
                    f"feature {feature_name!r} has malformed phase acceptance"
                )
            command_count = 0
            for index, item in enumerate(evidence):
                if not isinstance(item, dict):
                    raise PhaseEvidenceError(
                        f"feature {feature_name!r} acceptance {acceptance_id!r} has malformed evidence"
                    )
                evidence_type = item.get("type")
                value = item.get("value")
                if evidence_type == "command":
                    try:
                        command = parse_command(value)
                    except UnsupportedCommandError as exc:
                        review_items.append(
                            {
                                "feature": feature_name,
                                "acceptance": acceptance_id,
                                "type": "unsupported-command",
                                "value": value,
                                "required": required,
                                "reason": str(exc),
                            }
                        )
                        continue
                    except PhaseEvidenceError as exc:
                        raise PhaseEvidenceError(
                            f"feature {feature_name!r} acceptance {acceptance_id!r}: {exc}"
                        ) from exc
                    if is_nonexact_named_go_test(command):
                        review_items.append(
                            {
                                "feature": feature_name,
                                "acceptance": acceptance_id,
                                "type": "non-exact-command",
                                "value": value,
                                "required": required,
                                "reason": "named Go tests must use `mss test evidence` before qualification",
                            }
                        )
                    key = (
                        command.working_directory,
                        *tuple(f"{name}={value}" for name, value in sorted(command.environment.items())),
                        "--",
                        *tuple(command.argv),
                    )
                    reference = Reference(feature_name, acceptance_id, index, required)
                    step = by_command.get(key)
                    if step is None:
                        digest = hashlib.sha256("\0".join(key).encode("utf-8")).hexdigest()[:12]
                        identifier = f"{feature_name}.{acceptance_id}.{digest}"
                        step = CommandStep(
                            identifier,
                            command.argv,
                            [],
                            command.working_directory,
                            command.environment,
                        )
                        by_command[key] = step
                    step.references.append(reference)
                    command_count += 1
                elif evidence_type in {"manual", "report"}:
                    review_items.append(
                        {
                            "feature": feature_name,
                            "acceptance": acceptance_id,
                            "type": evidence_type,
                            "value": value,
                            "required": required,
                        }
                    )
            if required and command_count == 0:
                review_items.append(
                    {
                        "feature": feature_name,
                        "acceptance": acceptance_id,
                        "type": "no-command-evidence",
                        "required": True,
                    }
                )
    return list(by_command.values()), review_items


def _sha256(path: Path) -> str:
    try:
        return hashlib.sha256(path.read_bytes()).hexdigest()
    except OSError as exc:
        raise PhaseEvidenceError(f"cannot read {path}: {exc}") from exc


def validate_binding(
    root: Path,
    *,
    policy_path: Path,
    target_version: str,
    commit: str,
    require_clean: bool,
) -> str:
    if not FULL_COMMIT_RE.fullmatch(commit):
        raise PhaseEvidenceError("commit must be a lowercase full commit SHA")
    try:
        policy = load_policy(policy_path)
    except PolicyError as exc:
        raise PhaseEvidenceError(str(exc)) from exc
    if target_version != policy["nextPublicVersion"]:
        raise PhaseEvidenceError("target version does not match release policy")
    head = _run_checked(["git", "rev-parse", "HEAD"], cwd=root).stdout.strip()
    if head != commit:
        raise PhaseEvidenceError("commit does not match repository HEAD")
    if require_clean:
        status = _run_checked(
            ["git", "status", "--porcelain=v1", "--untracked-files=all"], cwd=root
        ).stdout
        if status.strip():
            raise PhaseEvidenceError("release phase commands require a clean repository")
    return _sha256(policy_path)


def _write_log(path: Path, stdout: bytes, stderr: bytes) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("wb") as stream:
        stream.write(b"[stdout]\n")
        stream.write(stdout)
        stream.write(b"\n[stderr]\n")
        stream.write(stderr)
    path.chmod(0o600)


def execute_steps(
    steps: list[CommandStep], *, root: Path, logs_dir: Path, timeout_seconds: int
) -> list[dict[str, object]]:
    results: list[dict[str, object]] = []
    for index, step in enumerate(steps, start=1):
        environment = os.environ.copy()
        environment.update(step.environment)
        working_directory = (root / step.working_directory).resolve()
        try:
            working_directory.relative_to(root.resolve())
        except ValueError as exc:
            raise PhaseEvidenceError("command working directory escaped repository") from exc
        if not working_directory.is_dir():
            raise PhaseEvidenceError(
                f"command working directory does not exist: {step.working_directory}"
            )
        started = time.monotonic()
        timed_out = False
        try:
            completed = subprocess.run(
                step.argv,
                cwd=working_directory,
                env=environment,
                check=False,
                capture_output=True,
                timeout=timeout_seconds,
            )
            exit_code: int | None = completed.returncode
            stdout = completed.stdout
            stderr = completed.stderr
        except subprocess.TimeoutExpired as exc:
            timed_out = True
            exit_code = None
            stdout = exc.stdout or b""
            stderr = exc.stderr or b""
        except OSError as exc:
            exit_code = None
            stdout = b""
            stderr = str(exc).encode("utf-8", errors="replace")
        duration_ms = round((time.monotonic() - started) * 1000)
        safe_id = SAFE_ID_RE.sub("-", step.identifier).strip("-")
        relative_log_path = logs_dir / f"{index:03d}-{safe_id}.log"
        log_path = root / relative_log_path
        _write_log(log_path, stdout, stderr)
        results.append(
            {
                "id": step.identifier,
                "argv": step.argv,
                "workingDirectory": step.working_directory,
                "environment": sorted(step.environment),
                "references": [dataclasses.asdict(reference) for reference in step.references],
                "exitCode": exit_code,
                "timedOut": timed_out,
                "durationMs": duration_ms,
                "stdoutSha256": hashlib.sha256(stdout).hexdigest(),
                "stderrSha256": hashlib.sha256(stderr).hexdigest(),
                "stdoutBytes": len(stdout),
                "stderrBytes": len(stderr),
                "log": relative_log_path.as_posix(),
                "success": exit_code == 0 and not timed_out,
            }
        )
    return results


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
    parser.add_argument("mode", choices=("plan", "run"))
    parser.add_argument("--root", default=".")
    parser.add_argument("--qualification", default=".mss/release-qualification.json")
    parser.add_argument("--policy", default=".mss/release-policy.yaml")
    parser.add_argument("--target-version", required=True)
    parser.add_argument("--commit", required=True)
    parser.add_argument("--phase", choices=PHASES, required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--go-binary", default="go")
    parser.add_argument("--timeout-seconds", type=int, default=3600)
    parser.add_argument("--include-optional", action="store_true")
    return parser


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    root = Path(args.root).resolve()
    output = (root / args.output).resolve()
    policy_path = (root / args.policy).resolve()
    if args.timeout_seconds <= 0:
        raise PhaseEvidenceError("timeout must be positive")
    try:
        output.relative_to(root)
    except ValueError as exc:
        raise PhaseEvidenceError("output must stay inside the repository") from exc

    policy_digest = validate_binding(
        root,
        policy_path=policy_path,
        target_version=args.target_version,
        commit=args.commit,
        require_clean=args.mode == "run",
    )
    feature_paths = load_qualification(root, Path(args.qualification), args.target_version)
    plans = discover_feature_plans(root, feature_paths, args.go_binary)
    steps, review_items = collect_phase_commands(
        plans, phase=args.phase, include_optional=args.include_optional
    )
    logs_dir = output.parent / f"{output.stem}-logs"
    results = (
        execute_steps(
            steps,
            root=root,
            logs_dir=logs_dir.relative_to(root),
            timeout_seconds=args.timeout_seconds,
        )
        if args.mode == "run"
        else []
    )
    required_nonexact = any(
        item.get("required") is True
        and item.get("type") in {"non-exact-command", "unsupported-command"}
        for item in review_items
    )
    command_complete = (
        args.mode == "run"
        and len(results) == len(steps)
        and all(result["success"] is True for result in results)
        and not required_nonexact
    )
    report: dict[str, object] = {
        "schema": SCHEMA,
        "generatedAt": datetime.now(timezone.utc).isoformat(),
        "targetVersion": args.target_version,
        "commit": args.commit,
        "phase": args.phase,
        "policySha256": policy_digest,
        "mode": args.mode,
        "featureCount": len(plans),
        "commandCount": len(steps),
        "commandEvidenceComplete": command_complete,
        "releaseManagerReviewRequired": len(review_items) > 0,
        "reviewItems": review_items,
        "commands": results
        if args.mode == "run"
        else [
            {
                "id": step.identifier,
                "argv": step.argv,
                "workingDirectory": step.working_directory,
                "environment": sorted(step.environment),
                "references": [dataclasses.asdict(reference) for reference in step.references],
            }
            for step in steps
        ],
    }
    _write_json_atomic(output, report)
    json.dump(
        {
            "success": command_complete if args.mode == "run" else True,
            "output": output.relative_to(root).as_posix(),
            "phase": args.phase,
            "commands": len(steps),
            "reviewItems": len(review_items),
        },
        sys.stdout,
        sort_keys=True,
    )
    sys.stdout.write("\n")
    return 0 if args.mode == "plan" or command_complete else 1


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except PhaseEvidenceError as exc:
        print(f"release phase evidence error: {exc}", file=sys.stderr)
        raise SystemExit(2) from exc
