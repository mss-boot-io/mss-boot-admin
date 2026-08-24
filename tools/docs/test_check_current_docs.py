from __future__ import annotations

import importlib.util
import tempfile
import unittest
from pathlib import Path


SCRIPT = Path(__file__).with_name("check_current_docs.py")
SPEC = importlib.util.spec_from_file_location("check_current_docs", SCRIPT)
assert SPEC and SPEC.loader
check_current_docs = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(check_current_docs)


class CurrentDocsContractTest(unittest.TestCase):
    def test_repository_satisfies_contract(self) -> None:
        self.assertEqual(check_current_docs.collect_errors(), [])

    def test_rejects_source_checkout_commands_and_old_versions(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            page = root / "page.md"
            page.write_text(
                "git clone https://example.invalid/foundation\n"
                "go run ./cmd/mss verify --changed\n"
                "mss upgrade admin v1.3.2 --foundation ../foundation\n"
                "sh ./install-mss.sh --version v1.3.3\n",
                encoding="utf-8",
            )
            errors = check_current_docs.forbidden_content_errors(
                root, "v1.3.3", [Path("page.md")]
            )
        joined = "\n".join(errors)
        self.assertIn("Foundation clone command", joined)
        self.assertIn("source-only mss invocation", joined)
        self.assertIn("checkout-dependent upgrade", joined)
        self.assertIn("stale distribution token v1.3.2", joined)
        self.assertIn("POSIX sh invocation for Bash installer", joined)

    def test_rejects_manual_bootstrap_password_prompts(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            page = root / "page.md"
            page.write_text(
                "read -rsp 'Password: ' MSS_ADMIN_INITIAL_PASSWORD\n"
                "[System.Net.NetworkCredential]::new('', $secret).Password\n",
                encoding="utf-8",
            )
            errors = check_current_docs.forbidden_content_errors(
                root, "v1.3.3", [Path("page.md")]
            )
        joined = "\n".join(errors)
        self.assertIn("manual shell bootstrap password prompt", joined)
        self.assertIn("manual PowerShell bootstrap password conversion", joined)

    def test_rejects_retired_operations_and_literal_credentials(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            page = root / "page.md"
            page.write_text(
                "Copy config/application.yml from HotGo.\n"
                "INSERT INTO jobs(name) VALUES ('unsafe');\n"
                "smtpPassword: committed-example\n",
                encoding="utf-8",
            )
            errors = check_current_docs.forbidden_content_errors(
                root, "v1.3.3", [Path("page.md")]
            )
        joined = "\n".join(errors)
        self.assertIn("retired monolithic application config", joined)
        self.assertIn("obsolete HotGo comparison context", joined)
        self.assertIn("direct SQL mutation example", joined)
        self.assertIn("literal credential example", joined)

    def test_route_resolution_requires_real_page(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            route = root / "docs/docs/getting-started"
            route.mkdir(parents=True)
            (route / "index.md").write_text("# start\n", encoding="utf-8")
            self.assertTrue(
                check_current_docs.route_exists(root, "/getting-started")
            )
            self.assertFalse(check_current_docs.route_exists(root, "/deleted"))

    def test_link_check_includes_root_changelogs(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            (root / "docs/docs").mkdir(parents=True)
            (root / "CHANGELOG.md").write_text(
                "[removed](docs/docs/releases/removed.md)\n",
                encoding="utf-8",
            )
            errors = check_current_docs.internal_link_errors(root)
        self.assertTrue(any("CHANGELOG.md:1" in error for error in errors))

    def test_bootstrap_password_contract_requires_environment_only(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            for relative in check_current_docs.BOOTSTRAP_PASSWORD_FILES:
                path = root / relative
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text(
                    "Use MSS_ADMIN_INITIAL_PASSWORD only for the first migration.\n",
                    encoding="utf-8",
                )
            self.assertEqual(check_current_docs.bootstrap_password_errors(root), [])

            unsafe = root / check_current_docs.BOOTSTRAP_PASSWORD_FILES[0]
            unsafe.write_text(
                "MSS_ADMIN_INITIAL_PASSWORD\nadmin migrate --password example\n",
                encoding="utf-8",
            )
            errors = check_current_docs.bootstrap_password_errors(root)
            self.assertTrue(any("command arguments" in error for error in errors))

    def test_first_login_contract_requires_username_and_address(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            for relative in check_current_docs.FIRST_LOGIN_FILES:
                path = root / relative
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text(
                    "Sign in as `admin` at http://127.0.0.1:8001.\n",
                    encoding="utf-8",
                )
            self.assertEqual(check_current_docs.first_login_errors(root), [])

            incomplete = root / check_current_docs.FIRST_LOGIN_FILES[0]
            incomplete.write_text("Open the generated application.\n", encoding="utf-8")
            errors = check_current_docs.first_login_errors(root)
            self.assertTrue(any("initial admin username" in error for error in errors))
            self.assertTrue(any("local Admin Web address" in error for error in errors))

    def test_upgrade_contract_requires_manifest_backup_and_no_op_proof(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            complete = (
                "Back up code and database. Hand-assembled repositories missing their manifest migrate to a clean baseline.\n"
                "mss --version\nmss-mcp --version\n.mss/blueprint-manifest.json\n"
                "mss upgrade admin v1.3.3\n"
                "mss upgrade admin v1.3.3 --apply --yes\n"
                "mss doctor --strict\nmss verify --all\n"
                "mss upgrade admin v1.3.3\n"
            )
            for relative in check_current_docs.UPGRADE_CONTRACT_FILES:
                path = root / relative
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text(complete, encoding="utf-8")
            self.assertEqual(check_current_docs.upgrade_contract_errors(root), [])

            incomplete = root / check_current_docs.UPGRADE_CONTRACT_FILES[0]
            incomplete.write_text("mss upgrade admin v1.3.3\n", encoding="utf-8")
            errors = check_current_docs.upgrade_contract_errors(root)
            self.assertTrue(any("blueprint-manifest" in error for error in errors))
            self.assertTrue(any("final no-op" in error for error in errors))

    def test_mcp_contract_requires_stdio_client_and_project_boundary(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            for relative in check_current_docs.MCP_CONTRACT_FILES:
                path = root / relative
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text(
                    "stdio tools/list mss_plan_application .mss/project.yaml\n",
                    encoding="utf-8",
                )
            tooling = root / check_current_docs.MCP_CONTRACT_FILES[0]
            tooling.write_text(
                'stdio tools/list mss_plan_application .mss/project.yaml '
                '"mcpServers" "command" "args"\n',
                encoding="utf-8",
            )
            self.assertEqual(check_current_docs.mcp_contract_errors(root), [])

            tooling.write_text("stdio only\n", encoding="utf-8")
            errors = check_current_docs.mcp_contract_errors(root)
            self.assertTrue(any("tools/list" in error for error in errors))
            self.assertTrue(any('"mcpServers"' in error for error in errors))

    def test_public_skill_list_is_exact(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            path = root / "docs/docs/agent/skills-and-mcp.md"
            path.parent.mkdir(parents=True)
            path.write_text(
                "\n".join(f"`{name}`" for name in check_current_docs.PUBLIC_THIN_HOST_SKILLS),
                encoding="utf-8",
            )
            self.assertEqual(
                check_current_docs.public_skill_documentation_errors(root), []
            )

            path.write_text("`mss-add-workflow`\n", encoding="utf-8")
            errors = check_current_docs.public_skill_documentation_errors(root)
            self.assertTrue(any("mss-thin-host" in error for error in errors))
            self.assertTrue(any("unsupported Skill" in error for error in errors))

    def test_template_container_bases_require_exact_versions_and_digests(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            packages = root / "docs/docs/getting-started/packages.md"
            packages.parent.mkdir(parents=True)
            packages.write_text(
                "go get github.com/mss-boot-io/mss-boot-admin/admin@v1.3.3\n"
                "go get github.com/mss-boot-io/mss-boot-admin/mss-boot@v1.3.3\n"
                "$previousGowork\nRemove-Item Env:GOWORK\n",
                encoding="utf-8",
            )
            docker = root / "docs/docs/admin/docker.md"
            docker.parent.mkdir(parents=True)
            docker.write_text(
                "不能直接部署 docker buildx build OCI index digest 业务镜像\n",
                encoding="utf-8",
            )
            dockerfile = root / "templates/application/Dockerfile"
            dockerfile.parent.mkdir(parents=True)
            digest = "a" * 64
            dockerfile.write_text(
                f"FROM --platform=$BUILDPLATFORM golang:1.26.6-alpine@sha256:{digest} AS backend\n"
                f"FROM --platform=$BUILDPLATFORM node:24.19.0-bookworm-slim@sha256:{digest} AS frontend\n"
                f"FROM alpine:3.24.1@sha256:{digest}\n",
                encoding="utf-8",
            )
            dockerignore = root / "templates/application/.dockerignore"
            dockerignore.write_text(
                ".git\n.env\n.mss/logs\n.mss/reports\n*.db\nweb/node_modules\n",
                encoding="utf-8",
            )
            self.assertEqual(
                check_current_docs.package_and_container_contract_errors(root), []
            )

            dockerfile.write_text("FROM golang:1.26 AS backend\n", encoding="utf-8")
            errors = check_current_docs.package_and_container_contract_errors(root)
            self.assertTrue(any("immutable digest" in error for error in errors))


if __name__ == "__main__":
    unittest.main()
