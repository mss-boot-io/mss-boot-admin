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
            name
            for name, job in self.jobs.items()
            if job.get("environment") == "release-auto"
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
            "github.event_name == 'push' && "
            "github.ref_type == 'tag' && "
            "github.ref_name == needs.build.outputs.version }}",
        )
        self.assertNotIn("workflow_dispatch", self.workflow["on"])
        self.assertEqual(
            set(self.workflow["on"]["workflow_call"]["inputs"]), {"version"}
        )
        self.assertIn("github.event_name == 'workflow_call'", self.workflow["run-name"])
        self.assertNotIn("workflow_dispatch", self.workflow["run-name"])

    def test_root_tag_container_run_does_not_share_the_root_release_lock(self):
        self.assertEqual(
            self.workflow["concurrency"]["group"],
            "container-${{ github.event.pull_request.number || github.ref }}",
        )
        self.assertEqual(
            self.workflow["concurrency"]["cancel-in-progress"],
            "${{ !startsWith(github.ref, 'refs/tags/') }}",
        )
        root_content = (
            REPOSITORY_ROOT / ".github" / "workflows" / "release.yml"
        ).read_text(encoding="utf-8")
        self.assertNotIn("foundation-release-${{ github.ref }}", root_content)
        self.assertNotIn("foundation-release-{0}", self.content)

    def test_root_preview_qualifies_amd64_smoke_and_both_release_platforms(self):
        build_steps = self.jobs["build"]["steps"]
        build_info = next(step for step in build_steps if step.get("id") == "build-info")
        for required in (
            "publish=false",
            'if [[ "${GITHUB_EVENT_NAME}" == "workflow_call" ]]',
            'version="${INPUT_VERSION}"',
            'elif [[ "${GITHUB_REF_TYPE}" == "tag" ]]',
            "publish=true",
            'stable=false',
            'if [[ "${version}" =~ ^v(0|[1-9][0-9]*)\\.',
            'echo "stable=${stable}"',
            "--intent qualify",
        ):
            self.assertIn(required, build_info["run"])
        self.assertNotIn("--intent publish", build_info["run"])
        self.assertNotIn("INPUT_PUBLISH", build_info["run"])
        self.assertNotIn("workflow_dispatch", build_info["run"])
        self.assertNotIn("verify_readiness_run.sh", build_info["run"])

        operator_gate = next(
            step
            for step in build_steps
            if step.get("name") == "Require authorized release operator"
        )
        self.assertEqual(
            operator_gate["if"],
            "${{ github.event_name == 'workflow_call' || github.ref_type == 'tag' }}",
        )

        self.assertEqual(
            set(self.workflow["on"]["workflow_call"]["inputs"]), {"version"}
        )
        self.assertEqual(
            self.workflow["on"]["workflow_call"]["inputs"]["version"]["required"],
            "true",
        )

        preview = next(
            step
            for step in build_steps
            if step.get("name") == "Require successful exact preview"
        )
        self.assertEqual(
            preview["if"], "steps.build-info.outputs.publish == 'true'"
        )
        for required in (
            "tools/release/resolve_successful_preview.sh",
            '--repository "${GITHUB_REPOSITORY}"',
            '--commit "${GITHUB_SHA}"',
            '--version "${RELEASE_VERSION}"',
            "--actor lwnmengjing",
        ):
            self.assertIn(required, preview["run"])

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
        self.assertEqual(smoke["if"], "github.ref_type != 'tag'")
        for label in (
            "org.opencontainers.image.title=mss-boot-admin",
            "org.opencontainers.image.description=Complete Go Admin backend for the mss-boot agent-native management-system distribution.",
        ):
            self.assertIn(label, build_image["with"]["labels"])
            self.assertIn(label.split("=", 1)[1], smoke["run"])

        qualification = next(
            step
            for step in build_steps
            if step.get("name") == "Qualify the exact multi-platform release image"
        )
        self.assertEqual(
            qualification["if"], "github.event_name == 'workflow_call'"
        )
        self.assertEqual(qualification["with"]["push"], "false")
        self.assertEqual(
            qualification["with"]["platforms"], "linux/amd64,linux/arm64"
        )
        self.assertEqual(qualification["with"]["provenance"], "true")
        self.assertEqual(qualification["with"]["sbom"], "true")

        non_tag_ci_steps = {
            "Setup Go",
            "Vendor workspace for Docker context",
            "Set up Docker Buildx",
            "Extract Docker metadata",
            "Build candidate image",
            "Smoke-test candidate image",
        }
        for step in build_steps:
            if step.get("name") in non_tag_ci_steps:
                with self.subTest(step=step["name"]):
                    self.assertEqual(step["if"], "github.ref_type != 'tag'")

        root_preview_only_steps = {
            "Set up QEMU",
            "Qualify the exact multi-platform release image",
        }
        for step in build_steps:
            if step.get("name") in root_preview_only_steps:
                with self.subTest(step=step["name"]):
                    self.assertEqual(step["if"], "github.event_name == 'workflow_call'")

    def test_publish_job_keeps_policy_and_image_evidence_without_manual_gate(self):
        publish_steps = self.jobs["publish"]["steps"]
        authority = next(
            step
            for step in publish_steps
            if step.get("name") == "Require exact release publication authority"
        )
        for required in (
            "check_release_policy.py",
            "--intent publish",
            '"${GITHUB_EVENT_NAME}" != "push"',
            '"${GITHUB_REF_NAME}" != "${RELEASE_VERSION}"',
        ):
            self.assertIn(required, authority["run"])
        for forbidden in (
            "INPUT_PUBLISH",
            "READINESS_RUN_ID",
            "verify_readiness_run.sh",
            "workflow_dispatch",
        ):
            self.assertNotIn(forbidden, authority["run"])
        self.assertNotIn("inputs.publish", self.content)

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

    def test_candidate_and_publish_metadata_never_create_latest_alias(self):
        metadata_steps = [
            step
            for job in self.jobs.values()
            for step in job.get("steps", [])
            if step.get("uses", "").startswith("docker/metadata-action@")
        ]
        self.assertEqual(len(metadata_steps), 2)
        for step in metadata_steps:
            with self.subTest(step=step.get("name")):
                self.assertEqual(step["with"]["flavor"].strip(), "latest=false")
                self.assertIn("type=ref,event=tag", step["with"]["tags"])
                self.assertIn(
                    "type=sha,prefix=,format=long,enable=true,priority=100",
                    step["with"]["tags"],
                )
                self.assertNotIn("value=latest", step["with"]["tags"])
                self.assertNotIn("latest=auto", step["with"]["flavor"])
        self.assertNotIn(":latest", self.content)
        self.assertNotIn("Restore stable alias", self.content)
        self.assertNotIn("imagetools create", self.content)

    def test_immutable_version_tag_fails_closed_on_the_published_manifest_digest(self):
        publish_steps = self.jobs["publish"]["steps"]
        verification = next(
            step
            for step in publish_steps
            if step.get("name")
            == "Verify immutable image tag resolves to the published digest"
        )
        self.assertEqual(
            verification["env"]["EXPECTED_DIGEST"],
            "${{ steps.publish.outputs.digest || steps.existing-image.outputs.digest }}",
        )
        self.assertEqual(
            verification["env"]["RELEASE_VERSION"],
            "${{ needs.build.outputs.version }}",
        )
        for required in (
            '"${image_repository}:${RELEASE_VERSION}"',
            "docker buildx imagetools inspect",
            "--format '{{json .Manifest}}'",
            "version_digest=\"$(jq -er '.digest'",
            '"${version_digest}" == "${EXPECTED_DIGEST}"',
            "immutable image tag does not resolve to the published manifest digest",
            "for attempt in $(seq 1 12)",
            "exit 1",
        ):
            self.assertIn(required, verification["run"])
        self.assertNotIn(":latest", verification["run"])
        self.assertNotIn("imagetools create", verification["run"])
        publish_image = next(
            step
            for step in publish_steps
            if step.get("uses", "").startswith("docker/build-push-action@")
        )
        self.assertLess(
            publish_steps.index(publish_image), publish_steps.index(verification)
        )

    def test_root_container_publication_does_not_wait_for_root_or_npm(self):
        for forbidden in (
            "gh run watch",
            "gh workflow run",
            "npm-release.yml",
            "/actions/workflows/release.yml/runs",
            "/actions/workflows/npm-release.yml/runs",
        ):
            self.assertNotIn(forbidden, self.content)

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
