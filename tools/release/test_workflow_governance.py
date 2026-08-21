import json
import unittest
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
            '"@mss-boot-io/admin-web@file:../.mss/qualification/admin-web.tgz"',
            "entry.get('specifier') != expected",
            "resolved.startswith(expected + '(')",
            "fetch --frozen-lockfile",
            "install --offline --frozen-lockfile",
            'mss_start_process_group \\\n  backend_pid',
            'mss_start_process_group \\\n  web_pid',
            'mss_stop_process_group "${web_pid}"',
            'mss_stop_process_group "${backend_pid}"',
        ):
            with self.subTest(required=required):
                self.assertIn(required, thin_host_script)
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
        probe = next(
            step
            for step in steps
            if step.get("name")
            == "Probe the tagged Admin module from a clean external workspace"
        )["run"]
        self.assertIn("GOPROXY=direct", probe)
        self.assertIn("GOWORK=off", probe)
        self.assertIn(".Origin.Hash == $commit", probe)
        self.assertIn("command, err := application.Command()", probe)
        self.assertIn("fmt.Println(command.Name())", probe)
        self.assertNotIn("application.Command().Name()", probe)

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
