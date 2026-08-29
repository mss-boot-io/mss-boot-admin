import copy
import json
import os
import re
import subprocess
import tempfile
import unittest
from pathlib import Path


REPOSITORY_ROOT = Path(__file__).resolve().parents[2]
SCRIPT_PATH = REPOSITORY_ROOT / "tools" / "release" / "verify_remote_release_governance.sh"
REPOSITORY = "mss-boot-io/mss-boot-admin"
REPOSITORY_ID = 4242
RELEASE_ACTOR_ID = 12806223

RELEASE_REFS = [
    "refs/tags/admin/v*",
    "refs/tags/docs/v*",
    "refs/tags/mss-boot/v*",
    "refs/tags/v*",
    "refs/tags/web/antd-v6/v*",
    "refs/tags/web/antd/v*",
]
STOPPED_V135_REFS = [
    "refs/tags/admin/v1.3.5",
    "refs/tags/docs/v1.3.5",
    "refs/tags/mss-boot/v1.3.5",
    "refs/tags/v1.3.5",
    "refs/tags/web/antd/v1.3.5",
    "refs/tags/web/antd-v6/v1.3.5",
]
STOPPED_V136_REFS = [
    "refs/tags/admin/v1.3.6",
    "refs/tags/docs/v1.3.6",
    "refs/tags/mss-boot/v1.3.6",
    "refs/tags/v1.3.6",
    "refs/tags/web/antd/v1.3.6",
    "refs/tags/web/antd-v6/v1.3.6",
]
ACTIVE_ENVIRONMENT_POLICIES = {
    "release-auto": [
        {"name": "admin/v*", "type": "tag"},
        {"name": "mss-boot/v*", "type": "tag"},
        {"name": "v*", "type": "tag"},
    ],
    "release-v6-auto": [{"name": "web/antd-v6/v*", "type": "tag"}],
    "npm-auto": [{"name": "v*", "type": "tag"}],
    "prod": [{"name": "docs/v*", "type": "tag"}],
}
ALL_ENVIRONMENTS = (
    "release",
    "release-v6",
    "release-auto",
    "release-v6-auto",
    "npm-auto",
    "prod",
)


def environment(name):
    return {
        "name": name,
        "can_admins_bypass": False,
        "protection_rules": [{"id": 1, "type": "branch_policy"}],
        "deployment_branch_policy": {
            "protected_branches": False,
            "custom_branch_policies": True,
        },
    }


def policies(entries):
    return {
        "total_count": len(entries),
        "branch_policies": [
            {"id": index + 1, **entry} for index, entry in enumerate(entries)
        ],
    }


def named_items(key, names):
    return {"total_count": len(names), key: [{"name": name} for name in names]}


