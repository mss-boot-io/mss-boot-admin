import importlib.util
import json
import tempfile
import unittest
from pathlib import Path


MODULE_PATH = Path(__file__).with_name("check_docs_deployment_state.py")
SPEC = importlib.util.spec_from_file_location("check_docs_deployment_state", MODULE_PATH)
assert SPEC and SPEC.loader
STATE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(STATE)


class DocsDeploymentStateTest(unittest.TestCase):
    def identity(self, version: str, commit: str = "a" * 40):
        return STATE.parse_version(version, commit)

    def test_new_product_base_can_advance(self):
        self.assertEqual(
            STATE.deployment_action(
                self.identity("v1.3.7", "b" * 40),
                self.identity("v1.3.2+docs.1"),
            ),
            "deploy",
        )

    def test_exact_public_identity_is_idempotent(self):
        identity = self.identity("v1.3.7+docs.1")
        self.assertEqual(STATE.deployment_action(identity, identity), "current")

    def test_same_version_with_different_commit_is_rejected(self):
        with self.assertRaisesRegex(STATE.DeploymentStateError, "not requested"):
            STATE.deployment_action(
                self.identity("v1.3.7", "b" * 40),
                self.identity("v1.3.7", "a" * 40),
            )

    def test_product_rollback_is_rejected(self):
        with self.assertRaisesRegex(STATE.DeploymentStateError, "roll back product"):
            STATE.deployment_action(
                self.identity("v1.3.6"),
                self.identity("v1.3.7"),
            )

    def test_revision_must_follow_the_public_base(self):
        with self.assertRaisesRegex(STATE.DeploymentStateError, "skip its base"):
            STATE.deployment_action(
                self.identity("v1.3.7+docs.1"),
                self.identity("v1.3.2+docs.1"),
            )

    def test_revision_rollback_and_gap_are_rejected(self):
        for requested, current, message in (
            ("v1.3.7", "v1.3.7+docs.1", "roll back revision"),
            ("v1.3.7+docs.1", "v1.3.7+docs.2", "roll back revision"),
            ("v1.3.7+docs.3", "v1.3.7+docs.1", "skip a production revision"),
        ):
            with self.subTest(requested=requested, current=current):
                with self.assertRaisesRegex(STATE.DeploymentStateError, message):
                    STATE.deployment_action(
                        self.identity(requested, "b" * 40),
                        self.identity(current),
                    )

    def test_next_revision_can_advance(self):
        self.assertEqual(
            STATE.deployment_action(
                self.identity("v1.3.7+docs.2", "b" * 40),
                self.identity("v1.3.7+docs.1"),
            ),
            "deploy",
        )

    def test_current_identity_file_is_strict(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "release.json"
            path.write_text(
                json.dumps(
                    {
                        "application": "other",
                        "version": "v1.3.7",
                        "commit": "a" * 40,
                    }
                ),
                encoding="utf-8",
            )
            with self.assertRaisesRegex(STATE.DeploymentStateError, "mss-boot-docs"):
                STATE.load_current_identity(path)

    def test_revision_is_bounded_before_arithmetic_or_iteration(self):
        with self.assertRaisesRegex(STATE.DeploymentStateError, "maximum supported"):
            self.identity("v1.3.7+docs.1000")


if __name__ == "__main__":
    unittest.main()
