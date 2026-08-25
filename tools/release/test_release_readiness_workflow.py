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
RUNTIME_PHASE_CONDITION = (
    "${{ inputs.phase == 'feature-freeze' || inputs.phase == 'pre-framework' || "
    "inputs.phase == 'pre-root' }}"
)
COMMAND_EXECUTION_CONDITION = (
    "${{ success() && (inputs.phase == 'feature-freeze' || "
    "inputs.phase == 'pre-framework' || inputs.phase == 'pre-root') }}"
)


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
        self.assertEqual(phase["if"], COMMAND_EXECUTION_CONDITION)
        self.assertEqual(phase["env"]["READINESS_PHASE"], "${{ inputs.phase }}")
        self.assertEqual(phase["env"]["NODE_AUTH_TOKEN"], "${{ github.token }}")
        self.assertEqual(self.workflow["permissions"]["packages"], "read")
        self.assertIn("MSS_SUPPLIER_TEST_MYSQL_DSN", phase["env"])
        self.assertIn("MSS_SUPPLIER_TEST_POSTGRES_DSN", phase["env"])
        self.assertIn("release_phase_evidence.py run", phase["run"])
        self.assertIn('--phase "${READINESS_PHASE}"', phase["run"])
        self.assertEqual(
            self.step("Install documentation dependencies")["run"],
            "corepack pnpm@9.15.9 --dir docs install --frozen-lockfile",
        )
        frontend_install = self.step("Install frontend dependencies")["run"]
        self.assertEqual(
            frontend_install,
            "pnpm --dir web/antd-v6 install --frozen-lockfile --ignore-scripts",
        )
        playwright_install = self.step("Install Playwright Chromium")
        self.assertEqual(playwright_install["if"], RUNTIME_PHASE_CONDITION)
        self.assertEqual(playwright_install["working-directory"], "web/antd-v6")
        self.assertEqual(
            playwright_install["run"],
            "pnpm exec playwright install --with-deps chromium",
        )
        self.assertLess(
            self.steps.index(playwright_install),
            self.steps.index(phase),
        )
        self.assertEqual(self.step("Setup pnpm")["with"]["version"], "10.34.5")

    def test_publication_authority_phases_execute_and_only_checkpoint_plans(self):
        for step_name in (
            "Setup pnpm",
            "Setup Node",
            "Install frontend dependencies",
            "Install Playwright Chromium",
        ):
            self.assertEqual(self.step(step_name)["if"], RUNTIME_PHASE_CONDITION)

        execute = self.step("Execute phase-scoped Feature command evidence")
        self.assertEqual(execute["if"], COMMAND_EXECUTION_CONDITION)
        self.assertIn("release_phase_evidence.py run", execute["run"])
        self.assertIn('--phase "${READINESS_PHASE}"', execute["run"])
        self.assertIn(
            ".mss/reports/release-phase-command-evidence.json", execute["run"]
        )

        plan = self.step("Plan phase-scoped Feature command evidence")
        self.assertEqual(plan["if"], "${{ success() && inputs.phase == 'checkpoint' }}")
        self.assertIn("release_phase_evidence.py plan", plan["run"])
        self.assertIn('--phase "${READINESS_PHASE}"', plan["run"])

    def test_v134_qualification_selects_package_first_and_presentation_features(self):
        selected = PHASE_EVIDENCE.load_qualification(
            REPOSITORY_ROOT,
            Path(".mss/release-qualification.json"),
            "v1.3.4",
        )
        self.assertEqual(
            [path.relative_to(REPOSITORY_ROOT).as_posix() for path in selected],
            [
                ".mss/features/foundation-v1-3-4-release-recovery.yaml",
                ".mss/features/admin-presentation-configuration.yaml",
                ".mss/features/admin-presentation-publication-workflow.yaml",
            ],
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

    def test_v134_checkpoint_and_release_phases_are_exact_and_executable(self):
        feature_paths = PHASE_EVIDENCE.load_qualification(
            REPOSITORY_ROOT,
            Path(".mss/release-qualification.json"),
            "v1.3.4",
        )
        plans = []
        for feature_path in feature_paths:
            feature = yaml.safe_load(feature_path.read_text(encoding="utf-8"))
            plans.append(
                {
                    "feature": {"name": feature["metadata"]["name"]},
                    "acceptance": feature["spec"]["acceptance"],
                }
            )
        commands_by_phase = {}
        for phase in ("checkpoint", "feature-freeze", "pre-framework", "pre-root"):
            steps, review = PHASE_EVIDENCE.collect_phase_commands(plans, phase=phase)
            blockers = [
                item
                for item in review
                if item.get("type") in {"non-exact-command", "unsupported-command"}
            ]
            self.assertEqual(blockers, [], msg=f"{phase}: {blockers}")
            commands_by_phase[phase] = {
                " ".join(
                    [
                        step.working_directory,
                        *(f"{name}={value}" for name, value in sorted(step.environment.items())),
                        *step.argv,
                    ]
                )
                for step in steps
            }
        self.assertEqual(
            commands_by_phase["checkpoint"],
            {
                ". python3 -m unittest discover -s tools/release -p test_*.py",
                ". bash tools/install/test-install-mss.sh",
                ". go run ./cmd/mss spec validate .mss/features/admin-presentation-configuration.yaml --format json",
                ". go test ./internal/mss/spec",
                ". corepack pnpm@10.34.5 --dir web/antd-v6 test src/shared/presentation",
                ". corepack pnpm@10.34.5 --dir web/antd-v6 lint",
                ". corepack pnpm@9.15.9 --dir docs build",
                ". go run ./cmd/mss spec validate .mss/features/admin-presentation-publication-workflow.yaml --format json",
                ". corepack pnpm@10.34.5 --dir web/antd-v6 test src/modules/presentation-config src/shared/presentation",
                "admin GOWORK=off go test ./presentation ./models ./service ./apis ./middleware ./cmd/migrate/migration/system ./router",
                ". corepack pnpm@10.34.5 --dir web/antd-v6 tsc",
                ". corepack pnpm@10.34.5 --dir web/antd-v6 build",
                ". go run ./cmd/mss verify --changed",
            },
        )
        self.assertEqual(
            commands_by_phase["feature-freeze"],
            {
                ". bash tools/compatibility/test-standalone-mss-consumer.sh",
                ". bash tools/compatibility/test-standalone-mss-consumer.sh --lifecycle",
                ". bash tools/compatibility/test-standalone-mss-consumer.sh --upgrade",
                ". bash tools/compatibility/test-thin-host-external-consumer.sh",
                ". python3 tools/docs/check_current_docs.py",
                ". corepack pnpm@9.15.9 --dir docs build",
            },
        )
        self.assertEqual(
            commands_by_phase["pre-framework"],
            {
                ". python3 tools/release/verify_framework_admin_checksum.py --version v1.3.4",
                "mss-boot GOWORK=off go test ./...",
                ". bash tools/compatibility/test-admin-external-consumer.sh",
                ". bash tools/compatibility/test-standalone-mss-consumer.sh",
                ". python3 tools/release/check_release_policy.py --component framework --version v1.3.4 --tag mss-boot/v1.3.4 --intent qualify",
            },
        )
        self.assertEqual(
            commands_by_phase["pre-root"],
            {
                ". python3 tools/release/check_release_policy.py --component root --version v1.3.4 --tag v1.3.4 --intent qualify",
                ". bash tools/compatibility/test-standalone-mss-consumer.sh --public-packages --lifecycle --upgrade",
            },
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
