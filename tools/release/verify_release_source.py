#!/usr/bin/env python3
"""Verify that a release source is an exact PR-produced commit on remote main."""

from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
from collections.abc import Callable
from pathlib import Path
from typing import Any

from check_release_policy import PolicyError, load_policy


SHA_RE = re.compile(r"^[0-9a-f]{40}$")
REPOSITORY_RE = re.compile(r"^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$")
GRAPHQL_QUERY = """
query($owner: String!, $name: String!, $oid: GitObjectID!) {
  repository(owner: $owner, name: $name) {
    object(oid: $oid) {
      ... on Commit {
        oid
        associatedPullRequests(first: 100) {
          nodes {
            number
            url
            merged
            mergedAt
            baseRefName
            baseRepository {
              nameWithOwner
            }
            mergeCommit {
              oid
            }
          }
          pageInfo {
            hasNextPage
          }
        }
      }
    }
  }
}
""".strip()


class SourceError(ValueError):
    pass


def run_command(
    argv: list[str],
    *,
    cwd: Path | None = None,
    env: dict[str, str] | None = None,
    accepted_returncodes: set[int] | None = None,
) -> subprocess.CompletedProcess[str]:
    accepted = accepted_returncodes or {0}
    try:
        result = subprocess.run(
            argv,
            cwd=cwd,
            env=env,
            check=False,
            capture_output=True,
            text=True,
        )
    except OSError as exc:
        raise SourceError(f"cannot execute {argv[0]}: {exc}") from exc
    if result.returncode not in accepted:
        detail = (
            result.stderr.strip() or result.stdout.strip() or "no diagnostic output"
        )
        raise SourceError(f"{argv[0]} command failed: {detail}")
    return result


def git(repository_root: Path, *args: str) -> str:
    result = run_command(["git", "-C", str(repository_root), *args])
    return result.stdout.strip()


def select_pr_merge_evidence(
    payload: dict[str, Any], repository: str, commit: str, branch: str
) -> dict[str, Any]:
    if payload.get("errors"):
        raise SourceError("GitHub GraphQL rejected the pull-request source query")
    try:
        commit_object = payload["data"]["repository"]["object"]
        if commit_object["oid"] != commit:
            raise SourceError("GitHub returned a different commit object")
        connection = commit_object["associatedPullRequests"]
        nodes = connection["nodes"]
    except (KeyError, TypeError) as exc:
        raise SourceError(
            "GitHub returned incomplete pull-request source data"
        ) from exc

    if not isinstance(nodes, list):
        raise SourceError("GitHub returned invalid pull-request source data")
    for node in nodes:
        if not isinstance(node, dict):
            continue
        base_repository = node.get("baseRepository") or {}
        merge_commit = node.get("mergeCommit") or {}
        if (
            node.get("merged") is True
            and isinstance(node.get("mergedAt"), str)
            and node["mergedAt"]
            and node.get("baseRefName") == branch
            and base_repository.get("nameWithOwner") == repository
            and merge_commit.get("oid") == commit
        ):
            return node

    if (connection.get("pageInfo") or {}).get("hasNextPage") is True:
        raise SourceError(
            "GitHub pull-request source results were truncated before an exact merge was found"
        )
    raise SourceError(
        f"commit {commit} is not the merge/squash commit produced by a merged "
        f"pull request targeting {repository}:{branch}"
    )


def query_pr_merge_evidence(
    repository: str, commit: str, branch: str
) -> dict[str, Any]:
    token = os.environ.get("GH_TOKEN") or os.environ.get("GITHUB_TOKEN")
    if not token:
        raise SourceError("GH_TOKEN or GITHUB_TOKEN is required for pull-request proof")
    owner, name = repository.split("/", 1)
    command_env = os.environ.copy()
    command_env["GH_TOKEN"] = token
    result = run_command(
        [
            "gh",
            "api",
            "graphql",
            "-f",
            f"query={GRAPHQL_QUERY}",
            "-f",
            f"owner={owner}",
            "-f",
            f"name={name}",
            "-f",
            f"oid={commit}",
        ],
        env=command_env,
    )
    try:
        payload = json.loads(result.stdout)
    except json.JSONDecodeError as exc:
        raise SourceError("GitHub returned non-JSON pull-request source data") from exc
    if not isinstance(payload, dict):
        raise SourceError("GitHub returned invalid pull-request source data")
    return select_pr_merge_evidence(payload, repository, commit, branch)


