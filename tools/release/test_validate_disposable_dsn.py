from __future__ import annotations

import unittest

from validate_disposable_dsn import validate_mysql, validate_postgres


DATABASE = "mss_release_upgrade_test"


class ValidateDisposableDSNTest(unittest.TestCase):
    def test_accepts_exact_loopback_targets(self) -> None:
        validate_mysql(
            "user:pass@tcp(127.0.0.1:3306)/mss_release_upgrade_test?parseTime=true",
            DATABASE,
        )
        validate_postgres(
            "postgres://user:pass@localhost:5432/mss_release_upgrade_test?sslmode=disable",
            DATABASE,
        )

    def test_rejects_query_string_database_spoofing(self) -> None:
        with self.assertRaisesRegex(ValueError, "unexpected database"):
            validate_mysql(
                "user:pass@tcp(127.0.0.1:3306)/production?note=/mss_release_upgrade_test?",
                DATABASE,
            )
        with self.assertRaisesRegex(ValueError, "unexpected database"):
            validate_postgres(
                "postgres://user:pass@127.0.0.1:5432/production?next=/mss_release_upgrade_test?",
                DATABASE,
            )

    def test_rejects_non_loopback_hosts(self) -> None:
        with self.assertRaisesRegex(ValueError, "loopback"):
            validate_mysql(
                "user:pass@tcp(db.internal:3306)/mss_release_upgrade_test?parseTime=true",
                DATABASE,
            )
        with self.assertRaisesRegex(ValueError, "loopback"):
            validate_postgres(
                "postgres://user:pass@10.0.0.8:5432/mss_release_upgrade_test?sslmode=disable",
                DATABASE,
            )


if __name__ == "__main__":
    unittest.main()
