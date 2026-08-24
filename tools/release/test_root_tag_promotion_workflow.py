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

    def test_only_manual_protected_job_can_write_root_tag(self):
        self.assertEqual(set(self.workflow["on"]), {"workflow_dispatch"})
        self.assertEqual(
            set(self.workflow["on"]["workflow_dispatch"]["inputs"]),
            {"version", "frozen_commit", "readiness_run_id"},
        )
        self.assertEqual(self.workflow["permissions"]["contents"], "read")
        self.assertEqual(self.job["environment"], "root-promotion")
        self.assertEqual(self.job["permissions"]["contents"], "write")
        self.assertEqual(self.job["permissions"]["actions"], "write")
        self.assertEqual(
            [name for name, job in self.workflow["jobs"].items() if job.get("environment")],
            ["promote"],
        )

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

    def test_tag_creation_is_exact_and_resumable(self):
        promotion = self.step("Create or resume the exact immutable root tag")["run"]
        for required in (
            'git/ref/tags/${RELEASE_VERSION}',
            '"${object_type}" != "tag"',
            '.object.type == "commit" and .object.sha == $commit',
            'git tag -a "${RELEASE_VERSION}" "${FROZEN_COMMIT}"',
            'git push origin "refs/tags/${RELEASE_VERSION}"',
            "checking candidate runs",
            "waiting for the exact annotated root tag to propagate",
            '"${remote_ready}" != "true"',
        ):
            self.assertIn(required, promotion)
        self.assertNotIn("git tag -f", promotion)
        self.assertNotIn("--force", promotion)
        self.assertNotIn("delete", promotion.lower())

    def test_actions_dispatch_candidates_because_token_tag_push_does_not_recurse(self):
        dispatch = self.step("Dispatch exact-tag root image and candidate assembly")["run"]
        for required in (
            "exact_run_state",
            "Root Image publish ${RELEASE_VERSION}",
            "Root Release candidate ${RELEASE_VERSION}",
            '.display_title == $title',
            '.head_sha == $commit',
            '.status != "completed" or .conclusion == "success"',
            '"live-or-success"',
            '"failed"',
            '"missing"',
            "already failed; inspect or rerun that run",
            "gh workflow run container.yml",
            "gh workflow run release.yml",
            '--ref "${RELEASE_VERSION}"',
            "-f publish=true",
            "-f publish=false",
            '-f readiness_run_id="${READINESS_RUN_ID}"',
        ):
            self.assertIn(required, dispatch)

        container_workflow = (
            REPOSITORY_ROOT / ".github" / "workflows" / "container.yml"
        ).read_text(encoding="utf-8")
        self.assertIn("Root Image publish {0}", container_workflow)

        release_workflow = (
            REPOSITORY_ROOT / ".github" / "workflows" / "release.yml"
        ).read_text(encoding="utf-8")
        self.assertIn("Root Release candidate {0}", release_workflow)
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
