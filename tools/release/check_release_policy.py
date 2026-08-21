#!/usr/bin/env python3
"""Reject public release refs that do not match the reviewed release policy."""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path


VERSION_RE = re.compile(
    r"^v(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)"
    r"(?:-(?:0|[1-9][0-9]*|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*)"
    r"(?:\.(?:0|[1-9][0-9]*|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*))*)?$"
)
REQUIRED_KEYS = {
    "mode",
    "releaseBranch",
    "requireMergedPullRequestSource",
    "currentStableVersion",
    "currentStableCommit",
    "nextPublicVersion",
    "distributionVersion",
    "distributionComponents",
    "publicationWorkflowsReady",
    "publicPrereleases",
    "rootTagTemplate",
    "frameworkTagTemplate",
    "adminTagTemplate",
    "frontendTagTemplate",
    "docsTagTemplate",
}
COMPONENT_TEMPLATE_KEYS = {
    "root": "rootTagTemplate",
    "framework": "frameworkTagTemplate",
    "admin": "adminTagTemplate",
    "frontend": "frontendTagTemplate",
    "docs": "docsTagTemplate",
}
COORDINATED_COMPONENTS = ("root", "framework", "admin", "frontend")


class PolicyError(ValueError):
    pass


def parse_scalar(value: str) -> str | bool:
    value = value.strip()
    if len(value) >= 2 and value[0] == value[-1] and value[0] in {'"', "'"}:
        value = value[1:-1]
    if value == "true":
        return True
    if value == "false":
        return False
    return value


def load_policy(path: Path) -> dict[str, str | bool]:
    try:
        lines = path.read_text(encoding="utf-8").splitlines()
    except OSError as exc:
        raise PolicyError(f"cannot read release policy {path}: {exc}") from exc

    in_spec = False
    policy: dict[str, str | bool] = {}
    for line_number, line in enumerate(lines, start=1):
        if line == "spec:":
            if in_spec:
                raise PolicyError("release policy contains more than one spec block")
            in_spec = True
            continue
        if not in_spec:
            continue
        if line and not line.startswith("  "):
            break
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        match = re.fullmatch(r"  ([A-Za-z][A-Za-z0-9]*):\s*(.+)", line)
        if not match:
            raise PolicyError(f"unsupported release policy syntax at line {line_number}")
        key, raw_value = match.groups()
        if key in policy:
            raise PolicyError(f"duplicate release policy key: {key}")
        policy[key] = parse_scalar(raw_value)

    missing = sorted(REQUIRED_KEYS - policy.keys())
    extra = sorted(policy.keys() - REQUIRED_KEYS)
    if missing:
        raise PolicyError(f"release policy is missing keys: {', '.join(missing)}")
    if extra:
        raise PolicyError(f"release policy contains unknown keys: {', '.join(extra)}")
    if policy["mode"] != "development-first":
        raise PolicyError("release policy mode must be development-first")
    if policy["releaseBranch"] != "main":
        raise PolicyError("release policy releaseBranch must be main")
    if policy["requireMergedPullRequestSource"] is not True:
        raise PolicyError(
            "release policy requireMergedPullRequestSource must be true"
        )
    for key in (
        "publicationWorkflowsReady",
        "publicPrereleases",
    ):
        if not isinstance(policy[key], bool):
            raise PolicyError(f"release policy {key} must be a boolean")
    for key in ("currentStableVersion", "nextPublicVersion", "distributionVersion"):
        value = policy[key]
        if not isinstance(value, str) or not VERSION_RE.fullmatch(value):
            raise PolicyError(f"release policy {key} must be a valid semantic version")
    if policy["distributionVersion"] != policy["nextPublicVersion"]:
        raise PolicyError(
            "release policy distributionVersion must equal nextPublicVersion"
        )
    if "-" in policy["currentStableVersion"]:
        raise PolicyError("release policy currentStableVersion must not be a prerelease")
    if "-" in policy["nextPublicVersion"] and policy["publicPrereleases"] is not True:
        raise PolicyError(
            "release policy publicPrereleases must be true for a prerelease target"
        )
    if "-" not in policy["nextPublicVersion"] and policy["publicPrereleases"] is not False:
        raise PolicyError(
            "release policy publicPrereleases must be false for a stable target"
        )
    expected_components = ",".join(COORDINATED_COMPONENTS)
    if policy["distributionComponents"] != expected_components:
        raise PolicyError(
            "release policy distributionComponents must be " + expected_components
        )
    commit = policy["currentStableCommit"]
    if not isinstance(commit, str) or not re.fullmatch(r"[0-9a-f]{40}", commit):
        raise PolicyError("release policy currentStableCommit must be a full commit SHA")
    return policy


def coordinated_tags(
    policy: dict[str, str | bool], version: str
) -> dict[str, str]:
    """Resolve and validate every coordinated Admin Distribution tag."""

    tags: dict[str, str] = {}
    for component in COORDINATED_COMPONENTS:
        template = policy[COMPONENT_TEMPLATE_KEYS[component]]
        if not isinstance(template, str) or template.count("{version}") != 1:
            raise PolicyError(f"invalid tag template for component {component}")
        tag = template.format(version=version)
        check_public_ref(policy, component, version, tag, intent="qualify")
        tags[component] = tag
    return tags


def check_public_ref(
    policy: dict[str, str | bool],
    component: str,
    version: str,
    tag: str,
    intent: str = "publish",
) -> None:
    if not VERSION_RE.fullmatch(version):
        raise PolicyError(f"invalid release version: {version}")
    target = policy["nextPublicVersion"]
    if version != target:
        raise PolicyError(
            f"public version {version} is forbidden while the reviewed target is {target}"
        )
    if intent not in {"qualify", "publish"}:
        raise PolicyError(f"unsupported release intent: {intent}")
    if intent == "publish" and policy["publicationWorkflowsReady"] is not True:
        raise PolicyError(
            "public component tags and artifacts remain disabled until the complete phase "
            "runner, evidence attestation, protected write jobs, and tag rules are ready"
        )
    if "-" in version and policy["publicPrereleases"] is not True:
        raise PolicyError("public prerelease tags are disabled during development-first mode")
    template = policy[COMPONENT_TEMPLATE_KEYS[component]]
    if not isinstance(template, str) or template.count("{version}") != 1:
        raise PolicyError(f"invalid tag template for component {component}")
    expected = template.format(version=version)
    if tag != expected:
        raise PolicyError(
            f"tag {tag!r} does not match the {component} release ref {expected!r}"
        )


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--policy", type=Path, default=Path(".mss/release-policy.yaml"))
    parser.add_argument("--component", choices=sorted(COMPONENT_TEMPLATE_KEYS), required=True)
    parser.add_argument("--version", required=True)
    parser.add_argument("--tag", required=True)
    parser.add_argument("--intent", choices=("qualify", "publish"), default="publish")
    args = parser.parse_args(argv)
    try:
        policy = load_policy(args.policy)
        check_public_ref(policy, args.component, args.version, args.tag, args.intent)
    except PolicyError as exc:
        print(f"release policy rejected the request: {exc}", file=sys.stderr)
        return 1
    print(
        f"release policy accepted {args.intent} for {args.component} {args.version} from {args.tag}",
        file=sys.stdout,
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
