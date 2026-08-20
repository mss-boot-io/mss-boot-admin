import importlib.util
import re
import subprocess
import tempfile
import unittest
from pathlib import Path

import yaml


MODULE_PATH = Path(__file__).with_name("check_release_policy.py")
SPEC = importlib.util.spec_from_file_location("check_release_policy", MODULE_PATH)
assert SPEC and SPEC.loader
POLICY = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(POLICY)


REPOSITORY_ROOT = Path(__file__).resolve().parents[2]
POLICY_PATH = REPOSITORY_ROOT / ".mss" / "release-policy.yaml"
GITHUB_EXPRESSION = re.compile(r"\$\{\{.*?\}\}", re.DOTALL)


class ReleasePolicyTest(unittest.TestCase):
    def setUp(self):
        self.policy = POLICY.load_policy(POLICY_PATH)

    def test_v130_rc1_matches_every_distribution_component_namespace(self):
        cases = {
            "root": "v1.3.0-rc.1",
            "framework": "mss-boot/v1.3.0-rc.1",
            "admin": "admin/v1.3.0-rc.1",
            "frontend": "web/antd-v6/v1.3.0-rc.1",
            "docs": "docs/v1.3.0-rc.1",
        }
        for component, tag in cases.items():
            with self.subTest(component=component):
                POLICY.check_public_ref(
                    self.policy, component, "v1.3.0-rc.1", tag, intent="qualify"
                )

        self.assertEqual(
            POLICY.coordinated_tags(self.policy, "v1.3.0-rc.1"),
            {component: cases[component] for component in POLICY.COORDINATED_COMPONENTS},
        )

    def test_publication_is_enabled_after_protected_workflows_are_ready(self):
        self.assertIs(self.policy["publicationWorkflowsReady"], True)
        self.assertIs(self.policy["publicPrereleases"], True)
        POLICY.check_public_ref(
            self.policy, "root", "v1.3.0-rc.1", "v1.3.0-rc.1"
        )

    def test_policy_requires_pr_merged_main_release_source(self):
        self.assertEqual(self.policy["releaseBranch"], "main")
        self.assertIs(self.policy["requireMergedPullRequestSource"], True)

    def test_policy_rejects_versions_other_than_v130_rc1(self):
        for version in (
            "v1.0.1",
            "v1.1.0",
            "v1.2.0",
            "v1.2.1",
            "v1.2.2",
            "v1.2.3",
            "v1.3.0",
            "v1.3.1",
        ):
            with self.subTest(version=version):
                with self.assertRaisesRegex(POLICY.PolicyError, "forbidden"):
                    POLICY.check_public_ref(
                        self.policy, "root", version, version, intent="qualify"
                    )

    def test_policy_rejects_other_prereleases_and_wrong_namespace(self):
        with self.assertRaises(POLICY.PolicyError):
            POLICY.check_public_ref(
                self.policy,
                "root",
                "v1.3.0-rc.2",
                "v1.3.0-rc.2",
                intent="qualify",
            )
        with self.assertRaisesRegex(POLICY.PolicyError, "does not match"):
            POLICY.check_public_ref(
                self.policy,
                "framework",
                "v1.3.0-rc.1",
                "v1.3.0-rc.1",
                intent="qualify",
            )

    def test_policy_rejects_distribution_version_or_component_drift(self):
        original = POLICY_PATH.read_text(encoding="utf-8")
        replacements = (
            ("  distributionVersion: v1.3.0-rc.1\n", "  distributionVersion: v1.3.1\n"),
            (
                '  distributionComponents: "root,framework,admin,frontend"\n',
                '  distributionComponents: "root,framework,frontend"\n',
            ),
        )
        for old, new in replacements:
            with self.subTest(replacement=new.strip()):
                with tempfile.TemporaryDirectory() as directory:
                    candidate = Path(directory) / "policy.yaml"
                    candidate.write_text(original.replace(old, new), encoding="utf-8")
                    with self.assertRaises(POLICY.PolicyError):
                        POLICY.load_policy(candidate)

    def test_policy_rejects_invalid_or_disabled_prerelease_contracts(self):
        original = POLICY_PATH.read_text(encoding="utf-8")
        replacements = (
            ("  publicPrereleases: true\n", "  publicPrereleases: false\n"),
            ("  nextPublicVersion: v1.3.0-rc.1\n", "  nextPublicVersion: v1.3.0-rc.01\n"),
            ("  currentStableVersion: v1.2.3\n", "  currentStableVersion: v1.2.3-rc.1\n"),
        )
        for old, new in replacements:
            with self.subTest(replacement=new.strip()):
                with tempfile.TemporaryDirectory() as directory:
                    candidate = Path(directory) / "policy.yaml"
                    content = original.replace(old, new)
                    if "nextPublicVersion" in new:
                        content = content.replace(
                            "  distributionVersion: v1.3.0-rc.1\n",
                            "  distributionVersion: v1.3.0-rc.01\n",
                        )
                    candidate.write_text(content, encoding="utf-8")
                    with self.assertRaises(POLICY.PolicyError):
                        POLICY.load_policy(candidate)

    def test_policy_parser_rejects_unknown_or_duplicate_keys(self):
        original = POLICY_PATH.read_text(encoding="utf-8")
        for suffix in (
            "  unexpected: true\n",
            "  nextPublicVersion: v1.3.0-rc.1\n",
        ):
            with self.subTest(suffix=suffix.strip()):
                with tempfile.TemporaryDirectory() as directory:
                    candidate = Path(directory) / "policy.yaml"
                    candidate.write_text(original + suffix, encoding="utf-8")
                    with self.assertRaises(POLICY.PolicyError):
                        POLICY.load_policy(candidate)

    def test_policy_parser_rejects_weakened_release_source_governance(self):
        original = POLICY_PATH.read_text(encoding="utf-8")
        replacements = (
            ("  releaseBranch: main\n", "  releaseBranch: release\n"),
            (
                "  requireMergedPullRequestSource: true\n",
                "  requireMergedPullRequestSource: false\n",
            ),
        )
        for old, new in replacements:
            with self.subTest(replacement=new.strip()):
                with tempfile.TemporaryDirectory() as directory:
                    candidate = Path(directory) / "policy.yaml"
                    candidate.write_text(original.replace(old, new), encoding="utf-8")
                    with self.assertRaises(POLICY.PolicyError):
                        POLICY.load_policy(candidate)

    def test_publication_workflows_share_policy_and_exact_attestation_guards(self):
        workflows = (
            "release.yml",
            "framework-release.yml",
            "admin-release.yml",
            "frontend-v6-release.yml",
            "container.yml",
        )
        for workflow in workflows:
            with self.subTest(workflow=workflow):
                content = (
                    REPOSITORY_ROOT / ".github" / "workflows" / workflow
                ).read_text(encoding="utf-8")
                self.assertIn("check_release_policy.py", content)
                self.assertIn("verify_readiness_run.sh", content)
                self.assertIn("RELEASE_READINESS_RUN_ID", content)
                self.assertNotIn(
                    "/actions/workflows/release-readiness.yml/runs", content
                )

    def test_release_workflows_require_pr_merged_main_source_and_exact_tag(self):
        cases = {
            "release.yml": ("release-evidence", True),
            "framework-release.yml": ("release", True),
            "admin-release.yml": ("release", True),
            "frontend-v6-release.yml": ("release", True),
            "container.yml": ("publish", True),
            "docs.yml": ("build", True),
            "release-readiness.yml": ("full-verification", False),
        }
        for workflow_name, (job_name, requires_tag) in cases.items():
            with self.subTest(workflow=workflow_name):
                content = (
                    REPOSITORY_ROOT / ".github" / "workflows" / workflow_name
                ).read_text(encoding="utf-8")
                workflow = yaml.load(content, Loader=yaml.BaseLoader)
                job = workflow["jobs"][job_name]
                permissions = job.get("permissions", workflow.get("permissions", {}))
                self.assertEqual(permissions.get("pull-requests"), "read")
                guard_index, guard = next(
                    (index, step)
                    for index, step in enumerate(job["steps"])
                    if step.get("name") == "Verify merged-main release source"
                )
                self.assertIn("verify_release_source.py", guard["run"])
                if requires_tag:
                    self.assertIn("--tag", guard["run"])
                else:
                    self.assertNotIn("--tag", guard["run"])
                self.assertLess(guard_index, len(job["steps"]) - 1)

        root_release = yaml.load(
            (
                REPOSITORY_ROOT / ".github" / "workflows" / "release.yml"
            ).read_text(encoding="utf-8"),
            Loader=yaml.BaseLoader,
        )
        self.assertIn(
            "release-evidence", root_release["jobs"]["release"]["needs"]
        )

    def test_release_workflow_yaml_and_run_blocks_are_valid(self):
        workflows = (
            "release.yml",
            "framework-release.yml",
            "admin-release.yml",
            "admin-distribution-compatibility.yml",
            "frontend-v6-release.yml",
            "frontend-v6-ci.yml",
            "container.yml",
            "release-readiness.yml",
            "docs.yml",
        )
        for workflow_name in workflows:
            with self.subTest(workflow=workflow_name):
                content = (
                    REPOSITORY_ROOT / ".github" / "workflows" / workflow_name
                ).read_text(encoding="utf-8")
                workflow = yaml.load(content, Loader=yaml.BaseLoader)
                self.assertIsInstance(workflow.get("jobs"), dict)
                for job in workflow["jobs"].values():
                    for step in job.get("steps", []):
                        script = step.get("run")
                        if script is None:
                            continue
                        result = subprocess.run(
                            ["bash", "-n"],
                            input=GITHUB_EXPRESSION.sub("gha_expression", script),
                            text=True,
                            capture_output=True,
                            check=False,
                        )
                        self.assertEqual(
                            result.returncode,
                            0,
                            msg=(
                                f"invalid bash in {workflow_name} step "
                                f"{step.get('name')}: {result.stderr}"
                            ),
                        )

    def test_every_frontend_release_upload_uses_portable_archive_only(self):
        cases = {
            "frontend-v6-release.yml": (
                "release",
                "Embed and verify v6 artifact identity",
                "Upload v6 build artifact",
            ),
            "release.yml": (
                "frontend-build",
                "Package and verify portable primary V6 artifact",
                "Upload primary V6 artifact",
            ),
        }
        expected_paths = [
            "web/antd-v6/dist-v6.tar.gz",
            "web/antd-v6/FRONTEND-V6-BUILD-INFO",
            "web/antd-v6/SHA256SUMS.frontend-v6",
        ]
        for workflow_name, (job_name, package_name, upload_name) in cases.items():
            with self.subTest(workflow=workflow_name):
                workflow = yaml.load(
                    (
                        REPOSITORY_ROOT / ".github" / "workflows" / workflow_name
                    ).read_text(encoding="utf-8"),
                    Loader=yaml.BaseLoader,
                )
                steps = workflow["jobs"][job_name]["steps"]
                package = next(
                    step for step in steps if step.get("name") == package_name
                )
                upload = next(
                    step for step in steps if step.get("name") == upload_name
                )
                paths = [
                    line.strip()
                    for line in upload["with"]["path"].splitlines()
                    if line.strip()
                ]

                for required in (
                    "tar",
                    "--create",
                    "--gzip",
                    "--file dist-v6.tar.gz",
                    "dist",
                ):
                    self.assertIn(required, package["run"])
                self.assertIn("check_portable_paths.py", package["run"])
                self.assertEqual(paths, expected_paths)

    def test_all_raw_directory_uploads_have_portability_guards(self):
        extensionless_files = {"FRONTEND-V6-BUILD-INFO", "SHA256SUMS"}
        raw_uploads = []
        for workflow_path in sorted(
            (REPOSITORY_ROOT / ".github" / "workflows").glob("*.yml")
        ):
            workflow = yaml.load(
                workflow_path.read_text(encoding="utf-8"), Loader=yaml.BaseLoader
            )
            for job_name, job in workflow.get("jobs", {}).items():
                steps = job.get("steps", [])
                for upload_index, step in enumerate(steps):
                    if "actions/upload-artifact@" not in step.get("uses", ""):
                        continue
                    paths = [
                        line.strip()
                        for line in step.get("with", {}).get("path", "").splitlines()
                        if line.strip()
                    ]
                    for upload_path in paths:
                        basename = upload_path.rsplit("/", 1)[-1]
                        portable_basename = GITHUB_EXPRESSION.sub(
                            "expression", basename
                        )
                        is_file = (
                            "." in portable_basename
                            or portable_basename in extensionless_files
                            or portable_basename.startswith("SHA256SUMS")
                        )
                        if is_file:
                            continue
                        guard = next(
                            (
                                previous
                                for previous in reversed(steps[:upload_index])
                                if "check_portable_paths.py" in previous.get("run", "")
                            ),
                            None,
                        )
                        self.assertIsNotNone(
                            guard,
                            msg=(
                                f"{workflow_path.name}:{job_name}:{step.get('name')} "
                                f"uploads raw directory {upload_path} without a portability guard"
                            ),
                        )
                        self.assertIn(basename, guard["run"])
                        raw_uploads.append(
                            (workflow_path.name, job_name, upload_path)
                        )

        self.assertEqual(
            raw_uploads,
            [
                (
                    "frontend-v6-ci.yml",
                    "browser",
                    "web/antd-v6/playwright-report",
                ),
                ("frontend-v6-ci.yml", "browser", "web/antd-v6/test-results"),
                ("release-readiness.yml", "full-verification", ".mss/reports"),
                (
                    "release.yml",
                    "backend-build",
                    "mss-boot-admin-${{ matrix.os }}-${{ matrix.arch }}",
                ),
            ],
        )

    def test_root_release_validates_extracted_frontend_and_every_final_zip(self):
        content = (
            REPOSITORY_ROOT / ".github" / "workflows" / "release.yml"
        ).read_text(encoding="utf-8")
        workflow = yaml.load(content, Loader=yaml.BaseLoader)
        assemble = next(
            step
            for step in workflow["jobs"]["release"]["steps"]
            if step.get("name") == "Assemble release packages"
        )
        script = assemble["run"]
        self.assertIn("sha256sum --check SHA256SUMS.frontend-v6", script)
        self.assertIn("frontend-v6-extracted/dist", script)
        self.assertIn(
            "check_portable_paths.py mss-boot-admin-*.zip", script
        )

    def test_static_release_builds_normalize_dynamic_route_placeholders(self):
        frontend_package = (
            REPOSITORY_ROOT / "web" / "antd-v6" / "package.json"
        ).read_text(encoding="utf-8")
        docs_package = (REPOSITORY_ROOT / "docs" / "package.json").read_text(
            encoding="utf-8"
        )
        self.assertIn("prepare_portable_frontend.py dist", frontend_package)
        self.assertIn("prepare_portable_frontend.py dist", docs_package)

    def test_static_ci_runs_when_portability_tooling_changes(self):
        for workflow_name in ("frontend-v6-ci.yml", "docs.yml"):
            with self.subTest(workflow=workflow_name):
                content = (
                    REPOSITORY_ROOT / ".github" / "workflows" / workflow_name
                ).read_text(encoding="utf-8")
                self.assertGreaterEqual(content.count("'tools/release/**'"), 2)

    def test_docs_release_is_component_scoped_and_merged_main_only(self):
        content = (
            REPOSITORY_ROOT / ".github" / "workflows" / "docs.yml"
        ).read_text(encoding="utf-8")
        workflow = yaml.load(content, Loader=yaml.BaseLoader)
        build_steps = workflow["jobs"]["build"]["steps"]
        deployment = workflow["jobs"]["deployment"]

        self.assertIn("docs/v*.*.*", content)
        self.assertIn("refs/tags/docs/v", deployment["if"])
        self.assertNotIn("refs/heads/main", deployment["if"])
        self.assertTrue(
            any(
                "verify_release_source.py" in step.get("run", "")
                and "--tag" in step.get("run", "")
                for step in build_steps
            )
        )
        self.assertTrue(
            any(
                "check_release_policy.py" in step.get("run", "")
                and "--component docs" in step.get("run", "")
                for step in build_steps
            )
        )
        for required in (
            "dist/release.json",
            "DOCS-BUILD-INFO.txt",
            "SHA256SUMS.docs",
            "gh release create",
            "https://docs.mss-boot-io.top/release.json",
        ):
            self.assertIn(required, content)

    def test_local_browser_qualification_cannot_reuse_the_development_server(self):
        playwright = (
            REPOSITORY_ROOT / "web" / "antd-v6" / "playwright.config.ts"
        ).read_text(encoding="utf-8")
        package = (
            REPOSITORY_ROOT / "web" / "antd-v6" / "package.json"
        ).read_text(encoding="utf-8")
        support = (
            REPOSITORY_ROOT / "web" / "antd-v6" / "e2e" / "support" / "session.ts"
        ).read_text(encoding="utf-8")
        backend = (
            REPOSITORY_ROOT
            / "web"
            / "antd-v6"
            / "scripts"
            / "start-e2e-backend.sh"
        ).read_text(encoding="utf-8")
        e2e_config = (
            REPOSITORY_ROOT / "admin" / "config" / "application-e2e.yml"
        ).read_text(encoding="utf-8")
        package_business_config = (
            REPOSITORY_ROOT / "web" / "antd-v6" / "package" / "business.cjs"
        ).read_text(encoding="utf-8")
        generated_supplier = (
            REPOSITORY_ROOT
            / "web"
            / "antd-v6"
            / "e2e"
            / "generated"
            / "supplier.spec.ts"
        ).read_text(encoding="utf-8")
        supplier_template = (
            REPOSITORY_ROOT
            / "templates"
            / "module"
            / "frontend-v6"
            / "e2e.spec.ts.tmpl"
        ).read_text(encoding="utf-8")

        self.assertNotIn("reuseExistingServer: !process.env.CI", playwright)
        self.assertEqual(playwright.count("reuseExistingServer: false"), 2)
        self.assertIn("http://127.0.0.1:18001", playwright)
        self.assertIn('"start:e2e"', package)
        self.assertIn("MSS_V6_E2E=1", package)
        self.assertIn(
            "persistentCaching: !browserQualification", package_business_config
        )
        for content in (
            support,
            backend,
            e2e_config,
            generated_supplier,
            supplier_template,
        ):
            self.assertIn("18001", content)
        for content in (support, backend, generated_supplier, supplier_template):
            self.assertNotIn("http://127.0.0.1:8001", content)

    def test_publication_workflows_require_the_phase_they_publish_from(self):
        expected_phases = {
            "framework-release.yml": "--phase pre-framework",
            "admin-release.yml": "--phase pre-framework",
            "frontend-v6-release.yml": "--phase pre-framework",
            "container.yml": "--phase pre-root",
            "release.yml": "--phase pre-root",
        }
        for workflow, expected_phase in expected_phases.items():
            with self.subTest(workflow=workflow):
                content = (
                    REPOSITORY_ROOT / ".github" / "workflows" / workflow
                ).read_text(encoding="utf-8")
                self.assertIn(expected_phase, content)

    def test_readiness_is_manual_and_release_bound(self):
        content = (
            REPOSITORY_ROOT / ".github" / "workflows" / "release-readiness.yml"
        ).read_text(encoding="utf-8")
        self.assertNotIn("\n  pull_request:", content)
        self.assertNotIn("\n  push:", content)
        for required in (
            "workflow_dispatch:",
            "frozen_commit:",
            "feature_freeze_confirmed:",
            "phase:",
            "publication_authority:",
            "release-readiness-metadata.json",
            "release-readiness-attestation-${{ github.run_id }}",
            "release_readiness_attestation.py",
            "check_release_policy.py",
        ):
            self.assertIn(required, content)

    def test_development_push_does_not_publish_a_checkpoint_image(self):
        content = (
            REPOSITORY_ROOT / ".github" / "workflows" / "container.yml"
        ).read_text(encoding="utf-8")
        self.assertNotIn(
            '[[ "${GITHUB_EVENT_NAME}" == "push" ]]',
            content,
        )
        self.assertIn("verify_readiness_run.sh", content)
        self.assertNotIn(
            "/actions/workflows/release-readiness.yml/runs", content
        )

    def test_selected_run_helper_verifies_run_and_artifact_identity(self):
        content = (
            REPOSITORY_ROOT / "tools" / "release" / "verify_readiness_run.sh"
        ).read_text(encoding="utf-8")
        for required in (
            "/actions/runs/${run_id}",
            '.head_sha == $commit',
            '.conclusion == "success"',
            '.path == $workflow_path',
            '.html_url == $workflow_run_url',
            "release-readiness-attestation-${run_id}",
            "--intent publish",
        ):
            self.assertIn(required, content)

    def test_root_release_has_no_published_version_default(self):
        content = (
            REPOSITORY_ROOT / ".github" / "workflows" / "release.yml"
        ).read_text(encoding="utf-8")
        self.assertNotIn("default: v1.0.0", content)


if __name__ == "__main__":
    unittest.main()
