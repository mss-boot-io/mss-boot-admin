import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


TOOLS_DIR = Path(__file__).resolve().parent
REPOSITORY_ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(TOOLS_DIR))
import verify_release_source as SOURCE  # noqa: E402


POLICY_PATH = REPOSITORY_ROOT / ".mss" / "release-policy.yaml"
REPOSITORY = "mss-boot-io/mss-boot-admin"


def run_git(repository: Path, *args: str) -> str:
    result = subprocess.run(
        ["git", "-C", str(repository), *args],
        check=True,
        capture_output=True,
        text=True,
    )
    return result.stdout.strip()


def pr_payload(
    commit: str,
    *,
    merge_commit: str | None = None,
    branch: str = "main",
    repository: str = REPOSITORY,
    merged: bool = True,
) -> dict:
    return {
        "data": {
            "repository": {
                "object": {
                    "oid": commit,
                    "associatedPullRequests": {
                        "nodes": [
                            {
                                "number": 474,
                                "url": "https://github.com/mss-boot-io/mss-boot-admin/pull/474",
                                "merged": merged,
                                "mergedAt": "2026-08-11T12:00:00Z" if merged else None,
                                "baseRefName": branch,
                                "baseRepository": {"nameWithOwner": repository},
                                "mergeCommit": (
                                    {"oid": merge_commit or commit}
                                    if merge_commit is not None or merged
                                    else None
                                ),
                            }
                        ],
                        "pageInfo": {"hasNextPage": False},
                    },
                }
            }
        }
    }


class PullRequestEvidenceTest(unittest.TestCase):
    def test_accepts_exact_merged_main_pr_commit(self):
        commit = "1" * 40
        evidence = SOURCE.select_pr_merge_evidence(
            pr_payload(commit), REPOSITORY, commit, "main"
        )
        self.assertEqual(evidence["number"], 474)

    def test_rejects_rebase_or_direct_commit_without_exact_merge_commit(self):
        commit = "1" * 40
        cases = (
            pr_payload(commit, merge_commit="2" * 40),
            pr_payload(commit, merged=False),
            pr_payload(commit, branch="release"),
            pr_payload(commit, repository="someone/else"),
        )
        for payload in cases:
            with self.subTest(payload=payload):
                with self.assertRaisesRegex(SOURCE.SourceError, "merge/squash"):
                    SOURCE.select_pr_merge_evidence(payload, REPOSITORY, commit, "main")

    def test_rejects_graphql_errors_and_truncated_nonmatch(self):
        commit = "1" * 40
        with self.assertRaisesRegex(SOURCE.SourceError, "GraphQL rejected"):
            SOURCE.select_pr_merge_evidence(
                {"errors": [{"message": "denied"}]}, REPOSITORY, commit, "main"
            )
        payload = pr_payload(commit, merge_commit="2" * 40)
        payload["data"]["repository"]["object"]["associatedPullRequests"]["pageInfo"][
            "hasNextPage"
        ] = True
        with self.assertRaisesRegex(SOURCE.SourceError, "truncated"):
            SOURCE.select_pr_merge_evidence(payload, REPOSITORY, commit, "main")


