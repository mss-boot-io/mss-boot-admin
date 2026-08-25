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
    "web-editor-usage.yml",
    "web-lockfile-finalize.yml",
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
        self.assertIn("test_release_phase_evidence.py", governance["run"])
        self.assertIn("test_release_qualification_decision.py", governance["run"])
        self.assertIn("test_release_readiness_attestation.py", governance["run"])
        self.assertIn("test_release_readiness_workflow.py", governance["run"])
        self.assertIn("test_root_release_workflow.py", governance["run"])
        self.assertIn("test_verify_framework_admin_checksum.py", governance["run"])
        self.assertIn("test_verify_release_source.py", governance["run"])
        self.assertIn("test_workflow_governance.py", governance["run"])

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
        self.assertEqual(release["environment"], "release-v6")
        for permission in ("attestations", "id-token"):
            self.assertEqual(release["permissions"][permission], "write")
        steps = release["steps"]
        identity = next(
            step
            for step in steps
            if step.get("name") == "Resolve immutable v6 release identity"
        )["run"]
        self.assertIn("npm_dist_tag=latest", identity)
        self.assertIn("npm_dist_tag=next", identity)
        self.assertIn('echo "npm_dist_tag=${npm_dist_tag}"', identity)
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
        package_qualification = next(
            step
            for step in steps
            if step.get("name")
            == "Inject and qualify the Admin Distribution package version"
        )["run"]
        self.assertIn("manifest.gitHead = process.env.GITHUB_SHA", package_qualification)
        image_build = next(
            step
            for step in steps
            if step.get("name") == "Build and push v6 Docker image"
        )
        package_description = json.loads(
            (REPOSITORY_ROOT / "web" / "antd-v6" / "package.json").read_text(
                encoding="utf-8"
            )
        )["description"]
        self.assertIn(
            f"org.opencontainers.image.description={package_description}",
            image_build["with"]["labels"],
        )
        image_identity = next(
            step
            for step in steps
            if step.get("name") == "Verify published v6 image identity"
        )["run"]
        self.assertIn("org.opencontainers.image.description", image_identity)
        self.assertIn(package_description, image_identity)
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
        preflight = next(
            step
            for step in steps
            if step.get("name")
            == "Preflight immutable Admin Web publication without mutation"
        )
        self.assertEqual(
            preflight["if"], "steps.npm-version.outputs.publish == 'true'"
        )
        self.assertEqual(
            preflight["env"]["NPM_DIST_TAG"],
            "${{ steps.version.outputs.npm_dist_tag }}",
        )
        self.assertEqual(preflight["env"]["NODE_AUTH_TOKEN"], "${{ github.token }}")
        self.assertIn("npm publish", preflight["run"])
        self.assertIn("--dry-run", preflight["run"])
        self.assertIn('--tag "${NPM_DIST_TAG}"', preflight["run"])
        self.assertIn("https://npm.pkg.github.com", preflight["run"])

        attestation = next(
            step
            for step in steps
            if step.get("name") == "Attest Admin Web package provenance"
        )
        self.assertRegex(
            attestation["uses"],
            r"^actions/attest-build-provenance@[0-9a-f]{40}$",
        )
        publish = next(
            step
            for step in steps
            if step.get("name") == "Publish immutable Admin Web package to GitHub Packages"
        )
        self.assertIn("npm publish", publish["run"])
        self.assertIn('--tag "${NPM_DIST_TAG}"', publish["run"])
        self.assertIn("https://npm.pkg.github.com", publish["run"])
        self.assertIn("latest|next", publish["run"])
        self.assertEqual(
            publish["if"], "steps.npm-version.outputs.publish == 'true'"
        )
        self.assertEqual(
            publish["env"]["NPM_DIST_TAG"],
            "${{ steps.version.outputs.npm_dist_tag }}",
        )
        self.assertEqual(publish["env"]["NODE_AUTH_TOKEN"], "${{ github.token }}")
        self.assertLess(steps.index(preflight), steps.index(publish))
        verify_package = next(
            step
            for step in steps
            if step.get("name")
            == "Verify GitHub Packages identity and repository binding"
        )
        self.assertIn('.visibility == "private"', verify_package["run"])
        self.assertIn('.visibility == "public"', verify_package["run"])
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

        image_state = next(
            step
            for step in steps
            if step.get("name") == "Inspect immutable v6 image publication state"
        )
        self.assertLess(steps.index(preflight), steps.index(image_state))
        self.assertIn("imagetools inspect", image_state["run"])
        self.assertIn("exists=true", image_state["run"])
        self.assertIn("authoritative not-found", image_state["run"])
        build = next(
            step
            for step in steps
            if step.get("name") == "Build and push v6 Docker image"
        )
        self.assertEqual(build["if"], "steps.image-state.outputs.exists != 'true'")
        image_verification = next(
            step
            for step in steps
            if step.get("name") == "Verify published v6 image identity"
        )["run"]
        for required in (
            ".manifests[]",
            ".platform.architecture == $architecture",
            'platform_image="${image_repository}@${platform_digest}"',
            'docker create --platform "linux/${architecture}" "${platform_image}"',
        ):
            self.assertIn(required, image_verification)
        self.assertNotIn(
            'docker create --platform "linux/${architecture}" "${image}"',
            image_verification,
        )
        metadata = next(
            step
            for step in steps
            if step.get("name") == "Extract v6 Docker metadata"
        )
        self.assertNotIn("latest", metadata["with"]["tags"])

        github_release = next(
            step
            for step in steps
            if step.get("name") == "Publish immutable v6 GitHub release"
        )
        self.assertIn("gh release download", github_release["run"])
        self.assertIn("cmp --", github_release["run"])
        self.assertIn(".isDraft'", github_release["run"])
        stable_alias = next(
            step
            for step in steps
            if step.get("name") == "Update the mutable stable v6 image alias last"
        )
        self.assertGreater(steps.index(stable_alias), steps.index(github_release))
        self.assertIn("imagetools create", stable_alias["run"])
        self.assertIn(":latest", stable_alias["run"])

    def test_admin_module_release_is_exact_tag_and_external_module_qualified(self):
        workflow = self.workflows["admin-release.yml"]
        self.assertEqual(workflow["on"]["push"]["tags"], ["admin/v*.*.*"])
        release = workflow["jobs"]["release"]
        self.assertEqual(release["environment"], "release")
        steps = release["steps"]
        source = next(
            step for step in steps if step.get("name") == "Verify merged-main release source"
        )["run"]
        self.assertIn("verify_release_source.py", source)
        self.assertIn('--tag "${GITHUB_REF_NAME}"', source)
        policy = next(
            step
            for step in steps
            if step.get("name") == "Enforce coordinated Admin Distribution target"
        )["run"]
        self.assertIn("--component admin", policy)
        framework_predecessor = next(
            step
            for step in steps
            if step.get("name")
            == "Require the already-published matching Framework release"
        )["run"]
        self.assertIn('mss-boot/${ADMIN_VERSION}', framework_predecessor)
        self.assertIn('${GITHUB_SHA}', framework_predecessor)
        self.assertIn(".isDraft == false", framework_predecessor)
        self.assertIn(".isPrerelease == $prerelease", framework_predecessor)
        framework = next(
            step
            for step in steps
            if step.get("name") == "Require the matching Framework dependency"
        )["run"]
        self.assertIn('"${framework_version}" != "${ADMIN_VERSION}"', framework)
        parity = next(
            step
            for step in steps
            if step.get("name") == "Verify final Framework and Admin checksum parity"
        )
        self.assertEqual(parity["id"], "candidate-checksums")
        self.assertIn("verify_framework_admin_checksum.py", parity["run"])
        self.assertIn('--version "${ADMIN_VERSION}"', parity["run"])
        for required in (
            ".sum",
            ".goModSum",
            ".adminSum",
            ".adminGoModSum",
            'echo "framework_sum=${framework_sum}"',
            'echo "framework_go_mod_sum=${framework_go_mod_sum}"',
            'echo "admin_sum=${admin_sum}"',
            'echo "admin_go_mod_sum=${admin_go_mod_sum}"',
            "Framework Module Sum:",
            "Framework GoModSum:",
            "Admin Module Sum:",
            "Admin GoModSum:",
        ):
            self.assertIn(required, parity["run"])
        self.assertLess(steps.index(parity), steps.index(next(
            step for step in steps if step.get("name") == "Qualify the independent Admin module"
        )))
        probe_step = next(
            step
            for step in steps
            if step.get("name")
            == "Probe the tagged Admin module from a clean external workspace"
        )
        probe = probe_step["run"]
        self.assertEqual(
            probe_step["env"]["ADMIN_SUM"],
            "${{ steps.candidate-checksums.outputs.admin_sum }}",
        )
        self.assertEqual(
            probe_step["env"]["ADMIN_GO_MOD_SUM"],
            "${{ steps.candidate-checksums.outputs.admin_go_mod_sum }}",
        )
        self.assertIn("GOPROXY=direct", probe)
        self.assertIn("GOWORK=off", probe)
        self.assertIn(".Origin.Hash == $commit", probe)
        self.assertIn('--arg sum "${ADMIN_SUM}"', probe)
        self.assertIn('--arg go_mod_sum "${ADMIN_GO_MOD_SUM}"', probe)
        self.assertIn(".Sum == $sum", probe)
        self.assertIn(".GoModSum == $go_mod_sum", probe)
        self.assertNotIn('startswith("h1:")', probe)
        self.assertIn("application.ExecuteArgsContext(context.Background(), args)", probe)
        self.assertIn('{"--help"}', probe)
        self.assertIn('{"server", "--help"}', probe)
        self.assertIn('{"migrate", "--help"}', probe)
        self.assertIn("go build -trimpath -o mss-admin-release-probe .", probe)
        self.assertIn("./mss-admin-release-probe >/dev/null", probe)
        self.assertNotIn(".Command()", probe)
        self.assertNotIn("admin/internal/cmd", probe)

        marker = 'cat > "${probe_dir}/main.go" <<\'EOF\'\n'
        self.assertIn(marker, probe)
        probe_source = probe.split(marker, 1)[1].split("\nEOF\n", 1)[0]
        self.assertNotIn(".Command()", probe_source)
        self.assertIn("ExecuteArgsContext", probe_source)

        framework_module = "github.com/mss-boot-io/mss-boot-admin/mss-boot"
        fixture_version = None
        for line in (REPOSITORY_ROOT / "admin" / "go.mod").read_text(
            encoding="utf-8"
        ).splitlines():
            fields = line.split()
            if len(fields) >= 2 and fields[0] == framework_module:
                fixture_version = fields[1]
                break
        self.assertIsNotNone(fixture_version)

        module = "github.com/mss-boot-io/mss-boot-admin/admin"
        with tempfile.TemporaryDirectory(
            prefix="mss-admin-release-probe-governance-"
        ) as temporary_directory:
            probe_dir = Path(temporary_directory)
            proxy_dir = probe_dir / "proxy"
            write_file_module_proxy(
                proxy_dir,
                framework_module,
                fixture_version,
                REPOSITORY_ROOT / "mss-boot",
            )
            write_file_module_proxy(
                proxy_dir,
                module,
                fixture_version,
                REPOSITORY_ROOT / "admin",
            )
            admin_archive = proxy_dir.joinpath(
                *module.split("/"), "@v", f"{fixture_version}.zip"
            )
            with zipfile.ZipFile(admin_archive) as archive:
                self.assertEqual(
                    archive.read(f"{module}@{fixture_version}/LICENSE"),
                    (REPOSITORY_ROOT / "LICENSE").read_bytes(),
                )
            go_mod = (
                "module example.com/mss-admin-release-probe-governance\n\n"
                "go 1.26.6\n\n"
                f"require {module} {fixture_version}\n"
            )
            self.assertNotIn("replace ", go_mod)
            (probe_dir / "go.mod").write_text(go_mod, encoding="utf-8")
            (probe_dir / "main.go").write_text(
                probe_source + "\n", encoding="utf-8"
            )
            environment = os.environ.copy()
            # The release workflow itself remains fail-closed on GOPROXY=direct
            # and verifies the tag origin hash above. Before that tag exists,
            # this executable fixture exposes the exact candidate source through
            # the Go proxy protocol. It uses no replace directive and exercises
            # the Admin dependency graph; dependency module go.sum files are not
            # consumed by Go, so verify_framework_admin_checksum.py separately
            # compares Admin's committed sums with this final candidate tree.
            environment.update(
                {
                    "GOWORK": "off",
                    "GOPROXY": f"{proxy_dir.as_uri()},https://proxy.golang.org,direct",
                    "GONOPROXY": "none",
                    "GONOSUMDB": "github.com/mss-boot-io/mss-boot-admin/*",
                    "GOTOOLCHAIN": "local",
                }
            )
            commands = (
                ("go", "mod", "tidy"),
                ("go", "test", "./..."),
                ("go", "vet", "./..."),
                ("go", "build", "-trimpath", "-o", "release-probe", "."),
                (str(probe_dir / "release-probe"),),
            )
            for command in commands:
                result = subprocess.run(
                    command,
                    cwd=probe_dir,
                    env=environment,
                    capture_output=True,
                    text=True,
                    timeout=300,
                    check=False,
                )
                self.assertEqual(
                    result.returncode,
                    0,
                    msg=(
                        f"release probe command failed: {' '.join(command)}\n"
                        f"stdout:\n{result.stdout}\n"
                        f"stderr:\n{result.stderr}"
                    ),
                )

    def test_framework_release_rechecks_final_admin_checksum_before_module_qualification(self):
        steps = self.workflows["framework-release.yml"]["jobs"]["release"]["steps"]
        parity = next(
            step
            for step in steps
            if step.get("name") == "Verify final Framework and Admin checksum parity"
        )
        qualification = next(
            step for step in steps if step.get("name") == "Test framework module"
        )
        probe_step = next(
            step
            for step in steps
            if step.get("name")
            == "Probe the published module from a clean external workspace"
        )
        self.assertEqual(parity["id"], "candidate-checksums")
        self.assertIn("verify_framework_admin_checksum.py", parity["run"])
        self.assertIn('--version "${FRAMEWORK_VERSION}"', parity["run"])
        for required in (
            ".sum",
            ".goModSum",
            ".adminSum",
            ".adminGoModSum",
            'echo "framework_sum=${framework_sum}"',
            'echo "framework_go_mod_sum=${framework_go_mod_sum}"',
            'echo "admin_sum=${admin_sum}"',
            'echo "admin_go_mod_sum=${admin_go_mod_sum}"',
            "Framework Module Sum:",
            "Framework GoModSum:",
            "Admin Module Sum:",
            "Admin GoModSum:",
        ):
            self.assertIn(required, parity["run"])
        self.assertLess(steps.index(parity), steps.index(qualification))
        self.assertEqual(
            probe_step["env"]["FRAMEWORK_SUM"],
            "${{ steps.candidate-checksums.outputs.framework_sum }}",
        )
        self.assertEqual(
            probe_step["env"]["FRAMEWORK_GO_MOD_SUM"],
            "${{ steps.candidate-checksums.outputs.framework_go_mod_sum }}",
        )
        probe = probe_step["run"]
        self.assertIn(".Origin.Hash == $commit", probe)
        self.assertIn('--arg sum "${FRAMEWORK_SUM}"', probe)
        self.assertIn('--arg go_mod_sum "${FRAMEWORK_GO_MOD_SUM}"', probe)
        self.assertIn(".Sum == $sum", probe)
        self.assertIn(".GoModSum == $go_mod_sum", probe)
        self.assertNotIn('startswith("h1:")', probe)

    def test_root_publication_requires_the_complete_distribution_train(self):
        workflow = self.workflows["release.yml"]
        evidence_steps = workflow["jobs"]["release-evidence"]["steps"]
        evidence = next(
            step
            for step in evidence_steps
            if step.get("name") == "Require successful exact-commit release evidence"
        )["run"]
        for required in (
            'resolve_tag_commit "mss-boot/${RELEASE_VERSION}"',
            'resolve_tag_commit "admin/${RELEASE_VERSION}"',
            'resolve_tag_commit "web/antd-v6/${RELEASE_VERSION}"',
            '"mss-boot/${RELEASE_VERSION}"',
            '"admin/${RELEASE_VERSION}"',
            '"web/antd-v6/${RELEASE_VERSION}"',
            "framework-release.yml",
            "admin-release.yml",
            "frontend-v6-release.yml",
            "github.com/mss-boot-io/mss-boot-admin/admin",
            "@mss-boot-io/admin-web@${RELEASE_VERSION#v}",
        ):
            self.assertIn(required, evidence)
        self.assertIn("gitHead", evidence)
        self.assertIn("dist.integrity", evidence)
        self.assertIn("https://npm.pkg.github.com", evidence)
        self.assertIn('.visibility == "public"', evidence)
        self.assertEqual(
            workflow["jobs"]["release-evidence"]["permissions"]["packages"],
            "read",
        )
        for required in (
            '.event == "push"',
            ".head_branch == $component_ref",
            ".head_sha == $commit",
            ".path == $workflow_path",
            "mss-boot/${RELEASE_VERSION}",
            "admin/${RELEASE_VERSION}",
            "web/antd-v6/${RELEASE_VERSION}",
        ):
            self.assertIn(required, evidence)
        workflow_list = evidence.split("for workflow in", 1)[1].split("; do", 1)[0]
        self.assertNotIn("docs.yml", workflow_list)

    def test_docs_publication_requires_the_stable_matching_root_release(self):
        workflow = self.workflows["docs.yml"]
        self.assertEqual(workflow["permissions"]["actions"], "read")
        steps = workflow["jobs"]["build"]["steps"]
        predecessor = next(
            step
            for step in steps
            if step.get("name")
            == "Require the already-published matching root release"
        )
        script = predecessor["run"]
        for required in (
            'git rev-parse "refs/tags/${root_tag}^{commit}"',
            'gh release view "${root_tag}"',
            ".isDraft == false",
            ".isPrerelease == false",
            'actions/workflows/release.yml/runs',
            '.event == "workflow_dispatch"',
            ".head_branch == $root_tag",
            ".head_sha == $commit",
            '.path == ".github/workflows/release.yml"',
            '--arg display_title "Root Release publish ${root_tag}"',
            ".display_title == $display_title",
            '.conclusion == "success"',
        ):
            self.assertIn(required, script)
        self.assertNotIn('Root Release candidate ${root_tag}', script)
        self.assertLess(
            steps.index(predecessor),
            next(
                index
                for index, step in enumerate(steps)
                if step.get("name") == "Build documentation"
            ),
        )

    def test_root_release_keeps_the_foundation_checkout_immutable_for_evals(self):
        makefile = (REPOSITORY_ROOT / "Makefile").read_text(encoding="utf-8")
        self.assertIn(
            "deps-admin:\n\tcd $(ADMIN_DIR) && GOWORK=off go mod download",
            makefile,
        )

        steps = self.workflows["release.yml"]["jobs"]["test"]["steps"]
        agent_contracts = next(
            step
            for step in steps
            if step.get("name") == "Verify Agent module and contracts"
        )["run"]
        eval_index = agent_contracts.index("go run ./cmd/mss eval run --all")
        self.assertLess(agent_contracts.index("git diff --exit-code"), eval_index)
        self.assertLess(agent_contracts.index("git diff --cached --exit-code"), eval_index)


if __name__ == "__main__":
    unittest.main()
