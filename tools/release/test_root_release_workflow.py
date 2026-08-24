import re
import subprocess
import unittest
from pathlib import Path

import yaml


REPOSITORY_ROOT = Path(__file__).resolve().parents[2]
WORKFLOW_PATH = REPOSITORY_ROOT / ".github" / "workflows" / "release.yml"
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
        checkout = self.step("release", "Checkout release validation tooling")
        sparse_paths = set(checkout["with"]["sparse-checkout"].splitlines())
        self.assertEqual(sparse_paths, {"tools/install", "tools/release"})

    def test_every_tag_candidate_verifies_exact_merged_main_source(self):
        checkout = self.step("release-evidence", "Checkout release source")
        verify = self.step(
            "release-evidence", "Verify merged-main release source"
        )
        self.assertEqual(checkout.get("if"), "github.ref_type == 'tag'")
        self.assertEqual(verify.get("if"), "github.ref_type == 'tag'")
        script = verify["run"]
        for required in (
            "tools/release/verify_release_source.py",
            '--commit "${RELEASE_COMMIT}"',
            '--tag "${RELEASE_VERSION}"',
            "--policy .mss/release-policy.yaml",
        ):
            self.assertIn(required, script)

    def test_assembly_produces_and_qualifies_all_tool_archives(self):
        assemble = self.step("release", "Assemble release packages")["run"]
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
            'admin_web_candidate="frontend-v6-dist/admin-web-candidate.tgz"',
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

        upload = self.step("release", "Upload assembled packages")["with"]["path"]
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

    def test_tool_smoke_uses_exact_admin_web_candidate_before_public_npm(self):
        pack = self.step(
            "frontend-build", "Pack exact Admin Web candidate for tool smoke"
        )["run"]
        for required in (
            '"${RELEASE_COMMIT}:web/antd-v6"',
            'manifest["version"] = sys.argv[2]',
            'manifest["gitHead"] = sys.argv[3]',
            'pnpm pack --pack-destination "${candidate_output}"',
            'tar -xOf admin-web-candidate.tgz package/package.json',
        ):
            self.assertIn(required, pack)

        uploaded = self.step(
            "frontend-build", "Upload primary V6 artifact"
        )["with"]["path"].splitlines()
        self.assertIn("web/antd-v6/admin-web-candidate.tgz", uploaded)

        assemble = self.step("release", "Assemble release packages")["run"]
        smoke = assemble.split('smoke_dir="$(mktemp -d)"', 1)[1]
        self.assertIn('package_name = "@mss-boot-io/admin-web"', smoke)
        self.assertIn('"integrity": integrity', smoke)
        self.assertIn('--contributor-npm-registry "${registry_url}"', smoke)
        self.assertNotIn("registry.npmjs.org", smoke)

    def test_public_release_asset_set_is_exact_and_has_no_retired_tools(self):
        publish = self.step(
            "release", "Stage, verify, and publish GitHub release atomically"
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
        self.assertIn("is already public; refusing to mutate it", publish)

    def test_release_notes_are_package_first(self):
        notes = self.step("release", "Prepare deterministic root release notes")["run"]
        for required in (
            "## Package-first runtime and tools",
            "install-mss.sh",
            "install-mss.ps1",
            "SHA256SUMS.tools-*",
            "versioned Foundation Blueprint embedded in the installed tool",
            "without a Foundation source checkout",
        ):
            self.assertIn(required, notes)
        for forbidden in (
            "must run from a checked-out Foundation source tree",
            "must clone",
            "mss-pr",
        ):
            self.assertNotIn(forbidden, notes)

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
