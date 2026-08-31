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
MAX_DOCS_REVISION = 999
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
    "immutableStoppedTrains",
    "publicationWorkflowsReady",
    "docsRevisionPublicationReady",
    "docsRevisionVersion",
    "docsRevisionCommit",
    "stablePromotionReady",
    "stablePromotionVersion",
    "stablePromotionCommit",
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
IMMUTABLE_STOPPED_COMPONENTS = tuple(COMPONENT_TEMPLATE_KEYS)
STOPPED_TRAIN_KEYS = {"version", "commit", "refs"}
PERMANENTLY_STOPPED_TRAINS = {
    "v1.3.5": {
        "version": "v1.3.5",
        "commit": "396f60615cdfa589353b16ef9d3531e249e65432",
        "refs": {
            "root": "v1.3.5",
            "framework": "mss-boot/v1.3.5",
            "admin": "admin/v1.3.5",
            "frontend": "web/antd-v6/v1.3.5",
            "docs": "docs/v1.3.5",
            "npm": "@mss-boot-io/admin-web@1.3.5",
        },
    },
    "v1.3.6": {
        "version": "v1.3.6",
        "commit": "b1fe47a3a83209574e09d53526b122dd2cbc5277",
        "refs": {
            "root": "v1.3.6",
            "framework": "mss-boot/v1.3.6",
            "admin": "admin/v1.3.6",
            "frontend": "web/antd-v6/v1.3.6",
            "docs": "docs/v1.3.6",
            "npm": "@mss-boot-io/admin-web@1.3.6",
        },
    },
}


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
    policy: dict[str, object], component: str, version: str
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


def _parse_stopped_train_block(
    lines: list[str], start: int
) -> tuple[list[dict[str, object]], int]:
    """Parse the deliberately small, dependency-free YAML subset used by the policy."""

    trains: list[dict[str, object]] = []
    current: dict[str, object] | None = None
    current_refs: dict[str, object] | None = None
    index = start
    while index < len(lines):
        line = lines[index]
        line_number = index + 1
        if not line.strip() or line.lstrip().startswith("#"):
            index += 1
            continue
        if not line.startswith("    "):
            break

        item_match = re.fullmatch(
            r"    - ([A-Za-z][A-Za-z0-9]*):\s*(.*)", line
        )
        field_match = re.fullmatch(
            r"      ([A-Za-z][A-Za-z0-9]*):\s*(.*)", line
        )
        ref_match = re.fullmatch(
            r"        ([A-Za-z][A-Za-z0-9]*):\s*(.+)", line
        )
        if item_match:
            current = {}
            trains.append(current)
            current_refs = None
            key, raw_value = item_match.groups()
            if not raw_value:
                raise PolicyError(
                    f"immutable stopped train field {key} is empty at line {line_number}"
                )
            current[key] = parse_scalar(raw_value)
        elif field_match:
            if current is None:
                raise PolicyError(
                    f"immutable stopped train field appears before a list item at line {line_number}"
                )
            key, raw_value = field_match.groups()
            if key in current:
                raise PolicyError(f"immutable stopped train duplicates field {key}")
            if key == "refs":
                if raw_value:
                    raise PolicyError(
                        "immutable stopped train refs must be a YAML mapping"
                    )
                current_refs = {}
                current[key] = current_refs
            else:
                if not raw_value:
                    raise PolicyError(
                        f"immutable stopped train field {key} is empty at line {line_number}"
                    )
                current[key] = parse_scalar(raw_value)
                current_refs = None
        elif ref_match:
            if current_refs is None:
                raise PolicyError(
                    f"immutable stopped train ref appears outside refs at line {line_number}"
                )
            component, raw_value = ref_match.groups()
            if component in current_refs:
                raise PolicyError(
                    f"immutable stopped train duplicates ref component {component}"
                )
            current_refs[component] = parse_scalar(raw_value)
        else:
            raise PolicyError(
                f"unsupported immutable stopped train syntax at line {line_number}"
            )
        index += 1
    return trains, index


