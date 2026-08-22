import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


TOOLS_DIR = Path(__file__).resolve().parent
REPOSITORY_ROOT = TOOLS_DIR.parents[1]
POLICY_PATH = REPOSITORY_ROOT / ".mss" / "release-policy.yaml"
sys.path.insert(0, str(TOOLS_DIR))
import release_phase_evidence as EVIDENCE  # noqa: E402


class ReleasePhaseEvidenceTest(unittest.TestCase):
    def plan(self, name, acceptance):
        return {
            "success": True,
            "feature": {"name": name},
            "acceptance": acceptance,
        }

    def criterion(self, identifier, command, *, phase="feature-freeze", required=True):
        return {
            "id": identifier,
            "phase": phase,
            "required": required,
            "evidence": [{"type": "command", "value": command}],
        }

    def test_collects_required_phase_commands_and_deduplicates_references(self):
        command = "python3 -m unittest tools/release/test_check_release_policy.py"
        plans = [
            self.plan("one", [self.criterion("a", command)]),
            self.plan(
                "two",
                [
                    self.criterion("b", command),
                    self.criterion("later", "python3 -V", phase="pre-root"),
                    self.criterion("optional", "python3 -V", required=False),
                ],
            ),
        ]
        steps, review = EVIDENCE.collect_phase_commands(plans, phase="feature-freeze")
        self.assertEqual(len(steps), 1)
        self.assertEqual(len(steps[0].references), 2)
        self.assertEqual(review, [])

    def test_qualification_contract_requires_every_feature_to_be_selected_or_excluded(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            feature_dir = root / ".mss" / "features"
            feature_dir.mkdir(parents=True)
            (feature_dir / "one.yaml").write_text("kind: Feature\n", encoding="utf-8")
            (feature_dir / "old.yaml").write_text("kind: Feature\n", encoding="utf-8")
            contract = root / ".mss" / "release-qualification.json"
            contract.write_text(
                json.dumps(
                    {
                        "schema": EVIDENCE.QUALIFICATION_SCHEMA,
                        "targetVersion": "v1.1.0",
                        "features": [".mss/features/one.yaml"],
                        "excludedFeatures": [],
                    }
                ),
                encoding="utf-8",
            )
            with self.assertRaisesRegex(EVIDENCE.PhaseEvidenceError, "undeclared features"):
                EVIDENCE.load_qualification(root, Path(".mss/release-qualification.json"), "v1.1.0")
            value = json.loads(contract.read_text(encoding="utf-8"))
            value["excludedFeatures"] = [
                {"path": ".mss/features/old.yaml", "reason": "published historical release"}
            ]
            contract.write_text(json.dumps(value), encoding="utf-8")
            selected = EVIDENCE.load_qualification(
                root, Path(".mss/release-qualification.json"), "v1.1.0"
            )
            self.assertEqual([path.name for path in selected], ["one.yaml"])

    def test_manual_and_report_evidence_remain_explicit_release_manager_review(self):
        plan = self.plan(
            "release",
            [
                {
                    "id": "authority",
                    "phase": "pre-framework",
                    "required": True,
                    "evidence": [
                        {"type": "manual", "value": "approve exact SHA"},
                        {"type": "report", "value": "recovery report"},
                    ],
                }
            ],
        )
        steps, review = EVIDENCE.collect_phase_commands([plan], phase="pre-framework")
        self.assertEqual(steps, [])
        self.assertEqual({item["type"] for item in review}, {"manual", "report", "no-command-evidence"})

    def test_rejects_shell_controls_and_untrusted_bash(self):
        rejected = (
            "python3 -V && python3 -V",
            "bash -c 'echo unsafe'",
            "curl https://example.invalid",
        )
        for command in rejected:
            with self.subTest(command=command):
                with self.assertRaises(EVIDENCE.PhaseEvidenceError):
                    EVIDENCE.parse_command(command)

    def test_named_raw_go_test_is_collected_but_marked_nonexact(self):
        command = "go test ./internal/mss/project -run TestCatalog"
        steps, review = EVIDENCE.collect_phase_commands(
            [self.plan("catalog", [self.criterion("status", command)])],
            phase="feature-freeze",
        )
        self.assertEqual(len(steps), 1)
        self.assertEqual(review[0]["type"], "non-exact-command")
        self.assertTrue(review[0]["required"])

    def test_unknown_checked_in_executor_is_visible_review_instead_of_hidden_plan_failure(self):
        steps, review = EVIDENCE.collect_phase_commands(
            [
                self.plan(
                    "browser",
                    [self.criterion("browser", "in-app-browser /suppliers")],
                )
            ],
            phase="feature-freeze",
        )
        self.assertEqual(steps, [])
        self.assertEqual(review[0]["type"], "unsupported-command")
        self.assertIn("not allowlisted", review[0]["reason"])

    def test_allows_test_evidence_regex_and_checked_in_release_script(self):
        argv = EVIDENCE.parse_command(
            "go run ./cmd/mss test evidence --directory admin --package ./models "
            "--run '^Test(One|Two)$' --require TestOne --require TestTwo"
        )
        self.assertIn("^Test(One|Two)$", argv.argv)
        self.assertEqual(
            EVIDENCE.parse_command("bash tools/release/verify_readiness_run.sh --help").argv[:2],
            ["bash", "tools/release/verify_readiness_run.sh"],
        )
        for script in (
            "tools/compatibility/test-admin-external-consumer.sh",
            "tools/compatibility/test-thin-host-external-consumer.sh",
        ):
            with self.subTest(script=script):
                self.assertEqual(
                    EVIDENCE.parse_command(f"bash {script}").argv,
                    ["bash", script],
                )

    def test_rejects_nonqualification_bash_scripts(self):
        for script in (
            "tools/compatibility/process-groups.sh",
            "tools/release/../compatibility/test-admin-external-consumer.sh",
            "scripts/arbitrary.sh",
        ):
            with self.subTest(script=script):
                with self.assertRaisesRegex(
                    EVIDENCE.PhaseEvidenceError, "explicitly allowlisted"
                ):
                    EVIDENCE.parse_command(f"bash {script}")

    def test_parses_safe_repository_cwd_and_allowlisted_environment_without_shell(self):
        command = EVIDENCE.parse_command(
            "cd admin && GOWORK=off GOTOOLCHAIN=local go test ./models"
        )
        self.assertEqual(command.working_directory, "admin")
        self.assertEqual(command.environment, {"GOWORK": "off", "GOTOOLCHAIN": "local"})
        self.assertEqual(command.argv, ["go", "test", "./models"])

    def test_execute_steps_records_digests_and_failure_without_shell(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            steps = [
                EVIDENCE.CommandStep("pass", ["python3", "-c", "print('ok')"], []),
                EVIDENCE.CommandStep("fail", ["python3", "-c", "raise SystemExit(3)"], []),
            ]
            results = EVIDENCE.execute_steps(
                steps,
                root=root,
                logs_dir=Path("reports/logs"),
                timeout_seconds=10,
            )
            self.assertTrue(results[0]["success"])
            self.assertEqual(results[1]["exitCode"], 3)
            self.assertFalse(results[1]["success"])
            self.assertTrue((root / results[0]["log"]).is_file())
            self.assertEqual(len(results[0]["stdoutSha256"]), 64)

    def test_atomic_report_writer_produces_strict_json(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "reports" / "phase.json"
            EVIDENCE._write_json_atomic(output, {"schema": EVIDENCE.SCHEMA})
            self.assertEqual(json.loads(output.read_text(encoding="utf-8"))["schema"], EVIDENCE.SCHEMA)
            self.assertTrue(output.read_text(encoding="utf-8").endswith("\n"))

    def test_run_binding_rejects_any_dirty_frozen_worktree(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            policy = root / ".mss" / "release-policy.yaml"
            policy.parent.mkdir()
            policy.write_bytes(POLICY_PATH.read_bytes())
            subprocess.run(["git", "init", "-q"], cwd=root, check=True)
            subprocess.run(["git", "config", "user.email", "release@example.invalid"], cwd=root, check=True)
            subprocess.run(["git", "config", "user.name", "Release Test"], cwd=root, check=True)
            subprocess.run(["git", "config", "core.autocrlf", "false"], cwd=root, check=True)
            subprocess.run(["git", "add", "."], cwd=root, check=True)
            subprocess.run(["git", "commit", "-qm", "fixture"], cwd=root, check=True)
            commit = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=root, text=True).strip()
            EVIDENCE.validate_binding(
                root,
                policy_path=policy,
                target_version="v1.3.2",
                commit=commit,
                require_clean=True,
            )
            (root / "untracked.txt").write_text("dirty", encoding="utf-8")
            with self.assertRaisesRegex(EVIDENCE.PhaseEvidenceError, "clean repository"):
                EVIDENCE.validate_binding(
                    root,
                    policy_path=policy,
                    target_version="v1.3.2",
                    commit=commit,
                    require_clean=True,
                )


if __name__ == "__main__":
    unittest.main()
