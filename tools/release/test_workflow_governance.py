import hashlib
import json
import os
import subprocess
import tempfile
import unittest
import zipfile
from pathlib import Path

import yaml


REPOSITORY_ROOT = Path(__file__).resolve().parents[2]
WORKFLOW_DIR = REPOSITORY_ROOT / ".github" / "workflows"
RETIRED_WORKFLOWS = (
    "docs-drift.yml",
    "frontend-cloudflare.yml",
    "release-draft.yml",
    "release-readiness.yml",
    "root-tag-promotion.yml",
    "web-editor-usage.yml",
    "web-lockfile-finalize.yml",
)
RETIRED_RELEASE_HELPERS = (
    "tools/release/verify_readiness_run.sh",
    "tools/release/release_readiness_attestation.py",
    "tools/release/release_phase_evidence.py",
    "tools/release/release_qualification_decision.py",
)
RETIRED_RELEASE_HELPER_TESTS = (
    "tools/release/test_release_readiness_attestation.py",
    "tools/release/test_release_phase_evidence.py",
    "tools/release/test_release_qualification_decision.py",
)


def load_workflow(name):
    path = WORKFLOW_DIR / name
    return yaml.load(path.read_text(encoding="utf-8"), Loader=yaml.BaseLoader)


def write_file_module_proxy(proxy_root, module, version, source_root):
    """Expose one current source tree through a replace-free Go file proxy."""
    version_root = proxy_root.joinpath(*module.split("/"), "@v")
    version_root.mkdir(parents=True, exist_ok=True)
    (version_root / "list").write_text(f"{version}\n", encoding="utf-8")
    (version_root / f"{version}.info").write_text(
        json.dumps({"Version": version, "Time": "1980-01-01T00:00:00Z"})
        + "\n",
        encoding="utf-8",
    )
    (version_root / f"{version}.mod").write_bytes(
        (source_root / "go.mod").read_bytes()
    )

    archive_prefix = f"{module}@{version}/"
    module_relative = source_root.relative_to(REPOSITORY_ROOT).as_posix()
    tracked = subprocess.run(
        ["git", "ls-files", "-z", "--", module_relative],
        cwd=REPOSITORY_ROOT,
        check=True,
        capture_output=True,
        text=True,
        encoding="utf-8",
    ).stdout.split("\0")
    sources = [REPOSITORY_ROOT / path for path in tracked if path]
    nested_modules = {
        source.parent.relative_to(source_root)
        for source in sources
        if source.name == "go.mod" and source.parent != source_root
    }
    with zipfile.ZipFile(
        version_root / f"{version}.zip", "w", compression=zipfile.ZIP_DEFLATED
    ) as archive:
        for source in sources:
            relative = source.relative_to(source_root)
            if (
                source.is_symlink()
                or not source.is_file()
                or "vendor" in relative.parts
                or any(
                    part in {".git", ".hg", ".svn", ".bzr"}
                    for part in relative.parts
                )
                or any(
                    relative == nested or nested in relative.parents
                    for nested in nested_modules
                )
            ):
                continue
            archive.write(source, archive_prefix + relative.as_posix())
        module_license = source_root / "LICENSE"
        repository_license = REPOSITORY_ROOT / "LICENSE"
        if (
            source_root != REPOSITORY_ROOT
            and not module_license.is_file()
            and repository_license.is_file()
        ):
            archive.writestr(
                archive_prefix + "LICENSE", repository_license.read_bytes()
            )


class WorkflowGovernanceTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.workflows = {
            path.name: load_workflow(path.name)
            for path in sorted(WORKFLOW_DIR.glob("*.yml"))
        }

    def test_admin_ci_context_is_repository_unique(self):
        contexts = []
        for workflow_name, workflow in self.workflows.items():
            for job_name, job in workflow.get("jobs", {}).items():
                if job.get("name", job_name) == "admin-ci":
                    contexts.append((workflow_name, job_name))

        self.assertEqual(contexts, [("ci.yml", "build")])
        self.assertEqual(
            self.workflows["ci.yml"]["jobs"]["build"]["name"], "admin-ci"
        )

    def test_release_tags_do_not_retrigger_component_ci(self):
        for workflow_name in ("ci.yml", "mss-boot-ci.yml"):
            workflow = self.workflows[workflow_name]
            self.assertEqual(workflow["on"]["push"]["branches"], ["main"])
            self.assertNotIn("tags", workflow["on"]["push"])
            self.assertEqual(
                workflow["on"]["pull_request"]["branches"], ["main"]
            )
        self.assertEqual(
            self.workflows["release.yml"]["on"]["push"]["tags"],
            ["v*.*.*"],
        )
        self.assertEqual(
            self.workflows["framework-release.yml"]["on"]["push"]["tags"],
            ["mss-boot/v*.*.*"],
        )

    def test_macos_portable_tests_use_the_real_runner_temp_directory(self):
        job = self.workflows["agent-native-ci.yml"]["jobs"][
            "cross-platform-agent-tools"
        ]
        self.assertIn("macos-15", job["strategy"]["matrix"]["os"])
        test_step = next(
            step
            for step in job["steps"]
            if step.get("name") == "Test portable Agent packages"
        )
        self.assertEqual(test_step["env"]["TMPDIR"], "${{ runner.temp }}")

    def test_retired_workflows_stay_absent(self):
        present = [
            name for name in RETIRED_WORKFLOWS if (WORKFLOW_DIR / name).exists()
        ]
        self.assertEqual(present, [])

    def test_pr_guard_owns_the_documentation_contract(self):
        workflow = self.workflows["pr-guard.yml"]
        docs_job = workflow["jobs"]["docs-contract"]
        self.assertEqual(docs_job["name"], "docs-contract")
        step = next(
            item
            for item in docs_job["steps"]
            if item.get("name")
            == "Require documentation for contract and workflow changes"
        )
        script = step["run"]
        for required in (
            "gh api",
            "--paginate",
            "--jq '.[].filename'",
            "pulls/${PR_NUMBER}/files?per_page=100",
            "docsChanged",
            "governedChange",
            "migration",
            "sql",
            r"^\.github\/workflows\/",
        ):
            with self.subTest(required=required):
                self.assertIn(required, script)

    def test_foundation_compatibility_registry_serves_exact_tarball_metadata(self):
        workflow = self.workflows["foundation-compatibility.yml"]
        steps = workflow["jobs"]["downstream-generation-and-upgrade"]["steps"]
        registry = next(
            step
            for step in steps
            if step.get("name") == "Start temporary Admin Web metadata registry"
        )
        script = registry["run"]
        for required in (
            "tarball = f'foundation-compatibility-admin-web-{version}'.encode('utf-8')",
            "sha512(tarball).digest()",
            "tarball_path = f'/artifacts/foundation-compatibility-admin-web-{version}.tgz'",
            "'tarball': f'http://127.0.0.1:{self.server.server_port}{tarball_path}'",
            "if path == tarball_path:",
            "self.wfile.write(tarball)",
        ):
            with self.subTest(required=required):
                self.assertIn(required, script)

    def test_foundation_compatibility_runs_agent_evals_against_candidate_registry(self):
        foundation = self.workflows["foundation-compatibility.yml"]
        steps = foundation["jobs"]["downstream-generation-and-upgrade"]["steps"]
        evaluation = next(
            step
            for step in steps
            if step.get("name") == "Run deterministic Agent evaluations"
        )
        script = evaluation["run"]
        self.assertIn('"${RUNNER_TEMP}/mss-current" eval run --all', script)
        self.assertIn(
            '--contributor-npm-registry "${COMPATIBILITY_FRONTEND_REGISTRY_URL}"',
            script,
        )
        self.assertNotIn("npm publish", script)
        self.assertNotIn("pnpm publish", script)

        root = self.workflows["release.yml"]
        self.assertNotIn("test", root["jobs"])
        self.assertNotIn("foundation-compatibility", root["jobs"])

    def test_scorecard_does_not_run_on_every_main_push(self):
        triggers = self.workflows["scorecard.yml"]["on"]
        self.assertEqual(
            set(triggers),
            {"branch_protection_rule", "schedule", "workflow_dispatch"},
        )
        self.assertNotIn("push", triggers)

    def test_agent_ci_runs_this_governance_suite(self):
        agent_workflow = self.workflows["agent-native-ci.yml"]
        for event in ("push", "pull_request"):
            paths = agent_workflow["on"][event]["paths"]
            self.assertIn(".github/workflows/**", paths)
            self.assertIn("tools/ci/**", paths)
            for current_document in (
                "admin/README.md",
                "mss-boot/README.md",
                "mss-boot/README.Zh-cn.md",
                "web/antd-v6/README.md",
            ):
                self.assertIn(current_document, paths)
            self.assertNotIn("docs/docs/agent/**", paths)
            self.assertNotIn(
                "docs/docs/architecture/agent-native-foundation.zh-CN.md",
                paths,
            )

        steps = agent_workflow["jobs"]["contracts-and-go"]["steps"]
        governance = next(
            step
            for step in steps
            if step.get("name") == "Verify release and workflow governance"
        )
        self.assertIn("test_detect_component_scope.py", governance["run"])
        self.assertIn("test_check_release_policy.py", governance["run"])
        for retired_test in RETIRED_RELEASE_HELPER_TESTS:
            self.assertNotIn(Path(retired_test).name, governance["run"])
        self.assertNotIn("test_release_readiness_workflow.py", governance["run"])
        self.assertIn("test_root_release_workflow.py", governance["run"])
        self.assertIn("test_verify_framework_admin_checksum.py", governance["run"])
        self.assertIn("test_verify_release_source.py", governance["run"])
        self.assertIn("test_workflow_governance.py", governance["run"])

    def test_retired_release_evidence_chain_has_no_active_entrypoint(self):
        for relative_path in RETIRED_RELEASE_HELPERS + RETIRED_RELEASE_HELPER_TESTS:
            with self.subTest(path=relative_path):
                self.assertFalse((REPOSITORY_ROOT / relative_path).exists())

        active_surfaces = (
            ".mss/commands.yaml",
            ".agents/skills/mss-release/SKILL.md",
            ".github/workflows/agent-native-ci.yml",
            "tools/compatibility/test-thin-host-external-consumer.sh",
        )
        retired_names = tuple(
            Path(relative_path).name
            for relative_path in RETIRED_RELEASE_HELPERS
            + RETIRED_RELEASE_HELPER_TESTS
        )
        for active_surface in active_surfaces:
            content = (REPOSITORY_ROOT / active_surface).read_text(encoding="utf-8")
            for retired_name in retired_names:
                with self.subTest(surface=active_surface, retired=retired_name):
                    self.assertNotIn(retired_name, content)

    def test_reusable_scope_classifier_owns_component_routing(self):
        workflow = self.workflows["component-scope.yml"]
        outputs = workflow["on"]["workflow_call"]["outputs"]
        self.assertEqual(set(outputs), {"scope", "go_modules", "changed_count"})
        detect = workflow["jobs"]["detect"]
        self.assertEqual(detect["outputs"]["scope"], "${{ steps.scope.outputs.scope }}")
        self.assertEqual(
            detect["outputs"]["go_modules"],
            "${{ steps.scope.outputs.go_modules }}",
        )
        step = next(
            item
            for item in detect["steps"]
            if item.get("name") == "Classify changed component scope"
        )
        self.assertIn("tools/ci/detect_component_scope.py", step["run"])
        self.assertIn('--github-output "${GITHUB_OUTPUT}"', step["run"])

    def test_admin_required_context_routes_heavy_jobs_by_component(self):
        jobs = self.workflows["ci.yml"]["jobs"]
        for name in ("test", "race", "static-and-module", "compile"):
            condition = jobs[name]["if"]
            self.assertIn("scope == 'admin'", condition)
            self.assertIn("scope == 'shared'", condition)
            self.assertNotIn("scope == 'framework'", condition)

        compatibility = jobs["compatibility"]["if"]
        for scope in ("admin", "framework", "shared"):
            self.assertIn(f"scope == '{scope}'", compatibility)

        aggregate = next(
            step
            for step in jobs["build"]["steps"]
            if step.get("name") == "Require every Admin check to pass"
        )["run"]
        for route in ("admin|shared)", "framework)", "docs|web)"):
            self.assertIn(route, aggregate)

    def test_admin_module_metadata_gate_uses_a_confined_prepublication_replace(self):
        job = self.workflows["ci.yml"]["jobs"]["static-and-module"]
        step = next(
            item
            for item in job["steps"]
            if item.get("name") == "Verify Admin module metadata"
        )
        self.assertEqual(
            step["run"], "bash tools/ci/verify-admin-module-metadata.sh"
        )

        script = (
            REPOSITORY_ROOT / "tools" / "ci" / "verify-admin-module-metadata.sh"
        ).read_text(encoding="utf-8")
        self.assertIn('GOWORK=off go mod tidy -modfile="${temporary_mod}"', script)
        self.assertIn(
            '-replace="${framework_module}@${framework_version}=${framework_dir}"',
            script,
        )
        self.assertIn(
            '-dropreplace="${framework_module}@${framework_version}"', script
        )
        self.assertIn(
            'diff -u -- "${admin_dir}/go.mod" "${temporary_mod}"', script
        )
        self.assertIn(
            '!($1 == module && ($2 == version || $2 == version "/go.mod"))',
            script,
        )
        self.assertIn(
            'diff -u -- "${tracked_sum_without_framework}" "${temporary_sum}"',
            script,
        )

    def test_v0_7_upgrade_candidate_uses_the_explicit_prepublication_workspace(self):
        workflow = self.workflows["v0.7-upgrade-integration.yml"]
        step = next(
            item
            for item in workflow["jobs"]["upgrade"]["steps"]
            if item.get("name") == "Upgrade real v0.7 schema to the candidate twice"
        )
        self.assertEqual(
            step["env"]["MSS_RELEASE_CANDIDATE_GOWORK"],
            "${{ github.workspace }}/go.work",
        )

        script = (
            REPOSITORY_ROOT / "tools" / "release" / "verify-v0.7-upgrade.sh"
        ).read_text(encoding="utf-8")
        self.assertIn('candidate_dependency_mode="repository-workspace"', script)
        self.assertIn('candidate_dependency_mode="public-module"', script)
        self.assertIn(
            'GOWORK="${candidate_gowork}" go build -trimpath', script
        )
        self.assertIn(
            'GOWORK="${candidate_gowork}" go test ./cmd/migrate/migration/system',
            script,
        )
        self.assertIn(
            'GOWORK="${candidate_gowork}" go list -m', script
        )
        self.assertIn('GOWORK=off go build -trimpath', script)
        self.assertIn('"candidateDependencyMode": sys.argv[16]', script)
        current_migrations = script.split(
            '"${work_dir}/bin/admin-current" migrate', 1
        )[1]
        self.assertNotIn('--password "${admin_password}"', current_migrations)
        self.assertIn(
            'MSS_ADMIN_INITIAL_PASSWORD="${admin_password}"',
            script,
        )
        self.assertIn("unset MSS_ADMIN_INITIAL_PASSWORD", current_migrations)

    def test_go_vulnerability_scans_follow_owned_modules(self):
        workflow = self.workflows["govulncheck.yml"]
        self.assertEqual(
            set(workflow["on"]["push"]["paths-ignore"]),
            {"docs/**", "web/**"},
        )
        jobs = workflow["jobs"]
        self.assertEqual(jobs["root"]["if"], "needs.scope.outputs.scope == 'shared'")
        self.assertEqual(
            jobs["nested-modules"]["strategy"]["matrix"]["module"],
            "${{ fromJSON(needs.scope.outputs.go_modules) }}",
        )
        nested_steps = jobs["nested-modules"]["steps"]
        skip = next(
            step
            for step in nested_steps
            if step.get("name") == "Skip Go vulnerability scan for this component"
        )
        self.assertEqual(skip["if"], "matrix.module == 'none'")
        for step in nested_steps:
            if step.get("name") in {
                "Checkout",
                "Setup Go",
                "Install govulncheck",
                "Scan nested module",
            }:
                self.assertEqual(step["if"], "matrix.module != 'none'")
        scan = next(
            step
            for step in nested_steps
            if step.get("name") == "Scan nested module"
        )
        self.assertEqual(
            scan["env"]["GOWORK"],
            "${{ matrix.module == 'admin' && format('{0}/go.work', github.workspace) || 'off' }}",
        )

    def test_codeql_languages_follow_component_ownership(self):
        workflow = self.workflows["codeql.yml"]
        matrix = workflow["jobs"]["analyze"]["strategy"]["matrix"]["include"]
        scopes = {item["language"]: item["scopes"] for item in matrix}
        self.assertEqual(scopes["actions"], '["shared"]')
        self.assertEqual(scopes["go"], '["shared","admin","framework"]')
        self.assertEqual(scopes["javascript-typescript"], '["shared","web"]')

        steps = workflow["jobs"]["analyze"]["steps"]
        for step in steps:
            condition = step.get("if", "")
            if step.get("name") == "Skip analysis outside the changed component":
                self.assertIn("!contains(fromJSON(matrix.scopes)", condition)
            elif step.get("name") in {
                "Checkout",
                "Initialize CodeQL",
                "Perform CodeQL Analysis",
            }:
                self.assertIn("contains(fromJSON(matrix.scopes)", condition)
                self.assertNotIn("!contains", condition)

    def test_component_owned_workflows_do_not_watch_other_owned_roots(self):
        frontend = self.workflows["frontend-v6-ci.yml"]
        for event in ("push", "pull_request"):
            paths = frontend["on"][event]["paths"]
            self.assertNotIn("admin/**", paths)
            self.assertNotIn("admin/modules/**/module.yaml", paths)
            self.assertNotIn("mss-boot/**", paths)

        mirror = self.workflows["mirror.yml"]
        self.assertIn("docs/**", mirror["on"]["push"]["paths-ignore"])

    def test_distribution_compatibility_routes_heavy_external_consumption(self):
        workflow = self.workflows["admin-distribution-compatibility.yml"]
        self.assertEqual(workflow["on"]["pull_request"]["branches"], ["main"])
        self.assertNotIn("paths", workflow["on"]["pull_request"])
        push_paths = set(workflow["on"]["push"]["paths"])
        for path in (
            "admin/**",
            "mss-boot/**",
            "web/antd-v6/**",
            "cmd/mss/**",
            "internal/mss/**",
            "templates/**",
            ".mss/**",
            "tools/compatibility/**",
            "tools/release/**",
            ".github/workflows/**",
        ):
            self.assertIn(path, push_paths)

        jobs = workflow["jobs"]
        condition = jobs["external-consumer"]["if"]
        for scope in ("admin", "framework", "web", "shared"):
            self.assertIn(f"scope == '{scope}'", condition)
        self.assertNotIn("scope == 'docs'", condition)
        external_script = next(
            step
            for step in jobs["external-consumer"]["steps"]
            if step.get("name") == "Qualify a real external Thin Host"
        )["run"]
        self.assertIn("test-thin-host-external-consumer.sh", external_script)
        process_supervision = next(
            step
            for step in jobs["external-consumer"]["steps"]
            if step.get("name") == "Test external-host process supervision"
        )["run"]
        self.assertEqual(
            process_supervision,
            "bash tools/compatibility/test-process-groups.sh",
        )
        browser_install = next(
            step
            for step in jobs["external-consumer"]["steps"]
            if step.get("name")
            == "Install Chromium for external-host qualification"
        )
        self.assertEqual(browser_install["working-directory"], "web/antd-v6")
        self.assertIn(
            "playwright install --with-deps chromium", browser_install["run"]
        )
        setup_pnpm = next(
            step
            for step in jobs["external-consumer"]["steps"]
            if step.get("name") == "Setup pnpm"
        )
        self.assertEqual(setup_pnpm["with"]["version"], "10.34.5")

        thin_host_script = (
            REPOSITORY_ROOT
            / "tools"
            / "compatibility"
            / "test-thin-host-external-consumer.sh"
        ).read_text(encoding="utf-8")
        for required in (
            "STAGE=e2e",
            "CONFIG_PROVIDER=fs",
            "MSS_V6_EXTERNAL_BACKEND=1",
            "MSS_V6_EXTERNAL_SERVER=1",
            "e2e/generated/supplier.spec.ts",
            "e2e/permission.spec.ts",
            "e2e/parity.spec.ts",
            "--project=chromium-desktop",
            "--reporter=json",
            'external-e2e.json',
            'required_titles = {',
            'external E2E reporter titles',
            '"@mss-boot-io/admin-web@file:../.mss/qualification/admin-web.tgz"',
            "entry.get('specifier') != expected",
            "resolved.startswith(expected + '(')",
            "fetch --frozen-lockfile",
            "install --offline --frozen-lockfile",
            "expected_pnpm_version='10.34.5'",
            'actual_pnpm_version="$(pnpm --version)"',
            "pnpm pack --pack-destination",
            'mss_start_process_group \\\n  backend_pid',
            'mss_start_process_group \\\n  web_pid',
            'mss_stop_process_group "${web_pid}"',
            'mss_stop_process_group "${backend_pid}"',
        ):
            with self.subTest(required=required):
                self.assertIn(required, thin_host_script)
        self.assertNotIn("corepack pnpm@10.34.5", thin_host_script)
        process_group_helper = (
            REPOSITORY_ROOT
            / "tools"
            / "compatibility"
            / "process-groups.sh"
        ).read_text(encoding="utf-8")
        self.assertIn("setsid --fork", process_group_helper)
        self.assertNotIn('wait "${pid}"', process_group_helper)
        self.assertLess(
            thin_host_script.index("fetch --frozen-lockfile"),
            thin_host_script.index("install --offline --frozen-lockfile"),
        )
        aggregate = jobs["required"]
        self.assertEqual(aggregate["name"], "admin-distribution-compatibility")
        aggregate_script = aggregate["steps"][0]["run"]
        self.assertIn("admin|framework|web|shared)", aggregate_script)
        self.assertIn("docs)", aggregate_script)
        self.assertIn("'skipped'", aggregate_script)

    def test_frontend_ci_qualifies_the_publishable_tarball_contract(self):
        workflow = self.workflows["frontend-v6-ci.yml"]
        package_job = workflow["jobs"]["package-contract"]
        package_script = next(
            step
            for step in package_job["steps"]
            if step.get("name") == "Pack and inventory the complete Admin Web package"
        )["run"]
        for required in (
            "pnpm pack",
            "manifest.gitHead = process.env.GITHUB_SHA",
            "verify_admin_web_package.py",
            "generate_admin_web_sbom.py",
            "SHA256SUMS.admin-web",
            "check_portable_paths.py",
        ):
            self.assertIn(required, package_script)
        aggregate = workflow["jobs"]["build"]
        self.assertIn("package-contract", aggregate["needs"])
        self.assertIn("PACKAGE_RESULT", aggregate["steps"][0]["run"])

    def test_frontend_release_is_protected_and_fails_closed_without_github_packages_authority(self):
        workflow = self.workflows["frontend-v6-release.yml"]
        release = workflow["jobs"]["release"]
        self.assertEqual(release["environment"], "release-v6-auto")
        for permission in ("attestations", "id-token"):
            self.assertEqual(release["permissions"][permission], "write")
        steps = release["steps"]
        identity = next(
            step
            for step in steps
            if step.get("name") == "Resolve immutable v6 release identity"
        )["run"]
        self.assertIn('"release-${version#v}"', identity)
        self.assertIn("tr '[:upper:].' '[:lower:]-'", identity)
        self.assertIn('echo "npm_dist_tag=${npm_dist_tag}"', identity)
        self.assertNotIn("npm_dist_tag=latest", identity)
        self.assertNotIn("npm_dist_tag=next", identity)
        preview = next(
            step
            for step in steps
            if step.get("name") == "Require successful exact preview"
        )
        self.assertEqual(preview["id"], "preview")
        self.assertIn("resolve_successful_preview.sh", preview["run"])
        self.assertIn('--commit "${GITHUB_SHA}"', preview["run"])
        self.assertIn('--version "${FRONTEND_V6_VERSION}"', preview["run"])
        self.assertIn('echo "run-id=${preview_run_id}"', preview["run"])
        admin_predecessor = next(
            step
            for step in steps
            if step.get("name")
            == "Require the already-published matching Admin release"
        )
        self.assertIn('admin/${FRONTEND_V6_VERSION}', admin_predecessor["run"])
        self.assertIn('${GITHUB_SHA}', admin_predecessor["run"])
        self.assertIn(".isDraft == false", admin_predecessor["run"])
        self.assertIn(".isPrerelease == $prerelease", admin_predecessor["run"])
        download = next(
            step
            for step in steps
            if step.get("name")
            == "Download exact Frontend package from the successful preview"
        )
        self.assertRegex(
            download["uses"], r"^actions/download-artifact@[0-9a-f]{40}$"
        )
        self.assertEqual(download["with"]["name"], "frontend-v6-dist")
        self.assertEqual(
            download["with"]["run-id"],
            "${{ steps.preview.outputs.run-id }}",
        )
        self.assertEqual(download["with"]["github-token"], "${{ github.token }}")
        self.assertEqual(download["with"]["repository"], "${{ github.repository }}")

        stage = next(
            step
            for step in steps
            if step.get("name")
            == "Verify and stage the exact Frontend preview package"
        )["run"]
        for required in (
            'sha256sum --check SHA256SUMS.frontend-v6',
            'sha256sum --check SHA256SUMS.admin-web',
            'sha256sum --check SHA256SUMS.frontend-v6-image',
            'version=${RELEASE_VERSION}',
            'commit=${GITHUB_SHA}',
            'tar -xOf "${preview_dir}/dist-v6.tar.gz" dist/release.json',
            'tar -xOf "${tarball}" package/package.json',
            'artifact.get("integrity") == expected_integrity',
            '"blobs/sha256/${image_manifest_hash}"',
            'test "${actual_image_digest}" = "${expected_image_digest}"',
            'cp -a "${preview_package_dir}/." "${package_dir}/"',
        ):
            self.assertIn(required, stage)
        for forbidden in (
            "pnpm install",
            "pnpm pack",
            "pnpm list",
            "generate_admin_web_sbom.py",
            "verify_admin_web_package.py",
            "check_portable_paths.py",
            '.spdxVersion == "SPDX-2.3"',
            "tar --extract",
            "build:release",
            "delivery:smoke",
        ):
            self.assertNotIn(forbidden, stage)

        step_names = [step.get("name") for step in steps]
        for forbidden in (
            "Setup pnpm",
            "Setup Node",
            "Install frozen v6 dependency graph",
            "Build same-origin v6 release",
            "Smoke-test v6 production delivery",
            "Inject and qualify the Admin Distribution package version",
            "Upload Admin Web package evidence",
            "Upload v6 build artifact",
            "Set up QEMU",
            "Set up Docker Buildx",
            "Inspect immutable v6 image publication state",
            "Extract v6 Docker metadata",
            "Build and push v6 Docker image",
            "Verify published v6 image identity",
        ):
            self.assertNotIn(forbidden, step_names)

        auth = next(
            step
            for step in steps
            if step.get("name") == "Configure GitHub Packages authentication"
        )
        self.assertEqual(auth["env"]["NODE_AUTH_TOKEN"], "${{ github.token }}")
        self.assertIn("NPM_CONFIG_USERCONFIG", auth["run"])
        self.assertIn("npm.pkg.github.com/:_authToken", auth["run"])

        credential = next(
            step
            for step in steps
            if step.get("name")
            == "Require GitHub Packages authority and reconcile version identity"
        )
        self.assertEqual(credential["env"]["NODE_AUTH_TOKEN"], "${{ github.token }}")
        self.assertIn('-z "${NODE_AUTH_TOKEN}"', credential["run"])
        self.assertIn("E404", credential["run"])
        self.assertIn("dist.integrity", credential["run"])
        self.assertIn("https://npm.pkg.github.com", credential["run"])
        self.assertIn("existing npm version has different immutable content", credential["run"])
        self.assertNotIn(
            "Preflight immutable Admin Web publication without mutation",
            [step.get("name") for step in steps],
        )
        self.assertNotIn("--dry-run", (WORKFLOW_DIR / "frontend-v6-release.yml").read_text(encoding="utf-8"))

        attestation = next(
            step
            for step in steps
            if step.get("name") == "Attest Admin Web package provenance"
        )
        self.assertRegex(
            attestation["uses"],
            r"^actions/attest-build-provenance@[0-9a-f]{40}$",
        )
        self.assertEqual(
            attestation["if"], "steps.npm-version.outputs.publish == 'true'"
        )
        publish = next(
            step
            for step in steps
            if step.get("name") == "Publish immutable Admin Web package to GitHub Packages"
        )
        self.assertIn("npm publish", publish["run"])
        self.assertIn('--tag "${NPM_DIST_TAG}"', publish["run"])
        self.assertIn("https://npm.pkg.github.com", publish["run"])
        self.assertIn("release-*", publish["run"])
        self.assertNotIn("latest|next", publish["run"])
        self.assertEqual(
            publish["if"], "steps.npm-version.outputs.publish == 'true'"
        )
        self.assertEqual(
            publish["env"]["NPM_DIST_TAG"],
            "${{ steps.version.outputs.npm_dist_tag }}",
        )
        self.assertEqual(publish["env"]["NODE_AUTH_TOKEN"], "${{ github.token }}")
        self.assertLess(steps.index(attestation), steps.index(publish))
        require_public = next(
            step
            for step in steps
            if step.get("name")
            == "Require the GitHub Packages mirror to be public"
        )
        self.assertIn('.visibility == "public"', require_public["run"])
        self.assertNotIn('.visibility == "private"', require_public["run"])
        verify_package = next(
            step
            for step in steps
            if step.get("name")
            == "Verify GitHub Packages identity and repository binding"
        )
        self.assertIn('.visibility == "public"', verify_package["run"])
        self.assertNotIn('.visibility == "private"', verify_package["run"])
        self.assertIn("npm dist-tag ls", verify_package["run"])
        self.assertNotIn(
            "npm view '@mss-boot-io/admin-web' dist-tags",
            verify_package["run"],
        )
        self.assertIn('$1 == expected_tag ":"', verify_package["run"])
        self.assertIn(
            "did not resolve after bounded verification",
            verify_package["run"],
        )
        self.assertIn("sleep 5", verify_package["run"])
        self.assertEqual(
            verify_package["env"]["NPM_DIST_TAG"],
            "${{ steps.version.outputs.npm_dist_tag }}",
        )
        self.assertIn("admin-web", verify_package["run"])

        image_publication = next(
            step
            for step in steps
            if step.get("name") == "Publish the exact preview V6 OCI image"
        )
        self.assertEqual(image_publication["id"], "image")
        self.assertLess(steps.index(credential), steps.index(image_publication))
        image_script = image_publication["run"]
        for required in (
            'archive="${RUNNER_TEMP}/frontend-v6-preview/frontend-v6-image.oci.tar"',
            'digest="$(sed -n',
            "inspect_digest()",
            "preflight_reference()",
            "validate_preflight()",
            "publish_reference_if_missing()",
            "verify_exact_reference()",
            "skopeo copy",
            "--all",
            "--preserve-digests",
            "--dest-authfile",
            "--digestfile",
            '"oci-archive:${archive}"',
            'preflight_reference "${version_ref}" frontend-v6-version',
            'preflight_reference "${commit_ref}" frontend-v6-commit',
            'validate_preflight "${version_ref}" frontend-v6-version',
            'validate_preflight "${commit_ref}" frontend-v6-commit',
            "image publication preflight failed before any registry write",
            'publish_reference_if_missing "${version_ref}" frontend-v6-version',
            'publish_reference_if_missing "${commit_ref}" frontend-v6-commit',
            'verify_exact_reference "${version_ref}" frontend-v6-version',
            'verify_exact_reference "${commit_ref}" frontend-v6-commit',
            "already has $(cat \"${observed_digest_file}\"), expected ${digest}",
            '.platform.architecture == "amd64"',
            '.platform.architecture == "arm64"',
            '--config "docker://${version_ref}"',
            'echo "digest=${digest}"',
        ):
            self.assertIn(required, image_script)
        workflow_content = (WORKFLOW_DIR / "frontend-v6-release.yml").read_text(
            encoding="utf-8"
        )
        for forbidden in (
            "docker/build-push-action@",
            "docker/setup-buildx-action@",
            "docker/setup-qemu-action@",
            "docker buildx",
            "push: true",
            ":latest",
        ):
            self.assertNotIn(forbidden, workflow_content)

        github_release = next(
            step
            for step in steps
            if step.get("name") == "Publish immutable v6 GitHub release"
        )
        self.assertIn("gh release download", github_release["run"])
        self.assertIn("cmp --", github_release["run"])
        self.assertIn(".isDraft'", github_release["run"])
        self.assertNotIn(
            "Update the mutable stable v6 image alias last",
            [step.get("name") for step in steps],
        )

    def test_frontend_image_publication_preflights_both_refs_before_writes(self):
        workflow = self.workflows["frontend-v6-release.yml"]
        image_script = next(
            step
            for step in workflow["jobs"]["release"]["steps"]
            if step.get("name") == "Publish the exact preview V6 OCI image"
        )["run"]
        calls = [line.strip() for line in image_script.splitlines()]
        ordered_calls = (
            'preflight_reference "${version_ref}" frontend-v6-version',
            'preflight_reference "${commit_ref}" frontend-v6-commit',
            'validate_preflight "${version_ref}" frontend-v6-version || preflight_failed=1',
            'validate_preflight "${commit_ref}" frontend-v6-commit || preflight_failed=1',
            'publish_reference_if_missing "${version_ref}" frontend-v6-version',
            'publish_reference_if_missing "${commit_ref}" frontend-v6-commit',
            'verify_exact_reference "${version_ref}" frontend-v6-version',
            'verify_exact_reference "${commit_ref}" frontend-v6-commit',
        )
        positions = [calls.index(call) for call in ordered_calls]
        self.assertEqual(positions, sorted(positions))
        self.assertIn(
            "manifest unknown|manifest_unknown|name unknown|name_unknown",
            image_script,
        )
        self.assertNotIn("name_unknown|not found", image_script)

        version = "v9.8.7"
        commit = "0123456789abcdef0123456789abcdef01234567"
        manifest = json.dumps(
            {
                "schemaVersion": 2,
                "mediaType": "application/vnd.oci.image.index.v1+json",
                "manifests": [
                    {
                        "mediaType": "application/vnd.oci.image.manifest.v1+json",
                        "digest": f"sha256:{'a' * 64}",
                        "size": 1,
                        "platform": {"os": "linux", "architecture": "amd64"},
                    },
                    {
                        "mediaType": "application/vnd.oci.image.manifest.v1+json",
                        "digest": f"sha256:{'b' * 64}",
                        "size": 1,
                        "platform": {"os": "linux", "architecture": "arm64"},
                    },
                ],
            },
            separators=(",", ":"),
        )
        digest = f"sha256:{hashlib.sha256(manifest.encode()).hexdigest()}"
        conflict_manifest = json.dumps(
            {"schemaVersion": 2, "manifests": []}, separators=(",", ":")
        )
        config = json.dumps(
            {
                "config": {
                    "Labels": {
                        "org.opencontainers.image.title": "mss-boot-admin-antd-v6",
                        "org.opencontainers.image.description": "Complete React 19 and Ant Design 6 Admin application for mss-boot business hosts.",
                        "org.opencontainers.image.version": version,
                        "org.opencontainers.image.revision": commit,
                    }
                }
            },
            separators=(",", ":"),
        )
        cases = (
            ("both missing", "missing", "missing", ("version", "commit"), True),
            ("one missing", "missing", "matching", ("version",), True),
            ("both matching", "matching", "matching", (), True),
            ("commit conflicts", "missing", "conflict", (), False),
            ("version conflicts", "conflict", "missing", (), False),
            ("inspection error", "missing", "error", (), False),
            ("generic not found error", "missing", "generic-not-found", (), False),
        )

        with tempfile.TemporaryDirectory() as directory:
            fixture_root = Path(directory)
            fake_bin = fixture_root / "bin"
            fake_bin.mkdir()
            fake_skopeo = fake_bin / "skopeo"
            fake_skopeo.write_text(
                """#!/usr/bin/env bash
set -euo pipefail
reference="${!#}"
case "${reference}" in
  *":${VERSION}") key=version ;;
  *":${GITHUB_SHA}") key=commit ;;
  *) echo "unexpected reference: ${reference}" >&2; exit 64 ;;
esac
state_file="${FAKE_STATE_DIR}/${key}"
if [[ " $* " == *" --config "* ]]; then
  cat "${FAKE_CONFIG}"
  exit 0
fi
if [[ "${1:-}" == inspect ]]; then
  printf 'inspect:%s\n' "${key}" >> "${FAKE_OPERATIONS}"
  case "$(cat "${state_file}")" in
    matching) cat "${FAKE_MANIFEST}" ;;
    conflict) cat "${FAKE_CONFLICT_MANIFEST}" ;;
    missing) echo 'manifest unknown' >&2; exit 1 ;;
    error) echo 'temporary registry inspection failure' >&2; exit 1 ;;
    generic-not-found) echo 'credential helper executable not found' >&2; exit 1 ;;
    *) exit 65 ;;
  esac
  exit 0
fi
if [[ "${1:-}" == copy ]]; then
  printf 'copy:%s\n' "${key}" >> "${FAKE_OPERATIONS}"
  digest_file=''
  for ((argument_index=1; argument_index <= $#; argument_index++)); do
    if [[ "${!argument_index}" == --digestfile ]]; then
      digest_index=$((argument_index + 1))
      digest_file="${!digest_index}"
    fi
  done
  test -n "${digest_file}"
  printf '%s\n' "${EXPECTED_DIGEST}" > "${digest_file}"
  printf '%s\n' matching > "${state_file}"
  exit 0
fi
exit 66
""",
                encoding="utf-8",
            )
            fake_skopeo.chmod(0o755)
            manifest_path = fixture_root / "manifest.json"
            manifest_path.write_text(manifest, encoding="utf-8")
            conflict_manifest_path = fixture_root / "conflict-manifest.json"
            conflict_manifest_path.write_text(conflict_manifest, encoding="utf-8")
            config_path = fixture_root / "config.json"
            config_path.write_text(config, encoding="utf-8")

            for name, version_state, commit_state, expected_writes, succeeds in cases:
                with self.subTest(name=name):
                    case_root = fixture_root / name.replace(" ", "-")
                    runner_temp = case_root / "runner"
                    preview = runner_temp / "frontend-v6-preview"
                    state_dir = case_root / "state"
                    home = case_root / "home"
                    preview.mkdir(parents=True)
                    state_dir.mkdir(parents=True)
                    (home / ".docker").mkdir(parents=True)
                    (preview / "frontend-v6-image.oci.tar").write_bytes(b"oci")
                    (preview / "FRONTEND-V6-IMAGE-INFO").write_text(
                        f"application=mss-boot-admin-antd-v6\nversion={version}\ncommit={commit}\ndigest={digest}\n",
                        encoding="utf-8",
                    )
                    (home / ".docker" / "config.json").write_text(
                        "{}\n", encoding="utf-8"
                    )
                    (state_dir / "version").write_text(
                        f"{version_state}\n", encoding="utf-8"
                    )
                    (state_dir / "commit").write_text(
                        f"{commit_state}\n", encoding="utf-8"
                    )
                    operations = case_root / "operations"
                    output = case_root / "github-output"
                    environment = os.environ.copy()
                    environment.pop("DOCKER_CONFIG", None)
                    environment.update(
                        {
                            "PATH": f"{fake_bin}:{environment['PATH']}",
                            "HOME": str(home),
                            "RUNNER_TEMP": str(runner_temp),
                            "GITHUB_OUTPUT": str(output),
                            "GITHUB_SHA": commit,
                            "REGISTRY": "ghcr.example.invalid",
                            "IMAGE_NAME": "mss/example",
                            "VERSION": version,
                            "FAKE_STATE_DIR": str(state_dir),
                            "FAKE_OPERATIONS": str(operations),
                            "FAKE_MANIFEST": str(manifest_path),
                            "FAKE_CONFLICT_MANIFEST": str(conflict_manifest_path),
                            "FAKE_CONFIG": str(config_path),
                            "EXPECTED_DIGEST": digest,
                        }
                    )
                    result = subprocess.run(
                        ["bash", "-c", image_script],
                        cwd=REPOSITORY_ROOT,
                        env=environment,
                        capture_output=True,
                        text=True,
                        encoding="utf-8",
                    )
                    self.assertEqual(
                        result.returncode == 0,
                        succeeds,
                        msg=f"stdout:\n{result.stdout}\nstderr:\n{result.stderr}",
                    )
                    operation_lines = (
                        operations.read_text(encoding="utf-8").splitlines()
                        if operations.exists()
                        else []
                    )
                    self.assertEqual(
                        operation_lines[:2], ["inspect:version", "inspect:commit"]
                    )
                    writes = tuple(
                        line.removeprefix("copy:")
                        for line in operation_lines
                        if line.startswith("copy:")
                    )
                    self.assertEqual(writes, expected_writes)
                    if succeeds:
                        self.assertIn(f"digest={digest}", output.read_text(encoding="utf-8"))
                    else:
                        self.assertIn(
                            "before any registry write", result.stderr
                        )

    def test_admin_module_tag_is_a_light_preview_backed_publication(self):
        workflow = self.workflows["admin-release.yml"]
        self.assertEqual(workflow["on"]["push"]["tags"], ["admin/v*.*.*"])
        release = workflow["jobs"]["release"]
        self.assertEqual(release["environment"], "release-auto")
        steps = release["steps"]
        self.assertEqual(
            [step.get("name") for step in steps],
            [
                "Require authorized release operator",
                "Checkout",
                "Resolve immutable Admin module identity",
                "Verify merged-main release source",
                "Enforce coordinated Admin Distribution target",
                "Require successful exact preview",
                "Reconcile existing public Admin release",
                "Require the already-published matching Framework release",
                "Prepare component-scoped release notes",
                "Publish immutable Admin module release",
            ],
        )

        source = next(
            step
            for step in steps
            if step.get("name") == "Verify merged-main release source"
        )["run"]
        self.assertIn("verify_release_source.py", source)
        self.assertIn('--tag "${GITHUB_REF_NAME}"', source)

        policy = next(
            step
            for step in steps
            if step.get("name") == "Enforce coordinated Admin Distribution target"
        )["run"]
        self.assertIn("--component admin", policy)

        preview = next(
            step
            for step in steps
            if step.get("name") == "Require successful exact preview"
        )["run"]
        self.assertIn("resolve_successful_preview.sh", preview)
        self.assertIn('--commit "${GITHUB_SHA}"', preview)
        self.assertIn('--version "${ADMIN_VERSION}"', preview)

        release_state = next(
            step
            for step in steps
            if step.get("name") == "Reconcile existing public Admin release"
        )
        self.assertEqual(release_state["id"], "release-state")
        self.assertIn("--json tagName,isDraft,isPrerelease", release_state["run"])
        self.assertIn(".tagName == $tag", release_state["run"])
        self.assertIn(".isPrerelease == $prerelease", release_state["run"])

        framework = next(
            step
            for step in steps
            if step.get("name")
            == "Require the already-published matching Framework release"
        )["run"]
        self.assertIn('mss-boot/${ADMIN_VERSION}', framework)
        self.assertIn('${GITHUB_SHA}', framework)
        self.assertIn(".isDraft == false", framework)
        self.assertIn(".isPrerelease == $prerelease", framework)

        notes = next(
            step
            for step in steps
            if step.get("name") == "Prepare component-scoped release notes"
        )["run"]
        self.assertIn("successful Root preview", notes)
        self.assertIn("go get github.com/mss-boot-io/mss-boot-admin/admin@", notes)
        self.assertNotIn("generate-notes", notes)

        publish = next(
            step
            for step in steps
            if step.get("name") == "Publish immutable Admin module release"
        )
        self.assertRegex(
            publish["uses"],
            r"^softprops/action-gh-release@[0-9a-f]{40}$",
        )
        self.assertEqual(publish["with"]["make_latest"], "false")
        self.assertEqual(publish["with"]["target_commitish"], "${{ github.sha }}")
        self.assertEqual(
            publish["if"], "steps.release-state.outputs.public != 'true'"
        )

        tag_content = (WORKFLOW_DIR / "admin-release.yml").read_text(
            encoding="utf-8"
        )
        for preview_owned_check in (
            "Setup Go",
            "go test",
            "go vet",
            "go build",
            "verify_framework_admin_checksum.py",
            "Probe the tagged Admin module",
            "test-thin-host-external-consumer.sh",
        ):
            self.assertNotIn(preview_owned_check, tag_content)

    def test_framework_tag_is_a_light_preview_backed_publication(self):
        workflow = self.workflows["framework-release.yml"]
        self.assertEqual(workflow["on"]["push"]["tags"], ["mss-boot/v*.*.*"])
        release = workflow["jobs"]["release"]
        self.assertEqual(release["environment"], "release-auto")
        steps = release["steps"]
        self.assertEqual(
            [step.get("name") for step in steps],
            [
                "Require authorized release operator",
                "Checkout",
                "Resolve framework version",
                "Verify merged-main release source",
                "Enforce reviewed public release target",
                "Require successful exact preview",
                "Reconcile existing public component release",
                "Prepare component-scoped release notes",
                "Publish framework release",
            ],
        )

        source = next(
            step
            for step in steps
            if step.get("name") == "Verify merged-main release source"
        )["run"]
        self.assertIn("verify_release_source.py", source)
        self.assertIn('--tag "${GITHUB_REF_NAME}"', source)

        policy = next(
            step
            for step in steps
            if step.get("name") == "Enforce reviewed public release target"
        )["run"]
        self.assertIn("--component framework", policy)

        preview = next(
            step
            for step in steps
            if step.get("name") == "Require successful exact preview"
        )["run"]
        self.assertIn("resolve_successful_preview.sh", preview)
        self.assertIn('--commit "${GITHUB_SHA}"', preview)
        self.assertIn('--version "${FRAMEWORK_VERSION}"', preview)

        release_state = next(
            step
            for step in steps
            if step.get("name")
            == "Reconcile existing public component release"
        )
        self.assertEqual(release_state["id"], "release-state")
        self.assertIn("--json tagName,isDraft,isPrerelease", release_state["run"])
        self.assertIn(".tagName == $tag", release_state["run"])
        self.assertIn(".isPrerelease == $prerelease", release_state["run"])

        notes = next(
            step
            for step in steps
            if step.get("name") == "Prepare component-scoped release notes"
        )["run"]
        self.assertIn("successful Root preview", notes)
        self.assertIn("go get github.com/mss-boot-io/mss-boot-admin/mss-boot@", notes)
        self.assertNotIn("generate-notes", notes)

        publish = next(
            step
            for step in steps
            if step.get("name") == "Publish framework release"
        )
        self.assertRegex(
            publish["uses"],
            r"^softprops/action-gh-release@[0-9a-f]{40}$",
        )
        self.assertEqual(publish["with"]["make_latest"], "false")
        self.assertEqual(publish["with"]["target_commitish"], "${{ github.sha }}")
        self.assertEqual(
            publish["if"], "steps.release-state.outputs.public != 'true'"
        )

        tag_content = (WORKFLOW_DIR / "framework-release.yml").read_text(
            encoding="utf-8"
        )
        for preview_owned_check in (
            "Setup Go",
            "go test",
            "go vet",
            "go build",
            "verify_framework_admin_checksum.py",
            "Probe the published module",
            "test-thin-host-external-consumer.sh",
        ):
            self.assertNotIn(preview_owned_check, tag_content)

    def test_root_publication_reuses_preview_and_checks_component_identity_once(self):
        workflow = self.workflows["release.yml"]
        evidence_steps = workflow["jobs"]["release-evidence"]["steps"]
        preview = next(
            step
            for step in evidence_steps
            if step.get("name") == "Require successful exact preview"
        )["run"]
        self.assertIn("resolve_successful_preview.sh", preview)
        self.assertIn('--commit "${RELEASE_COMMIT}"', preview)
        self.assertIn('--version "${RELEASE_VERSION}"', preview)
        self.assertIn('--actor "${RELEASE_ACTOR_LOGIN}"', preview)
        evidence = next(
            step
            for step in evidence_steps
            if step.get("name") == "Require exact component releases"
        )["run"]
        for required in (
            "mss-boot/${RELEASE_VERSION}",
            "admin/${RELEASE_VERSION}",
            "web/antd-v6/${RELEASE_VERSION}",
            "required component tag ${component_tag} is not available",
            "required component release ${component_tag} is not public",
        ):
            self.assertIn(required, evidence)
        for obsolete_wait in (
            "for attempt in",
            "sleep ",
            "component_train_ready",
            "/actions/workflows/",
            ".workflow_runs",
            "framework-release.yml",
            "admin-release.yml",
            "frontend-v6-release.yml",
        ):
            self.assertNotIn(obsolete_wait, evidence)
        for duplicate_probe in (
            "GOPROXY=direct",
            "npm view",
            "https://npm.pkg.github.com",
            "go mod download",
        ):
            self.assertNotIn(duplicate_probe, evidence)
        self.assertNotIn(
            "packages", workflow["jobs"]["release-evidence"]["permissions"]
        )
        self.assertNotIn("docs.yml", evidence)
        publish = workflow["jobs"]["publish"]
        self.assertEqual(publish["environment"], "release-auto")
        self.assertEqual(
            [step.get("name") for step in publish["steps"]],
            [
                "Require authorized release operator",
                "Download exact preview packages",
                "Prepare deterministic root release notes",
                "Stage, verify, and publish GitHub release atomically",
            ],
        )

    def test_docs_publication_can_follow_from_a_later_merged_main_commit(self):
        workflow = self.workflows["docs.yml"]
        self.assertNotIn("actions", workflow["permissions"])
        steps = workflow["jobs"]["build"]["steps"]
        predecessor = next(
            step
            for step in steps
            if step.get("name")
            == "Require the already-published base root release"
        )
        script = predecessor["run"]
        for required in (
            'root_tag="${DOCS_VERSION%%+docs.*}"',
            'git rev-parse "refs/tags/${root_tag}^{commit}"',
            'gh release view "${root_tag}"',
            "--json tagName,targetCommitish,isDraft,isPrerelease",
            '--arg commit "${root_commit}"',
            ".targetCommitish == $commit",
            ".isDraft == false",
            ".isPrerelease == false",
        ):
            self.assertIn(required, script)
        for forbidden in (
            "actions/workflows/release.yml/runs",
            "workflow_runs",
            'head_sha="${root_commit}"',
            "display_title",
            'workflow_dispatch',
        ):
            self.assertNotIn(forbidden, script)
        self.assertNotIn('"${root_commit}" != "${GITHUB_SHA}"', script)
        self.assertLess(
            steps.index(predecessor),
            next(
                index
                for index, step in enumerate(steps)
                if step.get("name") == "Build documentation"
            ),
        )

        deployment = workflow["jobs"]["deployment"]
        self.assertEqual(deployment["environment"], "prod")
        deployment_steps = deployment["steps"]
        credential = next(
            step
            for step in deployment_steps
            if step.get("name")
            == "Require organization-managed Cloudflare credential"
        )
        self.assertEqual(
            credential["env"],
            {"CF_API_TOKEN": "${{ secrets.CF_API_TOKEN }}"},
        )
        self.assertIn('[[ -z "${CF_API_TOKEN:-}" ]]', credential["run"])
        publish = next(
            step
            for step in deployment_steps
            if step.get("name") == "Deploy production documentation"
        )
        self.assertEqual(
            publish["with"]["apiToken"], "${{ secrets.CF_API_TOKEN }}"
        )
        self.assertLess(
            deployment_steps.index(credential), deployment_steps.index(publish)
        )

    def test_root_tag_automatically_drives_root_image_and_npm_without_docs(self):
        root = self.workflows["release.yml"]
        metadata = next(
            step
            for step in root["jobs"]["metadata"]["steps"]
            if step.get("name") == "Resolve and validate release metadata"
        )
        self.assertIn("publish=true", metadata["run"])
        self.assertIn("publish=false", metadata["run"])
        self.assertEqual(
            set(root["on"]["workflow_dispatch"]["inputs"]), {"version"}
        )
        self.assertEqual(root["on"]["push"]["tags"], ["v*.*.*"])
        self.assertEqual(root["jobs"]["publish"]["environment"], "release-auto")
        self.assertIn(
            "'root-release-publication'", root["concurrency"]["group"]
        )

        container = self.workflows["container.yml"]
        self.assertEqual(container["on"]["push"]["tags"], ["v*.*.*"])
        self.assertIn("github.event_name == 'push'", container["jobs"]["publish"]["if"])
        self.assertEqual(container["jobs"]["publish"]["environment"], "release-auto")
        npm = self.workflows["npm-release.yml"]
        self.assertEqual(npm["on"]["push"]["tags"], ["v*.*.*"])
        self.assertEqual(npm["jobs"]["publish"]["environment"], "npm-auto")
        self.assertEqual(npm["permissions"]["id-token"], "write")
        self.assertEqual(
            npm["jobs"]["publish"]["permissions"]["id-token"], "write"
        )
        self.assertEqual(
            npm["concurrency"],
            {
                "group": "official-npm-publication",
                "cancel-in-progress": "false",
            },
        )
        npm_content = (WORKFLOW_DIR / "npm-release.yml").read_text(encoding="utf-8")
        self.assertIn("Require the exact frontend release", npm_content)
        official_publish = next(
            step
            for step in npm["jobs"]["publish"]["steps"]
            if step.get("name") == "Publish the existing tarball to official npm"
        )
        self.assertIn("unset NPM_TOKEN NODE_AUTH_TOKEN NPM_CONFIG_USERCONFIG", official_publish["run"])
        self.assertIn("npm publish", official_publish["run"])
        self.assertIn("--provenance", official_publish["run"])
        self.assertNotIn("NODE_AUTH_TOKEN", official_publish.get("env", {}))
        self.assertNotIn("NPM_TOKEN", official_publish.get("env", {}))
        self.assertNotIn("secrets.NPM_TOKEN", npm_content)
        self.assertNotIn("secrets.NODE_AUTH_TOKEN", npm_content)
        self.assertNotIn("Root Release ${RELEASE_VERSION}", npm_content)
        self.assertNotIn("container.yml", npm_content)
        self.assertNotIn("docs.yml", npm_content)
        self.assertNotIn("docs/${RELEASE_VERSION}", npm_content)
        for forbidden in (
            "root-tag-promotion.yml",
            "readiness_run_id",
            "Root Release publish",
        ):
            self.assertNotIn(forbidden, (WORKFLOW_DIR / "release.yml").read_text(encoding="utf-8"))

    def test_release_authority_requires_the_exact_operator_before_checkout_or_write(self):
        gated_jobs = {
            ("npm-release.yml", "publish"): None,
            ("framework-release.yml", "release"): None,
            ("admin-release.yml", "release"): None,
            ("frontend-v6-release.yml", "release"): None,
            ("release.yml", "metadata"): (
                "${{ github.event_name == 'workflow_dispatch' || "
                "github.ref_type == 'tag' }}"
            ),
            ("release.yml", "publish"): None,
            ("container.yml", "build"): (
                "${{ inputs.release_preview == true || "
                "github.ref_type == 'tag' }}"
            ),
            ("container.yml", "publish"): None,
            ("docs.yml", "build"): "startsWith(github.ref, 'refs/tags/docs/v')",
            ("docs.yml", "deployment"): None,
        }
        for (workflow_name, job_name), expected_condition in gated_jobs.items():
            with self.subTest(workflow=workflow_name, job=job_name):
                job = self.workflows[workflow_name]["jobs"][job_name]
                gate = job["steps"][0]
                self.assertEqual(gate["name"], "Require authorized release operator")
                self.assertEqual(gate["env"]["RELEASE_ACTOR_LOGIN"], "lwnmengjing")
                self.assertEqual(gate.get("if"), expected_condition)
                self.assertIn(
                    '"${GITHUB_ACTOR}" != "${RELEASE_ACTOR_LOGIN}"', gate["run"]
                )
                self.assertIn(
                    '"${GITHUB_TRIGGERING_ACTOR}" != "${RELEASE_ACTOR_LOGIN}"',
                    gate["run"],
                )
                self.assertNotIn(
                    "SullivanPrime",
                    (WORKFLOW_DIR / workflow_name).read_text(encoding="utf-8"),
                )

        root_gate = self.workflows["release.yml"]["jobs"]["metadata"]["steps"][0]
        self.assertIn("github.ref_type == 'tag'", root_gate["if"])
        self.assertIn("github.event_name == 'workflow_dispatch'", root_gate["if"])

        container_gate = self.workflows["container.yml"]["jobs"]["build"][
            "steps"
        ][0]
        self.assertIn("github.ref_type == 'tag'", container_gate["if"])
        self.assertIn("inputs.release_preview == true", container_gate["if"])
        self.assertNotIn("github.event_name == 'workflow_call'", container_gate["if"])
        self.assertNotIn("github.event_name == 'workflow_dispatch'", container_gate["if"])

        docs_gate = self.workflows["docs.yml"]["jobs"]["build"]["steps"][0]
        self.assertNotIn("github.event_name == 'push'", docs_gate["if"])
        self.assertIn("refs/tags/docs/v", docs_gate["if"])

        for workflow_name, job_name in (
            ("frontend-v6-release.yml", "release"),
            ("docs.yml", "build"),
        ):
            with self.subTest(workflow=workflow_name, pre_checkout_directory=True):
                gate = self.workflows[workflow_name]["jobs"][job_name]["steps"][0]
                self.assertEqual(
                    gate["working-directory"], "${{ github.workspace }}"
                )

    def test_local_release_qualification_owns_quality_gates_before_artifact_preview(self):
        makefile = (REPOSITORY_ROOT / "Makefile").read_text(encoding="utf-8")
        self.assertIn(
            "deps-admin:\n\tcd $(ADMIN_DIR) && GOWORK=off go mod download",
            makefile,
        )
        self.assertIn(
            'temporary_gowork="$$(mktemp "$(CURDIR)/.release-preview-go.XXXXXX.work")"',
            makefile,
        )
        self.assertIn(
            'GOWORK="$$temporary_gowork" go mod download',
            makefile,
        )
        self.assertIn(
            'rm -f -- "$$temporary_gowork" "$$temporary_gowork.sum"',
            makefile,
        )
        self.assertIn(
            "deps-release-preview: deps-agent deps-admin-workspace deps-framework",
            makefile,
        )
        self.assertIn(
            "tidy-admin-prepublication-check:\n\tbash tools/ci/verify-admin-module-metadata.sh",
            makefile,
        )
        self.assertIn(
            "verify-admin: test-admin-race coverage-admin vet-admin tidy-admin-check compatibility-admin build-admin",
            makefile,
        )
        self.assertIn(
            "verify-admin-preview: test-admin-race coverage-admin vet-admin tidy-admin-prepublication-check compatibility-admin build-admin",
            makefile,
        )

        root_jobs = self.workflows["release.yml"]["jobs"]
        self.assertNotIn("test", root_jobs)
        self.assertNotIn("foundation-compatibility", root_jobs)
        for required in (
            "corepack pnpm@10.34.5 deps:check",
            "corepack pnpm@10.34.5 dedupe --check",
            "corepack pnpm@10.34.5 audit:release",
            "$(MAKE) web-v6-lint",
            "$(MAKE) web-v6-test",
            "$(MAKE) web-v6-build",
            "corepack pnpm@10.34.5 delivery:smoke",
            "bash tools/verification/run-frontend-e2e.sh",
        ):
            self.assertIn(required, makefile)

        compatibility_steps = self.workflows["foundation-compatibility.yml"]["jobs"][
            "downstream-generation-and-upgrade"
        ]["steps"]
        names = [step.get("name") for step in compatibility_steps]
        self.assertLess(
            names.index("Generate a standalone downstream repository"),
            names.index("Run deterministic Agent evaluations"),
        )
        self.assertLess(
            names.index("Run deterministic Agent evaluations"),
            names.index("Stop temporary Admin Web metadata registry"),
        )


if __name__ == "__main__":
    unittest.main()
