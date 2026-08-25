import re
import subprocess
import unittest
from pathlib import Path

import yaml


REPOSITORY_ROOT = Path(__file__).resolve().parents[2]
WORKFLOW_PATH = REPOSITORY_ROOT / ".github" / "workflows" / "root-tag-promotion.yml"
GITHUB_EXPRESSION = re.compile(r"\$\{\{.*?\}\}", re.DOTALL)


class RootTagPromotionWorkflowTest(unittest.TestCase):
    def setUp(self):
        self.content = WORKFLOW_PATH.read_text(encoding="utf-8")
        self.workflow = yaml.load(self.content, Loader=yaml.BaseLoader)
        self.job = self.workflow["jobs"]["promote"]
        self.steps = self.job["steps"]

    def step(self, name):
        return next(step for step in self.steps if step.get("name") == name)

    def test_only_manual_protected_job_can_use_the_deploy_key(self):
        self.assertEqual(set(self.workflow["on"]), {"workflow_dispatch"})
        self.assertEqual(
            set(self.workflow["on"]["workflow_dispatch"]["inputs"]),
            {"version", "frozen_commit", "readiness_run_id"},
        )
        self.assertEqual(self.workflow["permissions"]["contents"], "read")
        self.assertEqual(self.job["environment"], "root-promotion")
        self.assertEqual(self.job["permissions"]["contents"], "read")
        self.assertEqual(self.job["permissions"]["actions"], "read")
        self.assertEqual(
            [name for name, job in self.workflow["jobs"].items() if job.get("environment")],
            ["promote"],
        )
        secret_steps = [
            step
            for step in self.steps
            if "ROOT_TAG_PROMOTION_SSH_PRIVATE_KEY" in step.get("env", {})
        ]
        self.assertEqual(len(secret_steps), 1)
        self.assertEqual(
            secret_steps[0]["env"]["ROOT_TAG_PROMOTION_SSH_PRIVATE_KEY"],
            "${{ secrets.ROOT_TAG_PROMOTION_SSH_PRIVATE_KEY }}",
        )
        self.assertNotIn("contents: write", self.content)
        self.assertNotIn("actions: write", self.content)

    def test_untrusted_commit_is_rejected_before_checkout_or_repo_code(self):
        self.assertEqual(
            self.steps[0]["name"],
            "Require workflow dispatch from exact remote main tip",
        )
        preflight = self.steps[0]["run"]
        for required in (
            '"${GITHUB_REF}" != "refs/heads/main"',
            '"${FROZEN_COMMIT}" =~ ^[0-9a-f]{40}$',
            'git/ref/heads/main',
            '"${GITHUB_SHA}" != "${FROZEN_COMMIT}"',
            '"${remote_main}" != "${FROZEN_COMMIT}"',
        ):
            self.assertIn(required, preflight)

        checkout_index = next(
            index
            for index, step in enumerate(self.steps)
            if step.get("name") == "Checkout exact frozen source"
        )
        self.assertGreater(checkout_index, 0)
        self.assertEqual(
            self.steps[checkout_index]["with"]["ref"], "${{ inputs.frozen_commit }}"
        )
        self.assertEqual(
            self.steps[checkout_index]["with"]["persist-credentials"], "false"
        )

    def test_promotion_rechecks_source_readiness_and_component_publication(self):
        source = self.step("Verify exact merged-main source and root policy")["run"]
        self.assertIn("verify_release_source.py", source)
        self.assertIn('--intent publish', source)

        readiness = self.step("Require completed pre-root publication authority")["run"]
        self.assertIn("verify_readiness_run.sh", readiness)
        self.assertIn("--phase pre-root", readiness)

        components = self.step("Require exact public component releases")["run"]
        for required in (
            "mss-boot/${RELEASE_VERSION}|framework-release.yml",
            "admin/${RELEASE_VERSION}|admin-release.yml",
            "web/antd-v6/${RELEASE_VERSION}|frontend-v6-release.yml",
            '.event == "push"',
            ".head_sha == $commit",
            ".conclusion == \"success\"",
            "isDraft,isPrerelease",
        ):
            self.assertIn(required, components)

        authority = self.step("Require non-overlapping root-tag rule structure")[
            "run"
        ]
        for required in (
            "mapfile -t creation_names",
            "exactly the component and root creation rulesets may create release tags",
            "root-release-tag-controlled-creation",
            'include == ["refs/tags/v*"]',
            "release-tags-controlled-creation",
            'index("refs/tags/v*") == null',
            "component tag authority must not overlap root tags",
            "release-tags-immutable",
            '(.conditions.ref_name.exclude == [])',
        ):
            self.assertIn(required, authority)
        self.assertNotIn("bypass_actors", authority)

    def test_tag_creation_uses_only_the_dedicated_ssh_deploy_key(self):
        inspection = self.step("Inspect the exact immutable root tag")["run"]
        for required in (
            'git/ref/tags/${RELEASE_VERSION}',
            '"${object_type}" != "tag"',
            '.message == ("mss-boot-admin " + $version)',
            '.object.type == "commit"',
            ".object.sha == $commit",
            'echo "create=false" >> "${GITHUB_OUTPUT}"',
            'echo "create=true" >> "${GITHUB_OUTPUT}"',
        ):
            self.assertIn(required, inspection)

        creation_step = self.step(
            "Create the exact annotated root tag with the dedicated deploy key"
        )
        self.assertEqual(creation_step["if"], "steps.root-tag.outputs.create == 'true'")
        self.assertNotIn("GH_TOKEN", creation_step["env"])
        creation = creation_step["run"]
        for required in (
            'trap cleanup_root_tag_key EXIT',
            "trap 'exit 129' HUP",
            "trap 'exit 130' INT",
            "trap 'exit 143' TERM",
            'unset GIT_SSH_COMMAND ROOT_TAG_PROMOTION_SSH_PRIVATE_KEY',
            "ssh-keygen -y -P ''",
            "root-tag deploy key must use Ed25519",
            "github.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl",
            "SHA256:+DiY3wvvV6TuJJhbpZisF/zLDA0zPMSvHdkr4UvCOqU",
            "GlobalKnownHostsFile=/dev/null",
            "StrictHostKeyChecking=yes",
            "IdentitiesOnly=yes",
            'git tag -a "${RELEASE_VERSION}" "${FROZEN_COMMIT}"',
            '"git@github.com:${GITHUB_REPOSITORY}.git"',
            '"refs/tags/${RELEASE_VERSION}:refs/tags/${RELEASE_VERSION}"',
        ):
            self.assertIn(required, creation)
        self.assertNotIn("ssh-keyscan", creation)
        self.assertNotIn("git push origin", creation)
        self.assertNotIn("git tag -f", creation)
        self.assertNotIn("--force", creation)
        self.assertNotRegex(creation, re.compile(r"\bPAT\b|Authorization:"))

        verification = self.step("Require the exact annotated root tag")["run"]
        for required in (
            "waiting for the exact annotated root tag to propagate",
            '"${remote_ready}" != "true"',
        ):
            self.assertIn(required, verification)

    def test_deploy_key_push_creates_one_exact_natural_run_per_stage(self):
        natural = self.step("Require exact natural root-tag push runs")["run"]
        for required in (
            "require_natural_push_run",
            "Container Image ${RELEASE_VERSION}",
            "Root Release ${RELEASE_VERSION}",
            "-f event=push",
            '-f branch="${RELEASE_VERSION}"',
            '.event == "push"',
            ".head_branch == $branch",
            ".head_sha != $commit",
            ".head_sha == $commit",
            ".path == $path",
            '.display_title == $title',
            '"${exact_count}" -gt 1',
            '"${status}" == "completed" && "${conclusion}" != "success"',
            "did not appear",
        ):
            self.assertIn(required, natural)
        self.assertNotIn("gh workflow run", natural)
        self.assertNotIn("workflow_dispatch", natural)

        container_workflow = (
            REPOSITORY_ROOT / ".github" / "workflows" / "container.yml"
        ).read_text(encoding="utf-8")
        self.assertIn("format('Container Image {0}', github.ref_name)", container_workflow)
        self.assertIn("tags:\n      - 'v*.*.*'", container_workflow)

        release_workflow = (
            REPOSITORY_ROOT / ".github" / "workflows" / "release.yml"
        ).read_text(encoding="utf-8")
        self.assertIn("format('Root Release {0}', github.ref_name)", release_workflow)
        self.assertIn("tags:\n      - 'v*.*.*'", release_workflow)
        readiness_step = release_workflow.split(
            "- name: Require selected pre-root readiness attestation", 1
        )[1].split("- name:", 1)[0]
        self.assertIn("if: github.ref_type == 'tag'", readiness_step)
        self.assertNotIn("needs.metadata.outputs.publish == 'true'", readiness_step)

    def test_all_run_blocks_are_valid_bash(self):
        for index, step in enumerate(self.steps):
            script = step.get("run")
            if script is None:
                continue
            sanitized = GITHUB_EXPRESSION.sub("gha_expression", script)
            result = subprocess.run(
                ["bash", "-n"],
                input=sanitized,
                text=True,
                capture_output=True,
                check=False,
            )
            self.assertEqual(
                result.returncode,
                0,
                msg=f"invalid bash in step {index} ({step.get('name')}): {result.stderr}",
            )


if __name__ == "__main__":
    unittest.main()
