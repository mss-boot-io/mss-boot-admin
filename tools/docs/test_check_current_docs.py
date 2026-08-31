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


def candidate_state() -> check_current_docs.ReleaseDocumentationState:
    return check_current_docs.ReleaseDocumentationState(
        distribution_version="v1.3.7",
        current_stable_version="v1.3.2",
        immutable_stopped_versions=("v1.3.5", "v1.3.6"),
        publication_workflows_ready=True,
        release_status="candidate",
    )


class CurrentDocsContractTest(unittest.TestCase):
    def test_repository_satisfies_contract(self) -> None:
        self.assertEqual(check_current_docs.collect_errors(), [])

    def test_release_state_reads_policy_and_matching_feature(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            policy = root / ".mss/release-policy.yaml"
            policy.parent.mkdir(parents=True)
            policy.write_text(
                "spec:\n"
                "  currentStableVersion: v1.3.2\n"
                "  distributionVersion: v1.3.7\n"
                "  immutableStoppedTrains:\n"
                "    - version: v1.3.5\n"
                "    - version: v1.3.6\n"
                "  publicationWorkflowsReady: true\n",
                encoding="utf-8",
            )
            feature = root / ".mss/features/release.yaml"
            feature.parent.mkdir(parents=True)
            feature.write_text(
                "metadata:\n"
                "  labels:\n"
                "    target-version: v1.3.7\n"
                "    release-status: candidate\n",
                encoding="utf-8",
            )
            state = check_current_docs.release_documentation_state(root)

        self.assertEqual(state.distribution_version, "v1.3.7")
        self.assertEqual(state.current_stable_version, "v1.3.2")
        self.assertEqual(
            state.immutable_stopped_versions,
            ("v1.3.5", "v1.3.6"),
        )
        self.assertTrue(state.publication_workflows_ready)
        self.assertEqual(state.release_status, "candidate")
        self.assertFalse(state.operational_onboarding_allowed)

    def test_release_state_fails_closed_on_invalid_stopped_train_list(self) -> None:
        policies = (
            "  immutableStoppedVersion: v1.3.5\n",
            "  immutableStoppedTrains:\n    - commit: deadbeef\n",
            "  immutableStoppedTrains:\n"
            "    - version: v1.3.5\n"
            "    - version: v1.3.5\n",
        )
        for stopped_contract in policies:
            with self.subTest(stopped_contract=stopped_contract):
                with tempfile.TemporaryDirectory() as temp:
                    root = Path(temp)
                    policy = root / ".mss/release-policy.yaml"
                    policy.parent.mkdir(parents=True)
                    policy.write_text(
                        "spec:\n"
                        "  currentStableVersion: v1.3.2\n"
                        "  distributionVersion: v1.3.7\n"
                        f"{stopped_contract}"
                        "  publicationWorkflowsReady: false\n",
                        encoding="utf-8",
                    )
                    with self.assertRaisesRegex(ValueError, "immutableStoppedTrains"):
                        check_current_docs.release_documentation_state(root)

    def test_partial_release_rejects_version_specific_dead_paths(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            relative = Path("docs/docs/getting-started/tooling.md")
            page = root / relative
            page.parent.mkdir(parents=True)
            page.write_text(
                "https://github.com/mss-boot-io/mss-boot-admin/releases/download/"
                "v1.3.7/install-mss.sh\n"
                "bash ./install-mss.sh --version v1.3.7\n"
                "& .\\install-mss.ps1 -Version v1.3.7\n"
                "mss new app demo\n"
                "mss setup\n"
                "mss upgrade admin v1.3.7\n"
                "corepack pnpm add @mss-boot-io/admin-web@1.3.7\n"
                "docker pull ghcr.io/mss-boot-io/mss-boot-admin:v1.3.7\n"
                '{\"command\": \"mss-mcp\"}\n'
                "go install example.invalid/cmd/mss@v1.3.7\n",
                encoding="utf-8",
            )
            state = candidate_state()
            errors = check_current_docs.partial_release_operational_errors(root, state)

        joined = "\n".join(errors)
        self.assertIn("unpublished v1.3.7 installer URL", joined)
        self.assertIn("unpublished shell installer invocation", joined)
        self.assertIn("unpublished PowerShell installer invocation", joined)
        self.assertIn("unpublished v1.3.7 Admin upgrade", joined)
        self.assertIn("unpublished official npmjs install", joined)
        self.assertIn("unpublished Root image command", joined)
        self.assertIn("source-built v1.3.7 Root tool", joined)
        self.assertNotIn("mss new app", joined)
        self.assertNotIn("mss setup", joined)
        self.assertNotIn("mss-mcp client", joined)

    def test_partial_release_scans_deep_active_pages_but_not_archives(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            deep = root / "docs/docs/admin/deep.md"
            archived = root / "docs/docs/releases/archive/v1-3-5.md"
            contributor = root / "docs/docs/coding/first-contribution.md"
            for page in (deep, archived, contributor):
                page.parent.mkdir(parents=True, exist_ok=True)
                page.write_text(
                    "mss upgrade admin v1.3.7\n",
                    encoding="utf-8",
                )
            state = candidate_state()
            errors = check_current_docs.partial_release_operational_errors(root, state)

        joined = "\n".join(errors)
        self.assertIn("docs/docs/admin/deep.md:1", joined)
        self.assertNotIn("docs/docs/releases/archive", joined)
        self.assertNotIn("docs/docs/coding/first-contribution.md", joined)

    def test_partial_release_allows_general_development_contracts(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            page = root / "docs/docs/agent/development.md"
            page.parent.mkdir(parents=True)
            page.write_text(
                "mss new app demo\n"
                "mss setup\n"
                "mss doctor --strict\n"
                "mss verify --all\n"
                "mss-mcp --root /workspace\n",
                encoding="utf-8",
            )
            state = candidate_state()
            errors = check_current_docs.partial_release_operational_errors(root, state)

        self.assertEqual(errors, [])

    def test_release_status_claims_must_remain_stage_neutral(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            relative = Path("docs/docs/status.md")
            page = root / relative
            page.parent.mkdir(parents=True)
            claims = (
                "v1.3.7 remains unpublished.\n",
                "v1.3.7 is not public yet.\n",
                "v1.3.7 is not published.\n",
                "unpublished v1.3.7 must not be adopted.\n",
                "v1.3.7 is the selected train but is not yet\npublished.\n",
                "v1.3.7 尚未发布。\n",
                "未公开的版本是 v1.3.7。\n",
            )
            for claim in claims:
                with self.subTest(claim=claim):
                    page.write_text(claim, encoding="utf-8")
                    errors = check_current_docs.absolute_unpublished_claim_errors(
                        root, "v1.3.7", [relative]
                    )
                    self.assertTrue(any("stage-neutral" in error for error in errors))

            page.write_text(
                "v1.3.7 is not yet stable or adoptable. Candidate surfaces may "
                "become public in stages before final policy and Docs reconciliation.\n",
                encoding="utf-8",
            )
            self.assertEqual(
                check_current_docs.absolute_unpublished_claim_errors(
                    root, "v1.3.7", [relative]
                ),
                [],
            )

    def test_packaged_stage_claim_paths_cover_current_artifact_markdown(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            included = (
                Path("CONTRIBUTING.md"),
                Path("CHANGELOG.md"),
                Path("admin/README.md"),
                Path("mss-boot/CHANGELOG.md"),
                Path("web/antd-v6/CHANGELOG.md"),
                Path("docs/README.md"),
                Path("docs/CONTRIBUTING.md"),
                Path("templates/application/README.md"),
            )
            excluded = (
                Path("docs/docs/releases/v1-3-5.md"),
                Path("docs/docs/releases/v1-3-6.md"),
                Path("docs/docs/releases/archive/v1-3-2.md"),
            )
            for relative in (*included, *excluded):
                path = root / relative
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text("release evidence\n", encoding="utf-8")

            actual = set(check_current_docs.packaged_stage_claim_paths(root))

        self.assertTrue(set(included).issubset(actual))
        self.assertTrue(set(excluded).isdisjoint(actual))

    def test_partial_release_semantics_require_status_and_boundary_anchors(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            relative = Path("docs/docs/admin/status.md")
            page = root / relative
            page.parent.mkdir(parents=True)
            page.write_text(
                "v1.3.7 components; v1.3.5 is permanently stopped; "
                "current stable is v1.3.2.\n",
                encoding="utf-8",
            )
            state = candidate_state()
            errors = check_current_docs.partial_release_semantic_errors(
                root,
                state,
                status_paths=[relative],
                claim_paths=[relative],
            )
            self.assertTrue(any("release-status=candidate" in error for error in errors))
            self.assertTrue(any("future-contract" in error for error in errors))
            self.assertTrue(any("immutable stopped v1.3.6" in error for error in errors))

            page.write_text(
                "v1.3.7 is a release candidate; v1.3.5 and v1.3.6 are "
                "permanently stopped; "
                "current stable is v1.3.2. "
                "This is a future contract, not an adoptable release.\n",
                encoding="utf-8",
            )
            self.assertEqual(
                check_current_docs.partial_release_semantic_errors(
                    root,
                    state,
                    status_paths=[relative],
                    claim_paths=[relative],
                ),
                [],
            )

    def test_partial_release_semantics_reject_current_adoption_claims(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            relative = Path("docs/docs/admin/status.md")
            page = root / relative
            page.parent.mkdir(parents=True)
            page.write_text(
                "v1.3.7 is a release candidate; v1.3.5 and v1.3.6 are "
                "permanently stopped; "
                "current stable is v1.3.2. "
                "This is a future contract, not an adoptable release.\n"
                "The v1.3.7 candidate publishes a complete adopter package.\n",
                encoding="utf-8",
            )
            state = candidate_state()
            errors = check_current_docs.partial_release_semantic_errors(
                root,
                state,
                status_paths=[relative],
                claim_paths=[relative],
            )

        self.assertTrue(
            any("candidate publication claim" in error for error in errors)
        )

    def test_partial_release_semantics_reject_stale_candidate_admin_web_identity(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            relative = Path("docs/docs/admin/status.md")
            page = root / relative
            page.parent.mkdir(parents=True)
            page.write_text(
                "v1.3.7 is a release candidate; v1.3.5 and v1.3.6 are "
                "permanently stopped; current stable is v1.3.2. "
                "This is a future contract, not an adoptable release.\n"
                "Admin Web candidate identity: "
                "@mss-boot-io/admin-web@1.3.6.\n",
                encoding="utf-8",
            )
            errors = check_current_docs.partial_release_semantic_errors(
                root,
                candidate_state(),
                status_paths=[relative],
                claim_paths=[relative],
            )

        self.assertTrue(
            any("stale candidate Admin Web identity 1.3.6" in error for error in errors)
        )

    def test_candidate_rejects_current_stable_claim_before_reconciliation(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            relative = Path("docs/docs/admin/status.md")
            page = root / relative
            page.parent.mkdir(parents=True)
            page.write_text(
                "v1.3.7 is a release candidate; v1.3.5 and v1.3.6 are "
                "permanently stopped; "
                "current stable is v1.3.2. This is not an adoptable release.\n"
                "Current stable version is v1.3.7.\n",
                encoding="utf-8",
            )
            state = candidate_state()
            errors = check_current_docs.partial_release_semantic_errors(
                root,
                state,
                status_paths=[relative],
                claim_paths=[relative],
            )

        self.assertTrue(any("current-stable claim" in error for error in errors))

    def test_stopped_release_history_is_separate_from_active_candidate(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            for path, version in zip(
                check_current_docs.STOPPED_RELEASE_PAGES,
                ("v1.3.5", "v1.3.6"),
                strict=True,
            ):
                page = root / path
                page.parent.mkdir(parents=True, exist_ok=True)
                page.write_text(
                    f"{version} is permanently stopped as immutable-partial "
                    "history. The rollback baseline is v1.3.2.\n",
                    encoding="utf-8",
                )
            state = candidate_state()
            self.assertEqual(
                check_current_docs.stopped_release_history_errors(root, state),
                [],
            )

            page.write_text("v1.3.7 candidate only.\n", encoding="utf-8")
            joined = "\n".join(
                check_current_docs.stopped_release_history_errors(root, state)
            )
            self.assertIn("immutable stopped v1.3.6", joined)
            self.assertIn("rollback baseline v1.3.2", joined)
            self.assertIn("immutable-partial", joined)

    def test_partial_release_allows_audit_identities_without_commands(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            relative = Path("docs/docs/releases/v1-3-5.md")
            page = root / relative
            page.parent.mkdir(parents=True)
            page.write_text(
                "Missing assets: install-mss.sh, mss-tools-v1.3.5-linux-amd64.tar.gz.\n"
                "Published component identity: "
                "github.com/mss-boot-io/mss-boot-admin/admin@v1.3.5.\n"
                "Missing npm identity: @mss-boot-io/admin-web@1.3.5.\n"
                "Missing image identity: "
                "ghcr.io/mss-boot-io/mss-boot-admin:v1.3.5.\n",
                encoding="utf-8",
            )
            state = candidate_state()
            errors = check_current_docs.partial_release_operational_errors(root, state)

        self.assertEqual(errors, [])

    def test_partial_release_success_message_is_not_package_first(self) -> None:
        state = candidate_state()
        message = check_current_docs.success_message(state)
        self.assertNotIn("package-first", message)
        self.assertIn("candidate", message)
        self.assertIn("current stable v1.3.2", message)
        self.assertIn("immutable stopped v1.3.5, v1.3.6", message)
        self.assertIn("operational onboarding disabled", message)

    def test_rejects_source_checkout_commands_and_old_versions(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            page = root / "page.md"
            page.write_text(
                "git clone https://example.invalid/foundation\n"
                "go run ./cmd/mss verify --changed\n"
                "mss upgrade admin v1.3.2 --foundation ../foundation\n"
                "sh ./install-mss.sh --version v1.3.5\n",
                encoding="utf-8",
            )
            errors = check_current_docs.forbidden_content_errors(
                root, "v1.3.5", [Path("page.md")]
            )
        joined = "\n".join(errors)
        self.assertIn("Foundation clone command", joined)
        self.assertIn("source-only mss invocation", joined)
        self.assertIn("checkout-dependent upgrade", joined)
        self.assertIn("stale distribution token v1.3.2", joined)
        self.assertIn("POSIX sh invocation for Bash installer", joined)

    def test_stopped_version_is_allowed_only_as_non_operational_history(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            path = Path("page.md")
            (root / path).write_text(
                "v1.3.5 is permanently stopped immutable history.\n"
                "v1.3.6 is permanently stopped immutable history.\n"
                "mss upgrade admin v1.3.5\n"
                "mss upgrade admin v1.3.6\n"
                "pnpm add @mss-boot-io/admin-web@1.3.6\n",
                encoding="utf-8",
            )
            errors = check_current_docs.forbidden_content_errors(
                root,
                "v1.3.7",
                [path],
                current_stable_version="v1.3.2",
                immutable_stopped_versions=("v1.3.5", "v1.3.6"),
            )

        self.assertEqual(len(errors), 3)
        joined = "\n".join(errors)
        self.assertIn("stale distribution token v1.3.5", joined)
        self.assertIn("stale distribution token v1.3.6", joined)
        self.assertIn("stale Admin Web version 1.3.6", joined)

    def test_allows_bounded_release_history_but_rejects_stale_commands(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            relative = Path("docs/docs/releases/v1-3-5.md")
            page = root / relative
            page.parent.mkdir(parents=True)
            page.write_text(
                "v1.3.4 is immutable component-partial history; "
                "@mss-boot-io/admin-web@1.3.4 was not published.\n",
                encoding="utf-8",
            )
            self.assertEqual(
                check_current_docs.forbidden_content_errors(
                    root, "v1.3.5", [relative]
                ),
                [],
            )

            page.write_text(
                "v1.3.4 is immutable component-partial history.\n"
                "mss upgrade admin v1.3.4\n",
                encoding="utf-8",
            )
            errors = check_current_docs.forbidden_content_errors(
                root, "v1.3.5", [relative]
            )
        self.assertTrue(
            any("stale distribution token v1.3.4" in error for error in errors)
        )

    def test_rejects_historical_version_inside_current_code_block(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            relative = Path("docs/docs/releases/v1-3-5.md")
            page = root / relative
            page.parent.mkdir(parents=True)
            page.write_text(
                "v1.3.4 remains immutable history.\n"
                "```text\n"
                "candidate metadata: v1.3.4\n"
                "```\n",
                encoding="utf-8",
            )
            errors = check_current_docs.forbidden_content_errors(
                root, "v1.3.5", [relative]
            )
        self.assertTrue(
            any("stale distribution token v1.3.4" in error for error in errors)
        )

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
                root, "v1.3.5", [Path("page.md")]
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
                root, "v1.3.5", [Path("page.md")]
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
                "mss upgrade admin v1.3.7\n"
                "mss upgrade admin v1.3.7 --apply --yes\n"
                "mss doctor --strict\nmss verify --all\n"
                "mss upgrade admin v1.3.7\n"
            )
            for relative in check_current_docs.UPGRADE_CONTRACT_FILES:
                path = root / relative
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text(complete, encoding="utf-8")
            self.assertEqual(check_current_docs.upgrade_contract_errors(root), [])

            incomplete = root / check_current_docs.UPGRADE_CONTRACT_FILES[0]
            incomplete.write_text("mss upgrade admin v1.3.7\n", encoding="utf-8")
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
                "## Foundation 维护者 Skills\n"
                + "\n".join(
                    f"`{name}`" for name in check_current_docs.FOUNDATION_MAINTAINER_SKILLS
                )
                + "\n## Thin Host 分发 Skills\n"
                + "\n".join(
                    f"`{name}`" for name in check_current_docs.PUBLIC_THIN_HOST_SKILLS
                ),
                encoding="utf-8",
            )
            self.assertEqual(
                check_current_docs.public_skill_documentation_errors(root), []
            )

            path.write_text(
                "## Foundation 维护者 Skills\n"
                "`mss-add-workflow`\n"
                "## Thin Host 分发 Skills\n"
                "`mss-add-workflow`\n",
                encoding="utf-8",
            )
            errors = check_current_docs.public_skill_documentation_errors(root)
            self.assertTrue(any("mss-thin-host" in error for error in errors))
            self.assertTrue(any("Thin Host section" in error for error in errors))

    def test_documentation_audience_map_separates_human_and_agent_authority(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            for relative, markers in check_current_docs.AUDIENCE_BOUNDARY_MARKERS.items():
                path = root / relative
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text("\n".join(markers), encoding="utf-8")
            self.assertEqual(check_current_docs.documentation_audience_errors(root), [])

            agent_index = root / "docs/docs/agent/index.md"
            agent_index.write_text("Foundation 源码\n生成 Thin Host\n", encoding="utf-8")
            errors = check_current_docs.documentation_audience_errors(root)
            self.assertTrue(any("给人类看的公开说明" in error for error in errors))

            duplicate = root / "docs/docs/agent/architecture.md"
            duplicate.write_text("duplicate\n", encoding="utf-8")
            errors = check_current_docs.documentation_audience_errors(root)
            self.assertTrue(any("duplicate Agent architecture" in error for error in errors))

    def test_stopped_versions_cannot_define_active_admin_guide_scope(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            for relative in check_current_docs.ACTIVE_ADMIN_SCOPE_FILES:
                path = root / relative
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text("---\ntitle: Current Admin guide\n---\n", encoding="utf-8")
            self.assertEqual(
                check_current_docs.active_admin_scope_errors(
                    root, ("v1.3.5", "v1.3.6")
                ),
                [],
            )

            stale = root / check_current_docs.ACTIVE_ADMIN_SCOPE_FILES[0]
            stale.write_text(
                "---\ntitle: v1.3.5 Admin guide\n---\n",
                encoding="utf-8",
            )
            errors = check_current_docs.active_admin_scope_errors(
                root, ("v1.3.5", "v1.3.6")
            )
            self.assertTrue(any("must not define an active title" in error for error in errors))

    def test_template_container_bases_require_exact_versions_and_digests(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            packages = root / "docs/docs/getting-started/packages.md"
            packages.parent.mkdir(parents=True)
            packages.write_text(
                "go get github.com/mss-boot-io/mss-boot-admin/admin@v1.3.7\n"
                "go get github.com/mss-boot-io/mss-boot-admin/mss-boot@v1.3.7\n"
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

    def test_repository_context_rejects_stale_contributor_release_and_ui_text(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            stopped_versions = ("v1.3.5", "v1.3.6")
            contributor = root / "CONTRIBUTING.md"
            contributor.write_text(
                "v1.3.5 已永久停止\n"
                "v1.3.6 已永久停止\n"
                "v1.3.2 稳定记录\n"
                "本文只适用于修改 Foundation 本身的贡献者\n"
                "go run ./cmd/mss context\n"
                "go run ./cmd/mss verify --changed\n"
                "corepack pnpm@10.34.5 --dir web/antd-v6 run start:dev\n",
                encoding="utf-8",
            )
            monorepo = root / "MONOREPO.md"
            valid_monorepo = (
                "v1.3.5 is permanently stopped as an immutable partial release. "
                "v1.3.6 is permanently stopped as an immutable partial release. "
                "For a future unused version, run one non-publishing Root preview, "
                "then publish the Framework, Admin, and Admin Web tags in order. "
                "The Root tag starts only the Root Release and backend-image candidate. "
                "GitHub Latest and npmjs `latest` remain v1.3.2. Stable promotion is a "
                "separate reviewed policy decision. The operator may manually dispatch "
                "`npm-release.yml` from the exact `v1.3.7` Root tag and use "
                "npm publish --tag latest --provenance. Only then may it promote the "
                "exact Root Release to GitHub Latest. The final current-stable "
                "policy reconciliation follows through another PR. "
                "Formal workflows do not repeat expensive qualification.\n"
            )
            monorepo.write_text(valid_monorepo, encoding="utf-8")
            layout = root / "web/antd-v6/src/shared/layout/LayoutChrome.tsx"
            layout.parent.mkdir(parents=True)
            layout.write_text(
                'href="https://github.com/mss-boot-io/mss-boot-admin"\n',
                encoding="utf-8",
            )
            changelog = root / "CHANGELOG.md"
            changelog.write_text(
                "## [Unreleased]\n\nNo unreleased changes are recorded.\n\n"
                "## [v1.3.5]\n",
                encoding="utf-8",
            )
            self.assertEqual(
                check_current_docs.repository_context_errors(
                    root,
                    immutable_stopped_versions=stopped_versions,
                ),
                [],
            )

            monorepo.write_text(
                valid_monorepo
                + "Use protected Root tag promotion and finally npm Trusted "
                "Publishing.\n",
                encoding="utf-8",
            )
            obsolete = "\n".join(
                check_current_docs.repository_context_errors(
                    root,
                    immutable_stopped_versions=stopped_versions,
                )
            )
            self.assertIn("protected Root tag promotion", obsolete)
            self.assertIn("finally npm Trusted Publishing", obsolete)
            monorepo.write_text(valid_monorepo, encoding="utf-8")

            changelog.write_text(
                "## [Unreleased]\n\n- future change\n\n## [v1.3.5]\n",
                encoding="utf-8",
            )
            self.assertEqual(
                check_current_docs.repository_context_errors(
                    root,
                    immutable_stopped_versions=stopped_versions,
                ),
                [],
            )

            contributor.write_text(
                contributor.read_text(encoding="utf-8")
                + "v1.3.5 快速开始\n"
                + "gofmt -w .\ntail -f logs/app.log\n",
                encoding="utf-8",
            )
            monorepo.write_text(
                "After this migration is merged, publish Root before Frontend.\n",
                encoding="utf-8",
            )
            layout.write_text(
                'href="https://github.com/mss-boot-io/mss-boot"\n',
                encoding="utf-8",
            )
            changelog.write_text(
                "## [Unreleased]\n\n- hidden change\n\n"
                "No unreleased changes are recorded.\n\n## [v1.3.5]\n",
                encoding="utf-8",
            )
            joined = "\n".join(
                check_current_docs.repository_context_errors(
                    root,
                    immutable_stopped_versions=stopped_versions,
                )
            )
            self.assertIn("stopped-version adopter quick start", joined)
            self.assertIn("repository-wide gofmt", joined)
            self.assertIn("uncontracted log file", joined)
            self.assertIn("simplified release contract", joined)
            self.assertIn("completed import", joined)
            self.assertIn("retired Framework repository", joined)
            self.assertIn("Unreleased cannot list changes", joined)


if __name__ == "__main__":
    unittest.main()
