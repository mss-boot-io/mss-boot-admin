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

    def test_stable_promotion_dispatch_binds_the_exact_root_tag_identity(self):
        self.assertEqual(set(self.workflow["on"]), {"workflow_dispatch"})
        dispatch_inputs = self.workflow["on"]["workflow_dispatch"]["inputs"]
        self.assertEqual(set(dispatch_inputs), {"version"})
        self.assertEqual(dispatch_inputs["version"]["required"], "true")

        identity = self.step("Validate exact root tag identity")["run"]
        identity_step = self.step("Validate exact root tag identity")
        self.assertEqual(identity_step["env"]["REQUESTED_VERSION"], "${{ inputs.version }}")
        for required in (
            "^v(0|[1-9][0-9]*)",
            "^[0-9a-f]{40}$",
            '"${GITHUB_EVENT_NAME}" != \'workflow_dispatch\'',
            '"${GITHUB_REF_TYPE}" != \'tag\'',
            '"${REQUESTED_VERSION}" != "${GITHUB_REF_NAME}"',
            "RELEASE_VERSION=${GITHUB_REF_NAME}",
            "RELEASE_COMMIT=${GITHUB_SHA}",
        ):
            self.assertIn(required, identity)
        self.assertNotIn("${{ inputs.", identity)

        checkout = self.step("Checkout exact release commit")
        self.assertEqual(checkout["with"]["ref"], "${{ github.sha }}")
        self.assertEqual(checkout["with"]["fetch-depth"], "0")
        for forbidden in (
            "inputs.commit",
            "allow_npm_token_bootstrap",
            "Root Release publish",
        ):
            self.assertNotIn(forbidden, self.content)
        self.assertNotRegex(self.content, r"\bnpm dist-tag (?:add|rm)\b")

    def test_uses_the_trusted_publisher_environment_and_minimal_permissions(self):
        self.assertEqual(self.job["runs-on"], "ubuntu-24.04")
        self.assertEqual(self.job["environment"], "npm-auto")
        self.assertEqual(
            self.workflow["concurrency"],
            {
                "group": "official-npm-publication",
                "cancel-in-progress": "false",
            },
        )
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

    def test_requires_merged_main_and_only_the_exact_frontend_release(self):
        source = self.step("Verify merged-main release source")["run"]
        self.assertIn("verify_release_source.py", source)
        self.assertIn('--commit "${RELEASE_COMMIT}"', source)
        self.assertIn('--tag "${RELEASE_VERSION}"', source)
        self.assertIn("--source-mode promotion", source)
        self.assertIn("--intent publish", source)

        self.assertNotIn("Require successful exact preview", self.content)
        self.assertNotIn("resolve_successful_preview.sh", self.content)

        train = self.step("Require the exact frontend release")["run"]
        for required in (
            "web/antd-v6/${RELEASE_VERSION}",
            "--json tagName,isDraft,isPrerelease",
            ".isDraft == false",
            ".isPrerelease == false",
            "required frontend tag ${frontend_tag} is not available",
            "required frontend release ${frontend_tag} is not public",
        ):
            self.assertIn(required, train)
        for unrelated_prerequisite in (
            "mss-boot/${RELEASE_VERSION}",
            "admin/${RELEASE_VERSION}",
            "container.yml",
            "release.yml",
            "docs.yml",
        ):
            self.assertNotIn(unrelated_prerequisite, train)
        self.assertNotIn("for attempt in", train)
        self.assertNotIn("sleep ", train)

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

        for forbidden in ("pnpm ", "npm run", "build:release"):
            self.assertNotIn(forbidden, self.content)
        self.assertNotRegex(self.content, r"(?m)^\s*npm pack(?:\s|$)")
        self.assertNotRegex(self.content, r"(?m)^\s*docker build(?:\s|$)")
        self.assertNotIn("generate_admin_web_sbom.py", self.content)

    def test_github_packages_must_match_the_release_asset_byte_for_byte(self):
        github_packages = self.step(
            "Verify GitHub Packages has the identical package"
        )["run"]
        for required in (
            "https://npm.pkg.github.com",
            'version gitHead dist.integrity dist.tarball --json',
            '."dist.integrity" == $integrity',
            '"release-${UNPREFIXED_VERSION}"',
            "tr '[:upper:].' '[:lower:]-'",
            '"dist-tags.${mirror_dist_tag}"',
            'Authorization: Bearer ${NODE_AUTH_TOKEN}',
            'cmp -- "${PACKAGE_TARBALL}" "${github_tarball}"',
            "/packages/npm/admin-web",
            ".repository.full_name == $repository",
        ):
            self.assertIn(required, github_packages)
        self.assertNotIn("dist-tags.latest", github_packages)

    def test_official_publish_moves_latest_only_after_current_stable_preflight(self):
        selector = self.step("Require current stable npm alias before promotion")
        self.assertEqual(selector["id"], "npm-tag")
        script = selector["run"]
        for required in (
            "selected_tag=latest",
            'expected_latest="${UNPREFIXED_VERSION}"',
            'npm view "${package}" dist-tags.latest --json',
            'reviewed_latest="${CURRENT_STABLE_VERSION#v}"',
            'case "${GITHUB_LATEST_VERSION}:${current_latest}" in',
            '"${CURRENT_STABLE_VERSION}:${reviewed_latest}"',
            '"${CURRENT_STABLE_VERSION}:${UNPREFIXED_VERSION}"',
            '"${RELEASE_VERSION}:${UNPREFIXED_VERSION}")',
            '"${RELEASE_VERSION}:${reviewed_latest}")',
            "GitHub Latest cannot advance before npmjs latest",
            'echo "dist-tag=${selected_tag}"',
            'echo "expected-latest=${expected_latest}"',
        ):
            self.assertIn(required, script)
        self.assertNotIn("sort -V", script)
        self.assertNotIn("release-${UNPREFIXED_VERSION}", script)

        publish = self.step("Publish the existing tarball to official npm")
        self.assertEqual(
            publish["env"]["NPM_DIST_TAG"],
            "${{ steps.npm-tag.outputs.dist-tag }}",
        )
        self.assertIn("latest)", publish["run"])
        self.assertIn('--tag "${NPM_DIST_TAG}"', publish["run"])

        verify = self.step(
            "Verify npmjs integrity provenance and immutable identity"
        )
        self.assertEqual(
            verify["env"]["EXPECTED_LATEST"],
            "${{ steps.npm-tag.outputs.expected-latest }}",
        )
        self.assertIn('"${latest}" == "${EXPECTED_LATEST}"', verify["run"])
        self.assertNotIn('"dist-tags.${NPM_DIST_TAG}"', verify["run"])

    def test_stable_promotion_requires_reviewed_policy_and_distribution_ledger(self):
        preflight = self.step(
            "Require reviewed stable promotion and complete distribution ledger"
        )["run"]
        for required in (
            "origin/main:.mss/release-policy.yaml",
            "--intent promote",
            '--commit "${RELEASE_COMMIT}"',
            '"mss-boot/${RELEASE_VERSION}"',
            '"admin/${RELEASE_VERSION}"',
            '"web/antd-v6/${RELEASE_VERSION}"',
            'releases/latest',
            '"${current_stable}"|"${RELEASE_VERSION}")',
        ):
            self.assertIn(required, preflight)
        for forbidden in (
            '"docs/${RELEASE_VERSION}"',
            "https://docs.mss-boot-io.top/release.json",
            '"mss-boot-docs"',
        ):
            self.assertNotIn(forbidden, preflight)

        go_modules = self.step("Verify exact public Go modules")["run"]
        self.assertIn("GOPROXY=https://proxy.golang.org", go_modules)
        self.assertIn("mss-boot", go_modules)
        self.assertIn("admin", go_modules)

        images = self.step("Verify exact Root and Admin Web image aliases")["run"]
        self.assertIn("ghcr.io/${GITHUB_REPOSITORY}", images)
        self.assertIn("mss-boot-admin-antd-v6", images)
        self.assertIn('"${image}:${RELEASE_COMMIT}"', images)
        self.assertIn('.platform.os == "linux"', images)
        self.assertIn('.platform.architecture == "amd64"', images)
        self.assertIn('.platform.architecture == "arm64"', images)

    def test_github_latest_moves_only_after_verified_npm_latest(self):
        promotion = self.workflow["jobs"]["promote-github"]
        self.assertEqual(promotion["needs"], "publish")
        self.assertEqual(promotion["environment"], "release-auto")
        self.assertEqual(promotion["permissions"]["contents"], "write")
        steps = promotion["steps"]
        verify = next(
            step for step in steps if step.get("name") == "Validate exact completed npm promotion"
        )["run"]
        mutate = next(
            step for step in steps if step.get("name") == "Promote exact Root release to GitHub Latest"
        )["run"]
        self.assertIn("dist-tags.latest", verify)
        self.assertIn("dist.attestations", verify)
        self.assertIn('gh release edit "${RELEASE_VERSION}" --latest=true', mutate)
        self.assertLess(
            next(i for i, step in enumerate(steps) if step.get("name") == "Validate exact completed npm promotion"),
            next(i for i, step in enumerate(steps) if step.get("name") == "Promote exact Root release to GitHub Latest"),
        )

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
        self.assertNotRegex(self.content, r"\bnpm dist-tag (?:add|rm)\b")

    def test_official_publish_is_the_only_external_mutation(self):
        publish = self.step("Publish the existing tarball to official npm")
        self.assertEqual(publish["if"], "steps.npmjs.outputs.publish == 'true'")
        self.assertNotIn("NPM_TOKEN", publish["env"])
        script = publish["run"]
        for required in (
            "PACKAGE_ABSENT",
            "the first npmjs package creation requires a separately reviewed one-time bootstrap",
            "the automatic tag workflow is trusted-publishing only",
            "publish_authority_ref='refs/remotes/origin/npm-publish-authority'",
            'git fetch --no-tags origin',
            '"+refs/heads/main:${publish_authority_ref}"',
            'git show "${publish_authority_ref}:.mss/release-policy.yaml"',
            "python3 tools/release/check_release_policy.py",
            '--component npm',
            '--version "${RELEASE_VERSION}"',
            '--tag "@mss-boot-io/admin-web@${UNPREFIXED_VERSION}"',
            '--intent promote',
            '--commit "${RELEASE_COMMIT}"',
            "unset NPM_TOKEN NODE_AUTH_TOKEN NPM_CONFIG_USERCONFIG",
            "npm publish",
            "--access public",
            '--tag "${NPM_DIST_TAG}"',
            "--provenance",
            "--registry=https://registry.npmjs.org",
        ):
            self.assertIn(required, script)
        self.assertNotIn("ALLOW_NPM_TOKEN_BOOTSTRAP", script)
        self.assertNotIn("secrets.NPM_TOKEN", self.content)
        self.assertEqual(self.content.count("npm publish"), 1)

        final_fetch_index = script.index("git fetch --no-tags origin")
        final_policy_index = script.index(
            "python3 tools/release/check_release_policy.py",
            final_fetch_index,
        )
        unset_index = script.index(
            "unset NPM_TOKEN NODE_AUTH_TOKEN NPM_CONFIG_USERCONFIG",
            final_policy_index,
        )
        mutation_index = script.index('npm publish "${PACKAGE_TARBALL}"')
        self.assertLess(final_fetch_index, final_policy_index)
        self.assertLess(final_policy_index, unset_index)
        self.assertLess(unset_index, mutation_index)
        self.assertNotIn("${{", script)
        between_policy_and_publish = script[final_policy_index:mutation_index]
        for forbidden in (
            "npm view",
            "gh api",
            "gh release",
            "curl ",
            "sleep ",
        ):
            self.assertNotIn(forbidden, between_policy_and_publish)

        publish_index = self.steps.index(publish)
        verify_index = self.steps.index(
            self.step("Verify npmjs integrity provenance and immutable identity")
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

    def test_post_publication_verification_is_bounded_and_identity_only(self):
        verify = self.step(
            "Verify npmjs integrity provenance and immutable identity"
        )["run"]
        for required in (
            "for attempt in $(seq 1 24)",
            ".gitHead == $commit",
            '."dist.integrity" == $integrity',
            '."dist.attestations".provenance.predicateType',
            "dist-tags.latest",
            'cmp -- "${PACKAGE_TARBALL}" "${published_tarball}"',
            '"${latest}" == "${EXPECTED_LATEST}"',
        ):
            self.assertIn(required, verify)
        self.assertNotIn('"dist-tags.${NPM_DIST_TAG}"', verify)
        for forbidden in (
            "npm install",
            "--ignore-scripts",
            "require('@mss-boot-io/admin-web/package.json')",
        ):
            self.assertNotIn(forbidden, verify)

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
