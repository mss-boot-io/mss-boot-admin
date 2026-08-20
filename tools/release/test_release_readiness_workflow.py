import re
import subprocess
import sys
import unittest
from pathlib import Path

import yaml


REPOSITORY_ROOT = Path(__file__).resolve().parents[2]
TOOLS_DIR = Path(__file__).resolve().parent
sys.path.insert(0, str(TOOLS_DIR))
import release_phase_evidence as PHASE_EVIDENCE  # noqa: E402


WORKFLOW_PATH = REPOSITORY_ROOT / ".github" / "workflows" / "release-readiness.yml"
FEATURE_PATH = REPOSITORY_ROOT / ".mss" / "features" / "foundation-v1-2-3-release.yaml"
GITHUB_EXPRESSION = re.compile(r"\$\{\{.*?\}\}", re.DOTALL)


class ReleaseReadinessWorkflowTest(unittest.TestCase):
    def setUp(self):
        self.content = WORKFLOW_PATH.read_text(encoding="utf-8")
        self.workflow = yaml.load(self.content, Loader=yaml.BaseLoader)
        self.job = self.workflow["jobs"]["full-verification"]
        self.steps = self.job["steps"]

    def step(self, name):
        return next(step for step in self.steps if step.get("name") == name)

    def test_manual_evidence_inputs_and_artifact_are_exact_sha_bound(self):
        inputs = self.workflow["on"]["workflow_dispatch"]["inputs"]
        for name in (
            "in_app_browser_evidence_commit",
            "in_app_browser_evidence_ref",
            "blueprint_rehearsal_evidence_commit",
            "blueprint_rehearsal_evidence_ref",
        ):
            self.assertIn(name, inputs)

        decision = self.step("Bind manual feature-freeze evidence to the frozen commit")
        self.assertEqual(decision["if"], "${{ inputs.phase == 'feature-freeze' }}")
        for required in (
            "release_qualification_decision.py",
            '--commit "${FROZEN_COMMIT}"',
            '--browser-evidence-commit "${BROWSER_EVIDENCE_COMMIT}"',
            '--blueprint-evidence-commit "${BLUEPRINT_EVIDENCE_COMMIT}"',
            ".mss/reports/release-qualification-decision.json",
        ):
            self.assertIn(required, decision["run"])

        upload = self.step("Upload exact feature-freeze decision")
        self.assertEqual(
            upload["with"]["name"], "release-qualification-decision-${{ github.run_id }}"
        )
        self.assertEqual(
            upload["with"]["path"], ".mss/reports/release-qualification-decision.json"
        )

    def test_workflow_runs_only_scoped_feature_commands(self):
        self.assertNotIn("verify --all", self.content)
        self.assertNotIn("eval run --all", self.content)
        self.assertNotIn("RustFS", self.content)
        phase = self.step("Execute phase-scoped Feature command evidence")
        self.assertIn("MSS_SUPPLIER_TEST_MYSQL_DSN", phase["env"])
        self.assertIn("MSS_SUPPLIER_TEST_POSTGRES_DSN", phase["env"])
        self.assertIn("release_phase_evidence.py run", phase["run"])
        self.assertEqual(
            self.step("Install documentation dependencies")["run"],
            "corepack pnpm@9.15.9 --dir docs install --frozen-lockfile",
        )
        frontend_install = self.step("Install frontend dependencies")["run"]
        self.assertEqual(
            frontend_install,
            "corepack pnpm@10.34.5 --dir web/antd-v6 install --frozen-lockfile --ignore-scripts",
        )
        playwright_install = self.step("Install Playwright Chromium")
        self.assertEqual(
            playwright_install["if"], "${{ inputs.phase == 'feature-freeze' }}"
        )
        self.assertEqual(playwright_install["working-directory"], "web/antd-v6")
        self.assertEqual(
            playwright_install["run"],
            "corepack pnpm@10.34.5 exec playwright install --with-deps chromium",
        )
        self.assertLess(
            self.steps.index(playwright_install),
            self.steps.index(phase),
        )
        self.assertEqual(self.step("Setup pnpm")["with"]["version"], "9.15.9")

    def test_v130_rc1_qualification_selects_the_distribution_feature(self):
        selected = PHASE_EVIDENCE.load_qualification(
            REPOSITORY_ROOT,
            Path(".mss/release-qualification.json"),
            "v1.3.0-rc.1",
        )
        self.assertEqual(
            [path.relative_to(REPOSITORY_ROOT).as_posix() for path in selected],
            [".mss/features/complete-admin-distribution-thin-host.yaml"],
        )
        contract = yaml.safe_load(
            (REPOSITORY_ROOT / ".mss" / "release-qualification.json").read_text(
                encoding="utf-8"
            )
        )
        self.assertTrue(contract["excludedFeatures"])
        self.assertTrue(
            all(
                item["reason"].startswith("carry-forward:")
                for item in contract["excludedFeatures"]
            )
        )

    def test_historical_v123_release_feature_has_scoped_exact_commands(self):
        feature = yaml.safe_load(FEATURE_PATH.read_text(encoding="utf-8"))
        plan = {
            "feature": {"name": feature["metadata"]["name"]},
            "acceptance": feature["spec"]["acceptance"],
        }
        steps, review = PHASE_EVIDENCE.collect_phase_commands(
            [plan], phase="feature-freeze"
        )
        blockers = [
            item
            for item in review
            if item.get("type") in {"non-exact-command", "unsupported-command"}
        ]
        self.assertEqual(blockers, [])
        commands = [
            " ".join(
                [
                    step.working_directory,
                    *(f"{name}={value}" for name, value in sorted(step.environment.items())),
                    *step.argv,
                ]
            )
            for step in steps
        ]
        joined = "\n".join(commands)
        for required in (
            "mss-boot GOWORK=off go test ./...",
            ". go test ./...",
            "admin go test ./...",
            "web/antd-v6 corepack pnpm@10.34.5 test:e2e",
            "web/antd-v6 corepack pnpm@10.34.5 build:release",
            ". corepack pnpm@9.15.9 --dir docs build",
        ):
            self.assertIn(required, joined)
        self.assertNotIn("verify --all", joined)
        self.assertNotIn("eval run --all", joined)

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
