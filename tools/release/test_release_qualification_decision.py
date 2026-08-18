import hashlib
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


TOOLS_DIR = Path(__file__).resolve().parent
sys.path.insert(0, str(TOOLS_DIR))
import release_qualification_decision as DECISION  # noqa: E402


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
                    "targetVersion": "v1.2.2",
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
            "target_version": "v1.2.2",
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
