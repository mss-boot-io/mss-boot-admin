import re
import subprocess
import unittest
from pathlib import Path

import yaml


REPOSITORY_ROOT = Path(__file__).resolve().parents[2]
WORKFLOW_PATH = REPOSITORY_ROOT / ".github" / "workflows" / "release.yml"
CONTAINER_WORKFLOW_PATH = (
    REPOSITORY_ROOT / ".github" / "workflows" / "container.yml"
)
GITHUB_EXPRESSION = re.compile(r"\$\{\{.*?\}\}", re.DOTALL)


class RootReleaseWorkflowTest(unittest.TestCase):
    def setUp(self):
        self.content = WORKFLOW_PATH.read_text(encoding="utf-8")
        self.workflow = yaml.load(self.content, Loader=yaml.BaseLoader)
        self.jobs = self.workflow["jobs"]

    def step(self, job_name, step_name):
        return next(
            step
            for step in self.jobs[job_name]["steps"]
            if step.get("name") == step_name
        )

    @staticmethod
    def bash_array(script, name):
        match = re.search(
            rf"^\s*{re.escape(name)}=\(\n(?P<body>.*?)^\s*\)",
            script,
            flags=re.MULTILINE | re.DOTALL,
        )
        if match is None:
            raise AssertionError(f"missing bash array {name}")
        return tuple(
            line.strip().strip('"').strip("'")
            for line in match.group("body").splitlines()
            if line.strip()
        )

    def test_agent_tool_identity_uses_the_exact_commit_timestamp(self):
        build = self.step("backend-build", "Build Admin runtime and Agent tools")[
            "run"
        ]
        for required in (
            'git show -s --format=%cI "${RELEASE_COMMIT}"',
            "internal/mss/buildinfo.Timestamp=${release_timestamp}",
            'echo "timestamp=${release_timestamp}"',
            "cp AGENTS.md CLAUDE.md LICENSE README.md README.zh-CN.md",
            "for command in mss mss-mcp; do",
            '[[ "${output}" == *"${release_timestamp}"* ]]',
        ):
            self.assertIn(required, build)

    def test_release_checkout_includes_installers_and_validation_tools(self):
        checkout = self.step("assemble", "Checkout release validation tooling")
        sparse_paths = set(checkout["with"]["sparse-checkout"].splitlines())
        self.assertEqual(sparse_paths, {"tools/install", "tools/release"})

    def test_every_tag_candidate_verifies_exact_merged_main_source(self):
        checkout = self.step("release-evidence", "Checkout release source")
        verify = self.step(
            "release-evidence", "Verify merged-main release source"
        )
        self.assertNotIn("if", checkout)
        self.assertNotIn("if", verify)
        self.assertEqual(
            self.jobs["release-evidence"]["if"],
            "needs.metadata.outputs.publish == 'true'",
        )
        script = verify["run"]
        for required in (
            "tools/release/verify_release_source.py",
            '--commit "${RELEASE_COMMIT}"',
            '--tag "${RELEASE_VERSION}"',
            "--policy .mss/release-policy.yaml",
        ):
            self.assertIn(required, script)

    def test_dispatch_is_preview_only_and_root_tag_push_publishes_automatically(self):
        dispatch_inputs = self.workflow["on"]["workflow_dispatch"]["inputs"]
        self.assertEqual(set(dispatch_inputs), {"version"})
        self.assertEqual(self.workflow["on"]["push"]["tags"], ["v*.*.*"])
        self.assertIn("Root Release candidate {0}", self.workflow["run-name"])
        self.assertNotIn("Root Release publish", self.workflow["run-name"])

        metadata = self.step("metadata", "Resolve and validate release metadata")[
            "run"
        ]
        for required in (
            'publish=true',
            'if [[ "${GITHUB_EVENT_NAME}" == "workflow_dispatch" ]]',
            'publish=false',
            '"${GITHUB_EVENT_NAME}" == "push"',
            '"${GITHUB_REF_TYPE}" == "tag"',
            "--intent \"${policy_intent}\"",
        ):
            self.assertIn(required, metadata)
        for forbidden in (
            "INPUT_PUBLISH",
            "manual_evidence",
            "evidence_url",
            "readiness_run_id",
            "root_tag_promotion_run_id",
            "verify_readiness_run.sh",
            "root-tag-promotion.yml",
        ):
            self.assertNotIn(forbidden, self.content)

        self.assertEqual(
            self.workflow["concurrency"]["group"],
            "${{ github.event_name == 'push' && 'root-release-publication' || "
            "format('root-release-preview-{0}', github.ref) }}",
        )
        self.assertEqual(
            self.workflow["concurrency"]["cancel-in-progress"], "false"
        )

        preview = self.step(
            "release-evidence", "Require successful exact preview"
        )
        self.assertEqual(preview["id"], "preview")
        self.assertNotIn("if", preview)
        self.assertEqual(
            self.jobs["release-evidence"]["outputs"]["preview-run-id"],
            "${{ steps.preview.outputs.run-id }}",
        )
        script = preview["run"]
        for required in (
            "tools/release/resolve_successful_preview.sh",
            '--repository "${GITHUB_REPOSITORY}"',
            '--commit "${RELEASE_COMMIT}"',
            '--version "${RELEASE_VERSION}"',
            '--actor "${RELEASE_ACTOR_LOGIN}"',
            'echo "run-id=${preview_run_id}"',
        ):
            self.assertIn(required, script)
        self.assertNotIn("gh release view", script)
        self.assertNotIn("refusing to mutate", script)
        for forbidden in ("gh run watch", "gh workflow run", "sleep "):
            self.assertNotIn(forbidden, script)

    def test_container_preview_allows_only_the_nested_permission_ceiling(self):
        container_workflow = yaml.load(
            CONTAINER_WORKFLOW_PATH.read_text(encoding="utf-8"),
            Loader=yaml.BaseLoader,
        )
        preview = self.jobs["container-preview"]
        self.assertEqual(
            preview["permissions"],
            container_workflow["jobs"]["publish"]["permissions"],
        )
        self.assertEqual(
            preview["if"], "needs.metadata.outputs.publish != 'true'"
        )
        self.assertEqual(
            set(preview["with"]), {"version", "release_preview"}
        )
        self.assertEqual(preview["with"]["release_preview"], "true")

    def test_root_requires_only_exact_public_component_releases(self):
        evidence = self.step(
            "release-evidence", "Require exact component releases"
        )["run"]
        for required in (
            'component_commit="$(resolve_tag_commit',
            '"${component_commit}" != "${RELEASE_COMMIT}"',
            'gh release view "${component_tag}"',
            '.tagName == $tag and .isDraft == false and .isPrerelease == $prerelease',
            "mss-boot/${RELEASE_VERSION}",
            "admin/${RELEASE_VERSION}",
            "web/antd-v6/${RELEASE_VERSION}",
        ):
            self.assertIn(required, evidence)
        for forbidden in (
            "for attempt in $(seq",
            "sleep ",
            "component_train_ready",
            "container.yml",
            "npm-release.yml",
            "gh run watch",
            "gh workflow run",
            "/actions/workflows/",
            ".workflow_runs",
            "framework-release.yml",
            "admin-release.yml",
            "frontend-v6-release.yml",
        ):
            self.assertNotIn(forbidden, evidence)

    def test_assembly_produces_and_qualifies_all_tool_archives(self):
        assemble = self.step("assemble", "Assemble release packages")["run"]
        expected_tools = (
            "mss-tools-${RELEASE_VERSION}-linux-amd64.tar.gz",
            "mss-tools-${RELEASE_VERSION}-linux-arm64.tar.gz",
            "mss-tools-${RELEASE_VERSION}-darwin-amd64.tar.gz",
            "mss-tools-${RELEASE_VERSION}-darwin-arm64.tar.gz",
            "mss-tools-${RELEASE_VERSION}-windows-amd64.zip",
            "mss-tools-${RELEASE_VERSION}-windows-arm64.zip",
        )
        self.assertEqual(self.bash_array(assemble, "tool_assets"), expected_tools)

        for required in (
            'release_timestamp="$(git show -s --format=%cI "${RELEASE_COMMIT}")"',
            'source_date_epoch="$(git show -s --format=%ct "${RELEASE_COMMIT}")"',
            'cmp -- "${reference_build_info}" "${dir}/BUILD-INFO"',
            'cmp -- "${reference_license}" "${dir}/LICENSE"',
            "--sort=name",
            "--owner=0",
            "--group=0",
            "--numeric-owner",
            '--mtime="@${source_date_epoch}"',
            "gzip -n",
            "TZ=UTC zip -X -q",
            "expected_unix_entries=$'BUILD-INFO\\nLICENSE\\nmss\\nmss-mcp'",
            "expected_windows_entries=$'BUILD-INFO\\nLICENSE\\nmss-mcp.exe\\nmss.exe'",
            "python3 tools/release/check_portable_paths.py",
            'install -m 0755 tools/install/install-mss.sh install-mss.sh',
            'install -m 0644 tools/install/install-mss.ps1 install-mss.ps1',
            'grep -Fx "readonly DEFAULT_VERSION=\\"${RELEASE_VERSION}\\"" install-mss.sh',
            'grep -Fx "    [string]\\$Version = \'${RELEASE_VERSION}\'," install-mss.ps1',
            "bash -n install-mss.sh",
            'tar -xzf "mss-tools-${RELEASE_VERSION}-linux-amd64.tar.gz"',
            'output="$("${smoke_dir}/tools/${command}" --version)"',
            '[[ "${output}" == *"${RELEASE_VERSION}"* ]]',
            '[[ "${output}" == *"${RELEASE_COMMIT}"* ]]',
            '[[ "${output}" == *"${release_timestamp}"* ]]',
            '"${smoke_dir}/tools/mss" new app release-smoke',
            '--destination "${smoke_dir}/release-smoke"',
            "'.success == true and .dryRun == false'",
            'test -f "${smoke_dir}/release-smoke/go.sum"',
            'test -f "${smoke_dir}/release-smoke/web/pnpm-lock.yaml"',
            'admin_web_package_dir="frontend-v6-dist/admin-web-release-package"',
            'sha256sum --check SHA256SUMS.admin-web',
            'admin_web_package="${admin_web_packages[0]}"',
            'registry_pid=""',
            'kill "${registry_pid}"',
            'wait "${registry_pid}"',
            'sha512(tarball.read_bytes()).digest()',
            'ThreadingHTTPServer(("127.0.0.1", 0), Handler)',
            '--contributor-npm-registry "${registry_url}"',
            'sha256sum "${tool_assets[@]}" install-mss.sh install-mss.ps1',
            '[[ "$(wc -l < "SHA256SUMS.tools-${RELEASE_VERSION}")" -eq 8 ]]',
            'sha256sum --check "SHA256SUMS.tools-${RELEASE_VERSION}"',
        ):
            self.assertIn(required, assemble)

        upload_step = self.step("assemble", "Upload assembled packages")
        upload = upload_step["with"]["path"]
        self.assertEqual(upload_step["with"]["retention-days"], "30")
        for expected in (
            "mss-boot-admin-*.zip",
            "SHA256SUMS",
            "mss-tools-*.tar.gz",
            "mss-tools-*.zip",
            "SHA256SUMS.tools-*",
            "install-mss.sh",
            "install-mss.ps1",
        ):
            self.assertIn(expected, upload.splitlines())

    def test_preview_packages_exact_admin_web_bytes_before_public_npm(self):
        pack = self.step(
            "frontend-build", "Pack exact Admin Web package and release evidence"
        )["run"]
        for required in (
            '"${RELEASE_COMMIT}:web/antd-v6"',
            'manifest["version"] = sys.argv[2]',
            'manifest["gitHead"] = sys.argv[3]',
            'pnpm pack --pack-destination "${package_dir}"',
            "verify_admin_web_package.py",
            "generate_admin_web_sbom.py",
            "SHA256SUMS.admin-web",
        ):
            self.assertIn(required, pack)
        self.assertNotIn("admin-web-candidate.tgz", pack)

        uploaded = self.step(
            "frontend-build", "Upload primary V6 artifact"
        )["with"]["path"].splitlines()
        self.assertNotIn("web/antd-v6/admin-web-candidate.tgz", uploaded)
        self.assertIn("web/antd-v6/admin-web-release-package/*.tgz", uploaded)
        self.assertIn(
            "web/antd-v6/admin-web-release-package/admin-web-package.json", uploaded
        )
        self.assertIn(
            "web/antd-v6/admin-web-release-package/admin-web.spdx.json", uploaded
        )
        self.assertIn(
            "web/antd-v6/admin-web-release-package/SHA256SUMS.admin-web", uploaded
        )

        assemble = self.step("assemble", "Assemble release packages")["run"]
        smoke = assemble.split('smoke_dir="$(mktemp -d)"', 1)[1]
        self.assertIn('package_name = "@mss-boot-io/admin-web"', smoke)
        self.assertIn('"integrity": integrity', smoke)
        self.assertIn('--contributor-npm-registry "${registry_url}"', smoke)
        self.assertNotIn("registry.npmjs.org", smoke)

    def test_preview_exports_one_verified_multiarch_oci_image_without_push(self):
        image = self.step(
            "frontend-build", "Qualify the exact V6 multi-platform release image"
        )
        self.assertEqual(image["id"], "image")
        self.assertRegex(
            image["uses"], r"^docker/build-push-action@[0-9a-f]{40}$"
        )
        self.assertEqual(image["with"]["push"], "false")
        self.assertEqual(
            image["with"]["outputs"],
            "type=oci,dest=${{ runner.temp }}/frontend-v6-image.oci.tar",
        )
        self.assertEqual(image["with"]["platforms"], "linux/amd64,linux/arm64")
        self.assertEqual(image["with"]["provenance"], "mode=max")
        self.assertEqual(image["with"]["sbom"], "true")

        verify = self.step(
            "frontend-build", "Verify and stage the exact V6 OCI image"
        )["run"]
        for required in (
            'archive_digest="$(' ,
            'test "${archive_digest}" = "${BUILD_DIGEST}"',
            '"blobs/sha256/${manifest_hash}"',
            '.platform.architecture == "amd64"',
            '.platform.architecture == "arm64"',
            'inspect --config "oci-archive:${source_archive}"',
            "FRONTEND-V6-IMAGE-INFO",
            "SHA256SUMS.frontend-v6-image",
        ):
            self.assertIn(required, verify)

        upload = self.step(
            "frontend-build", "Upload primary V6 artifact"
        )["with"]
        paths = upload["path"].splitlines()
        for expected in (
            "web/antd-v6/frontend-v6-image.oci.tar",
            "web/antd-v6/FRONTEND-V6-IMAGE-INFO",
            "web/antd-v6/SHA256SUMS.frontend-v6-image",
        ):
            self.assertIn(expected, paths)
        self.assertEqual(upload["compression-level"], "0")
        self.assertEqual(upload["retention-days"], "30")

    def test_public_release_asset_set_is_exact_and_has_no_retired_tools(self):
        publish = self.step(
            "publish", "Stage, verify, and publish GitHub release atomically"
        )["run"]
        expected_assets = (
            "mss-boot-admin-linux-amd64.zip",
            "mss-boot-admin-linux-arm64.zip",
            "mss-boot-admin-darwin-amd64.zip",
            "mss-boot-admin-darwin-arm64.zip",
            "mss-boot-admin-windows-amd64.zip",
            "mss-boot-admin-windows-arm64.zip",
            "SHA256SUMS",
            "mss-tools-${RELEASE_VERSION}-linux-amd64.tar.gz",
            "mss-tools-${RELEASE_VERSION}-linux-arm64.tar.gz",
            "mss-tools-${RELEASE_VERSION}-darwin-amd64.tar.gz",
            "mss-tools-${RELEASE_VERSION}-darwin-arm64.tar.gz",
            "mss-tools-${RELEASE_VERSION}-windows-amd64.zip",
            "mss-tools-${RELEASE_VERSION}-windows-arm64.zip",
            "SHA256SUMS.tools-${RELEASE_VERSION}",
            "install-mss.sh",
            "install-mss.ps1",
        )
        assets = self.bash_array(publish, "assets")
        self.assertEqual(assets, expected_assets)
        self.assertNotIn("admin", assets)
        self.assertFalse(any("mss-pr" in asset for asset in assets))
        self.assertNotIn("mss-pr", self.content)
        self.assertIn('sha256sum --check "SHA256SUMS.tools-${RELEASE_VERSION}"', publish)
        for required in (
            "verify_release_assets()",
            '.isDraft == false and .isPrerelease == $prerelease',
            "is already public with the exact preview assets",
            "exit 0",
            'current_latest="$(gh api',
            'sort -V | tail -n 1',
            'release_latest=false',
            '--latest="${release_latest}"',
        ):
            self.assertIn(required, publish)
        self.assertNotIn("is already public; refusing to mutate it", publish)

    def test_release_notes_are_package_first(self):
        notes = self.step("publish", "Prepare deterministic root release notes")["run"]
        for required in (
            "## Package-first runtime and tools",
            "install-mss.sh",
            "install-mss.ps1",
            "SHA256SUMS.tools-*",
            "versioned Foundation Blueprint embedded in the installed tool",
            "without a Foundation source checkout",
            "## Migration",
            "mss_boot_presentation_profiles",
            "mss_boot_presentation_revisions",
            "admin migrate",
            "admin server -a",
            "v1.3.3 through v1.3.6",
            "20260824121000",
            "20260830193000",
            "intentionally fails closed",
            "leaves its version pending",
            "restorable pre-upgrade backup",
            "docs/adr/2026-08-24-admin-presentation-publication-workflow.md",
            "mss_boot_menus",
            "mss_boot_casbin_rule",
            "Do not insert the attestation merely because current values look canonical",
            "only after the documented recovery review",
            "--password",
            "-p",
            "## Compatibility",
            "fourteen Foundation built-in management page capabilities",
            "exact startup allowlist",
            "## Security",
            "data-only",
            "Backend RBAC remains authoritative",
            "## Rollback",
            "presentation.recoveryMode",
            "one previously qualified coordinated Admin Distribution",
            "matching Framework, Admin, frontend, configuration, and lock",
            "Keep the live forward-compatible database schema and data",
            "full database-backup restore is disaster recovery only",
            "discards every write after the backup time",
            "post-backup audit and business data outside the target database",
            "## Known limits",
            "six accepted non-runtime advisories",
            "2026-11-08",
            "not a zero-vulnerability claim",
        ):
            self.assertIn(required, notes)
        for forbidden in (
            "must run from a checked-out Foundation source tree",
            "must clone",
            "mss-pr",
            "zero vulnerabilities",
            "current stable distribution",
            "production capability registration remains empty",
            "matching v1.3.2",
        ):
            self.assertNotIn(forbidden, notes)

    def test_preview_builds_exact_artifacts_without_repeating_local_or_pr_quality_gates(self):
        for job_name in (
            "container-preview",
            "backend-build",
            "frontend-build",
            "assemble",
        ):
            with self.subTest(job=job_name):
                self.assertEqual(
                    self.jobs[job_name]["if"],
                    "needs.metadata.outputs.publish != 'true'",
                )
                self.assertNotIn("environment", self.jobs[job_name])

        self.assertNotIn("foundation-compatibility", self.jobs)
        self.assertNotIn("test", self.jobs)
        self.assertEqual(self.jobs["backend-build"]["needs"], "metadata")
        self.assertEqual(
            self.jobs["assemble"]["needs"],
            ["metadata", "container-preview", "backend-build", "frontend-build"],
        )

        container_preview = self.jobs["container-preview"]
        self.assertEqual(
            container_preview["uses"], "./.github/workflows/container.yml"
        )
        self.assertEqual(
            container_preview["with"]["version"],
            "${{ needs.metadata.outputs.version }}",
        )
        self.assertEqual(
            container_preview["with"]["release_preview"], "true"
        )
        self.assertIn("container-preview", self.jobs["assemble"]["needs"])
        assemble = self.step("assemble", "Assemble release packages")["run"]
        for required in (
            'root_image_dir="root-image-preview-${RELEASE_VERSION}"',
            'sha256sum --check SHA256SUMS.root-image',
            'tar -xOf "${root_image_archive}" index.json',
            'Root OCI layout must expose exactly one top-level image',
            '.platform.architecture == "amd64"',
            '.platform.architecture == "arm64"',
            'grep -Fx "version=${RELEASE_VERSION}"',
            'grep -Fx "commit=${RELEASE_COMMIT}"',
            'grep -Fx "digest=${root_image_digest}"',
        ):
            self.assertIn(required, assemble)

        frontend_step_names = {
            step["name"] for step in self.jobs["frontend-build"]["steps"]
        }
        self.assertTrue(
            {
                "Build same-origin primary V6 frontend",
                "Smoke-test primary V6 delivery",
                "Package and verify portable primary V6 artifact",
                "Pack exact Admin Web package and release evidence",
                "Qualify the exact V6 multi-platform release image",
                "Verify and stage the exact V6 OCI image",
                "Upload primary V6 artifact",
            }.issubset(frontend_step_names)
        )
        for removed_step in (
            "Enforce V6 dependency policy",
            "Qualify V6 lint and unit behavior",
            "Create ephemeral browser administrator secret",
            "Qualify browser permission and parity behavior",
        ):
            self.assertNotIn(removed_step, frontend_step_names)
        frontend_scripts = "\n".join(
            step.get("run", "") for step in self.jobs["frontend-build"]["steps"]
        )
        for repeated_quality_gate in (
            "pnpm run deps:check",
            "pnpm run audit:release",
            "pnpm run lint",
            "pnpm run test:ci",
            "pnpm run test:e2e",
            "playwright install",
        ):
            self.assertNotIn(repeated_quality_gate, frontend_scripts)

        publish = self.jobs["publish"]
        self.assertEqual(
            publish["if"], "needs.metadata.outputs.publish == 'true'"
        )
        self.assertEqual(publish["needs"], ["metadata", "release-evidence"])
        self.assertEqual(publish["environment"], "release-auto")
        self.assertEqual(publish["permissions"]["contents"], "write")
        self.assertEqual(
            [step["name"] for step in publish["steps"]],
            [
                "Require authorized release operator",
                "Download exact preview packages",
                "Prepare deterministic root release notes",
                "Stage, verify, and publish GitHub release atomically",
            ],
        )
        download = self.step("publish", "Download exact preview packages")
        self.assertEqual(
            download["with"]["run-id"],
            "${{ needs.release-evidence.outputs.preview-run-id }}",
        )
        self.assertEqual(
            download["with"]["name"],
            "release-packages-${{ needs.metadata.outputs.version }}",
        )
        self.assertEqual(download["with"]["repository"], "${{ github.repository }}")
        publish_scripts = "\n".join(
            step.get("run", "") for step in publish["steps"]
        )
        for forbidden in (
            "go test",
            "pnpm ",
            "docker build",
            "docker push",
            "docker login",
            "container.yml",
            "npm-release.yml",
            "gh run watch",
            "gh workflow run",
            "sleep ",
        ):
            self.assertNotIn(forbidden, publish_scripts)

    def test_all_run_blocks_are_valid_bash(self):
        for job_name, job in self.jobs.items():
            for index, step in enumerate(job.get("steps", [])):
                script = step.get("run")
                if script is None:
                    continue
                sanitized = GITHUB_EXPRESSION.sub("gha_expression", script)
                result = subprocess.run(
                    ["bash", "-n"],
                    input=sanitized,
                    text=True,
                    capture_output=True,
                    check=False,
                )
                self.assertEqual(
                    result.returncode,
                    0,
                    msg=(
                        f"invalid bash in job {job_name} step {index} "
                        f"({step.get('name', 'unnamed')}): {result.stderr}"
                    ),
                )


if __name__ == "__main__":
    unittest.main()
