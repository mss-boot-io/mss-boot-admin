from __future__ import annotations

import importlib.util
from pathlib import Path
import unittest


MODULE_PATH = Path(__file__).with_name("prepare-db-config.py")
SPEC = importlib.util.spec_from_file_location("prepare_db_config", MODULE_PATH)
if SPEC is None or SPEC.loader is None:
    raise RuntimeError(f"cannot load {MODULE_PATH}")
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


class PrepareDatabaseConfigTest(unittest.TestCase):
    def test_database_rehearsal_disables_all_cache_backends(self) -> None:
        fixtures = {
            "v0.7": """
database:
  driver: sqlite
  source: old.db
  name: old
logger:
  level: info
  stdout: default
cache:
  queryCache: true
  queryCacheDuration: 1h
  queryCacheKeys:
    - '*'
  redis:
    addr: '127.0.0.1:6379'
    password: legacy-password
queue:
  memory:
    poolSize: 10
""".lstrip(),
            "current": """
database:
  driver: sqlite
  source: old.db
  name: old
logger:
  level: info
  stdout: default
cache:
  queryCache: true
  queryCacheDuration: 1h
  queryCacheKeys: []
  redis:
    addr: '127.0.0.1:6379'
    password: current-password
queue:
  memory:
    poolSize: 10
""".lstrip(),
        }

        for name, source in fixtures.items():
            with self.subTest(name=name):
                rendered = MODULE.replace_database_fields(
                    source,
                    "postgres",
                    "postgres://user:pass@127.0.0.1:5432/rehearsal",
                    "mss_release_upgrade_test",
                )

                self.assertIn('  driver: "postgres"', rendered)
                self.assertIn(
                    '  source: "postgres://user:pass@127.0.0.1:5432/rehearsal"',
                    rendered,
                )
                self.assertIn('  name: "mss_release_upgrade_test"', rendered)
                self.assertIn('  stdout: "stderr"', rendered)
                self.assertIn('  level: "error"', rendered)
                self.assertIn("  queryCache: false", rendered)
                self.assertNotIn("redis:", rendered)
                self.assertNotIn("127.0.0.1:6379", rendered)
                self.assertNotIn("-password", rendered)
                self.assertIn("queue:\n", rendered)


if __name__ == "__main__":
    unittest.main()
