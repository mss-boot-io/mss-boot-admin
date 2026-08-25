import re
import subprocess
import unittest
from pathlib import Path

import yaml


REPOSITORY_ROOT = Path(__file__).resolve().parents[2]
WORKFLOW_PATH = REPOSITORY_ROOT / ".github" / "workflows" / "container.yml"
DOCKERFILE_PATH = REPOSITORY_ROOT / "Dockerfile"
GITHUB_EXPRESSION = re.compile(r"\$\{\{.*?\}\}", re.DOTALL)


class ContainerWorkflowTest(unittest.TestCase):
    def setUp(self):
        self.content = WORKFLOW_PATH.read_text(encoding="utf-8")
        self.dockerfile = DOCKERFILE_PATH.read_text(encoding="utf-8")
        self.workflow = yaml.load(self.content, Loader=yaml.BaseLoader)
        self.jobs = self.workflow["jobs"]

    def test_multi_platform_go_build_cross_compiles_on_the_native_builder(self):
        self.assertIn("FROM --platform=$BUILDPLATFORM golang:", self.dockerfile)
        self.assertIn("ARG TARGETOS", self.dockerfile)
        self.assertIn("ARG TARGETARCH", self.dockerfile)
        self.assertIn(
            "CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build",
            self.dockerfile,
        )

        publish_timeout = int(self.jobs["publish"]["timeout-minutes"])
        self.assertGreaterEqual(publish_timeout, 75)

    def test_publish_authority_is_confined_to_the_release_job(self):
        self.assertNotIn("packages", self.workflow["permissions"])

        package_writers = [
            name
            for name, job in self.jobs.items()
            if job.get("permissions", {}).get("packages") == "write"
        ]
        self.assertEqual(package_writers, ["publish"])

        release_jobs = [
            name for name, job in self.jobs.items() if job.get("environment") == "release"
        ]
        self.assertEqual(release_jobs, ["publish"])
        self.assertNotIn("environment", self.jobs["build"])

        registry_logins = [
            name
            for name, job in self.jobs.items()
            if any(
                step.get("uses", "").startswith("docker/login-action@")
                for step in job.get("steps", [])
            )
        ]
        image_pushers = [
            name
            for name, job in self.jobs.items()
            if any(
                step.get("uses", "").startswith("docker/build-push-action@")
                and step.get("with", {}).get("push") == "true"
                for step in job.get("steps", [])
            )
        ]
        self.assertEqual(registry_logins, ["publish"])
        self.assertEqual(image_pushers, ["publish"])

    def test_publish_job_requires_an_exact_tag_publication_request(self):
        condition = " ".join(self.jobs["publish"]["if"].split())
        self.assertEqual(
            condition,
            "${{ needs.build.outputs.publish == 'true' && "
            "github.ref_type == 'tag' && "
            "github.ref_name == needs.build.outputs.version && "
            "( github.event_name == 'push' || "
            "(github.event_name == 'workflow_dispatch' && inputs.publish == true) ) }}",
        )

    def test_candidate_build_is_unpublished_and_smoke_tested(self):
        build_steps = self.jobs["build"]["steps"]
        build_info = next(step for step in build_steps if step.get("id") == "build-info")
        for required in (
            "publish=false",
            'version="${INPUT_VERSION}"',
            'publish="${INPUT_PUBLISH}"',
            'elif [[ "${GITHUB_REF_TYPE}" == "tag" ]]',
            'stable=false',
            'if [[ "${version}" =~ ^v(0|[1-9][0-9]*)\\.',
            'echo "stable=${stable}"',
            "--intent qualify",
        ):
            self.assertIn(required, build_info["run"])
        self.assertNotIn("--intent publish", build_info["run"])
        self.assertNotIn("verify_readiness_run.sh", build_info["run"])

        build_image = next(
            step
            for step in build_steps
            if step.get("uses", "").startswith("docker/build-push-action@")
        )
        self.assertEqual(build_image["with"]["push"], "false")
        self.assertEqual(build_image["with"]["load"], "true")
        self.assertEqual(build_image["with"]["platforms"], "linux/amd64")
        self.assertFalse(
            any(
                step.get("uses", "").startswith("docker/login-action@")
                for step in build_steps
            )
        )

        smoke = next(
            step for step in build_steps if step.get("name") == "Smoke-test candidate image"
        )
        self.assertNotIn("if", smoke)
        for label in (
            "org.opencontainers.image.title=mss-boot-admin",
            "org.opencontainers.image.description=Complete Go Admin backend for the mss-boot agent-native management-system distribution.",
        ):
            self.assertIn(label, build_image["with"]["labels"])
            self.assertIn(label.split("=", 1)[1], smoke["run"])

    def test_publish_job_keeps_policy_readiness_and_image_evidence(self):
        publish_steps = self.jobs["publish"]["steps"]
        authority = next(
            step
            for step in publish_steps
            if step.get("name") == "Require exact release publication authority"
        )
        for required in (
            "check_release_policy.py",
            "--intent publish",
            'readiness_run_id="${INPUT_READINESS_RUN_ID:-${RELEASE_READINESS_RUN_ID}}"',
            "verify_readiness_run.sh",
            '--run-id "${readiness_run_id}"',
            "--phase pre-root",
            '"${GITHUB_REF_NAME}" != "${RELEASE_VERSION}"',
        ):
            self.assertIn(required, authority["run"])

        publish_image = next(
            step
            for step in publish_steps
            if step.get("uses", "").startswith("docker/build-push-action@")
        )
        self.assertEqual(publish_image["with"]["push"], "true")
        self.assertEqual(publish_image["with"]["platforms"], "linux/amd64,linux/arm64")
        self.assertEqual(publish_image["with"]["provenance"], "true")
        self.assertEqual(publish_image["with"]["sbom"], "true")
        for label in (
            "org.opencontainers.image.title=mss-boot-admin",
            "org.opencontainers.image.description=Complete Go Admin backend for the mss-boot agent-native management-system distribution.",
        ):
            self.assertIn(label, publish_image["with"]["labels"])
        digest = next(
            step
            for step in publish_steps
            if step.get("name") == "Record published image digest"
        )
        self.assertIn("org.opencontainers.image.description", digest["run"])
        self.assertEqual(
            digest["env"]["IMAGE_DIGEST"],
            "${{ steps.publish.outputs.digest || steps.existing-image.outputs.digest }}",
        )

        immutability = next(
            step
            for step in publish_steps
            if step.get("name") == "Resolve immutable root image version"
        )
        self.assertEqual(immutability["id"], "existing-image")
        for required in (
            '"${REGISTRY}/${IMAGE_NAME}:${RELEASE_VERSION}"',
            "docker buildx imagetools inspect",
            "--raw",
            '"linux/amd64"',
            '"linux/arm64"',
            'org.opencontainers.image.revision"] == $commit',
            "Reusing exact immutable root image",
            "exists=true",
            "exists=false",
            'echo "digest=${digest}"',
            "authoritative not-found response",
        ):
            self.assertIn(required, immutability["run"])
        self.assertEqual(
            publish_image["if"],
            "${{ steps.existing-image.outputs.exists != 'true' }}",
        )
        self.assertLess(
            publish_steps.index(immutability), publish_steps.index(publish_image)
        )

        recovered_alias = next(
            step
            for step in publish_steps
            if step.get("name")
            == "Restore stable alias from a recovered immutable image"
        )
        self.assertEqual(
            recovered_alias["if"],
            "${{ needs.build.outputs.stable == 'true' && steps.existing-image.outputs.exists == 'true' }}",
        )
        self.assertEqual(
            recovered_alias["env"]["IMAGE_DIGEST"],
            "${{ steps.existing-image.outputs.digest }}",
        )
        for required in (
            "docker buildx imagetools create",
            '--tag "${image_repository}:latest"',
            '"${image_repository}@${IMAGE_DIGEST}"',
        ):
            self.assertIn(required, recovered_alias["run"])
        self.assertLess(
            publish_steps.index(publish_image), publish_steps.index(recovered_alias)
        )
        self.assertLess(publish_steps.index(recovered_alias), publish_steps.index(digest))

    def test_only_stable_tags_receive_the_latest_alias(self):
        metadata_steps = [
            step
            for job in self.jobs.values()
            for step in job.get("steps", [])
            if step.get("uses", "").startswith("docker/metadata-action@")
        ]
        self.assertEqual(len(metadata_steps), 2)
        stable_latest = (
            "type=raw,value=latest,enable=${{ steps.build-info.outputs.stable == 'true' }}",
            "type=raw,value=latest,enable=${{ needs.build.outputs.stable == 'true' }}",
        )
        for step, expected in zip(metadata_steps, stable_latest, strict=True):
            with self.subTest(step=step.get("name")):
                self.assertEqual(step["with"]["flavor"].strip(), "latest=false")
                self.assertIn(expected, step["with"]["tags"])
                self.assertNotIn("latest=auto", step["with"]["flavor"])

    def test_stable_aliases_fail_closed_on_the_published_manifest_digest(self):
        publish_steps = self.jobs["publish"]["steps"]
        alias = next(
            step
            for step in publish_steps
            if step.get("name")
            == "Verify stable image aliases resolve to the published digest"
        )
        self.assertEqual(alias["if"], "${{ needs.build.outputs.stable == 'true' }}")
        self.assertEqual(
            alias["env"]["EXPECTED_DIGEST"],
            "${{ steps.publish.outputs.digest || steps.existing-image.outputs.digest }}",
        )
        self.assertEqual(
            alias["env"]["RELEASE_VERSION"], "${{ needs.build.outputs.version }}"
        )
        for required in (
            '"${image_repository}:${RELEASE_VERSION}"',
            '"${image_repository}:latest"',
            "docker buildx imagetools inspect",
            "--format '{{json .Manifest}}'",
            "version_digest=\"$(jq -er '.digest'",
            "latest_digest=\"$(jq -er '.digest'",
            '"${version_digest}" == "${EXPECTED_DIGEST}"',
            '"${latest_digest}" == "${version_digest}"',
            "stable image aliases do not resolve to the published manifest digest",
            "exit 1",
        ):
            self.assertIn(required, alias["run"])
        self.assertNotIn("imagetools create", alias["run"])

        alias_readers = [
            step
            for step in publish_steps
            if ":latest" in step.get("run", "")
            and "imagetools inspect" in step.get("run", "")
        ]
        self.assertEqual(alias_readers, [alias])
        publish_image = next(
            step
            for step in publish_steps
            if step.get("uses", "").startswith("docker/build-push-action@")
        )
        self.assertLess(publish_steps.index(publish_image), publish_steps.index(alias))

    def test_stable_classifier_accepts_only_exact_semver(self):
        build_info = next(
            step for step in self.jobs["build"]["steps"] if step.get("id") == "build-info"
        )
        script = build_info["run"]
        start = script.index("stable=false")
        end = script.index("\n{", start)
        classifier = script[start:end]
        for version, expected in (
            ("v1.3.0", "true"),
            ("v0.0.0", "true"),
            ("v1.3.0-rc.6", "false"),
            ("v1.3.0+build.1", "false"),
            ("v01.3.0", "false"),
            ("sha-0123456789ab", "false"),
        ):
            with self.subTest(version=version):
                result = subprocess.run(
                    [
                        "bash",
                        "-c",
                        'version="$1"\n' + classifier + '\nprintf "%s" "$stable"',
                        "stable-classifier",
                        version,
                    ],
                    text=True,
                    capture_output=True,
                    check=False,
                )
                self.assertEqual(result.returncode, 0, msg=result.stderr)
                self.assertEqual(result.stdout, expected)

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
                        f"invalid bash in {job_name} step {index} "
                        f"({step.get('name', 'unnamed')}): {result.stderr}"
                    ),
                )


if __name__ == "__main__":
    unittest.main()
