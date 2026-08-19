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
            paths = agent_workflow["on"][event]["paths"]
            self.assertIn(".github/workflows/**", paths)
            self.assertIn("tools/ci/**", paths)
            self.assertNotIn("docs/docs/agent/**", paths)
            self.assertNotIn(
                "docs/docs/architecture/agent-native-foundation.zh-CN.md",
                paths,
            )

        steps = agent_workflow["jobs"]["contracts-and-go"]["steps"]
        governance = next(
            step
            for step in steps
            if step.get("name") == "Verify release and workflow governance"
        )
        self.assertIn("test_detect_component_scope.py", governance["run"])
        self.assertIn("test_check_release_policy.py", governance["run"])
        self.assertIn("test_verify_release_source.py", governance["run"])
        self.assertIn("test_workflow_governance.py", governance["run"])

    def test_reusable_scope_classifier_owns_component_routing(self):
        workflow = self.workflows["component-scope.yml"]
        outputs = workflow["on"]["workflow_call"]["outputs"]
        self.assertEqual(set(outputs), {"scope", "go_modules", "changed_count"})
        detect = workflow["jobs"]["detect"]
        self.assertEqual(detect["outputs"]["scope"], "${{ steps.scope.outputs.scope }}")
        self.assertEqual(
            detect["outputs"]["go_modules"],
            "${{ steps.scope.outputs.go_modules }}",
        )
        step = next(
            item
            for item in detect["steps"]
            if item.get("name") == "Classify changed component scope"
        )
        self.assertIn("tools/ci/detect_component_scope.py", step["run"])
        self.assertIn('--github-output "${GITHUB_OUTPUT}"', step["run"])

    def test_admin_required_context_routes_heavy_jobs_by_component(self):
        jobs = self.workflows["ci.yml"]["jobs"]
        for name in ("test", "race", "static-and-module", "compile"):
            condition = jobs[name]["if"]
            self.assertIn("scope == 'admin'", condition)
            self.assertIn("scope == 'shared'", condition)
            self.assertNotIn("scope == 'framework'", condition)

        compatibility = jobs["compatibility"]["if"]
        for scope in ("admin", "framework", "shared"):
            self.assertIn(f"scope == '{scope}'", compatibility)

        aggregate = next(
            step
            for step in jobs["build"]["steps"]
            if step.get("name") == "Require every Admin check to pass"
        )["run"]
        for route in ("admin|shared)", "framework)", "docs|web)"):
            self.assertIn(route, aggregate)

    def test_go_vulnerability_scans_follow_owned_modules(self):
        workflow = self.workflows["govulncheck.yml"]
        self.assertEqual(
            set(workflow["on"]["push"]["paths-ignore"]),
            {"docs/**", "web/**"},
        )
        jobs = workflow["jobs"]
        self.assertEqual(jobs["root"]["if"], "needs.scope.outputs.scope == 'shared'")
        self.assertEqual(
            jobs["nested-modules"]["strategy"]["matrix"]["module"],
            "${{ fromJSON(needs.scope.outputs.go_modules) }}",
        )
        nested_steps = jobs["nested-modules"]["steps"]
        skip = next(
            step
            for step in nested_steps
            if step.get("name") == "Skip Go vulnerability scan for this component"
        )
        self.assertEqual(skip["if"], "matrix.module == 'none'")
        for step in nested_steps:
            if step.get("name") in {
                "Checkout",
                "Setup Go",
                "Install govulncheck",
                "Scan nested module",
            }:
                self.assertEqual(step["if"], "matrix.module != 'none'")

    def test_codeql_languages_follow_component_ownership(self):
        workflow = self.workflows["codeql.yml"]
        matrix = workflow["jobs"]["analyze"]["strategy"]["matrix"]["include"]
        scopes = {item["language"]: item["scopes"] for item in matrix}
        self.assertEqual(scopes["actions"], '["shared"]')
        self.assertEqual(scopes["go"], '["shared","admin","framework"]')
        self.assertEqual(scopes["javascript-typescript"], '["shared","web"]')

        steps = workflow["jobs"]["analyze"]["steps"]
        for step in steps:
            condition = step.get("if", "")
            if step.get("name") == "Skip analysis outside the changed component":
                self.assertIn("!contains(fromJSON(matrix.scopes)", condition)
            elif step.get("name") in {
                "Checkout",
                "Initialize CodeQL",
                "Perform CodeQL Analysis",
            }:
                self.assertIn("contains(fromJSON(matrix.scopes)", condition)
                self.assertNotIn("!contains", condition)

    def test_component_owned_workflows_do_not_watch_other_owned_roots(self):
        frontend = self.workflows["frontend-v6-ci.yml"]
        for event in ("push", "pull_request"):
            paths = frontend["on"][event]["paths"]
            self.assertNotIn("admin/**", paths)
            self.assertNotIn("admin/modules/**/module.yaml", paths)
            self.assertNotIn("mss-boot/**", paths)

        mirror = self.workflows["mirror.yml"]
        self.assertIn("docs/**", mirror["on"]["push"]["paths-ignore"])


if __name__ == "__main__":
    unittest.main()
