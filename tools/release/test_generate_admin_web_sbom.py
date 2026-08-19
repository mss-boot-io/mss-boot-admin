import sys
import unittest
from pathlib import Path


TOOLS_DIR = Path(__file__).resolve().parent
sys.path.insert(0, str(TOOLS_DIR))
import generate_admin_web_sbom as SBOM  # noqa: E402


COMMIT = "b" * 40
REPOSITORY = "mss-boot-io/mss-boot-admin"


def package_evidence():
    return {
        "schema": "mss.io/admin-web-package-evidence/v1",
        "package": {
            "name": "@mss-boot-io/admin-web",
            "version": "1.3.0",
            "license": "MIT",
            "gitHead": COMMIT,
            "repository": {
                "type": "git",
                "url": f"git+https://github.com/{REPOSITORY}.git",
                "directory": "web/antd-v6",
            },
            "homepage": "https://docs.mss-boot-io.top/",
            "bugs": {"url": f"https://github.com/{REPOSITORY}/issues"},
        },
        "source": {"repository": REPOSITORY, "commit": COMMIT},
        "artifact": {
            "sha256": "c" * 64,
            "integrity": "sha512-example",
        },
    }


def dependency_tree():
    return {
        "name": "@mss-boot-io/admin-web",
        "version": "0.0.0-development",
        "dependencies": {
            "react": {
                "version": "19.2.8",
                "resolved": "https://registry.npmjs.org/react/-/react-19.2.8.tgz",
                "dependencies": {
                    "scheduler": {
                        "version": "0.27.0",
                        "resolved": "https://registry.npmjs.org/scheduler/-/scheduler-0.27.0.tgz",
                    }
                },
            },
            "react-dom": {
                "version": "19.2.8",
                "resolved": "https://registry.npmjs.org/react-dom/-/react-dom-19.2.8.tgz",
                "dependencies": {
                    "react": {
                        "version": "19.2.8",
                        "resolved": "https://registry.npmjs.org/react/-/react-19.2.8.tgz",
                    }
                },
            },
        },
    }


class AdminWebSBOMTest(unittest.TestCase):
    def build(self):
        return SBOM.build_sbom(
            dependency_tree(),
            package_evidence(),
            source_repository=REPOSITORY,
            source_commit=COMMIT,
            created="2026-08-19T00:00:00Z",
        )

    def test_builds_deterministic_spdx_graph_with_source_identity(self):
        first = self.build()
        second = self.build()
        self.assertEqual(first, second)
        self.assertEqual(first["spdxVersion"], "SPDX-2.3")
        self.assertEqual(first["packages"][0]["name"], "@mss-boot-io/admin-web")
        self.assertEqual(first["packages"][0]["versionInfo"], "1.3.0")
        self.assertEqual(first["packages"][0]["licenseDeclared"], "MIT")
        self.assertIn(COMMIT, first["documentComment"])
        names = [package["name"] for package in first["packages"]]
        self.assertEqual(names.count("react"), 1)
        self.assertIn("scheduler", names)
        self.assertGreaterEqual(
            sum(
                relationship["relationshipType"] == "DEPENDS_ON"
                for relationship in first["relationships"]
            ),
            3,
        )

    def test_rejects_mismatched_source_or_dependency_root(self):
        with self.assertRaisesRegex(SBOM.SBOMError, "source commit"):
            SBOM.build_sbom(
                dependency_tree(),
                package_evidence(),
                source_repository=REPOSITORY,
                source_commit="d" * 40,
                created="2026-08-19T00:00:00Z",
            )
        tree = dependency_tree()
        tree["name"] = "wrong-package"
        with self.assertRaisesRegex(SBOM.SBOMError, "does not match"):
            SBOM.build_sbom(
                tree,
                package_evidence(),
                source_repository=REPOSITORY,
                source_commit=COMMIT,
                created="2026-08-19T00:00:00Z",
            )


if __name__ == "__main__":
    unittest.main()