def immutable_stopped_trains(
    policy: dict[str, object],
) -> dict[str, dict[str, object]]:
    raw_trains = policy["immutableStoppedTrains"]
    if not isinstance(raw_trains, list) or not raw_trains:
        raise PolicyError(
            "release policy immutableStoppedTrains must be a non-empty YAML list"
        )

    trains: dict[str, dict[str, object]] = {}
    public_refs: dict[str, str] = {}
    for index, raw_train in enumerate(raw_trains):
        if not isinstance(raw_train, dict):
            raise PolicyError(
                f"release policy immutableStoppedTrains item {index} must be a mapping"
            )
        keys = set(raw_train)
        missing = sorted(STOPPED_TRAIN_KEYS - keys)
        extra = sorted(keys - STOPPED_TRAIN_KEYS)
        if missing:
            raise PolicyError(
                f"immutable stopped train {index} is missing fields: {', '.join(missing)}"
            )
        if extra:
            raise PolicyError(
                f"immutable stopped train {index} has unknown fields: {', '.join(extra)}"
            )

        version = raw_train["version"]
        commit = raw_train["commit"]
        refs = raw_train["refs"]
        if not isinstance(version, str) or not VERSION_RE.fullmatch(version):
            raise PolicyError(
                f"immutable stopped train {index} version must be a semantic version"
            )
        if version in trains:
            raise PolicyError(
                f"release policy immutableStoppedTrains duplicates version {version}"
            )
        if not isinstance(commit, str) or not re.fullmatch(r"[0-9a-f]{40}", commit):
            raise PolicyError(
                f"immutable stopped train {version} commit must be a full commit SHA"
            )
        if not isinstance(refs, dict):
            raise PolicyError(
                f"immutable stopped train {version} refs must be a YAML mapping"
            )
        ref_keys = set(refs)
        missing_refs = sorted(set(IMMUTABLE_STOPPED_COMPONENTS) - ref_keys)
        extra_refs = sorted(ref_keys - set(IMMUTABLE_STOPPED_COMPONENTS))
        if missing_refs:
            raise PolicyError(
                f"immutable stopped train {version} is missing refs: "
                + ", ".join(missing_refs)
            )
        if extra_refs:
            raise PolicyError(
                f"immutable stopped train {version} has unknown refs: "
                + ", ".join(extra_refs)
            )
        normalized_refs: dict[str, str] = {}
        for component in IMMUTABLE_STOPPED_COMPONENTS:
            public_ref = refs[component]
            if not isinstance(public_ref, str) or not public_ref:
                raise PolicyError(
                    f"immutable stopped train {version} {component} ref must be a string"
                )
            expected = release_ref(policy, component, version)
            if public_ref != expected:
                raise PolicyError(
                    f"immutable stopped train {version} {component} ref must remain {expected!r}"
                )
            if public_ref in public_refs:
                raise PolicyError(
                    f"immutable stopped ref {public_ref!r} is duplicated by {version} "
                    f"and {public_refs[public_ref]}"
                )
            public_refs[public_ref] = version
            normalized_refs[component] = public_ref
        trains[version] = {
            "version": version,
            "commit": commit,
            "refs": normalized_refs,
        }

    for version, expected in PERMANENTLY_STOPPED_TRAINS.items():
        if trains.get(version) != expected:
            raise PolicyError(
                f"release policy immutableStoppedTrains must preserve the exact "
                f"{version} stopped train permanently"
            )
    return trains


