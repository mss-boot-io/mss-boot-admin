import unittest
from pathlib import Path

import yaml


REPOSITORY_ROOT = Path(__file__).resolve().parents[2]
WORKFLOW_DIR = REPOSITORY_ROOT / ".github" / "workflows"
RETIRED_WORKFLOWS = (
    "docs-drift.yml",
    "frontend-cloudflare.yml",
    "release-draft.yml",
    "web-editor-usage.yml",
    "web-lockfile-finalize.yml",
)


def load_workflow(name):
    path = WORKFLOW_DIR / name
    return yaml.load(path.read_text(encoding="utf-8"), Loader=yaml.BaseLoader)


class WorkflowGovernanceTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.workflows = {
            path.name: load_workflow(path.name)
            for path in sorted(WORKFLOW_DIR.glob("*.yml"))
        }

    def test_admin_ci_context_is_repository_unique(self):
        contexts = []
        for workflow_name, workflow in self.workflows.items():
            for job_name, job in workflow.get("jobs", {}).items():
                if job.get("name", job_name) == "admin-ci":
                    contexts.append((workflow_name, job_name))

        self.assertEqual(contexts, [("ci.yml", "build")])
        self.assertEqual(
            self.workflows["ci.yml"]["jobs"]["build"]["name"], "admin-ci"
        )

    def test_macos_portable_tests_use_the_real_runner_temp_directory(self):
        job = self.workflows["agent-native-ci.yml"]["jobs"][
            "cross-platform-agent-tools"
        ]
        self.assertIn("macos-15", job["strategy"]["matrix"]["os"])
        test_step = next(
            step
            for step in job["steps"]
            if step.get("name") == "Test portable Agent packages"
        )
        self.assertEqual(test_step["env"]["TMPDIR"], "${{ runner.temp }}")

    def test_retired_workflows_stay_absent(self):
        present = [
            name for name in RETIRED_WORKFLOWS if (WORKFLOW_DIR / name).exists()
        ]
        self.assertEqual(present, [])

    def test_pr_guard_owns_the_documentation_contract(self):
        workflow = self.workflows["pr-guard.yml"]
        docs_job = workflow["jobs"]["docs-contract"]
        self.assertEqual(docs_job["name"], "docs-contract")
        step = next(
            item
            for item in docs_job["steps"]
            if item.get("name")
            == "Require documentation for contract and workflow changes"
        )
        script = step["run"]
        for required in (
            "gh api",
            "--paginate",
            "--jq '.[].filename'",
            "pulls/${PR_NUMBER}/files?per_page=100",
            "docsChanged",
            "governedChange",
            "migration",
            "sql",
            r"^\.github\/workflows\/",
        ):
            with self.subTest(required=required):
                self.assertIn(required, script)

    def test_scorecard_does_not_run_on_every_main_push(self):
        triggers = self.workflows["scorecard.yml"]["on"]
        self.assertEqual(
            set(triggers),
            {"branch_protection_rule", "schedule", "workflow_dispatch"},
        )
        self.assertNotIn("push", triggers)

    def test_agent_ci_runs_this_governance_suite(self):
        agent_workflow = self.workflows["agent-native-ci.yml"]
        for event in ("push", "pull_request"):
            self.assertIn(".github/workflows/**", agent_workflow["on"][event]["paths"])

        steps = agent_workflow["jobs"]["contracts-and-go"]["steps"]
        governance = next(
            step
            for step in steps
            if step.get("name") == "Verify release and workflow governance"
        )
        self.assertIn("test_check_release_policy.py", governance["run"])
        self.assertIn("test_verify_release_source.py", governance["run"])
        self.assertIn("test_workflow_governance.py", governance["run"])


if __name__ == "__main__":
    unittest.main()
