from __future__ import annotations

import importlib.util
from pathlib import Path
import unittest


MODULE_PATH = Path(__file__).with_name("prepare-v0-7-fixture.py")
SPEC = importlib.util.spec_from_file_location("prepare_v0_7_fixture", MODULE_PATH)
if SPEC is None or SPEC.loader is None:
    raise RuntimeError(f"cannot load {MODULE_PATH}")
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


class PrepareV07FixtureTest(unittest.TestCase):
    def test_options_fixture_uses_v07_migration_registration_api(self) -> None:
        historical = f"""
package system

import "{MODULE.OLD_MODULE}"

func init() {{
    migration.Migrate.SetVersion(migration.GetFilename(fileName), _20260403225953EnhanceOptions)
}}

// ADD COLUMN IF NOT EXISTS
""".lstrip()
        candidate = f"""
package system

import "{MODULE.CURRENT_MODULE}"

func init() {{
    migration.Migrate.SetV100Version(fileName, _20260403225953EnhanceOptions)
}}

func ensureOptionsIndex() {{}}
""".lstrip()

        rendered = MODULE.prepare_options_migration(historical, candidate)

        self.assertIn(MODULE.OLD_MODULE, rendered)
        self.assertNotIn(MODULE.CURRENT_MODULE, rendered)
        self.assertIn(
            "migration.Migrate.SetVersion(migration.GetFilename(fileName), "
            "_20260403225953EnhanceOptions)",
            rendered,
        )
        self.assertNotIn("SetV100Version", rendered)

    def test_options_fixture_rejects_an_unknown_candidate_registration(self) -> None:
        historical = (
            f'import "{MODULE.OLD_MODULE}"\n'
            f"{MODULE.V07_REGISTRATION}\n"
            "// ADD COLUMN IF NOT EXISTS\n"
        )
        candidate = (
            f'import "{MODULE.CURRENT_MODULE}"\n'
            "migration.Migrate.SetFutureVersion(fileName, migrationFn)\n"
            "func ensureOptionsIndex() {}\n"
        )

        with self.assertRaisesRegex(ValueError, "unexpected registration API"):
            MODULE.prepare_options_migration(historical, candidate)


if __name__ == "__main__":
    unittest.main()
