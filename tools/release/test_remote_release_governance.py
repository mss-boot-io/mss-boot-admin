import re
import subprocess
import unittest
from pathlib import Path


REPOSITORY_ROOT = Path(__file__).resolve().parents[2]
SCRIPT_PATH = REPOSITORY_ROOT / "tools" / "release" / "verify_remote_release_governance.sh"


class RemoteReleaseGovernanceTest(unittest.TestCase):
    def setUp(self):
        self.content = SCRIPT_PATH.read_text(encoding="utf-8")

    def test_script_is_syntax_valid_and_argument_confined(self):
        result = subprocess.run(
            ["bash", "-n", str(SCRIPT_PATH)],
            text=True,
            capture_output=True,
            check=False,
        )
        self.assertEqual(result.returncode, 0, msg=result.stderr)
        self.assertIn("OWNER/REPO", self.content)
        self.assertIn("^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$", self.content)
        self.assertNotRegex(self.content, re.compile(r"gh auth token|Authorization:"))
        self.assertNotRegex(self.content, re.compile(r"secrets\[[^]]+\]\.value"))

    def test_root_creation_and_immutability_are_exact(self):
        for required in (
            "mapfile -t creation_names",
            "exactly the component and root creation rulesets may create release tags",
            "root-release-tag-controlled-creation",
            'include == ["refs/tags/v*"]',
            "actor_id: null",
            'actor_type: "DeployKey"',
            'bypass_mode: "always"',
            'gh api "/repos/${repository}/keys?per_page=100"',
            "length == 1",
            '.[0].title == "mss-root-tag-promotion"',
            ".[0].read_only == false",
            ".[0].verified == true",
            ".[0].enabled == true",
            'startswith("ssh-ed25519 ")',
            'gh api "/repositories/${repository_id}/environments/root-promotion/secrets?per_page=100"',
            ".total_count == 1",
            '(.secrets | length) == 1',
            '.secrets[0].name == "ROOT_TAG_PROMOTION_SSH_PRIVATE_KEY"',
            "release-tags-controlled-creation",
            "release-tags-immutable",
            ".bypass_actors == []",
            '["deletion", "non_fast_forward", "update"]',
        ):
            self.assertIn(required, self.content)
        self.assertNotIn("actor_id: 15368", self.content)
        self.assertNotIn('actor_type: "Integration"', self.content)

    def test_environment_requires_distinct_reviewer_and_main_only(self):
        for required in (
            'actor_id}" == "${reviewer_id}',
            ".can_admins_bypass == false",
            ".prevent_self_review == true",
            'environment: "root-promotion"',
            '.branch_policies[0].name == "main"',
            '.branch_policies[0].type == "branch"',
            'name: "ROOT_TAG_PROMOTION_SSH_PRIVATE_KEY"',
            "configured: true",
        ):
            self.assertIn(required, self.content)


if __name__ == "__main__":
    unittest.main()
