import re
import subprocess
import unittest
from pathlib import Path

import yaml


REPOSITORY_ROOT = Path(__file__).resolve().parents[2]
WORKFLOW_PATH = REPOSITORY_ROOT / ".github" / "workflows" / "container.yml"
GITHUB_EXPRESSION = re.compile(r"\$\{\{.*?\}\}", re.DOTALL)


class ContainerWorkflowTest(unittest.TestCase):
    def setUp(self):
        self.content = WORKFLOW_PATH.read_text(encoding="utf-8")
        self.workflow = yaml.load(self.content, Loader=yaml.BaseLoader)
        self.jobs = self.workflow["jobs"]

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
