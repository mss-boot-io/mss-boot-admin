import copy
import importlib.util
import unittest
from collections import Counter
from pathlib import Path


SCRIPT = Path(__file__).with_name("generate_antd_v5_retirement_inventory.py")
SPEC = importlib.util.spec_from_file_location("antd_v5_retirement_inventory", SCRIPT)
assert SPEC is not None and SPEC.loader is not None
inventory = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(inventory)


class RetirementInventoryTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.root = SCRIPT.resolve().parents[2]
        cls.document, cls.raw = inventory.load_source(cls.root, inventory.DEFAULT_SOURCE)

    def test_repository_inventory_is_complete_and_classified(self) -> None:
        candidates = inventory.validate_source(self.root, self.document)
        counts = Counter(candidate["decision"] for candidate in candidates)

        self.assertEqual(len(candidates), 20)
        self.assertEqual(counts, {"remove": 9, "retain": 3, "defer": 8})
        self.assertEqual(
            [candidate["id"] for candidate in candidates],
            sorted(candidate["id"] for candidate in candidates),
        )

    def test_missing_evidence_path_fails_closed(self) -> None:
        document = copy.deepcopy(self.document)
        document["spec"]["candidates"][0]["knownConsumers"].append(
            "missing/retirement-consumer.txt"
        )

        with self.assertRaisesRegex(inventory.InventoryError, "references missing"):
            inventory.validate_source(self.root, document)

    def test_repository_escape_is_rejected(self) -> None:
        with self.assertRaisesRegex(inventory.InventoryError, "normalized repository-relative"):
            inventory.confined_path(self.root, "../outside.json", label="test")

    def test_report_never_authorizes_deletion(self) -> None:
        candidates = inventory.validate_source(self.root, self.document)
        original_git_output = inventory.git_output
        inventory.git_output = lambda _root, *args: (
            "abc123" if args == ("rev-parse", "HEAD") else ""
        )
        try:
            report = inventory.create_report(
                self.root,
                inventory.DEFAULT_SOURCE,
                self.raw,
                candidates,
            )
        finally:
            inventory.git_output = original_git_output

        self.assertFalse(report["summary"]["readyForDeletion"])
        self.assertEqual(report["metadata"]["sourceCommit"], "abc123")
        self.assertTrue(report["metadata"]["trackedWorktreeClean"])


if __name__ == "__main__":
    unittest.main()