class RemoteReleaseGovernanceTest(unittest.TestCase):
    def setUp(self):
        self.content = SCRIPT_PATH.read_text(encoding="utf-8")
        self.tempdir = tempfile.TemporaryDirectory()
        self.temp_path = Path(self.tempdir.name)
        self.state_path = self.temp_path / "state.json"
        fake_gh = self.temp_path / "gh"
        fake_gh.write_text(
            """#!/usr/bin/env python3
import json
import os
import sys

if len(sys.argv) < 3 or sys.argv[1] != "api":
    print("fake gh supports only: gh api [FLAGS] ENDPOINT", file=sys.stderr)
    raise SystemExit(2)
with open(os.environ["FAKE_GH_STATE"], encoding="utf-8") as handle:
    responses = json.load(handle)
arguments = sys.argv[2:]
endpoints = [argument for argument in arguments if not argument.startswith("-")]
if len(endpoints) != 1:
    print("fake gh requires exactly one API endpoint", file=sys.stderr)
    raise SystemExit(2)
endpoint = endpoints[0]
if endpoint not in responses:
    print(f"unexpected gh api endpoint: {endpoint}", file=sys.stderr)
    raise SystemExit(3)
response = responses[endpoint]
if isinstance(response, dict) and "__pages__" in response:
    if "--paginate" not in arguments:
        print("multi-page fake response requires --paginate", file=sys.stderr)
        raise SystemExit(4)
    for page in response["__pages__"]:
        json.dump(page, sys.stdout)
        sys.stdout.write("\\n")
else:
    json.dump(response, sys.stdout)
""",
            encoding="utf-8",
        )
        fake_gh.chmod(0o755)
        self.state = self.make_valid_state()

    def tearDown(self):
        self.tempdir.cleanup()

    @staticmethod
    def make_valid_state():
        ruleset_summaries = [
            {
                "id": 101,
                "name": "release-tags-controlled-creation",
                "target": "tag",
                "enforcement": "active",
            },
            {
                "id": 102,
                "name": "v1.3.5-stopped-tags-never-create",
                "target": "tag",
                "enforcement": "active",
            },
            {
                "id": 103,
                "name": "release-tags-immutable",
                "target": "tag",
                "enforcement": "active",
            },
            {
                "id": 104,
                "name": "v1.3.6-stopped-tags-never-create",
                "target": "tag",
                "enforcement": "active",
            },
        ]
        common_ruleset = {
            "source_type": "Repository",
            "source": REPOSITORY,
            "target": "tag",
            "enforcement": "active",
            "conditions": {"ref_name": {"exclude": []}},
        }
        state = {
            "user": {"login": "admin-inspector", "id": 7},
            "/users/lwnmengjing": {
                "login": "lwnmengjing",
                "id": RELEASE_ACTOR_ID,
            },
            f"/repos/{REPOSITORY}": {
                "id": REPOSITORY_ID,
                "permissions": {"admin": True},
            },
            f"/repos/{REPOSITORY}/keys?per_page=100": [],
            f"/repos/{REPOSITORY}/environments?per_page=100": {
                "total_count": len(ALL_ENVIRONMENTS),
                "environments": [{"name": name} for name in ALL_ENVIRONMENTS],
            },
            f"/repos/{REPOSITORY}/actions/variables?per_page=100": named_items(
                "variables", []
            ),
            f"/repos/{REPOSITORY}/actions/secrets?per_page=100": named_items(
                "secrets", []
            ),
            f"/repos/{REPOSITORY}/actions/organization-secrets?per_page=100": named_items(
                "secrets", ["CF_API_TOKEN", "UNRELATED_ORG_SECRET"]
            ),
            f"/repos/{REPOSITORY}/rulesets?includes_parents=true&per_page=100": ruleset_summaries,
            f"/repos/{REPOSITORY}/rulesets/101?includes_parents=true": {
                **common_ruleset,
                "id": 101,
                "name": "release-tags-controlled-creation",
                "conditions": {
                    "ref_name": {"include": RELEASE_REFS, "exclude": []}
                },
                "rules": [{"type": "creation"}],
                "bypass_actors": [
                    {
                        "actor_id": RELEASE_ACTOR_ID,
                        "actor_type": "User",
                        "bypass_mode": "always",
                    }
                ],
            },
            f"/repos/{REPOSITORY}/rulesets/102?includes_parents=true": {
                **common_ruleset,
                "id": 102,
                "name": "v1.3.5-stopped-tags-never-create",
                "conditions": {
                    "ref_name": {"include": STOPPED_V135_REFS, "exclude": []}
                },
                "rules": [{"type": "creation"}],
                "bypass_actors": [],
            },
            f"/repos/{REPOSITORY}/rulesets/103?includes_parents=true": {
                **common_ruleset,
                "id": 103,
                "name": "release-tags-immutable",
                "conditions": {
                    "ref_name": {"include": RELEASE_REFS, "exclude": []}
                },
                "rules": [
                    {"type": "update"},
                    {"type": "deletion"},
                    {"type": "non_fast_forward"},
                ],
                "bypass_actors": [],
            },
            f"/repos/{REPOSITORY}/rulesets/104?includes_parents=true": {
                **common_ruleset,
                "id": 104,
                "name": "v1.3.6-stopped-tags-never-create",
                "conditions": {
                    "ref_name": {"include": STOPPED_V136_REFS, "exclude": []}
                },
                "rules": [{"type": "creation"}],
                "bypass_actors": [],
            },
        }
        for name in ALL_ENVIRONMENTS:
            state[f"/repos/{REPOSITORY}/environments/{name}"] = environment(name)
            entries = ACTIVE_ENVIRONMENT_POLICIES.get(name, [])
            state[
                f"/repos/{REPOSITORY}/environments/{name}/deployment-branch-policies?per_page=100"
            ] = policies(entries)
            state[
                f"/repositories/{REPOSITORY_ID}/environments/{name}/variables?per_page=100"
            ] = named_items("variables", [])
            state[
                f"/repositories/{REPOSITORY_ID}/environments/{name}/secrets?per_page=100"
            ] = named_items("secrets", [])
        return state

    def run_script(self, state=None):
        state = self.state if state is None else state
        self.state_path.write_text(json.dumps(state), encoding="utf-8")
        run_environment = os.environ.copy()
        run_environment["PATH"] = f"{self.temp_path}{os.pathsep}{run_environment['PATH']}"
        run_environment["FAKE_GH_STATE"] = str(self.state_path)
        return subprocess.run(
            [
                "bash",
                str(SCRIPT_PATH),
                "--repository",
                REPOSITORY,
                "--release-actor-login",
                "lwnmengjing",
            ],
            cwd=REPOSITORY_ROOT,
            env=run_environment,
            text=True,
            capture_output=True,
            check=False,
            timeout=30,
        )

    def assert_rejected(self, state, message):
        result = self.run_script(state)
        self.assertNotEqual(result.returncode, 0, msg=result.stdout)
        self.assertIn(message, result.stderr)

    def test_script_is_syntax_valid_argument_confined_and_secret_safe(self):
        result = subprocess.run(
            ["bash", "-n", str(SCRIPT_PATH)],
            text=True,
            capture_output=True,
            check=False,
        )
        self.assertEqual(result.returncode, 0, msg=result.stderr)
        self.assertIn("OWNER/REPO", self.content)
        self.assertIn("--release-actor-login LOGIN", self.content)
        self.assertIn("^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$", self.content)
        self.assertNotRegex(self.content, re.compile(r"gh auth token|Authorization:"))
        self.assertNotRegex(self.content, re.compile(r"secrets\[[^]]+\]\.value"))
        self.assertIn("gh api --paginate", self.content)
        self.assertIn("jq -s -e", self.content)

    def test_valid_simplified_governance_contract_is_reported(self):
        result = self.run_script()
        self.assertEqual(result.returncode, 0, msg=result.stderr)
        report = json.loads(result.stdout)
        self.assertTrue(report["success"])
        self.assertEqual(report["releaseActor"], "lwnmengjing")
        self.assertEqual(report["controlledCreationRuleset"], 101)
        self.assertEqual(report["stoppedV135CreationRuleset"], 102)
        self.assertEqual(report["stoppedV136CreationRuleset"], 104)
        self.assertEqual(report["immutableRuleset"], 103)
        self.assertEqual(
            report["environments"],
            {
                "release": [],
                "releaseV6": [],
                "releaseAuto": [
                    "refs/tags/admin/v*",
                    "refs/tags/mss-boot/v*",
                    "refs/tags/v*",
                ],
                "releaseV6Auto": ["refs/tags/web/antd-v6/v*"],
                "npmAuto": ["refs/tags/v*"],
                "prod": ["refs/tags/docs/v*"],
            },
        )
        self.assertEqual(
            report["environmentSecrets"],
            {
                "release": [],
                "releaseV6": [],
                "releaseAuto": [],
                "releaseV6Auto": [],
                "npmAuto": [],
                "prod": [],
            },
        )
        self.assertEqual(
            report["docsCredential"],
            {
                "name": "CF_API_TOKEN",
                "source": "organization",
                "repositoryOverride": False,
                "environmentOverride": False,
            },
        )

    def test_retired_environments_are_non_bypassable_and_allow_no_refs(self):
        state = copy.deepcopy(self.state)
        state[f"/repos/{REPOSITORY}/environments/release"]["can_admins_bypass"] = True
        self.assert_rejected(
            state, "release must remain a non-bypassable retired environment"
        )

        state = copy.deepcopy(self.state)
        state[
            f"/repos/{REPOSITORY}/environments/release-v6/deployment-branch-policies?per_page=100"
        ] = policies([{"name": "web/antd-v6/v*", "type": "tag"}])
        self.assert_rejected(
            state, "release-v6 must allow no branch or tag deployments"
        )

    def test_active_environments_have_no_reviewers_or_admin_bypass(self):
        state = copy.deepcopy(self.state)
        state[f"/repos/{REPOSITORY}/environments/release-auto"][
            "protection_rules"
        ].append({"id": 2, "type": "required_reviewers", "reviewers": []})
        self.assert_rejected(
            state,
            "release-auto environment must have no required reviewers and no administrator bypass",
        )

        state = copy.deepcopy(self.state)
        state[f"/repos/{REPOSITORY}/environments/npm-auto"][
            "can_admins_bypass"
        ] = True
        self.assert_rejected(
            state,
            "npm-auto environment must have no required reviewers and no administrator bypass",
        )

    def test_each_active_environment_requires_its_exact_tag_policies(self):
        for name in ACTIVE_ENVIRONMENT_POLICIES:
            with self.subTest(environment=name):
                state = copy.deepcopy(self.state)
                endpoint = (
                    f"/repos/{REPOSITORY}/environments/{name}/"
                    "deployment-branch-policies?per_page=100"
                )
                state[endpoint]["branch_policies"][0]["name"] += "-wrong"
                self.assert_rejected(
                    state,
                    f"{name} environment deployment branch or tag policies are not exact",
                )

    def test_environment_secret_name_sets_are_exact_and_docs_secret_is_org_scoped(self):
        for name in ALL_ENVIRONMENTS:
            with self.subTest(environment=name):
                state = copy.deepcopy(self.state)
                endpoint = (
                    f"/repositories/{REPOSITORY_ID}/environments/{name}/"
                    "secrets?per_page=100"
                )
                state[endpoint] = named_items("secrets", ["UNEXPECTED_SECRET"])
                self.assert_rejected(
                    state, f"{name} environment secret names are not exact"
                )

        state = copy.deepcopy(self.state)
        state[
            f"/repos/{REPOSITORY}/actions/organization-secrets?per_page=100"
        ] = named_items("secrets", ["UNRELATED_ORG_SECRET"])
        self.assert_rejected(
            state,
            "CF_API_TOKEN must be available to this repository from organization Actions secrets",
        )

        state = copy.deepcopy(self.state)
        state[f"/repos/{REPOSITORY}/actions/secrets?per_page=100"] = named_items(
            "secrets", ["CF_API_TOKEN"]
        )
        self.assert_rejected(
            state,
            "repository-level CF_API_TOKEN would override the organization secret",
        )

    def test_docs_secret_checks_all_repository_and_organization_pages(self):
        state = copy.deepcopy(self.state)
        state[
            f"/repos/{REPOSITORY}/actions/organization-secrets?per_page=100"
        ] = {
            "__pages__": [
                named_items("secrets", ["UNRELATED_ORG_SECRET"]),
                named_items("secrets", ["CF_API_TOKEN"]),
            ]
        }
        result = self.run_script(state)
        self.assertEqual(result.returncode, 0, msg=result.stderr)

        state = copy.deepcopy(self.state)
        state[f"/repos/{REPOSITORY}/actions/secrets?per_page=100"] = {
            "__pages__": [
                named_items("secrets", ["UNRELATED_REPOSITORY_SECRET"]),
                named_items("secrets", ["CF_API_TOKEN"]),
            ]
        }
        self.assert_rejected(
            state,
            "repository-level CF_API_TOKEN would override the organization secret",
        )

    def test_readiness_run_variable_is_absent_at_repository_and_every_environment(self):
        self.assertIn(
            "for environment_name in release release-v6 release-auto release-v6-auto npm-auto prod; do",
            self.content,
        )
        state = copy.deepcopy(self.state)
        state[f"/repos/{REPOSITORY}/actions/variables?per_page=100"] = named_items(
            "variables", ["RELEASE_READINESS_RUN_ID"]
        )
        self.assert_rejected(
            state,
            "the retired RELEASE_READINESS_RUN_ID repository variable is still present",
        )

        state = copy.deepcopy(self.state)
        state[
            f"/repositories/{REPOSITORY_ID}/environments/prod/variables?per_page=100"
        ] = named_items("variables", ["RELEASE_READINESS_RUN_ID"])
        self.assert_rejected(
            state,
            "the retired RELEASE_READINESS_RUN_ID variable is still present in prod",
        )

    def test_retired_root_promotion_environment_and_deploy_key_are_absent(self):
        state = copy.deepcopy(self.state)
        state[f"/repos/{REPOSITORY}/keys?per_page=100"] = [
            {"id": 1, "title": "mss-root-tag-promotion", "read_only": False}
        ]
        self.assert_rejected(state, "retired Root promotion DeployKey is still present")

        state = copy.deepcopy(self.state)
        state[f"/repos/{REPOSITORY}/environments?per_page=100"][
            "environments"
        ].append({"name": "root-promotion"})
        self.assert_rejected(
            state, "the retired root-promotion environment is still present"
        )

    def test_controlled_creation_stop_freeze_and_immutability_are_exact(self):
        state = copy.deepcopy(self.state)
        rulesets_endpoint = (
            f"/repos/{REPOSITORY}/rulesets?includes_parents=true&per_page=100"
        )
        state[rulesets_endpoint] = [
            ruleset
            for ruleset in state[rulesets_endpoint]
            if ruleset["name"] != "v1.3.5-stopped-tags-never-create"
        ]
        self.assert_rejected(
            state,
            "exactly the consolidated controlled-creation plus v1.3.5 and v1.3.6 stop rulesets may govern release-tag creation",
        )

        state = copy.deepcopy(self.state)
        state[f"/repos/{REPOSITORY}/rulesets/104?includes_parents=true"][
            "bypass_actors"
        ] = [
            {
                "actor_id": RELEASE_ACTOR_ID,
                "actor_type": "User",
                "bypass_mode": "always",
            }
        ]
        self.assert_rejected(
            state,
            "v1.3.6 stopped-tag creation must be blocked by the exact no-bypass ruleset",
        )

        state = copy.deepcopy(self.state)
        state[f"/repos/{REPOSITORY}/rulesets/104?includes_parents=true"][
            "conditions"
        ]["ref_name"]["include"].remove("refs/tags/docs/v1.3.6")
        self.assert_rejected(
            state,
            "v1.3.6 stopped-tag creation must be blocked by the exact no-bypass ruleset",
        )

        state = copy.deepcopy(self.state)
        state[f"/repos/{REPOSITORY}/rulesets/101?includes_parents=true"][
            "bypass_actors"
        ][0]["actor_id"] = 999
        self.assert_rejected(
            state,
            "Root, component, and Docs creation authority must belong only to the explicit release actor",
        )

        state = copy.deepcopy(self.state)
        state[f"/repos/{REPOSITORY}/rulesets/102?includes_parents=true"][
            "bypass_actors"
        ] = [
            {
                "actor_id": RELEASE_ACTOR_ID,
                "actor_type": "User",
                "bypass_mode": "always",
            }
        ]
        self.assert_rejected(
            state,
            "v1.3.5 stopped-tag creation must be blocked by the exact no-bypass ruleset",
        )

        state = copy.deepcopy(self.state)
        state[f"/repos/{REPOSITORY}/rulesets/102?includes_parents=true"][
            "conditions"
        ]["ref_name"]["include"].remove("refs/tags/docs/v1.3.5")
        self.assert_rejected(
            state,
            "v1.3.5 stopped-tag creation must be blocked by the exact no-bypass ruleset",
        )

        state = copy.deepcopy(self.state)
        state[f"/repos/{REPOSITORY}/rulesets/103?includes_parents=true"][
            "bypass_actors"
        ] = [
            {
                "actor_id": RELEASE_ACTOR_ID,
                "actor_type": "User",
                "bypass_mode": "always",
            }
        ]
        self.assert_rejected(
            state,
            "release tag immutability must cover every release tag with no bypass",
        )


if __name__ == "__main__":
    unittest.main()