@unittest.skipUnless(shutil.which("git"), "git is required")
class ReleaseSourceGitTest(unittest.TestCase):
    def setUp(self):
        self.temporary_directory = tempfile.TemporaryDirectory()
        root = Path(self.temporary_directory.name)
        self.origin = root / "origin.git"
        self.work = root / "work"
        subprocess.run(["git", "init", "--bare", str(self.origin)], check=True)
        subprocess.run(
            ["git", "init", "--initial-branch=main", str(self.work)], check=True
        )
        run_git(self.work, "config", "user.name", "Release Test")
        run_git(self.work, "config", "user.email", "release@example.test")
        (self.work / "tracked.txt").write_text("base\n", encoding="utf-8")
        run_git(self.work, "add", "tracked.txt")
        run_git(self.work, "commit", "-m", "base")
        self.base_commit = run_git(self.work, "rev-parse", "HEAD")
        run_git(self.work, "remote", "add", "origin", str(self.origin))
        run_git(self.work, "push", "-u", "origin", "main")

        run_git(self.work, "switch", "-c", "feature")
        (self.work / "tracked.txt").write_text("feature\n", encoding="utf-8")
        run_git(self.work, "add", "tracked.txt")
        run_git(self.work, "commit", "-m", "feature")
        run_git(self.work, "switch", "main")
        run_git(self.work, "merge", "--no-ff", "feature", "-m", "merge feature")
        self.candidate = run_git(self.work, "rev-parse", "HEAD")
        run_git(self.work, "push", "origin", "main")
        run_git(self.work, "tag", "v1.1.0", self.candidate)
        run_git(self.work, "push", "origin", "refs/tags/v1.1.0")

    def tearDown(self):
        self.temporary_directory.cleanup()

    def loader(self, repository: str, commit: str, branch: str) -> dict:
        return SOURCE.select_pr_merge_evidence(
            pr_payload(commit), repository, commit, branch
        )

    def verify(self, *, commit: str | None = None, tag: str | None = "v1.1.0"):
        return SOURCE.verify_release_source(
            repository_root=self.work,
            policy_path=POLICY_PATH,
            repository=REPOSITORY,
            commit=commit or self.candidate,
            tag=tag,
            pr_evidence_loader=self.loader,
        )

    def test_accepts_exact_merge_commit_on_origin_main_and_remote_tag(self):
        evidence = self.verify()
        self.assertEqual(evidence["number"], 474)

    def test_accepts_one_parent_squash_commit_with_github_merge_proof(self):
        run_git(self.work, "switch", "-c", "squash-feature")
        (self.work / "tracked.txt").write_text("squash\n", encoding="utf-8")
        run_git(self.work, "add", "tracked.txt")
        run_git(self.work, "commit", "-m", "squash feature work")
        run_git(self.work, "switch", "main")
        run_git(self.work, "merge", "--squash", "squash-feature")
        run_git(self.work, "commit", "-m", "squash feature PR")
        squash_commit = run_git(self.work, "rev-parse", "HEAD")
        self.assertEqual(
            len(run_git(self.work, "show", "-s", "--format=%P", squash_commit).split()),
            1,
        )
        run_git(self.work, "push", "origin", "main")
        run_git(self.work, "tag", "v1.1.1", squash_commit)
        run_git(self.work, "push", "origin", "refs/tags/v1.1.1")
        evidence = SOURCE.verify_release_source(
            repository_root=self.work,
            policy_path=POLICY_PATH,
            repository=REPOSITORY,
            commit=squash_commit,
            tag="v1.1.1",
            pr_evidence_loader=self.loader,
        )
        self.assertEqual(evidence["number"], 474)

    def test_rejects_candidate_not_on_remote_main(self):
        (self.work / "tracked.txt").write_text("local only\n", encoding="utf-8")
        run_git(self.work, "add", "tracked.txt")
        run_git(self.work, "commit", "-m", "local only")
        local_commit = run_git(self.work, "rev-parse", "HEAD")
        with self.assertRaisesRegex(SOURCE.SourceError, "not contained"):
            self.verify(commit=local_commit, tag=None)

    def test_rejects_checkout_that_does_not_match_candidate(self):
        run_git(self.work, "checkout", "--detach", self.base_commit)
        with self.assertRaisesRegex(SOURCE.SourceError, "does not equal candidate"):
            self.verify()

    def test_rejects_dirty_tracked_worktree(self):
        (self.work / "tracked.txt").write_text("dirty\n", encoding="utf-8")
        with self.assertRaisesRegex(SOURCE.SourceError, "not clean"):
            self.verify()

    def test_rejects_remote_tag_pointing_at_another_commit(self):
        run_git(self.work, "tag", "wrong", self.base_commit)
        run_git(self.work, "push", "origin", "refs/tags/wrong")
        with self.assertRaisesRegex(SOURCE.SourceError, "not candidate"):
            self.verify(tag="wrong")


if __name__ == "__main__":
    unittest.main()