def verify_release_source(
    *,
    repository_root: Path,
    policy_path: Path,
    repository: str,
    commit: str,
    tag: str | None,
    remote: str = "origin",
    pr_evidence_loader: Callable[[str, str, str], dict[str, Any]] = (
        query_pr_merge_evidence
    ),
) -> dict[str, Any]:
    try:
        policy = load_policy(policy_path)
    except PolicyError as exc:
        raise SourceError(str(exc)) from exc
    branch = policy["releaseBranch"]
    if (
        not isinstance(branch, str)
        or policy["requireMergedPullRequestSource"] is not True
    ):
        raise SourceError(
            "release policy does not require a merged pull-request source"
        )
    if not REPOSITORY_RE.fullmatch(repository):
        raise SourceError("repository must use OWNER/REPO syntax")
    if not SHA_RE.fullmatch(commit):
        raise SourceError("commit must be a lowercase full SHA")
    if not remote or remote.startswith("-"):
        raise SourceError("remote must be a named Git remote")

    root = Path(git(repository_root, "rev-parse", "--show-toplevel")).resolve()
    shallow = git(root, "rev-parse", "--is-shallow-repository")
    fetch_args = ["fetch", "--no-tags"]
    if shallow == "true":
        fetch_args.append("--unshallow")
    fetch_args.extend([remote, f"+refs/heads/{branch}:refs/remotes/{remote}/{branch}"])
    git(root, *fetch_args)

    resolved_commit = git(root, "rev-parse", "--verify", f"{commit}^{{commit}}")
    if resolved_commit != commit:
        raise SourceError(f"candidate {commit} does not resolve to itself")
    head_commit = git(root, "rev-parse", "--verify", "HEAD^{commit}")
    if head_commit != commit:
        raise SourceError(
            f"checked-out HEAD {head_commit} does not equal candidate {commit}"
        )

    ancestry = run_command(
        [
            "git",
            "-C",
            str(root),
            "merge-base",
            "--is-ancestor",
            commit,
            f"refs/remotes/{remote}/{branch}",
        ],
        accepted_returncodes={0, 1},
    )
    if ancestry.returncode != 0:
        raise SourceError(f"candidate {commit} is not contained in {remote}/{branch}")

    if tag is not None:
        if not tag or tag.startswith("-"):
            raise SourceError("release tag is empty or unsafe")
        check_ref = run_command(
            ["git", "check-ref-format", f"refs/tags/{tag}"],
            accepted_returncodes={0, 1},
        )
        if check_ref.returncode != 0:
            raise SourceError(f"invalid release tag: {tag}")
        git(root, "fetch", "--force", "--no-tags", remote, f"refs/tags/{tag}")
        tag_commit = git(root, "rev-parse", "--verify", "FETCH_HEAD^{commit}")
        if tag_commit != commit:
            raise SourceError(
                f"remote tag {tag} resolves to {tag_commit}, not candidate {commit}"
            )

    if git(root, "status", "--porcelain=v1", "--untracked-files=no"):
        raise SourceError("tracked worktree is not clean")

    return pr_evidence_loader(repository, commit, branch)


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--repository-root", type=Path, default=Path("."))
    parser.add_argument("--policy", type=Path, default=Path(".mss/release-policy.yaml"))
    parser.add_argument("--repository", required=True)
    parser.add_argument("--commit", required=True)
    parser.add_argument("--tag")
    parser.add_argument("--remote", default="origin")
    args = parser.parse_args(argv)
    try:
        evidence = verify_release_source(
            repository_root=args.repository_root,
            policy_path=args.policy,
            repository=args.repository,
            commit=args.commit,
            tag=args.tag,
            remote=args.remote,
        )
    except SourceError as exc:
        print(f"release source rejected: {exc}", file=sys.stderr)
        return 1
    tag_message = f"; tag {args.tag} resolves exactly" if args.tag else ""
    print(
        f"release source accepted: {args.commit} is PR #{evidence['number']} "
        f"on origin/main{tag_message}",
        file=sys.stdout,
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
