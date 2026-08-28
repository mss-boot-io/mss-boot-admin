import json
import os
import subprocess
import tempfile
import unittest
from pathlib import Path


REPOSITORY_ROOT = Path(__file__).resolve().parents[2]
HELPER = REPOSITORY_ROOT / "tools" / "release" / "resolve_successful_preview.sh"
COMMIT = "a" * 40


class ResolveSuccessfulPreviewTest(unittest.TestCase):
    def run_helper(self, workflow_runs, artifacts=None):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            runs_response = root / "runs.json"
            runs_response.write_text(
                json.dumps({"workflow_runs": workflow_runs}), encoding="utf-8"
            )
            artifacts_response = root / "artifacts.json"
            artifacts_response.write_text(
                json.dumps(
                    {
                        "artifacts": artifacts
                        if artifacts is not None
                        else [
                            {
                                "id": 900,
                                "name": "release-packages-v9.8.7",
                                "expired": False,
                                "size_in_bytes": 4096,
                            },
                            {
                                "id": 901,
                                "name": "root-image-preview-v9.8.7",
                                "expired": False,
                                "size_in_bytes": 8192,
                            },
                        ]
                    }
                ),
                encoding="utf-8",
            )
            args_log = root / "gh-args.log"
            fake_gh = root / "gh"
            fake_gh.write_text(
                """#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "${FAKE_GH_ARGS_LOG}"
case "$*" in
  *'/actions/workflows/release.yml/runs'*) cat "${FAKE_GH_RUNS_RESPONSE}" ;;
  *'/artifacts'*) cat "${FAKE_GH_ARTIFACTS_RESPONSE}" ;;
  *) echo "unexpected gh request: $*" >&2; exit 3 ;;
esac
""",
                encoding="utf-8",
            )
            fake_gh.chmod(0o755)
            env = os.environ.copy()
            env["FAKE_GH_RUNS_RESPONSE"] = str(runs_response)
            env["FAKE_GH_ARTIFACTS_RESPONSE"] = str(artifacts_response)
            env["FAKE_GH_ARGS_LOG"] = str(args_log)
            env["PATH"] = f"{root}:{env['PATH']}"
            result = subprocess.run(
                [
                    "bash",
                    str(HELPER),
                    "--repository",
                    "mss-boot-io/mss-boot-admin",
                    "--commit",
                    COMMIT,
                    "--version",
                    "v9.8.7",
                    "--actor",
                    "lwnmengjing",
                ],
                text=True,
                capture_output=True,
                check=False,
                env=env,
            )
            requests = (
                args_log.read_text(encoding="utf-8").splitlines()
                if args_log.exists()
                else []
            )
            return result, requests

    @staticmethod
    def successful_run(run_id, *, run_attempt=1, head_branch="main"):
        return {
            "id": run_id,
            "run_attempt": run_attempt,
            "event": "workflow_dispatch",
            "head_branch": head_branch,
            "head_sha": COMMIT,
            "path": ".github/workflows/release.yml",
            "display_title": "Root Release candidate v9.8.7",
            "actor": {"login": "lwnmengjing"},
            "triggering_actor": {"login": "lwnmengjing"},
            "status": "completed",
            "conclusion": "success",
        }

    def test_selects_the_highest_exact_run_id_and_its_artifact(self):
        result, requests = self.run_helper(
            [
                self.successful_run(100, run_attempt=99),
                self.successful_run(200, run_attempt=1),
            ]
        )
        self.assertEqual(result.returncode, 0, msg=result.stderr)
        self.assertEqual(result.stdout, "200\n")
        self.assertEqual(len(requests), 2)
        for required in (
            "--method GET",
            "/repos/mss-boot-io/mss-boot-admin/actions/workflows/release.yml/runs",
            f"head_sha={COMMIT}",
            "event=workflow_dispatch",
            "status=completed",
            "per_page=100",
        ):
            self.assertIn(required, requests[0])
        self.assertIn(
            "/repos/mss-boot-io/mss-boot-admin/actions/runs/200/artifacts",
            requests[1],
        )
        self.assertIn("per_page=100", requests[1])

    def test_rejects_successes_that_do_not_match_the_exact_preview_identity(self):
        mismatches = {
            "branch": {"head_branch": "codex/unmerged"},
            "commit": {"head_sha": "b" * 40},
            "workflow": {"path": ".github/workflows/container.yml"},
            "title": {"display_title": "Root Release candidate v9.8.6"},
            "actor": {"actor": {"login": "someone-else"}},
            "triggering actor": {
                "triggering_actor": {"login": "someone-else"}
            },
            "event": {"event": "push"},
            "status": {"status": "in_progress"},
            "conclusion": {"conclusion": "failure"},
        }
        for name, changes in mismatches.items():
            with self.subTest(identity=name):
                run = self.successful_run(200)
                run.update(changes)
                result, requests = self.run_helper([run])
                self.assertNotEqual(result.returncode, 0)
                self.assertIn(
                    "no successful exact Root Release candidate", result.stderr
                )
                self.assertEqual(len(requests), 1)

    def test_rejects_missing_expired_empty_or_duplicate_exact_artifacts(self):
        valid_packages = {
            "id": 900,
            "name": "release-packages-v9.8.7",
            "expired": False,
            "size_in_bytes": 4096,
        }
        valid_image = {
            "id": 901,
            "name": "root-image-preview-v9.8.7",
            "expired": False,
            "size_in_bytes": 8192,
        }
        cases = {
            "missing": [],
            "wrong package name": [
                {**valid_packages, "name": "release-packages-v9.8.6"},
                valid_image,
            ],
            "expired package": [{**valid_packages, "expired": True}, valid_image],
            "empty package": [{**valid_packages, "size_in_bytes": 0}, valid_image],
            "duplicate package": [
                valid_packages,
                {**valid_packages, "id": 902},
                valid_image,
            ],
            "missing image": [valid_packages],
            "expired image": [valid_packages, {**valid_image, "expired": True}],
            "empty image": [valid_packages, {**valid_image, "size_in_bytes": 0}],
            "duplicate image": [
                valid_packages,
                valid_image,
                {**valid_image, "id": 902},
            ],
        }
        for name, artifacts in cases.items():
            with self.subTest(artifact=name):
                result, requests = self.run_helper(
                    [self.successful_run(200)], artifacts=artifacts
                )
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("has no exact unexpired", result.stderr)
                self.assertEqual(len(requests), 2)
                self.assertIn("/actions/runs/200/artifacts", requests[1])


if __name__ == "__main__":
    unittest.main()
