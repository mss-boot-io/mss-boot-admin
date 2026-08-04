#!/usr/bin/env python3

import importlib.util
import json
import pathlib
import sys
import tempfile
import unittest


SCRIPT = pathlib.Path(__file__).with_name("check-go-coverage.py")
SPEC = importlib.util.spec_from_file_location("check_go_coverage", SCRIPT)
MODULE = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
sys.modules[SPEC.name] = MODULE
SPEC.loader.exec_module(MODULE)


class CoverageGateTest(unittest.TestCase):
    def test_profile_and_package_floors(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            profile = root / "coverage.out"
            profile.write_text(
                "mode: atomic\n"
                "example.com/admin/config/config.go:1.1,2.1 2 1\n"
                "example.com/admin/config/config.go:3.1,4.1 2 0\n"
                "example.com/admin/service/service.go:1.1,2.1 1 1\n",
                encoding="utf-8",
            )
            policy = root / "policy.json"
            policy.write_text(
                json.dumps(
                    {
                        "components": {
                            "admin": {
                                "minimum": 60,
                                "packages": {
                                    "example.com/admin/config": 50,
                                    "example.com/admin/service/...": 100,
                                },
                            }
                        }
                    }
                ),
                encoding="utf-8",
            )
            self.assertEqual(
                0,
                MODULE.main(
                    [
                        "--profile",
                        str(profile),
                        "--policy",
                        str(policy),
                        "--component",
                        "admin",
                    ]
                ),
            )

    def test_missing_package_fails_closed(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            profile = root / "coverage.out"
            profile.write_text(
                "mode: set\nexample.com/admin/config/config.go:1.1,2.1 1 1\n",
                encoding="utf-8",
            )
            policy = root / "policy.json"
            policy.write_text(
                json.dumps(
                    {
                        "components": {
                            "admin": {
                                "minimum": 100,
                                "packages": {"example.com/admin/router": 1},
                            }
                        }
                    }
                ),
                encoding="utf-8",
            )
            self.assertEqual(
                1,
                MODULE.main(
                    [
                        "--profile",
                        str(profile),
                        "--policy",
                        str(policy),
                        "--component",
                        "admin",
                    ]
                ),
            )

    def test_invalid_profile_is_configuration_error(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            profile = root / "coverage.out"
            profile.write_text("not a coverprofile\n", encoding="utf-8")
            policy = root / "policy.json"
            policy.write_text(
                json.dumps({"components": {"admin": {"minimum": 1}}}),
                encoding="utf-8",
            )
            self.assertEqual(
                2,
                MODULE.main(
                    [
                        "--profile",
                        str(profile),
                        "--policy",
                        str(policy),
                        "--component",
                        "admin",
                    ]
                ),
            )


if __name__ == "__main__":
    unittest.main()
