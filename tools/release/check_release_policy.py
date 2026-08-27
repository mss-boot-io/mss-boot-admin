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
DOCS_REVISION_RE = re.compile(
    r"^(?P<base>v(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\."
    r"(?:0|[1-9][0-9]*))\+docs\.(?P<revision>[1-9][0-9]*)$"
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
    "releaseTargetState",
    "immutableStoppedVersion",
    "immutableStoppedPublicRefs",
    "publicationWorkflowsReady",
    "docsRevisionPublicationReady",
    "publicPrereleases",
    "rootTagTemplate",
    "frameworkTagTemplate",
    "adminTagTemplate",
    "frontendTagTemplate",
    "docsTagTemplate",
    "npmPackageTemplate",
}
COMPONENT_TEMPLATE_KEYS = {
    "root": "rootTagTemplate",
    "framework": "frameworkTagTemplate",
    "admin": "adminTagTemplate",
    "frontend": "frontendTagTemplate",
    "docs": "docsTagTemplate",
    "npm": "npmPackageTemplate",
}
COORDINATED_COMPONENTS = ("root", "framework", "admin", "frontend")
PERMANENTLY_STOPPED_VERSION = "v1.3.5"
PERMANENTLY_STOPPED_PUBLIC_REFS = {
    "root": "v1.3.5",
    "framework": "mss-boot/v1.3.5",
    "admin": "admin/v1.3.5",
    "frontend": "web/antd-v6/v1.3.5",
    "docs": "docs/v1.3.5",
    "npm": "@mss-boot-io/admin-web@1.3.5",
}
IMMUTABLE_STOPPED_COMPONENTS = tuple(PERMANENTLY_STOPPED_PUBLIC_REFS)


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


def release_ref(
    policy: dict[str, str | bool], component: str, version: str
) -> str:
    template_key = COMPONENT_TEMPLATE_KEYS.get(component)
    if template_key is None:
        raise PolicyError(f"unsupported release component: {component}")
    template = policy[template_key]
    placeholder = "{npmVersion}" if component == "npm" else "{version}"
    if not isinstance(template, str) or template.count(placeholder) != 1:
        raise PolicyError(f"invalid tag or package template for component {component}")
    if component == "npm":
        if not version.startswith("v"):
            raise PolicyError("npm release version must use the canonical v-prefixed input")
        return template.format(npmVersion=version[1:])
    return template.format(version=version)


def immutable_stopped_public_refs(
    policy: dict[str, str | bool],
) -> dict[str, str]:
    raw_refs = policy["immutableStoppedPublicRefs"]
    if not isinstance(raw_refs, str) or not raw_refs:
        raise PolicyError(
            "release policy immutableStoppedPublicRefs must be a non-empty mapping"
        )

    refs: dict[str, str] = {}
    for entry in raw_refs.split(","):
        component, separator, public_ref = entry.partition("=")
        if not separator or not component or not public_ref:
            raise PolicyError(
                "release policy immutableStoppedPublicRefs must use component=ref entries"
            )
        if component not in IMMUTABLE_STOPPED_COMPONENTS:
            raise PolicyError(
                f"release policy immutableStoppedPublicRefs has unsupported component {component}"
            )
        if component in refs:
            raise PolicyError(
                f"release policy immutableStoppedPublicRefs duplicates component {component}"
            )
        refs[component] = public_ref

    missing = sorted(set(IMMUTABLE_STOPPED_COMPONENTS) - refs.keys())
    if missing:
        raise PolicyError(
            "release policy immutableStoppedPublicRefs is missing components: "
            + ", ".join(missing)
        )
    return refs


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
        "docsRevisionPublicationReady",
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
    target_state = policy["releaseTargetState"]
    if target_state not in {"active", "stopped"}:
        raise PolicyError("release policy releaseTargetState must be active or stopped")
    stopped_version = policy["immutableStoppedVersion"]
    if stopped_version != PERMANENTLY_STOPPED_VERSION:
        raise PolicyError(
            "release policy immutableStoppedVersion must preserve v1.3.5 permanently"
        )
    if policy["nextPublicVersion"] == stopped_version and target_state != "stopped":
        raise PolicyError(
            "release policy releaseTargetState must be stopped while v1.3.5 remains "
            "the unselectable legacy target"
        )
    if policy["nextPublicVersion"] != stopped_version and target_state != "active":
        raise PolicyError(
            "release policy releaseTargetState must be active for a reviewed target "
            "that is not permanently stopped"
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
    stopped_refs = immutable_stopped_public_refs(policy)
    if stopped_refs != PERMANENTLY_STOPPED_PUBLIC_REFS:
        raise PolicyError(
            "release policy immutableStoppedPublicRefs must preserve every exact "
            "v1.3.5 public or stopped ref permanently"
        )
    for component in IMMUTABLE_STOPPED_COMPONENTS:
        expected = release_ref(policy, component, stopped_version)
        if stopped_refs[component] != expected:
            raise PolicyError(
                f"release policy immutable stopped {component} ref must remain {expected!r}"
            )
    return policy


def coordinated_tags(
    policy: dict[str, str | bool], version: str
) -> dict[str, str]:
    """Resolve and validate every coordinated Admin Distribution tag."""

    tags: dict[str, str] = {}
    for component in COORDINATED_COMPONENTS:
        tag = release_ref(policy, component, version)
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
    if intent not in {"qualify", "publish"}:
        raise PolicyError(f"unsupported release intent: {intent}")
    docs_revision = (
        DOCS_REVISION_RE.fullmatch(version) if component == "docs" else None
    )
    if not VERSION_RE.fullmatch(version) and docs_revision is None:
        raise PolicyError(f"invalid release version: {version}")
    expected = release_ref(policy, component, version)
    if tag != expected:
        raise PolicyError(
            f"tag or package ref {tag!r} does not match the {component} release ref "
            f"{expected!r}"
        )

    stopped_version = policy["immutableStoppedVersion"]
    stopped_refs = immutable_stopped_public_refs(policy)
    if version == stopped_version or tag in stopped_refs.values():
        raise PolicyError(
            f"{component} public ref {tag!r} belongs to immutable stopped version "
            f"{stopped_version}; qualify and publish are permanently forbidden"
        )

    target = policy["nextPublicVersion"]
    if docs_revision is not None:
        stable = policy["currentStableVersion"]
        if docs_revision.group("base") != stable:
            raise PolicyError(
                f"docs revision base {docs_revision.group('base')} is forbidden "
                f"while current stable is {stable}"
            )
        if policy["docsRevisionPublicationReady"] is not True:
            raise PolicyError(
                "docs revision qualification and publication remain disabled until policy "
                "binds an exact new revision tag, current-stable baseline, and new "
                "merged-main source"
            )
    elif version != target:
        raise PolicyError(
            f"public version {version} is forbidden while the reviewed target is {target}"
        )
    if (
        intent == "publish"
        and docs_revision is None
        and policy["publicationWorkflowsReady"] is not True
    ):
        raise PolicyError(
            "public component tags and artifacts remain disabled until the complete "
            "tag-driven workflows, protected write jobs, and tag rules are ready"
        )
    if "-" in version and policy["publicPrereleases"] is not True:
        raise PolicyError("public prerelease tags are disabled during development-first mode")


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
