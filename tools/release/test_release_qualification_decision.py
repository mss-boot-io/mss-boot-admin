import hashlib
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


TOOLS_DIR = Path(__file__).resolve().parent
REPOSITORY_ROOT = TOOLS_DIR.parents[1]
sys.path.insert(0, str(TOOLS_DIR))
import release_qualification_decision as DECISION  # noqa: E402


TARGET_VERSION = "v1.3.0-rc.4"


class ReleaseQualificationDecisionTest(unittest.TestCase):
    def fixture(self):
        temporary = tempfile.TemporaryDirectory()
        root = Path(temporary.name)
        contract = root / ".mss" / "release-qualification.json"
        contract.parent.mkdir(parents=True)
        contract.write_text(
            json.dumps(
                {
                    "schema": "mss.io/release-qualification/v1",
                    "targetVersion": TARGET_VERSION,
                    "features": [DECISION.RELEASE_FEATURE],
                    "excludedFeatures": [
                        {
                            "path": ".mss/features/old.yaml",
                            "reason": "carry-forward: accepted checkpoint",
                        }
                    ],
                }
            ),
            encoding="utf-8",
        )
        subprocess.run(["git", "init", "-q"], cwd=root, check=True)
        subprocess.run(["git", "config", "user.email", "release@example.invalid"], cwd=root, check=True)
        subprocess.run(["git", "config", "user.name", "Release Test"], cwd=root, check=True)
        subprocess.run(["git", "add", "."], cwd=root, check=True)
        subprocess.run(["git", "commit", "-qm", "fixture"], cwd=root, check=True)
        commit = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=root, text=True).strip()
        return temporary, root, contract, commit

    def build(self, root, commit, **overrides):
        values = {
            "qualification": ".mss/release-qualification.json",
            "target_version": TARGET_VERSION,
            "commit": commit,
            "phase": "feature-freeze",
            "browser_commit": commit,
            "browser_reference": "codex-task/browser-supplier-flow",
            "blueprint_commit": commit,
            "blueprint_reference": "actions/foundation-compatibility/123",
        }
        values.update(overrides)
        return DECISION.build_decision(root, **values)

    def test_builds_exact_sha_bound_manual_evidence(self):
        temporary, root, contract, commit = self.fixture()
        self.addCleanup(temporary.cleanup)
        decision = self.build(root, commit)
        self.assertEqual(decision["frozenCommit"], commit)
        self.assertEqual(
            decision["qualification"]["sha256"], hashlib.sha256(contract.read_bytes()).hexdigest()
        )
        self.assertEqual(
            {entry["commit"] for entry in decision["evidence"]},
            {commit},
        )
        self.assertFalse(decision["scope"]["objectStoreProviderConformanceRequired"])
        self.assertFalse(decision["scope"]["rustfsRequired"])

    def test_repository_qualification_matches_active_release_binding(self):
        contract = REPOSITORY_ROOT / ".mss" / "release-qualification.json"
        value, digest = DECISION._load_qualification(contract, TARGET_VERSION)
        self.assertEqual(value["features"], [DECISION.RELEASE_FEATURE])
        self.assertEqual(digest, hashlib.sha256(contract.read_bytes()).hexdigest())

    def test_rejects_short_or_mismatched_evidence_commits(self):
        temporary, root, _, commit = self.fixture()
        self.addCleanup(temporary.cleanup)
        with self.assertRaisesRegex(DECISION.QualificationDecisionError, "full commit SHA"):
            self.build(root, commit, browser_commit=commit[:12])
        with self.assertRaisesRegex(DECISION.QualificationDecisionError, "not bound"):
            self.build(root, commit, blueprint_commit="0" * 40)

    def test_rejects_placeholder_references(self):
        temporary, root, _, commit = self.fixture()
        self.addCleanup(temporary.cleanup)
        with self.assertRaisesRegex(DECISION.QualificationDecisionError, "concrete single-line"):
            self.build(root, commit, browser_reference="pending")

    def test_rejects_unscoped_qualification_contract(self):
        temporary, root, contract, commit = self.fixture()
        self.addCleanup(temporary.cleanup)
        value = json.loads(contract.read_text(encoding="utf-8"))
        value["features"].append(".mss/features/storage-runtime-v2.yaml")
        contract.write_text(json.dumps(value), encoding="utf-8")
        with self.assertRaisesRegex(DECISION.QualificationDecisionError, "scoped release Feature"):
            self.build(root, commit)


if __name__ == "__main__":
    unittest.main()
