#!/usr/bin/env python3

import importlib.util
import json
import pathlib
import sys
import tempfile
import unittest


SCRIPT = pathlib.Path(__file__).with_name("check_peer_issues.py")
SPEC = importlib.util.spec_from_file_location("check_peer_issues", SCRIPT)
MODULE = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


def peer_record(*, missing_optional: bool = True) -> dict:
    return {
        "level": "error",
        "code": "ERR_PNPM_PEER_DEP_ISSUES",
        "issuesByProjects": {
            ".": {
                "bad": {
                    "react": [
                        {
                            "foundVersion": "18.2.0",
                            "wantedRange": "^16.0.0",
                            "parents": [
                                {"name": "legacy-editor", "version": "1.0.0"}
                            ],
                        }
                    ]
                },
                "missing": {
                    "canvas": [
                        {
                            "optional": missing_optional,
                            "wantedRange": "^2.5.0",
                            "parents": [
                                {"name": "jsdom", "version": "20.0.3"}
                            ],
                        }
                    ]
                },
            }
        },
    }


def policy() -> dict:
    return {
        "version": 1,
        "allowedBad": [
            {
                "dependency": "react",
                "foundVersion": "18.2.0",
                "wantedRange": "^16.0.0",
                "parents": ["legacy-editor@1.0.0"],
            }
        ],
        "allowedMissingOptional": ["canvas"],
    }


class PeerPolicyTest(unittest.TestCase):
    def write_fixture(self, record: dict, policy_value: dict):
        directory = tempfile.TemporaryDirectory()
        root = pathlib.Path(directory.name)
        report = root / "pnpm.ndjson"
        report.write_text(
            json.dumps({"level": "debug", "name": "pnpm:scope"})
            + "\n"
            + json.dumps(record)
            + "\n",
            encoding="utf-8",
        )
        policy_path = root / "policy.json"
        policy_path.write_text(json.dumps(policy_value), encoding="utf-8")
        self.addCleanup(directory.cleanup)
        return report, policy_path

    def test_exact_reviewed_issues_pass(self):
        report, policy_path = self.write_fixture(peer_record(), policy())
        self.assertEqual(
            0,
            MODULE.main(
                ["--report", str(report), "--policy", str(policy_path)]
            ),
        )

    def test_new_bad_range_fails(self):
        record = peer_record()
        record["issuesByProjects"]["."]["bad"]["react"][0][
            "wantedRange"
        ] = "^17.0.0"
        report, policy_path = self.write_fixture(record, policy())
        self.assertEqual(
            1,
            MODULE.main(
                ["--report", str(report), "--policy", str(policy_path)]
            ),
        )

    def test_missing_required_peer_fails(self):
        report, policy_path = self.write_fixture(
            peer_record(missing_optional=False), policy()
        )
        self.assertEqual(
            1,
            MODULE.main(
                ["--report", str(report), "--policy", str(policy_path)]
            ),
        )

    def test_clean_report_requires_policy_cleanup(self):
        directory = tempfile.TemporaryDirectory()
        self.addCleanup(directory.cleanup)
        root = pathlib.Path(directory.name)
        report = root / "pnpm.ndjson"
        report.write_text(
            json.dumps({"level": "info", "name": "pnpm:summary"}) + "\n",
            encoding="utf-8",
        )
        policy_path = root / "policy.json"
        policy_path.write_text(json.dumps(policy()), encoding="utf-8")
        self.assertEqual(
            1,
            MODULE.main(
                ["--report", str(report), "--policy", str(policy_path)]
            ),
        )


if __name__ == "__main__":
    unittest.main()
