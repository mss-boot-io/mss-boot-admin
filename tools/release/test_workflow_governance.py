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
    "foundation-compatibility.yml",
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

    def test_foundation_compatibility_is_a_single_local_gate(self):
        agent = self.workflows["agent-native-ci.yml"]
        root = self.workflows["release.yml"]
        self.assertNotIn("foundation-compatibility", agent["jobs"])
        self.assertNotIn("foundation-compatibility", root["jobs"])

        agent_source = (WORKFLOW_DIR / "agent-native-ci.yml").read_text(
            encoding="utf-8"
        )
        self.assertNotIn("test-standalone-mss-consumer.sh --upgrade", agent_source)

        makefile = (REPOSITORY_ROOT / "Makefile").read_text(encoding="utf-8")
        self.assertIn(
            "compatibility-foundation-next:\n"
            "\tbash tools/compatibility/test-standalone-mss-consumer.sh "
            "--upgrade --next-foundation",
            makefile,
        )

    def test_scorecard_does_not_run_on_every_main_push(self):
        triggers = self.workflows["scorecard.yml"]["on"]
        self.assertEqual(
            set(triggers),
            {"branch_protection_rule", "schedule", "workflow_dispatch"},
        )
        self.assertNotIn("push", triggers)

    def test_agent_ci_is_manual_and_runs_this_governance_suite(self):
        agent_workflow = self.workflows["agent-native-ci.yml"]
        self.assertEqual(set(agent_workflow["on"]), {"workflow_dispatch"})

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

        for name in ("race", "static-and-module", "compatibility", "compile"):
            self.assertIn("github.event_name != 'pull_request'", jobs[name]["if"])
        test_steps = jobs["test"]["steps"]
        ordinary = next(
            step
            for step in test_steps
            if step.get("name") == "Test coordinated Admin module"
        )
        self.assertEqual(ordinary["if"], "github.event_name == 'pull_request'")
        self.assertEqual(ordinary["run"], "go test -shuffle=on -count=1 ./...")
        for step_name in (
            "Test coverage policy tooling",
            "Test coordinated Admin module with atomic coverage",
            "Enforce Admin coverage policy",
            "Upload Admin coverage profile",
        ):
            step = next(item for item in test_steps if item.get("name") == step_name)
            self.assertEqual(step["if"], "github.event_name != 'pull_request'")

    def test_extended_agent_and_frontend_matrices_are_manual_only(self):
        expected_jobs = {
            "agent-native-ci.yml": {
                "contracts-and-go",
                "cross-platform-agent-tools",
            },
            "frontend-v6-ci.yml": {
                "generated-contract",
                "dependency-contract",
                "quality",
                "package-contract",
                "compile",
                "browser",
                "build",
            },
        }
        for workflow_name, required_jobs in expected_jobs.items():
            with self.subTest(workflow=workflow_name):
                workflow = self.workflows[workflow_name]
                self.assertEqual(set(workflow["on"]), {"workflow_dispatch"})
                self.assertEqual(set(workflow["jobs"]), required_jobs)

    def test_other_broad_component_matrices_are_post_merge_or_manual_not_pr_gates(self):
        for workflow_name in (
            "admin-distribution-compatibility.yml",
            "container.yml",
            "docs.yml",
            "pat-migration-integration.yml",
            "swagger.yml",
            "theme-settings-integration.yml",
            "v0.7-upgrade-integration.yml",
        ):
            with self.subTest(workflow=workflow_name):
                triggers = self.workflows[workflow_name]["on"]
                self.assertNotIn("pull_request", triggers)
                self.assertIn("push", triggers)
                self.assertEqual(triggers["push"]["branches"], ["main"])
                if workflow_name == "container.yml":
                    self.assertIn("workflow_call", triggers)
                else:
                    self.assertIn("workflow_dispatch", triggers)

        framework = self.workflows["mss-boot-ci.yml"]
        self.assertIn("pull_request", framework["on"])
        jobs = framework["jobs"]
        for name in ("race", "static-and-module"):
            self.assertIn("github.event_name != 'pull_request'", jobs[name]["if"])
        aggregate = jobs["required"]["steps"][0]["run"]
        self.assertIn('if [[ "${EVENT_NAME}" == "pull_request" ]]', aggregate)
        self.assertIn('test "${RACE_RESULT}" = "skipped"', aggregate)

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
        self.assertIn(
            'GOWORK=off GOFLAGS= go mod tidy -modfile="${temporary_mod}"', script
        )
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

    def test_mirror_does_not_watch_documentation_owned_roots(self):
        mirror = self.workflows["mirror.yml"]
        self.assertIn("docs/**", mirror["on"]["push"]["paths-ignore"])

    def test_distribution_compatibility_routes_heavy_external_consumption(self):
        workflow = self.workflows["admin-distribution-compatibility.yml"]
        self.assertNotIn("pull_request", workflow["on"])
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
            "CONFIG_PROVIDER=local",
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
            'pnpm_command=(corepack "pnpm@${expected_pnpm_version}")',
            'actual_pnpm_version="$("${pnpm_command[@]}" --version)"',
            "run_pnpm pack --pack-destination",
            "current.bind(('127.0.0.1', 0))",
            'backend_origin="http://127.0.0.1:${backend_port}"',
            'web_origin="http://127.0.0.1:${web_port}"',
            "run_pnpm exec playwright install chromium",
            "mss.io/thin-host-local-evidence/v1",
            "MSS_PERSIST_EVIDENCE",
            "evidence-manifest.json",
            "flock -w 600",
            '"${runtime_dir}/config/application.yml"',
            '"${runtime_dir}/config/application-e2e.yml"',
            'mss_start_process_group \\\n  backend_pid',
            'mss_start_process_group \\\n  web_pid',
            'mss_stop_process_group "${web_pid}"',
            'mss_stop_process_group "${backend_pid}"',
        ):
            with self.subTest(required=required):
                self.assertIn(required, thin_host_script)
        self.assertNotIn("PORT=18001", thin_host_script)
        self.assertNotIn(
            "MSS_ADMIN_API_TARGET=http://127.0.0.1:18080", thin_host_script
        )
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
        build = thin_host_script.index("run_pnpm run build")
        lock = thin_host_script.index("port_start_lock_path=", build)
        allocation = thin_host_script.index("read -r backend_port web_port", lock)
        backend_ready = thin_host_script.index(
            '"${backend_origin}/healthz"', allocation
        )
        frontend_stable = thin_host_script.index(
            '"external Thin Host frontend stability check"', backend_ready
        )
        unlock = thin_host_script.index("release_port_start_lock", frontend_stable)
        self.assertLess(build, lock)
        self.assertLess(lock, allocation)
        self.assertLess(allocation, backend_ready)
        self.assertLess(backend_ready, frontend_stable)
        self.assertLess(frontend_stable, unlock)
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
        github_release_script = github_release["run"]
        for required in (
            "gh release download",
            "cmp --",
            ".isDraft'",
            "--json tagName,targetCommitish,isDraft,isPrerelease,name,body,assets",
            ".targetCommitish == $commit",
            '--target "${GITHUB_SHA}"',
        ):
            self.assertIn(required, github_release_script)
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

    def test_admin_module_tag_keeps_only_public_dependency_and_publication_boundaries(self):
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
                "Reconcile existing public Admin release",
                "Require the already-published matching Framework release",
                "Setup Go for the public Framework boundary",
                "Resolve and test the exact public Framework",
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

        self.assertFalse(
            any(step.get("name") == "Require successful exact preview" for step in steps)
        )

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

        public_framework = next(
            step
            for step in steps
            if step.get("name") == "Resolve and test the exact public Framework"
        )
        self.assertEqual(public_framework["working-directory"], "admin")
        self.assertEqual(public_framework["env"]["GOWORK"], "off")
        self.assertEqual(public_framework["env"]["GOPROXY"], "https://proxy.golang.org")
        self.assertEqual(public_framework["env"]["GOFLAGS"], "-mod=readonly")
        for required in (
            'go mod download -json "${framework_module}@${ADMIN_VERSION}"',
            ".Path == $module and .Version == $version",
            "go test ./app ./business",
        ):
            self.assertIn(required, public_framework["run"])

        notes = next(
            step
            for step in steps
            if step.get("name") == "Prepare component-scoped release notes"
        )["run"]
        self.assertIn("Broad quality verification was completed locally", notes)
        self.assertIn("public Framework with GOWORK=off", notes)
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
        for local_quality_check in (
            "go test -race",
            "go vet",
            "go build",
            "verify_framework_admin_checksum.py",
            "Probe the tagged Admin module",
            "test-thin-host-external-consumer.sh",
        ):
            self.assertNotIn(local_quality_check, tag_content)

    def test_framework_tag_keeps_one_cheap_preview_boundary_before_publication(self):
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
                "Require successful exact artifact preview",
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
            if step.get("name") == "Require successful exact artifact preview"
        )["run"]
        self.assertIn("resolve_successful_preview.sh", preview)
        self.assertIn('--commit "${GITHUB_SHA}"', preview)
        self.assertIn('--version "${FRAMEWORK_VERSION}"', preview)
        self.assertIn("--actor lwnmengjing", preview)

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
        self.assertIn("Broad quality verification was completed locally", notes)
        self.assertIn("first irreversible component", notes)
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
        for local_quality_check in (
            "Setup Go",
            "go test",
            "go vet",
            "go build",
            "verify_framework_admin_checksum.py",
            "Probe the published module",
            "test-thin-host-external-consumer.sh",
        ):
            self.assertNotIn(local_quality_check, tag_content)

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

    def test_docs_base_is_exact_root_and_revision_is_exactly_authorized(self):
        workflow = self.workflows["docs.yml"]
        self.assertNotIn("actions", workflow["permissions"])
        steps = workflow["jobs"]["build"]["steps"]
        source = next(
            step
            for step in steps
            if step.get("name") == "Verify merged-main release source"
        )["run"]
        self.assertIn("source_mode='release'", source)
        self.assertIn("source_mode='promotion'", source)
        self.assertIn('--source-mode "${source_mode}"', source)

        policy = next(
            step
            for step in steps
            if step.get("name") == "Enforce reviewed docs release target"
        )["run"]
        for required in (
            "refs/remotes/origin/main:.mss/release-policy.yaml",
            '--policy "${policy_path}"',
            '--commit "${GITHUB_SHA}"',
        ):
            self.assertIn(required, policy)

        revision = next(
            step
            for step in steps
            if step.get("name") == "Require the lowest unused docs revision"
        )
        self.assertIn("+docs.", revision["if"])
        for required in (
            'revision="${DOCS_VERSION##*+docs.}"',
            'seq 1 "$((revision - 1))"',
            'show-ref --verify --quiet "${prior_tag}"',
            "is not the lowest unused revision",
        ):
            self.assertIn(required, revision["run"])

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
            'if [[ "${DOCS_VERSION}" != *+docs.* ]]; then',
            'if [[ "${GITHUB_SHA}" != "${root_commit}" ]]; then',
            'if [[ "${GITHUB_SHA}" == "${root_commit}" ]]; then',
            'merge-base --is-ancestor "${root_commit}" "${GITHUB_SHA}"',
            "would roll production content back before Root",
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
        self.assertEqual(
            deployment["concurrency"],
            {
                "group": "cloudflare-docs-prod",
                "queue": "max",
                "cancel-in-progress": "false",
            },
        )
        deployment_steps = deployment["steps"]
        checkout = next(
            step
            for step in deployment_steps
            if step.get("name") == "Checkout deployment configuration"
        )
        self.assertEqual(checkout["with"]["ref"], "${{ github.sha }}")
        self.assertEqual(checkout["with"]["fetch-depth"], "0")

        revalidate = next(
            step
            for step in deployment_steps
            if step.get("name") == "Revalidate serialized Docs publication authority"
        )
        self.assertEqual(revalidate["id"], "deployment-state")
        for required in (
            "git fetch --force --tags origin",
            "refs/remotes/origin/main",
            "origin/main:.mss/release-policy.yaml",
            "check_release_policy.py",
            '--commit "${GITHUB_SHA}"',
            'merge-base --is-ancestor "${root_commit}" "${GITHUB_SHA}"',
            "check_docs_deployment_state.py",
            'predecessor_tag="docs/${predecessor_version}"',
            'gh release view "${predecessor_tag}"',
            '["DOCS-BUILD-INFO.txt", "SHA256SUMS.docs", "docs-dist.tar.gz"]',
            'gh release download "${predecessor_tag}"',
            "sha256sum --strict --check SHA256SUMS.docs",
            "./release.json",
            "deploy=true",
            "deploy=false",
        ):
            self.assertIn(required, revalidate["run"])
        self.assertLess(
            revalidate["run"].index("check_release_policy.py"),
            revalidate["run"].index('seq 1 "$((revision - 1))"'),
        )

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
        self.assertEqual(
            credential["if"], "steps.deployment-state.outputs.deploy == 'true'"
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
        self.assertEqual(
            publish["if"], "steps.deployment-state.outputs.deploy == 'true'"
        )
        self.assertLess(
            deployment_steps.index(credential), deployment_steps.index(publish)
        )

        release = next(
            step
            for step in deployment_steps
            if step.get("name") == "Reconcile immutable docs GitHub release"
        )["run"]
        for required in (
            "--json tagName,targetCommitish,isDraft,isPrerelease,name,body,assets",
            ".targetCommitish == $commit",
            "gh release download",
            "cmp --",
            "--draft",
            "gh release upload",
            "--clobber",
            "gh release edit",
            "--draft=false",
            "--latest=false",
        ):
            self.assertIn(required, release)
        final_reconciliation = release[release.index('final_release="$(') :]
        self.assertIn('--rawfile body "${notes}"', final_reconciliation)
        self.assertNotIn("--arg body", final_reconciliation)
        self.assertIn(".body == $body", final_reconciliation)
        self.assertFalse(
            any(
                step.get("name") == "Refuse mutation of an existing docs release"
                for step in steps
            )
        )

    def test_root_tag_drives_only_root_release_and_image(self):
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

        root_release = next(
            step
            for step in root["jobs"]["publish"]["steps"]
            if step.get("name")
            == "Stage, verify, and publish GitHub release atomically"
        )["run"]
        self.assertIn("--latest=false", root_release)
        self.assertNotIn("releases/latest", root_release)

        npm = self.workflows["npm-release.yml"]
        self.assertNotIn("push", npm["on"])
        self.assertEqual(set(npm["on"]), {"workflow_dispatch"})
        self.assertEqual(
            set(npm["on"]["workflow_dispatch"]["inputs"]), {"version"}
        )
        self.assertEqual(
            npm["on"]["workflow_dispatch"]["inputs"]["version"]["type"],
            "string",
        )

    def test_npm_stable_promotion_requires_exact_root_and_complete_ledger(self):
        npm = self.workflows["npm-release.yml"]
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

        steps = npm["jobs"]["publish"]["steps"]
        identity = next(
            step
            for step in steps
            if step.get("name") == "Validate exact root tag identity"
        )
        self.assertEqual(identity["id"], "identity")
        self.assertEqual(
            identity["env"]["REQUESTED_VERSION"], "${{ inputs.version }}"
        )
        identity_script = identity["run"]
        for required in (
            '"${GITHUB_EVENT_NAME}" != \'workflow_dispatch\'',
            '"${GITHUB_REF_TYPE}" != \'tag\'',
            '"${REQUESTED_VERSION}" != "${GITHUB_REF_NAME}"',
            "requested stable promotion version must equal the selected Root tag",
            'echo "version=${GITHUB_REF_NAME}"',
            'echo "commit=${GITHUB_SHA}"',
        ):
            self.assertIn(required, identity_script)
        self.assertNotIn("${{ inputs.", identity_script)

        source = next(
            step
            for step in steps
            if step.get("name") == "Verify merged-main release source"
        )["run"]
        self.assertIn("--source-mode promotion", source)

        ledger = next(
            step
            for step in steps
            if step.get("name")
            == "Require reviewed stable promotion and complete candidate ledger"
        )
        self.assertEqual(ledger["id"], "promotion")
        ledger_script = ledger["run"]
        for required in (
            'git show origin/main:.mss/release-policy.yaml',
            "--component npm",
            "--component root",
            "--intent promote",
            '--commit "${RELEASE_COMMIT}"',
            '"mss-boot/${RELEASE_VERSION}"',
            '"admin/${RELEASE_VERSION}"',
            '"web/antd-v6/${RELEASE_VERSION}"',
            '"${RELEASE_VERSION}"',
            '"docs/${RELEASE_VERSION}"',
            'test "$(resolve_tag_commit "${release_tag}")" = "${RELEASE_COMMIT}"',
            ".isDraft == false and .isPrerelease == false",
            'releases/latest" --jq .tag_name',
            '"${current_stable}"|"${RELEASE_VERSION}")',
            "https://docs.mss-boot-io.top/release.json",
            '.application == "mss-boot-docs"',
            ".version == $version and .commit == $commit",
        ):
            self.assertIn(required, ledger_script)

        go_modules = next(
            step
            for step in steps
            if step.get("name") == "Verify exact public Go modules"
        )["run"]
        self.assertIn("github.com/mss-boot-io/mss-boot-admin/mss-boot", go_modules)
        self.assertIn("github.com/mss-boot-io/mss-boot-admin/admin", go_modules)
        self.assertIn("GOPROXY=https://proxy.golang.org", go_modules)

        images = next(
            step
            for step in steps
            if step.get("name") == "Verify exact Root and Admin Web image aliases"
        )["run"]
        for required in (
            '"ghcr.io/${GITHUB_REPOSITORY}"',
            '"ghcr.io/${GITHUB_REPOSITORY_OWNER}/mss-boot-admin-antd-v6"',
            '"${image}:${RELEASE_VERSION}"',
            '"${image}:${RELEASE_COMMIT}"',
            'cmp -- "${version_manifest}" "${commit_manifest}"',
            '.platform.architecture == "amd64"',
            '.platform.architecture == "arm64"',
        ):
            self.assertIn(required, images)

        npm_content = (WORKFLOW_DIR / "npm-release.yml").read_text(encoding="utf-8")
        self.assertIn("Require the exact frontend release", npm_content)
        current_alias = next(
            step
            for step in steps
            if step.get("name") == "Require current stable npm alias before promotion"
        )["run"]
        self.assertIn('reviewed_latest="${CURRENT_STABLE_VERSION#v}"', current_alias)
        for required in (
            'case "${GITHUB_LATEST_VERSION}:${current_latest}" in',
            '"${CURRENT_STABLE_VERSION}:${reviewed_latest}"',
            '"${CURRENT_STABLE_VERSION}:${UNPREFIXED_VERSION}"',
            '"${RELEASE_VERSION}:${UNPREFIXED_VERSION}")',
            '"${RELEASE_VERSION}:${reviewed_latest}")',
            "GitHub Latest cannot advance before npmjs latest",
        ):
            self.assertIn(required, current_alias)
        self.assertIn("selected_tag=latest", current_alias)
        self.assertNotIn("sort -V", current_alias)
        self.assertNotIn("release-${UNPREFIXED_VERSION}", current_alias)

        npm_state = next(
            step
            for step in steps
            if step.get("name") == "Reconcile immutable npmjs publication state"
        )
        official_publish = next(
            step
            for step in steps
            if step.get("name") == "Publish the existing tarball to official npm"
        )
        self.assertIn("unset NPM_TOKEN NODE_AUTH_TOKEN NPM_CONFIG_USERCONFIG", official_publish["run"])
        self.assertIn("npm publish", official_publish["run"])
        self.assertIn('case "${NPM_DIST_TAG}"', official_publish["run"])
        self.assertIn("latest)", official_publish["run"])
        self.assertNotIn("release-*", official_publish["run"])
        self.assertIn("--provenance", official_publish["run"])
        self.assertEqual(
            official_publish["if"], "steps.npmjs.outputs.publish == 'true'"
        )
        self.assertNotIn("NODE_AUTH_TOKEN", official_publish.get("env", {}))
        self.assertNotIn("NPM_TOKEN", official_publish.get("env", {}))
        self.assertNotIn("secrets.NPM_TOKEN", npm_content)
        self.assertNotIn("secrets.NODE_AUTH_TOKEN", npm_content)
        self.assertNotRegex(npm_content, r"\bnpm dist-tag (?:add|rm)\b")
        self.assertNotIn("npm stage publish", npm_content)

        required_before_publish = (
            ledger,
            next(
                step
                for step in steps
                if step.get("name") == "Require the exact frontend release"
            ),
            next(
                step
                for step in steps
                if step.get("name") == "Verify exact public Go modules"
            ),
            next(
                step
                for step in steps
                if step.get("name")
                == "Verify exact Root and Admin Web image aliases"
            ),
            next(
                step
                for step in steps
                if step.get("name")
                == "Download and verify existing frontend package assets"
            ),
            next(
                step
                for step in steps
                if step.get("name")
                == "Verify GitHub Packages has the identical package"
            ),
            next(
                step
                for step in steps
                if step.get("name")
                == "Require current stable npm alias before promotion"
            ),
            npm_state,
        )
        for gate in required_before_publish:
            self.assertLess(steps.index(gate), steps.index(official_publish))

    def test_github_latest_moves_only_after_exact_npm_latest(self):
        npm = self.workflows["npm-release.yml"]
        publish_job = npm["jobs"]["publish"]
        github_job = npm["jobs"]["promote-github"]
        self.assertEqual(github_job["needs"], "publish")
        self.assertEqual(github_job["environment"], "release-auto")
        self.assertEqual(publish_job["permissions"]["contents"], "read")
        self.assertEqual(publish_job["permissions"]["id-token"], "write")
        self.assertEqual(github_job["permissions"]["contents"], "write")
        self.assertNotIn("id-token", github_job["permissions"])

        github_steps = github_job["steps"]
        npm_verification = next(
            step
            for step in github_steps
            if step.get("name") == "Validate exact completed npm promotion"
        )
        npm_script = npm_verification["run"]
        for required in (
            '"${GITHUB_EVENT_NAME}" != \'workflow_dispatch\'',
            '"${GITHUB_REF_TYPE}" != \'tag\'',
            'test "${GITHUB_REF_NAME}" = "${RELEASE_VERSION}"',
            'test "${GITHUB_SHA}" = "${RELEASE_COMMIT}"',
            "version gitHead dist.integrity dist.attestations --json",
            '."dist.attestations".provenance.predicateType',
            'npm view "${package}" dist-tags.latest --json',
            ')" = "${unprefixed_version}"',
        ):
            self.assertIn(required, npm_script)

        github_latest = next(
            step
            for step in github_steps
            if step.get("name") == "Promote exact Root release to GitHub Latest"
        )
        github_script = github_latest["run"]
        self.assertIn('.isDraft == false and .isPrerelease == false', github_script)
        self.assertIn(
            'gh release edit "${RELEASE_VERSION}" --latest=true', github_script
        )
        self.assertIn('releases/latest" --jq .tag_name', github_script)
        self.assertLess(
            github_steps.index(npm_verification), github_steps.index(github_latest)
        )

    def test_release_authority_requires_the_exact_operator_before_checkout_or_write(self):
        gated_jobs = {
            ("npm-release.yml", "publish"): None,
            ("npm-release.yml", "promote-github"): None,
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

        self.assertIn(
            "compatibility-foundation-next:\n"
            "\tbash tools/compatibility/test-standalone-mss-consumer.sh "
            "--upgrade --next-foundation",
            makefile,
        )


if __name__ == "__main__":
    unittest.main()