def load_policy(path: Path) -> dict[str, object]:
    try:
        lines = path.read_text(encoding="utf-8").splitlines()
    except OSError as exc:
        raise PolicyError(f"cannot read release policy {path}: {exc}") from exc

    in_spec = False
    policy: dict[str, object] = {}
    index = 0
    while index < len(lines):
        line = lines[index]
        line_number = index + 1
        if line == "spec:":
            if in_spec:
                raise PolicyError("release policy contains more than one spec block")
            in_spec = True
            index += 1
            continue
        if not in_spec:
            index += 1
            continue
        if line and not line.startswith("  "):
            break
        if not line.strip() or line.lstrip().startswith("#"):
            index += 1
            continue
        if re.fullmatch(r"  immutableStoppedTrains:\s*", line):
            key = "immutableStoppedTrains"
            if key in policy:
                raise PolicyError(f"duplicate release policy key: {key}")
            policy[key], index = _parse_stopped_train_block(lines, index + 1)
            continue
        match = re.fullmatch(r"  ([A-Za-z][A-Za-z0-9]*):\s*(.+)", line)
        if not match:
            raise PolicyError(f"unsupported release policy syntax at line {line_number}")
        key, raw_value = match.groups()
        if key in policy:
            raise PolicyError(f"duplicate release policy key: {key}")
        policy[key] = parse_scalar(raw_value)
        index += 1

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
        "stablePromotionReady",
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
    promotion_version = policy["stablePromotionVersion"]
    if not isinstance(promotion_version, str) or not VERSION_RE.fullmatch(
        promotion_version
    ):
        raise PolicyError(
            "release policy stablePromotionVersion must be a valid semantic version"
        )
    if promotion_version != policy["nextPublicVersion"]:
        raise PolicyError(
            "release policy stablePromotionVersion must equal nextPublicVersion"
        )
    promotion_commit = policy["stablePromotionCommit"]
    if policy["stablePromotionReady"] is True:
        if promotion_version == policy["currentStableVersion"]:
            raise PolicyError(
                "release policy stable promotion authorization is already consumed: "
                "stablePromotionVersion must not equal currentStableVersion"
            )
        if not isinstance(promotion_commit, str) or not re.fullmatch(
            r"[0-9a-f]{40}", promotion_commit
        ):
            raise PolicyError(
                "release policy stablePromotionCommit must be a full commit SHA when promotion is ready"
            )
        if policy["publicationWorkflowsReady"] is not True:
            raise PolicyError(
                "release policy publicationWorkflowsReady must remain true during stable promotion"
            )
    elif promotion_commit != "disabled":
        raise PolicyError(
            "release policy stablePromotionCommit must be disabled until promotion is ready"
        )
    docs_revision_version = policy["docsRevisionVersion"]
    docs_revision_commit = policy["docsRevisionCommit"]
    if policy["docsRevisionPublicationReady"] is True:
        if not isinstance(docs_revision_version, str):
            raise PolicyError(
                "release policy docsRevisionVersion must be a docs revision when publication is ready"
            )
        docs_revision_match = DOCS_REVISION_RE.fullmatch(docs_revision_version)
        if docs_revision_match is None:
            raise PolicyError(
                "release policy docsRevisionVersion must use vX.Y.Z+docs.N when publication is ready"
            )
        if docs_revision_match.group("base") != policy["currentStableVersion"]:
            raise PolicyError(
                "release policy docsRevisionVersion must revise currentStableVersion"
            )
        if int(docs_revision_match.group("revision")) > MAX_DOCS_REVISION:
            raise PolicyError(
                f"release policy docsRevisionVersion exceeds maximum revision {MAX_DOCS_REVISION}"
            )
        if not isinstance(docs_revision_commit, str) or not re.fullmatch(
            r"[0-9a-f]{40}", docs_revision_commit
        ):
            raise PolicyError(
                "release policy docsRevisionCommit must be a full commit SHA when publication is ready"
            )
    elif docs_revision_version != "disabled" or docs_revision_commit != "disabled":
        raise PolicyError(
            "release policy docs revision version and commit must be disabled until publication is ready"
        )
    target_state = policy["releaseTargetState"]
    if target_state not in {"active", "stopped"}:
        raise PolicyError("release policy releaseTargetState must be active or stopped")
    stopped_trains = immutable_stopped_trains(policy)
    if policy["nextPublicVersion"] in stopped_trains and target_state != "stopped":
        raise PolicyError(
            "release policy releaseTargetState must be stopped when the reviewed "
            "target belongs to immutableStoppedTrains"
        )
    if policy["nextPublicVersion"] not in stopped_trains and target_state != "active":
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
    return policy


