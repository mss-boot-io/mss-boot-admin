import copy
import hashlib
import json
import re
import sys
import tempfile
import unittest
from pathlib import Path


TOOLS_DIR = Path(__file__).resolve().parent
sys.path.insert(0, str(TOOLS_DIR))
import release_readiness_attestation as ATTESTATION  # noqa: E402


REPOSITORY_ROOT = Path(__file__).resolve().parents[2]
POLICY_PATH = REPOSITORY_ROOT / ".mss" / "release-policy.yaml"
COMMIT = "0123456789abcdef0123456789abcdef01234567"
RUN_ID = 123456789
RUN_URL = (
    "https://github.com/mss-boot-io/mss-boot-admin/actions/runs/123456789"
)
TARGET_VERSION = "v1.3.0-rc.3"


class ReleaseReadinessAttestationTest(unittest.TestCase):
    def checkpoint(self):
        return ATTESTATION.build_attestation(
            policy_path=POLICY_PATH,
            target_version=TARGET_VERSION,
            commit=COMMIT,
            phase="checkpoint",
            workflow_run_id=RUN_ID,
            workflow_run_url=RUN_URL,
            publication_authority=False,
        )

    def policy_with_readiness(self, directory: str, ready: bool) -> Path:
        path = Path(directory) / "release-policy.yaml"
        path.write_text(
            re.sub(
                r"publicationWorkflowsReady: (?:true|false)",
                f"publicationWorkflowsReady: {str(ready).lower()}",
                POLICY_PATH.read_text(encoding="utf-8"),
            ),
            encoding="utf-8",
        )
        return path

    def ready_policy(self, directory: str) -> Path:
        return self.policy_with_readiness(directory, True)

    def test_checkpoint_attestation_binds_exact_v130_rc3_metadata(self):
        attestation = self.checkpoint()
        self.assertEqual(set(attestation), ATTESTATION.REQUIRED_KEYS)
        self.assertEqual(attestation["schema"], ATTESTATION.SCHEMA)
        self.assertEqual(attestation["targetVersion"], TARGET_VERSION)
        self.assertEqual(attestation["commit"], COMMIT)
        self.assertEqual(attestation["phase"], "checkpoint")
        self.assertEqual(attestation["workflowRunId"], RUN_ID)
        self.assertEqual(attestation["workflowRunUrl"], RUN_URL)
        self.assertIs(attestation["publicationAuthority"], False)
        self.assertEqual(
            attestation["policySha256"],
            hashlib.sha256(POLICY_PATH.read_bytes()).hexdigest(),
        )
        ATTESTATION.validate_attestation(
            attestation,
            policy_path=POLICY_PATH,
            target_version=TARGET_VERSION,
            commit=COMMIT,
            phase="checkpoint",
            workflow_run_id=RUN_ID,
            workflow_run_url=RUN_URL,
            intent="qualify",
        )

    def test_disabled_publication_policy_rejects_authoritative_or_later_phase(self):
        with tempfile.TemporaryDirectory() as directory:
            policy = self.policy_with_readiness(directory, False)
            with self.assertRaisesRegex(
                ATTESTATION.AttestationError, "permits only checkpoint"
            ):
                ATTESTATION.build_attestation(
                    policy_path=policy,
                    target_version=TARGET_VERSION,
                    commit=COMMIT,
                    phase="pre-framework",
                    workflow_run_id=RUN_ID,
                    workflow_run_url=RUN_URL,
                    publication_authority=True,
                )
            with self.assertRaisesRegex(
                ATTESTATION.AttestationError, "cannot grant publication authority"
            ):
                ATTESTATION.build_attestation(
                    policy_path=policy,
                    target_version=TARGET_VERSION,
                    commit=COMMIT,
                    phase="checkpoint",
                    workflow_run_id=RUN_ID,
                    workflow_run_url=RUN_URL,
                    publication_authority=True,
                )
            checkpoint = ATTESTATION.build_attestation(
                policy_path=policy,
                target_version=TARGET_VERSION,
                commit=COMMIT,
                phase="checkpoint",
                workflow_run_id=RUN_ID,
                workflow_run_url=RUN_URL,
                publication_authority=False,
            )
            with self.assertRaisesRegex(
                ATTESTATION.AttestationError, "cannot authorize publication"
            ):
                ATTESTATION.validate_attestation(
                    checkpoint,
                    policy_path=policy,
                    target_version=TARGET_VERSION,
                    commit=COMMIT,
                    phase="checkpoint",
                    workflow_run_id=RUN_ID,
                    workflow_run_url=RUN_URL,
                    intent="publish",
                )

    def test_ready_policy_can_issue_exact_publication_phase_authority(self):
        with tempfile.TemporaryDirectory() as directory:
            policy = self.ready_policy(directory)
            attestation = ATTESTATION.build_attestation(
                policy_path=policy,
                target_version=TARGET_VERSION,
                commit=COMMIT,
                phase="pre-framework",
                workflow_run_id=RUN_ID,
                workflow_run_url=RUN_URL,
                publication_authority=True,
            )
            self.assertIs(attestation["publicationAuthority"], True)
            ATTESTATION.validate_attestation(
                attestation,
                policy_path=policy,
                target_version=TARGET_VERSION,
                commit=COMMIT,
                phase="pre-framework",
                workflow_run_id=RUN_ID,
                workflow_run_url=RUN_URL,
                intent="publish",
            )

    def test_exact_bindings_reject_mutated_metadata(self):
        cases = {
            "schema": "mss.io/release-readiness-attestation/v2",
            "targetVersion": "v1.0.0",
            "commit": "f" * 40,
            "phase": "feature-freeze",
            "policySha256": "0" * 64,
            "workflowRunId": RUN_ID + 1,
            "workflowRunUrl": RUN_URL + "0",
            "publicationAuthority": True,
        }
        for key, value in cases.items():
            with self.subTest(key=key):
                candidate = copy.deepcopy(self.checkpoint())
                candidate[key] = value
                with self.assertRaises(ATTESTATION.AttestationError):
                    ATTESTATION.validate_attestation(
                        candidate,
                        policy_path=POLICY_PATH,
                        target_version=TARGET_VERSION,
                        commit=COMMIT,
                        phase="checkpoint",
                        workflow_run_id=RUN_ID,
                        workflow_run_url=RUN_URL,
                        intent="qualify",
                    )

    def test_loader_rejects_duplicate_and_unknown_keys(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "attestation.json"
            path.write_text('{"schema":"one","schema":"two"}\n', encoding="utf-8")
            with self.assertRaisesRegex(
                ATTESTATION.AttestationError, "duplicate attestation key"
            ):
                ATTESTATION.load_attestation(path)

            candidate = self.checkpoint()
            candidate["unexpected"] = True
            path.write_text(json.dumps(candidate), encoding="utf-8")
            with self.assertRaisesRegex(
                ATTESTATION.AttestationError, "unknown keys"
            ):
                ATTESTATION.validate_attestation(
                    ATTESTATION.load_attestation(path),
                    policy_path=POLICY_PATH,
                    target_version=TARGET_VERSION,
                    commit=COMMIT,
                    phase="checkpoint",
                    workflow_run_id=RUN_ID,
                    workflow_run_url=RUN_URL,
                    intent="qualify",
                )

    def test_cli_round_trip_uses_the_exact_selected_run(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "attestation.json"
            common = [
                "--policy",
                str(POLICY_PATH),
                "--target-version",
                TARGET_VERSION,
                "--commit",
                COMMIT,
                "--phase",
                "checkpoint",
                "--workflow-run-id",
                str(RUN_ID),
                "--workflow-run-url",
                RUN_URL,
            ]
            self.assertEqual(
                ATTESTATION.main(
                    [
                        "create",
                        "--output",
                        str(output),
                        "--publication-authority",
                        "false",
                        *common,
                    ]
                ),
                0,
            )
            self.assertEqual(
                ATTESTATION.main(
                    [
                        "verify",
                        "--attestation",
                        str(output),
                        "--intent",
                        "qualify",
                        *common,
                    ]
                ),
                0,
            )


if __name__ == "__main__":
    unittest.main()
