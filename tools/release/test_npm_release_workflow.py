import re
import subprocess
import unittest
from pathlib import Path

import yaml


REPOSITORY_ROOT = Path(__file__).resolve().parents[2]
WORKFLOW_PATH = REPOSITORY_ROOT / ".github" / "workflows" / "npm-release.yml"
GITHUB_EXPRESSION = re.compile(r"\$\{\{.*?\}\}", re.DOTALL)


class OfficialNpmReleaseWorkflowTest(unittest.TestCase):
    def setUp(self):
        self.content = WORKFLOW_PATH.read_text(encoding="utf-8")
        self.workflow = yaml.load(self.content, Loader=yaml.BaseLoader)
        self.job = self.workflow["jobs"]["publish"]
        self.steps = self.job["steps"]

    def step(self, name):
        return next(step for step in self.steps if step.get("name") == name)

    def test_is_manual_only_and_binds_provenance_to_the_exact_tag_commit(self):
        self.assertEqual(set(self.workflow["on"]), {"workflow_dispatch"})
        inputs = self.workflow["on"]["workflow_dispatch"]["inputs"]
        self.assertEqual(inputs["version"]["required"], "true")
        self.assertEqual(inputs["commit"]["required"], "true")
        self.assertEqual(inputs["allow_npm_token_bootstrap"]["default"], "false")

        identity = self.step("Validate exact dispatch identity")["run"]
        for required in (
            "^v(0|[1-9][0-9]*)",
            "^[0-9a-f]{40}$",
            '"${GITHUB_REF_TYPE}" != \'tag\'',
            '"${GITHUB_REF_NAME}" != "${INPUT_VERSION}"',
            '"${GITHUB_SHA}" != "${INPUT_COMMIT}"',
        ):
            self.assertIn(required, identity)

        checkout = self.step("Checkout exact release commit")
        self.assertEqual(checkout["with"]["ref"], "${{ inputs.commit }}")
        self.assertEqual(checkout["with"]["fetch-depth"], "0")

    def test_uses_the_trusted_publisher_environment_and_minimal_permissions(self):
        self.assertEqual(self.job["runs-on"], "ubuntu-24.04")
        self.assertEqual(self.job["environment"], "release-v6")
        self.assertEqual(
            self.job["permissions"],
            {
                "actions": "read",
                "contents": "read",
                "id-token": "write",
                "packages": "read",
                "pull-requests": "read",
            },
        )
        self.assertEqual(
            self.workflow["permissions"]["pull-requests"], "read"
        )
        setup = self.step("Setup Node 24")
        self.assertEqual(setup["with"]["node-version"], "24")
        self.assertEqual(setup["with"]["package-manager-cache"], "false")
        self.assertNotIn("cache", setup["with"])
        npm_setup = self.step("Install a trusted-publishing capable npm CLI")["run"]
        self.assertIn("npm@11.19.0", npm_setup)
        self.assertIn("minor < 5", npm_setup)
        self.assertIn("patch < 1", npm_setup)

    def test_requires_merged_main_and_the_complete_stable_release_train(self):
        source = self.step("Verify merged-main release source")["run"]
        self.assertIn("verify_release_source.py", source)
        self.assertIn('--commit "${RELEASE_COMMIT}"', source)
        self.assertIn('--tag "${RELEASE_VERSION}"', source)
        self.assertIn("--intent publish", source)

        train = self.step("Require the complete exact-commit release train")["run"]
        for required in (
            "${RELEASE_VERSION}\n",
            "mss-boot/${RELEASE_VERSION}",
            "admin/${RELEASE_VERSION}",
            "web/antd-v6/${RELEASE_VERSION}",
            "docs/${RELEASE_VERSION}",
            ".isDraft == false",
            ".isPrerelease == false",
            "framework-release.yml|mss-boot/${RELEASE_VERSION}|push|",
            "admin-release.yml|admin/${RELEASE_VERSION}|push|",
            "frontend-v6-release.yml|web/antd-v6/${RELEASE_VERSION}|push|",
            "docs.yml|docs/${RELEASE_VERSION}|push|",
            "container.yml|${RELEASE_VERSION}|workflow_dispatch|Root Image publish ${RELEASE_VERSION}",
            "release.yml|${RELEASE_VERSION}|workflow_dispatch|Root Release candidate ${RELEASE_VERSION}",
            "release.yml|${RELEASE_VERSION}|workflow_dispatch|Root Release publish ${RELEASE_VERSION}",
            ".head_branch == $component_ref",
            ".head_sha == $commit",
            ".display_title == $display_title",
            ".conclusion == \"success\"",
        ):
            self.assertIn(required, train)

    def test_downloads_and_qualifies_existing_assets_without_rebuilding(self):
        assets = self.step("Download and verify existing frontend package assets")[
            "run"
        ]
        for required in (
            "gh release download",
            "--pattern '*.tgz'",
            "admin-web-package.json",
            "admin-web.spdx.json",
            "SHA256SUMS.admin-web",
            "sha256sum --strict --check",
            "verify_admin_web_package.py",
            "qualified-admin-web-package.json",
            "cmp --",
            'spdxVersion == "SPDX-2.3"',
        ):
            self.assertIn(required, assets)

        for forbidden in (
            "pnpm ",
            "npm pack",
            "npm run",
            "build:release",
            "docker build",
            "generate_admin_web_sbom.py",
        ):
            self.assertNotIn(forbidden, self.content)

    def test_github_packages_must_match_the_release_asset_byte_for_byte(self):
        github_packages = self.step(
            "Verify GitHub Packages has the identical package"
        )["run"]
        for required in (
            "https://npm.pkg.github.com",
            'version gitHead dist.integrity dist.tarball --json',
            '."dist.integrity" == $integrity',
            "dist-tags.latest",
            'Authorization: Bearer ${NODE_AUTH_TOKEN}',
            'cmp -- "${PACKAGE_TARBALL}" "${github_tarball}"',
            "/packages/npm/admin-web",
            ".repository.full_name == $repository",
        ):
            self.assertIn(required, github_packages)

    def test_npmjs_preflight_is_fail_closed_and_safe_to_rerun(self):
        reconcile = self.step("Reconcile immutable npmjs publication state")
        self.assertEqual(reconcile["id"], "npmjs")
        script = reconcile["run"]
        for required in (
            "https://registry.npmjs.org",
            "E404|404 Not Found",
            'echo \'publish=false\'',
            'echo \'publish=true\'',
            'echo \'package_absent=false\'',
            'echo "package_absent=${package_absent}"',
            'npm view "${package}@*" name --json',
            'all(.[]; . == $package)',
            "npmjs package existence check failed without an authoritative E404",
            '."dist.integrity" == $integrity',
            '.gitHead == $commit',
        ):
            self.assertIn(required, script)
        self.assertNotIn("npm unpublish", self.content)
        self.assertNotIn("npm deprecate", self.content)
        self.assertNotIn("npm dist-tag", self.content)

    def test_official_publish_is_the_only_and_last_external_mutation(self):
        publish = self.step("Publish the existing tarball to official npm last")
        self.assertEqual(publish["if"], "steps.npmjs.outputs.publish == 'true'")
        self.assertEqual(
            publish["env"]["NPM_TOKEN"], "${{ secrets.NPM_TOKEN }}"
        )
        script = publish["run"]
        for required in (
            "ALLOW_NPM_TOKEN_BOOTSTRAP",
            "PACKAGE_ABSENT",
            "bootstrap is allowed only while the entire npmjs package is absent",
            'npm view "${package}@*" name --json',
            'all(.[]; . == $package)',
            "authenticated npmjs package existence check failed without an authoritative E404",
            "already exists for the bootstrap token; refusing bootstrap",
            "the first npmjs package creation requires the explicit one-time token bootstrap",
            "unset NPM_TOKEN NODE_AUTH_TOKEN NPM_CONFIG_USERCONFIG",
            "npm publish",
            "--access public",
            "--tag latest",
            "--provenance",
            "--registry=https://registry.npmjs.org",
        ):
            self.assertIn(required, script)
        self.assertLess(
            script.index('npm view "${package}@*" name --json'),
            script.index("npm publish"),
        )
        self.assertNotIn("::add-mask::${NPM_TOKEN}", script)
        self.assertNotIn('echo "${NPM_TOKEN}"', script)
        self.assertEqual(self.content.count("npm publish"), 1)

        publish_index = self.steps.index(publish)
        verify_index = self.steps.index(
            self.step("Verify npmjs integrity provenance identity and consumer install")
        )
        self.assertLess(publish_index, verify_index)
        after_publish = "\n".join(
            step.get("run", "") for step in self.steps[publish_index + 1 :]
        )
        for forbidden in (
            "npm publish",
            "npm dist-tag",
            "gh release create",
            "gh release upload",
            "gh release edit",
            "git push",
            "docker push",
        ):
            self.assertNotIn(forbidden, after_publish)
        self.assertNotIn("actions/upload-artifact", self.content)

    def test_post_publication_verification_is_bounded_and_installs_a_consumer(self):
        verify = self.step(
            "Verify npmjs integrity provenance identity and consumer install"
        )["run"]
        for required in (
            "for attempt in $(seq 1 24)",
            ".gitHead == $commit",
            '."dist.integrity" == $integrity',
            '."dist.attestations".provenance.predicateType',
            "dist-tags.latest",
            'cmp -- "${PACKAGE_TARBALL}" "${published_tarball}"',
            "npm install",
            "--ignore-scripts",
            "--registry=https://registry.npmjs.org",
            "require('@mss-boot-io/admin-web/package.json')",
        ):
            self.assertIn(required, verify)

    def test_all_run_blocks_are_valid_bash(self):
        for index, step in enumerate(self.steps):
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
                    f"invalid bash in step {index} "
                    f"({step.get('name', 'unnamed')}): {result.stderr}"
                ),
            )


if __name__ == "__main__":
    unittest.main()