def coordinated_tags(
    policy: dict[str, object], version: str
) -> dict[str, str]:
    """Resolve and validate every coordinated Admin Distribution tag."""

    tags: dict[str, str] = {}
    for component in COORDINATED_COMPONENTS:
        tag = release_ref(policy, component, version)
        check_public_ref(policy, component, version, tag, intent="qualify")
        tags[component] = tag
    return tags


def check_public_ref(
    policy: dict[str, object],
    component: str,
    version: str,
    tag: str,
    intent: str = "publish",
    commit: str | None = None,
) -> None:
    if intent not in {"qualify", "publish", "promote"}:
        raise PolicyError(f"unsupported release intent: {intent}")
    docs_revision = (
        DOCS_REVISION_RE.fullmatch(version) if component == "docs" else None
    )
    if not VERSION_RE.fullmatch(version) and docs_revision is None:
        raise PolicyError(f"invalid release version: {version}")
    if docs_revision is not None and int(docs_revision.group("revision")) > MAX_DOCS_REVISION:
        raise PolicyError(
            f"docs revision exceeds maximum supported value {MAX_DOCS_REVISION}"
        )
    expected = release_ref(policy, component, version)
    if tag != expected:
        raise PolicyError(
            f"tag or package ref {tag!r} does not match the {component} release ref "
            f"{expected!r}"
        )

    stopped_trains = immutable_stopped_trains(policy)
    stopped_refs = {
        public_ref
        for train in stopped_trains.values()
        for public_ref in train["refs"].values()
    }
    if version in stopped_trains or tag in stopped_refs:
        stopped_version = (
            version
            if version in stopped_trains
            else next(
                train_version
                for train_version, train in stopped_trains.items()
                if tag in train["refs"].values()
            )
        )
        raise PolicyError(
            f"{component} public ref {tag!r} belongs to immutable stopped train "
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
        if version != policy["docsRevisionVersion"]:
            raise PolicyError(
                f"docs revision {version} does not match reviewed exact revision "
                f"{policy['docsRevisionVersion']}"
            )
        if commit != policy["docsRevisionCommit"]:
            raise PolicyError(
                "docs revision commit does not match the reviewed exact merged-main source"
            )
    elif version != target:
        raise PolicyError(
            f"public version {version} is forbidden while the reviewed target is {target}"
        )
    if intent == "promote":
        if component not in {"root", "npm"}:
            raise PolicyError(
                "stable promotion is supported only for Root and official npm aliases"
            )
        if policy["stablePromotionReady"] is not True:
            raise PolicyError(
                "stable promotion remains disabled until a reviewed post-reconciliation policy binds it"
            )
        if policy["stablePromotionVersion"] == policy["currentStableVersion"]:
            raise PolicyError(
                "stable promotion authorization is already consumed because the target "
                "is current stable"
            )
        if version != policy["stablePromotionVersion"]:
            raise PolicyError(
                f"stable promotion version {version} does not match reviewed "
                f"{policy['stablePromotionVersion']}"
            )
        if commit != policy["stablePromotionCommit"]:
            raise PolicyError(
                "stable promotion commit does not match the reviewed exact release commit"
            )
    if (
        intent in {"publish", "promote"}
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
    parser.add_argument(
        "--intent", choices=("qualify", "publish", "promote"), default="publish"
    )
    parser.add_argument("--commit")
    args = parser.parse_args(argv)
    try:
        policy = load_policy(args.policy)
        check_public_ref(
            policy, args.component, args.version, args.tag, args.intent, args.commit
        )
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
