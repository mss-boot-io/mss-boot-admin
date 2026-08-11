import importlib.util
import tempfile
import unittest
from pathlib import Path


MODULE_PATH = Path(__file__).with_name("check_release_policy.py")
SPEC = importlib.util.spec_from_file_location("check_release_policy", MODULE_PATH)
assert SPEC and SPEC.loader
POLICY = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(POLICY)


REPOSITORY_ROOT = Path(__file__).resolve().parents[2]
POLICY_PATH = REPOSITORY_ROOT / ".mss" / "release-policy.yaml"


class ReleasePolicyTest(unittest.TestCase):
    def setUp(self):
        self.policy = POLICY.load_policy(POLICY_PATH)

    def test_only_v110_matches_component_tag_namespaces(self):
        cases = {
            "root": "v1.1.0",
            "framework": "mss-boot/v1.1.0",
            "frontend": "web/antd/v1.1.0",
        }
        for component, tag in cases.items():
            with self.subTest(component=component):
                POLICY.check_public_ref(
                    self.policy, component, "v1.1.0", tag, intent="qualify"
                )

    def test_publication_stays_disabled_until_workflows_are_ready(self):
        self.assertIs(self.policy["publicationWorkflowsReady"], False)
        with self.assertRaisesRegex(POLICY.PolicyError, "remain disabled"):
            POLICY.check_public_ref(self.policy, "root", "v1.1.0", "v1.1.0")

    def test_policy_rejects_v101_through_v10x_tags(self):
        for version in ("v1.0.1", "v1.0.2", "v1.0.99"):
            with self.subTest(version=version):
                with self.assertRaisesRegex(POLICY.PolicyError, "forbidden"):
                    POLICY.check_public_ref(
                        self.policy, "root", version, version, intent="qualify"
                    )

    def test_policy_rejects_public_prerelease_and_wrong_namespace(self):
        with self.assertRaises(POLICY.PolicyError):
            POLICY.check_public_ref(
                self.policy,
                "root",
                "v1.1.0-rc.1",
                "v1.1.0-rc.1",
                intent="qualify",
            )
        with self.assertRaisesRegex(POLICY.PolicyError, "does not match"):
            POLICY.check_public_ref(
                self.policy,
                "framework",
                "v1.1.0",
                "v1.1.0",
                intent="qualify",
            )

    def test_policy_parser_rejects_unknown_or_duplicate_keys(self):
        original = POLICY_PATH.read_text(encoding="utf-8")
        for suffix in (
            "  unexpected: true\n",
            "  nextPublicVersion: v1.1.0\n",
        ):
            with self.subTest(suffix=suffix.strip()):
                with tempfile.TemporaryDirectory() as directory:
                    candidate = Path(directory) / "policy.yaml"
                    candidate.write_text(original + suffix, encoding="utf-8")
                    with self.assertRaises(POLICY.PolicyError):
                        POLICY.load_policy(candidate)

    def test_publication_workflows_share_policy_and_exact_attestation_guards(self):
        workflows = (
            "release.yml",
            "framework-release.yml",
            "frontend-release.yml",
            "container.yml",
        )
        for workflow in workflows:
            with self.subTest(workflow=workflow):
                content = (
                    REPOSITORY_ROOT / ".github" / "workflows" / workflow
                ).read_text(encoding="utf-8")
                self.assertIn("check_release_policy.py", content)
                self.assertIn("verify_readiness_run.sh", content)
                self.assertIn("RELEASE_READINESS_RUN_ID", content)
                self.assertNotIn(
                    "/actions/workflows/release-readiness.yml/runs", content
                )

    def test_publication_workflows_require_the_phase_they_publish_from(self):
        expected_phases = {
            "framework-release.yml": "--phase pre-framework",
            "frontend-release.yml": "--phase pre-framework",
            "container.yml": "--phase pre-root",
            "release.yml": "--phase pre-root",
        }
        for workflow, expected_phase in expected_phases.items():
            with self.subTest(workflow=workflow):
                content = (
                    REPOSITORY_ROOT / ".github" / "workflows" / workflow
                ).read_text(encoding="utf-8")
                self.assertIn(expected_phase, content)

    def test_readiness_is_manual_and_release_bound(self):
        content = (
            REPOSITORY_ROOT / ".github" / "workflows" / "release-readiness.yml"
        ).read_text(encoding="utf-8")
        self.assertNotIn("\n  pull_request:", content)
        self.assertNotIn("\n  push:", content)
        for required in (
            "workflow_dispatch:",
            "frozen_commit:",
            "feature_freeze_confirmed:",
            "phase:",
            "publication_authority:",
            "release-readiness-metadata.json",
            "release-readiness-attestation-${{ github.run_id }}",
            "release_readiness_attestation.py",
            "check_release_policy.py",
        ):
            self.assertIn(required, content)

    def test_development_push_does_not_publish_a_checkpoint_image(self):
        content = (
            REPOSITORY_ROOT / ".github" / "workflows" / "container.yml"
        ).read_text(encoding="utf-8")
        self.assertNotIn(
            '[[ "${GITHUB_EVENT_NAME}" == "push" ]]',
            content,
        )
        self.assertIn("verify_readiness_run.sh", content)
        self.assertNotIn(
            "/actions/workflows/release-readiness.yml/runs", content
        )

    def test_selected_run_helper_verifies_run_and_artifact_identity(self):
        content = (
            REPOSITORY_ROOT / "tools" / "release" / "verify_readiness_run.sh"
        ).read_text(encoding="utf-8")
        for required in (
            "/actions/runs/${run_id}",
            '.head_sha == $commit',
            '.conclusion == "success"',
            '.path == $workflow_path',
            '.html_url == $workflow_run_url',
            "release-readiness-attestation-${run_id}",
            "--intent publish",
        ):
            self.assertIn(required, content)

    def test_root_release_has_no_published_version_default(self):
        content = (
            REPOSITORY_ROOT / ".github" / "workflows" / "release.yml"
        ).read_text(encoding="utf-8")
        self.assertNotIn("default: v1.0.0", content)


if __name__ == "__main__":
    unittest.main()
