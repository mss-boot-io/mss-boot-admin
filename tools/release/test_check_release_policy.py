import importlib.util
import re
import subprocess
import tempfile
import unittest
from pathlib import Path

import yaml


MODULE_PATH = Path(__file__).with_name("check_release_policy.py")
SPEC = importlib.util.spec_from_file_location("check_release_policy", MODULE_PATH)
assert SPEC and SPEC.loader
POLICY = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(POLICY)


REPOSITORY_ROOT = Path(__file__).resolve().parents[2]
POLICY_PATH = REPOSITORY_ROOT / ".mss" / "release-policy.yaml"
GITHUB_EXPRESSION = re.compile(r"\$\{\{.*?\}\}", re.DOTALL)


class ReleasePolicyTest(unittest.TestCase):
    def setUp(self):
        self.policy = POLICY.load_policy(POLICY_PATH)

    def test_v135_and_v136_trains_are_machine_readable_and_permanently_stopped(self):
        trains = POLICY.immutable_stopped_trains(self.policy)
        self.assertEqual(trains, POLICY.PERMANENTLY_STOPPED_TRAINS)
        self.assertEqual(self.policy["releaseTargetState"], "active")
        self.assertEqual(self.policy["nextPublicVersion"], "v1.3.7")

        for version, train in trains.items():
            self.assertRegex(train["commit"], r"^[0-9a-f]{40}$")
            for component, public_ref in train["refs"].items():
                for intent in ("qualify", "publish"):
                    with self.subTest(
                        version=version, component=component, intent=intent
                    ):
                        with self.assertRaisesRegex(
                            POLICY.PolicyError, "immutable stopped train"
                        ):
                            POLICY.check_public_ref(
                                self.policy,
                                component,
                                version,
                                public_ref,
                                intent=intent,
                            )

            with self.assertRaisesRegex(POLICY.PolicyError, "immutable stopped train"):
                POLICY.coordinated_tags(self.policy, version)

    def test_v137_can_qualify_and_publish_after_workflow_governance_is_ready(self):
        self.assertIs(self.policy["publicationWorkflowsReady"], True)
        self.assertIs(self.policy["publicPrereleases"], False)
        expected_refs = {
            "root": "v1.3.7",
            "framework": "mss-boot/v1.3.7",
            "admin": "admin/v1.3.7",
            "frontend": "web/antd-v6/v1.3.7",
            "docs": "docs/v1.3.7",
            "npm": "@mss-boot-io/admin-web@1.3.7",
        }
        for component, public_ref in expected_refs.items():
            with self.subTest(component=component, intent="qualify"):
                POLICY.check_public_ref(
                    self.policy,
                    component,
                    "v1.3.7",
                    public_ref,
                    intent="qualify",
                )
            with self.subTest(component=component, intent="publish"):
                POLICY.check_public_ref(
                    self.policy,
                    component,
                    "v1.3.7",
                    public_ref,
                    intent="publish",
                )

    def test_publication_switch_remains_fail_closed(self):
        original = POLICY_PATH.read_text(encoding="utf-8")
        with tempfile.TemporaryDirectory() as directory:
            candidate = Path(directory) / "policy.yaml"
            candidate.write_text(
                original.replace(
                    "  publicationWorkflowsReady: true\n",
                    "  publicationWorkflowsReady: false\n",
                ),
                encoding="utf-8",
            )
            disabled = POLICY.load_policy(candidate)
            POLICY.check_public_ref(
                disabled,
                "root",
                "v1.3.7",
                "v1.3.7",
                intent="qualify",
            )
            with self.assertRaisesRegex(POLICY.PolicyError, "remain disabled"):
                POLICY.check_public_ref(
                    disabled,
                    "root",
                    "v1.3.7",
                    "v1.3.7",
                    intent="publish",
                )

    def test_stable_promotion_requires_reviewed_exact_commit(self):
        release_commit = "77b53d41092741eac62fa6418c0bdbf87413c7cd"
        previous_stable_commit = "635fbb03a82976941e527d8ac1000fec0624abac"
        self.assertEqual(self.policy["currentStableVersion"], "v1.3.7")
        self.assertEqual(self.policy["currentStableCommit"], release_commit)
        self.assertIs(self.policy["stablePromotionReady"], False)
        self.assertEqual(self.policy["stablePromotionVersion"], "v1.3.7")
        self.assertEqual(self.policy["stablePromotionCommit"], "disabled")
        with self.assertRaisesRegex(POLICY.PolicyError, "promotion remains disabled"):
            POLICY.check_public_ref(
                self.policy,
                "npm",
                "v1.3.7",
                "@mss-boot-io/admin-web@1.3.7",
                intent="promote",
                commit=release_commit,
            )

        original = POLICY_PATH.read_text(encoding="utf-8")
        with tempfile.TemporaryDirectory() as directory:
            candidate = Path(directory) / "policy.yaml"
            candidate.write_text(
                original.replace(
                    "  currentStableVersion: v1.3.7\n",
                    "  currentStableVersion: v1.3.2\n",
                )
                .replace(
                    f"  currentStableCommit: {release_commit}\n",
                    f"  currentStableCommit: {previous_stable_commit}\n",
                )
                .replace(
                    "  stablePromotionReady: false\n",
                    "  stablePromotionReady: true\n",
                )
                .replace(
                    "  stablePromotionCommit: disabled\n",
                    f"  stablePromotionCommit: {release_commit}\n",
                ),
                encoding="utf-8",
            )
            promotion = POLICY.load_policy(candidate)
            for component, public_ref in (
                ("root", "v1.3.7"),
                ("npm", "@mss-boot-io/admin-web@1.3.7"),
            ):
                POLICY.check_public_ref(
                    promotion,
                    component,
                    "v1.3.7",
                    public_ref,
                    intent="promote",
                    commit=release_commit,
                )
            with self.assertRaisesRegex(POLICY.PolicyError, "exact release commit"):
                POLICY.check_public_ref(
                    promotion,
                    "npm",
                    "v1.3.7",
                    "@mss-boot-io/admin-web@1.3.7",
                    intent="promote",
                    commit="b" * 40,
                )

    def test_consumed_stable_promotion_authorization_cannot_be_reopened(self):
        release_commit = "77b53d41092741eac62fa6418c0bdbf87413c7cd"
        original = POLICY_PATH.read_text(encoding="utf-8")
        with tempfile.TemporaryDirectory() as directory:
            candidate = Path(directory) / "policy.yaml"
            candidate.write_text(
                original.replace(
                    "  stablePromotionReady: false\n",
                    "  stablePromotionReady: true\n",
                ).replace(
                    "  stablePromotionCommit: disabled\n",
                    f"  stablePromotionCommit: {release_commit}\n",
                ),
                encoding="utf-8",
            )
            with self.assertRaisesRegex(
                POLICY.PolicyError, "already consumed"
            ):
                POLICY.load_policy(candidate)

        reopened = dict(self.policy)
        reopened["stablePromotionReady"] = True
        reopened["stablePromotionCommit"] = release_commit
        with self.assertRaisesRegex(POLICY.PolicyError, "already consumed"):
            POLICY.check_public_ref(
                reopened,
                "npm",
                "v1.3.7",
                "@mss-boot-io/admin-web@1.3.7",
                intent="promote",
                commit=release_commit,
            )

    def test_current_stable_docs_tag_is_replaceable_across_merged_commits(self):
        self.assertIs(self.policy["docsTagMutable"], True)
        for commit in ("c" * 40, "d" * 40):
            for intent in ("qualify", "publish"):
                with self.subTest(commit=commit, intent=intent):
                    POLICY.check_public_ref(
                        self.policy,
                        "docs",
                        "v1.3.7",
                        "docs/v1.3.7",
                        intent=intent,
                        commit=commit,
                    )

    def test_docs_mutable_control_is_required_and_cannot_be_disabled(self):
        original = POLICY_PATH.read_text(encoding="utf-8")
        invalid_cases = (
            original.replace("  docsTagMutable: true\n", ""),
            original.replace("  docsTagMutable: true\n", "  docsTagMutable: false\n"),
        )
        for candidate_text in invalid_cases:
            with self.subTest(candidate=candidate_text.splitlines()[-6:]):
                with tempfile.TemporaryDirectory() as directory:
                    candidate = Path(directory) / "policy.yaml"
                    candidate.write_text(candidate_text, encoding="utf-8")
                    with self.assertRaises(POLICY.PolicyError):
                        POLICY.load_policy(candidate)

    def test_docs_tag_is_confined_to_current_stable_and_docs_namespace(self):
        cases = (
            ("docs", "v1.3.2", "docs/v1.3.2", "reviewed target"),
            ("docs", "v1.3.7+docs.1", "docs/v1.3.7+docs.1", "invalid"),
            ("docs", "v1.3.7+other.1", "docs/v1.3.7+other.1", "invalid"),
            ("root", "v1.3.7+docs.1", "v1.3.7+docs.1", "invalid"),
            ("docs", "v1.3.7", "docs/v1.3.8", "does not match"),
        )
        for component, version, tag, message in cases:
            with self.subTest(component=component, version=version, tag=tag):
                with self.assertRaisesRegex(POLICY.PolicyError, message):
                    POLICY.check_public_ref(
                        self.policy, component, version, tag, intent="qualify"
                    )

    def test_docs_publication_is_independent_of_core_publication_switch(self):
        disabled = dict(self.policy)
        disabled["publicationWorkflowsReady"] = False
        POLICY.check_public_ref(
            disabled,
            "docs",
            "v1.3.7",
            "docs/v1.3.7",
            intent="publish",
            commit="e" * 40,
        )

    def test_policy_requires_pr_merged_main_release_source(self):
        self.assertEqual(self.policy["releaseBranch"], "main")
        self.assertIs(self.policy["requireMergedPullRequestSource"], True)

    def test_admin_public_framework_dependency_matches_the_coordinated_target(self):
        admin_mod = (REPOSITORY_ROOT / "admin" / "go.mod").read_text(
            encoding="utf-8"
        )
        self.assertIn(
            "github.com/mss-boot-io/mss-boot-admin/mss-boot v1.3.7",
            admin_mod,
        )
        self.assertNotIn(
            "replace github.com/mss-boot-io/mss-boot-admin/mss-boot",
            admin_mod,
        )
        workspace = (REPOSITORY_ROOT / "go.work").read_text(encoding="utf-8")
        self.assertIn("\t./mss-boot", workspace)
        self.assertIn(
            "replace github.com/mss-boot-io/mss-boot-admin/mss-boot v1.3.7 => ./mss-boot",
            workspace,
        )

        admin_sum = (REPOSITORY_ROOT / "admin" / "go.sum").read_text(
            encoding="utf-8"
        )
        module_lines = [
            line
            for line in admin_sum.splitlines()
            if line.startswith(
                "github.com/mss-boot-io/mss-boot-admin/mss-boot v1.3.7"
            )
        ]
        self.assertEqual(len(module_lines), 2)
        self.assertTrue(all(line.split()[-1].startswith("h1:") for line in module_lines))

    def test_policy_rejects_versions_other_than_v137(self):
        for version in (
            "v1.0.1",
            "v1.1.0",
            "v1.2.0",
            "v1.2.1",
            "v1.2.2",
            "v1.2.3",
            "v1.3.0-rc.6",
            "v1.3.0",
            "v1.3.1",
            "v1.3.2",
            "v1.3.3",
            "v1.3.4",
            "v1.3.5",
            "v1.3.6",
        ):
            with self.subTest(version=version):
                with self.assertRaisesRegex(POLICY.PolicyError, "forbidden"):
                    POLICY.check_public_ref(
                        self.policy, "root", version, version, intent="qualify"
                    )

    def test_policy_rejects_other_prereleases_and_wrong_namespace(self):
        with self.assertRaises(POLICY.PolicyError):
            POLICY.check_public_ref(
                self.policy,
                "root",
                "v1.3.7-rc.1",
                "v1.3.7-rc.1",
                intent="qualify",
            )
        with self.assertRaisesRegex(POLICY.PolicyError, "does not match"):
            POLICY.check_public_ref(
                self.policy,
                "framework",
                "v1.3.7",
                "v1.3.7",
                intent="qualify",
            )

    def test_policy_rejects_distribution_version_or_component_drift(self):
        original = POLICY_PATH.read_text(encoding="utf-8")
        replacements = (
            ("  distributionVersion: v1.3.7\n", "  distributionVersion: v1.3.8\n"),
            (
                '  distributionComponents: "root,framework,admin,frontend"\n',
                '  distributionComponents: "root,framework,frontend"\n',
            ),
        )
        for old, new in replacements:
            with self.subTest(replacement=new.strip()):
                with tempfile.TemporaryDirectory() as directory:
                    candidate = Path(directory) / "policy.yaml"
                    candidate.write_text(original.replace(old, new), encoding="utf-8")
                    with self.assertRaises(POLICY.PolicyError):
                        POLICY.load_policy(candidate)

    def test_policy_rejects_weakening_any_permanent_stopped_train(self):
        original = POLICY_PATH.read_text(encoding="utf-8")
        replacements = (
            (
                "  releaseTargetState: active\n",
                "  releaseTargetState: stopped\n",
            ),
            (
                "      commit: 396f60615cdfa589353b16ef9d3531e249e65432\n",
                "      commit: 396f60615cdfa589353b16ef9d3531e249e65430\n",
            ),
            (
                "        docs: docs/v1.3.5\n",
                "",
            ),
            (
                "        framework: mss-boot/v1.3.6\n",
                "        framework: mss-boot/v1.3.5\n",
            ),
            (
                "      commit: b1fe47a3a83209574e09d53526b122dd2cbc5277\n",
                "      commit: b1fe47a3\n",
            ),
            (
                "    - version: v1.3.6\n",
                "    - version: v1.3.5\n",
            ),
            (
                "      commit: b1fe47a3a83209574e09d53526b122dd2cbc5277\n",
                "      commit: b1fe47a3a83209574e09d53526b122dd2cbc5277\n"
                "      unsupported: true\n",
            ),
            (
                "        docs: docs/v1.3.6\n",
                "        docs: docs/v1.3.6\n        docs: docs/v1.3.6\n",
            ),
        )
        for old, new in replacements:
            with self.subTest(replacement=new.strip()):
                with tempfile.TemporaryDirectory() as directory:
                    candidate = Path(directory) / "policy.yaml"
                    candidate.write_text(original.replace(old, new), encoding="utf-8")
                    with self.assertRaises(POLICY.PolicyError):
                        POLICY.load_policy(candidate)

        v136 = original.index("    - version: v1.3.6\n")
        end = original.index("  publicationWorkflowsReady:", v136)
        with tempfile.TemporaryDirectory() as directory:
            candidate = Path(directory) / "policy.yaml"
            candidate.write_text(original[:v136] + original[end:], encoding="utf-8")
            with self.assertRaisesRegex(POLICY.PolicyError, "exact v1.3.6"):
                POLICY.load_policy(candidate)

    def test_policy_rejects_invalid_release_channel_contracts(self):
        original = POLICY_PATH.read_text(encoding="utf-8")
        replacements = (
            ("  publicPrereleases: false\n", "  publicPrereleases: true\n"),
            ("  nextPublicVersion: v1.3.7\n", "  nextPublicVersion: v1.3.7-rc.01\n"),
            ("  currentStableVersion: v1.3.7\n", "  currentStableVersion: v1.3.7-rc.1\n"),
        )
        for old, new in replacements:
            with self.subTest(replacement=new.strip()):
                with tempfile.TemporaryDirectory() as directory:
                    candidate = Path(directory) / "policy.yaml"
                    content = original.replace(old, new)
                    if "nextPublicVersion" in new:
                        content = content.replace(
                            "  distributionVersion: v1.3.7\n",
                            "  distributionVersion: v1.3.7-rc.01\n",
                        )
                    candidate.write_text(content, encoding="utf-8")
                    with self.assertRaises(POLICY.PolicyError):
                        POLICY.load_policy(candidate)

    def test_policy_parser_rejects_unknown_or_duplicate_keys(self):
        original = POLICY_PATH.read_text(encoding="utf-8")
        for suffix in (
            "  unexpected: true\n",
            "  nextPublicVersion: v1.3.7\n",
        ):
            with self.subTest(suffix=suffix.strip()):
                with tempfile.TemporaryDirectory() as directory:
                    candidate = Path(directory) / "policy.yaml"
                    candidate.write_text(original + suffix, encoding="utf-8")
                    with self.assertRaises(POLICY.PolicyError):
                        POLICY.load_policy(candidate)

    def test_policy_parser_rejects_weakened_release_source_governance(self):
        original = POLICY_PATH.read_text(encoding="utf-8")
        replacements = (
            ("  releaseBranch: main\n", "  releaseBranch: release\n"),
            (
                "  requireMergedPullRequestSource: true\n",
                "  requireMergedPullRequestSource: false\n",
            ),
        )
        for old, new in replacements:
            with self.subTest(replacement=new.strip()):
                with tempfile.TemporaryDirectory() as directory:
                    candidate = Path(directory) / "policy.yaml"
                    candidate.write_text(original.replace(old, new), encoding="utf-8")
                    with self.assertRaises(POLICY.PolicyError):
                        POLICY.load_policy(candidate)

    def test_publication_workflows_share_policy_without_readiness_run_gates(self):
        workflows = (
            "release.yml",
            "framework-release.yml",
            "admin-release.yml",
            "frontend-v6-release.yml",
            "container.yml",
        )
        for workflow in workflows:
            with self.subTest(workflow=workflow):
                content = (
                    REPOSITORY_ROOT / ".github" / "workflows" / workflow
                ).read_text(encoding="utf-8")
                self.assertIn("check_release_policy.py", content)
                self.assertNotIn("verify_readiness_run.sh", content)
                self.assertNotIn("RELEASE_READINESS_RUN_ID", content)
                self.assertNotIn("readiness_run_id", content)

    def test_current_release_contract_uses_one_exact_main_complete_qualification(self):
        skill = (
            REPOSITORY_ROOT / ".agents" / "skills" / "mss-release" / "SKILL.md"
        ).read_text(encoding="utf-8")
        feature = (
            REPOSITORY_ROOT / ".mss" / "features" / "foundation-v1-3-7-release.yaml"
        ).read_text(encoding="utf-8")
        checklist = (
            REPOSITORY_ROOT
            / "docs"
            / "docs"
            / "admin"
            / "release-verification-checklist.md"
        ).read_text(encoding="utf-8")
        candidate = (
            REPOSITORY_ROOT / "docs" / "docs" / "releases" / "v1-3-7.md"
        ).read_text(encoding="utf-8")
        verification = (
            REPOSITORY_ROOT
            / "docs"
            / "docs"
            / "agent"
            / "verification-and-evals.md"
        ).read_text(encoding="utf-8")
        agent_contract = (REPOSITORY_ROOT / "AGENTS.md").read_text(
            encoding="utf-8"
        )

        documents = {
            "skill": skill,
            "feature": feature,
            "checklist": checklist,
            "candidate": candidate,
            "verification": verification,
        }
        for name, content in documents.items():
            with self.subTest(document=name):
                self.assertIn("verify --changed", content)

        complete_commands = {
            "skill": "go run ./cmd/mss verify --all \\\n  --release-evidence",
            "feature": "mss verify --all --release-evidence",
            "checklist": "mss verify --all --release-evidence",
            "candidate": "make verify-release-evidence",
            "verification": "mss verify --all --release-evidence",
        }
        for name, command in complete_commands.items():
            with self.subTest(document=name, complete_command=command):
                self.assertIn(command, documents[name])

        self.assertEqual(skill.count("go run ./cmd/mss verify --all"), 1)
        self.assertNotIn("pull-request Head passes mss verify --all", feature)
        self.assertNotIn("phase: feature-freeze", feature)
        self.assertNotIn("feature-freeze Admin qualification", agent_contract)

        publication_workflows = (
            "release.yml",
            "framework-release.yml",
            "admin-release.yml",
            "frontend-v6-release.yml",
            "container.yml",
            "npm-release.yml",
            "docs.yml",
        )
        broad_commands = (
            "mss verify --all",
            "make verify-all",
            "make test-all",
            "go test ./...",
            "go test -race ./...",
            "pnpm test:ci",
            "playwright test",
            "mss eval run",
        )
        for workflow in publication_workflows:
            content = (
                REPOSITORY_ROOT / ".github" / "workflows" / workflow
            ).read_text(encoding="utf-8")
            for command in broad_commands:
                with self.subTest(workflow=workflow, command=command):
                    self.assertNotIn(command, content)

    def test_release_workflows_require_pr_merged_main_source_and_exact_tag(self):
        cases = {
            "release.yml": ("release-evidence", True),
            "framework-release.yml": ("release", True),
            "admin-release.yml": ("release", True),
            "frontend-v6-release.yml": ("release", True),
            "container.yml": ("publish", True),
            "docs.yml": ("build", True),
            "npm-release.yml": ("publish", True),
        }
        for workflow_name, (job_name, requires_tag) in cases.items():
            with self.subTest(workflow=workflow_name):
                content = (
                    REPOSITORY_ROOT / ".github" / "workflows" / workflow_name
                ).read_text(encoding="utf-8")
                workflow = yaml.load(content, Loader=yaml.BaseLoader)
                job = workflow["jobs"][job_name]
                permissions = job.get("permissions", workflow.get("permissions", {}))
                self.assertEqual(permissions.get("pull-requests"), "read")
                guard_index, guard = next(
                    (index, step)
                    for index, step in enumerate(job["steps"])
                    if step.get("name") == "Verify merged-main release source"
                )
                self.assertIn("verify_release_source.py", guard["run"])
                if requires_tag:
                    self.assertIn("--tag", guard["run"])
                else:
                    self.assertNotIn("--tag", guard["run"])
                self.assertLess(guard_index, len(job["steps"]) - 1)

        root_release = yaml.load(
            (
                REPOSITORY_ROOT / ".github" / "workflows" / "release.yml"
            ).read_text(encoding="utf-8"),
            Loader=yaml.BaseLoader,
        )
        self.assertIn(
            "release-evidence", root_release["jobs"]["publish"]["needs"]
        )

    def test_release_workflow_yaml_and_run_blocks_are_valid(self):
        workflows = (
            "release.yml",
            "framework-release.yml",
            "admin-release.yml",
            "admin-distribution-compatibility.yml",
            "frontend-v6-release.yml",
            "frontend-v6-ci.yml",
            "container.yml",
            "docs.yml",
            "npm-release.yml",
        )
        for workflow_name in workflows:
            with self.subTest(workflow=workflow_name):
                content = (
                    REPOSITORY_ROOT / ".github" / "workflows" / workflow_name
                ).read_text(encoding="utf-8")
                workflow = yaml.load(content, Loader=yaml.BaseLoader)
                self.assertIsInstance(workflow.get("jobs"), dict)
                for job in workflow["jobs"].values():
                    for step in job.get("steps", []):
                        script = step.get("run")
                        if script is None:
                            continue
                        result = subprocess.run(
                            ["bash", "-n"],
                            input=GITHUB_EXPRESSION.sub("gha_expression", script),
                            text=True,
                            capture_output=True,
                            check=False,
                        )
                        self.assertEqual(
                            result.returncode,
                            0,
                            msg=(
                                f"invalid bash in {workflow_name} step "
                                f"{step.get('name')}: {result.stderr}"
                            ),
                        )

    def test_frontend_preview_upload_is_portable_and_tag_reuses_it(self):
        preview_workflow = yaml.load(
            (REPOSITORY_ROOT / ".github" / "workflows" / "release.yml").read_text(
                encoding="utf-8"
            ),
            Loader=yaml.BaseLoader,
        )
        preview_steps = preview_workflow["jobs"]["frontend-build"]["steps"]
        package = next(
            step
            for step in preview_steps
            if step.get("name") == "Package and verify portable primary V6 artifact"
        )
        upload = next(
            step
            for step in preview_steps
            if step.get("name") == "Upload primary V6 artifact"
        )

        for required in (
            "tar",
            "--create",
            "--gzip",
            "--file dist-v6.tar.gz",
            "dist",
            "--sort=name",
            'sha256sum --check SHA256SUMS.frontend-v6',
            "check_portable_paths.py",
        ):
            self.assertIn(required, package["run"])

        expected_paths = [
            "web/antd-v6/dist-v6.tar.gz",
            "web/antd-v6/FRONTEND-V6-BUILD-INFO",
            "web/antd-v6/SHA256SUMS.frontend-v6",
            "web/antd-v6/admin-web-release-package/*.tgz",
            "web/antd-v6/admin-web-release-package/admin-web-package.json",
            "web/antd-v6/admin-web-release-package/admin-web.spdx.json",
            "web/antd-v6/admin-web-release-package/SHA256SUMS.admin-web",
            "web/antd-v6/frontend-v6-image.oci.tar",
            "web/antd-v6/FRONTEND-V6-IMAGE-INFO",
            "web/antd-v6/SHA256SUMS.frontend-v6-image",
        ]
        paths = [
            line.strip()
            for line in upload["with"]["path"].splitlines()
            if line.strip()
        ]
        self.assertEqual(paths, expected_paths)

        tag_workflow = yaml.load(
            (
                REPOSITORY_ROOT
                / ".github"
                / "workflows"
                / "frontend-v6-release.yml"
            ).read_text(encoding="utf-8"),
            Loader=yaml.BaseLoader,
        )
        tag_steps = tag_workflow["jobs"]["release"]["steps"]
        download = next(
            step
            for step in tag_steps
            if step.get("name")
            == "Download exact Frontend package from the successful preview"
        )
        self.assertEqual(download["with"]["name"], "frontend-v6-dist")
        self.assertEqual(
            download["with"]["run-id"],
            "${{ steps.preview.outputs.run-id }}",
        )
        stage = next(
            step
            for step in tag_steps
            if step.get("name")
            == "Verify and stage the exact Frontend preview package"
        )["run"]
        for required in (
            'sha256sum --check SHA256SUMS.frontend-v6',
            'sha256sum --check SHA256SUMS.admin-web',
            'sha256sum --check SHA256SUMS.frontend-v6-image',
            'tar -xOf "${preview_dir}/dist-v6.tar.gz" dist/release.json',
            'tar -xOf "${tarball}" package/package.json',
            'artifact.get("integrity") == expected_integrity',
            '"blobs/sha256/${image_manifest_hash}"',
            'cp -a "${preview_package_dir}/." "${package_dir}/"',
        ):
            self.assertIn(required, stage)
        for forbidden in (
            "check_portable_paths.py",
            "verify_admin_web_package.py",
            "generate_admin_web_sbom.py",
            '.spdxVersion == "SPDX-2.3"',
            "tar --extract",
        ):
            self.assertNotIn(forbidden, stage)
        tag_content = (
            REPOSITORY_ROOT
            / ".github"
            / "workflows"
            / "frontend-v6-release.yml"
        ).read_text(encoding="utf-8")
        for forbidden in (
            "docker/build-push-action@",
            "docker/setup-buildx-action@",
            "docker/setup-qemu-action@",
            "docker buildx",
        ):
            self.assertNotIn(forbidden, tag_content)
        self.assertIn("skopeo copy", tag_content)
        self.assertIn("--preserve-digests", tag_content)
        self.assertFalse(
            any(
                "actions/upload-artifact@" in step.get("uses", "")
                for step in tag_steps
            )
        )

    def test_all_raw_directory_uploads_have_portability_guards(self):
        extensionless_files = {
            "FRONTEND-V6-BUILD-INFO",
            "FRONTEND-V6-IMAGE-INFO",
            "SHA256SUMS",
        }
        raw_uploads = []
        for workflow_path in sorted(
            (REPOSITORY_ROOT / ".github" / "workflows").glob("*.yml")
        ):
            workflow = yaml.load(
                workflow_path.read_text(encoding="utf-8"), Loader=yaml.BaseLoader
            )
            for job_name, job in workflow.get("jobs", {}).items():
                steps = job.get("steps", [])
                for upload_index, step in enumerate(steps):
                    if "actions/upload-artifact@" not in step.get("uses", ""):
                        continue
                    paths = [
                        line.strip()
                        for line in step.get("with", {}).get("path", "").splitlines()
                        if line.strip()
                    ]
                    for upload_path in paths:
                        basename = upload_path.rsplit("/", 1)[-1]
                        portable_basename = GITHUB_EXPRESSION.sub(
                            "expression", basename
                        )
                        is_file = (
                            "." in portable_basename
                            or portable_basename in extensionless_files
                            or portable_basename.startswith("SHA256SUMS")
                        )
                        if is_file:
                            continue
                        guard = next(
                            (
                                previous
                                for previous in reversed(steps[:upload_index])
                                if "check_portable_paths.py" in previous.get("run", "")
                            ),
                            None,
                        )
                        self.assertIsNotNone(
                            guard,
                            msg=(
                                f"{workflow_path.name}:{job_name}:{step.get('name')} "
                                f"uploads raw directory {upload_path} without a portability guard"
                            ),
                        )
                        self.assertIn(basename, guard["run"])
                        raw_uploads.append(
                            (workflow_path.name, job_name, upload_path)
                        )

        self.assertEqual(
            raw_uploads,
            [
                (
                    "frontend-v6-ci.yml",
                    "browser",
                    "web/antd-v6/playwright-report",
                ),
                ("frontend-v6-ci.yml", "browser", "web/antd-v6/test-results"),
                (
                    "release.yml",
                    "backend-build",
                    "mss-boot-admin-${{ matrix.os }}-${{ matrix.arch }}",
                ),
            ],
        )

    def test_root_release_validates_extracted_frontend_and_every_final_zip(self):
        content = (
            REPOSITORY_ROOT / ".github" / "workflows" / "release.yml"
        ).read_text(encoding="utf-8")
        workflow = yaml.load(content, Loader=yaml.BaseLoader)
        assemble = next(
            step
            for step in workflow["jobs"]["assemble"]["steps"]
            if step.get("name") == "Assemble release packages"
        )
        script = assemble["run"]
        self.assertIn("sha256sum --check SHA256SUMS.frontend-v6", script)
        self.assertIn("frontend-v6-extracted/dist", script)
        self.assertIn(
            "check_portable_paths.py mss-boot-admin-*.zip", script
        )

    def test_static_release_builds_normalize_dynamic_route_placeholders(self):
        frontend_package = (
            REPOSITORY_ROOT / "web" / "antd-v6" / "package.json"
        ).read_text(encoding="utf-8")
        docs_package = (REPOSITORY_ROOT / "docs" / "package.json").read_text(
            encoding="utf-8"
        )
        self.assertIn("prepare_portable_frontend.py dist", frontend_package)
        self.assertIn(
            "prepare_portable_frontend.py dist --markdown-root docs", docs_package
        )

    def test_manual_frontend_qualification_and_docs_cover_portability_tooling(self):
        frontend_content = (
            REPOSITORY_ROOT / ".github" / "workflows" / "frontend-v6-ci.yml"
        ).read_text(encoding="utf-8")
        frontend = yaml.load(frontend_content, Loader=yaml.BaseLoader)
        self.assertEqual(set(frontend["on"]), {"workflow_dispatch"})
        compile_steps = frontend["jobs"]["compile"]["steps"]
        self.assertTrue(
            any(step.get("run") == "pnpm run build:release" for step in compile_steps)
        )

        docs_content = (
            REPOSITORY_ROOT / ".github" / "workflows" / "docs.yml"
        ).read_text(encoding="utf-8")
        docs = yaml.load(docs_content, Loader=yaml.BaseLoader)
        self.assertNotIn("pull_request", docs["on"])
        self.assertIn("tools/release/**", docs["on"]["push"]["paths"])

    def test_docs_release_is_component_scoped_and_merged_main_only(self):
        content = (
            REPOSITORY_ROOT / ".github" / "workflows" / "docs.yml"
        ).read_text(encoding="utf-8")
        workflow = yaml.load(content, Loader=yaml.BaseLoader)
        build_steps = workflow["jobs"]["build"]["steps"]
        deployment = workflow["jobs"]["deployment"]

        self.assertIn("docs/v*.*.*", content)
        self.assertIn("refs/tags/docs/v", deployment["if"])
        self.assertNotIn("refs/heads/main", deployment["if"])
        self.assertTrue(
            any(
                "verify_release_source.py" in step.get("run", "")
                and "--tag" in step.get("run", "")
                for step in build_steps
            )
        )
        self.assertTrue(
            any(
                "check_release_policy.py" in step.get("run", "")
                and "--component docs" in step.get("run", "")
                and "--commit" in step.get("run", "")
                for step in build_steps
            )
        )
        source = next(
            step
            for step in build_steps
            if step.get("name") == "Verify merged-main release source"
        )["run"]
        self.assertIn("--source-mode", source)
        policy = next(
            step
            for step in build_steps
            if step.get("name") == "Enforce reviewed docs release target"
        )["run"]
        self.assertIn("--policy ../.mss/release-policy.yaml", policy)
        self.assertIn("--source-mode docs", source)
        for required in (
            "dist/release.json",
            "DOCS-BUILD-INFO.txt",
            "SHA256SUMS.docs",
            "gh release create",
            "https://docs.mss-boot-io.top/release.json",
        ):
            self.assertIn(required, content)

    def test_local_browser_qualification_cannot_reuse_the_development_server(self):
        playwright = (
            REPOSITORY_ROOT / "web" / "antd-v6" / "playwright.config.ts"
        ).read_text(encoding="utf-8")
        runner = (
            REPOSITORY_ROOT / "tools" / "verification" / "run-frontend-e2e.sh"
        ).read_text(encoding="utf-8")
        package = (
            REPOSITORY_ROOT / "web" / "antd-v6" / "package.json"
        ).read_text(encoding="utf-8")
        support = (
            REPOSITORY_ROOT / "web" / "antd-v6" / "e2e" / "support" / "session.ts"
        ).read_text(encoding="utf-8")
        backend = (
            REPOSITORY_ROOT
            / "web"
            / "antd-v6"
            / "scripts"
            / "start-e2e-backend.sh"
        ).read_text(encoding="utf-8")
        e2e_config = (
            REPOSITORY_ROOT / "admin" / "config" / "application-e2e.yml"
        ).read_text(encoding="utf-8")
        package_business_config = (
            REPOSITORY_ROOT / "web" / "antd-v6" / "package" / "business.cjs"
        ).read_text(encoding="utf-8")
        generated_supplier = (
            REPOSITORY_ROOT
            / "web"
            / "antd-v6"
            / "e2e"
            / "generated"
            / "supplier.spec.ts"
        ).read_text(encoding="utf-8")
        supplier_template = (
            REPOSITORY_ROOT
            / "templates"
            / "module"
            / "frontend-v6"
            / "e2e.spec.ts.tmpl"
        ).read_text(encoding="utf-8")

        self.assertNotIn("reuseExistingServer: !process.env.CI", playwright)
        self.assertEqual(playwright.count("reuseExistingServer: false"), 2)
        self.assertIn("http://127.0.0.1:18001", playwright)
        for required in (
            "MSS_V6_BACKEND_ORIGIN",
            "MSS_V6_BACKEND_PORT",
            "MSS_V6_WEB_PORT",
            "qualificationBackendOrigin",
            "qualificationWebPort",
            "corepack pnpm@10.34.5 exec max dev",
        ):
            self.assertIn(required, playwright)
        for required in (
            "flock -w 600",
            "current.bind(('127.0.0.1', 0))",
            'export MSS_V6_BACKEND_ORIGIN="http://127.0.0.1:${backend_port}"',
            'export MSS_V6_BASE_URL="http://127.0.0.1:${web_port}"',
            'export MSS_E2E_BACKEND_API_URL="${MSS_V6_BACKEND_ORIGIN}/admin/api"',
            'export MSS_V6_E2E_EVIDENCE_ROOT="${evidence_root}"',
            "corepack pnpm@10.34.5 run test:e2e",
        ):
            self.assertIn(required, runner)
        self.assertNotIn("exec playwright test", runner)
        self.assertLess(
            runner.index("playwright install chromium"),
            runner.index("read -r backend_port web_port"),
        )
        for required in (
            "MSS_V6_BACKEND_PORT",
            "MSS_V6_WEB_PORT",
            '"${runtime_dir}/config/application.yml"',
            '"${runtime_dir}/config/application-e2e.yml"',
            "MSS_V6_E2E_RUN_ID",
            'case "${run_root}/" in',
            '"${repo_dir}/.mss/run/"*)',
            'run_dir="$(realpath -m "${run_root}/${run_id}")"',
        ):
            self.assertIn(required, backend)
        self.assertIn('"start:e2e"', package)
        self.assertIn("MSS_V6_E2E=1", package)
        for required in (
            'MSS_V6_E2E_RUN_ID=baseline playwright test --grep-invert @presentation',
            'MSS_V6_E2E_RUN_ID=presentation playwright test --grep @presentation',
        ):
            self.assertIn(required, package)
        for required in (
            "MSS_V6_E2E_EVIDENCE_ROOT",
            "MSS_V6_E2E_RUN_ID",
            "outputDir",
            "outputFolder",
        ):
            self.assertIn(required, playwright)
        self.assertIn(
            "persistentCaching: !browserQualification", package_business_config
        )
        for content in (
            support,
            backend,
            e2e_config,
            generated_supplier,
            supplier_template,
        ):
            self.assertIn("18001", content)
        for content in (support, backend, generated_supplier, supplier_template):
            self.assertNotIn("http://127.0.0.1:8001", content)
        for content in (generated_supplier, supplier_template):
            self.assertIn("const authorizedMenu = page.waitForResponse", content)
            self.assertIn("{ timeout: 20_000 }", content)
            self.assertIn("toBeVisible({ timeout: 15_000 })", content)

    def test_e2e_backend_rejects_a_symlinked_run_root_outside_the_repository(self):
        source = (
            REPOSITORY_ROOT
            / "web"
            / "antd-v6"
            / "scripts"
            / "start-e2e-backend.sh"
        )
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            repository = root / "repository"
            script = repository / "web" / "antd-v6" / "scripts" / source.name
            script.parent.mkdir(parents=True)
            script.write_text(source.read_text(encoding="utf-8"), encoding="utf-8")
            run_parent = repository / ".mss" / "run"
            run_parent.mkdir(parents=True)
            outside = root / "outside"
            outside.mkdir()
            (run_parent / "antd-v6-e2e").symlink_to(
                outside, target_is_directory=True
            )

            result = subprocess.run(
                ["bash", str(script)],
                cwd=repository,
                text=True,
                capture_output=True,
                check=False,
            )

        self.assertEqual(result.returncode, 1)
        self.assertIn(
            "Refusing to use an E2E run root outside the repository",
            result.stderr,
        )

    def test_root_dispatch_is_the_only_preview_and_never_publishes(self):
        workflow_path = REPOSITORY_ROOT / ".github" / "workflows" / "release.yml"
        content = workflow_path.read_text(encoding="utf-8")
        workflow = yaml.load(content, Loader=yaml.BaseLoader)
        self.assertEqual(
            set(workflow["on"]["workflow_dispatch"]["inputs"]), {"version"}
        )
        metadata = next(
            step
            for step in workflow["jobs"]["metadata"]["steps"]
            if step.get("name") == "Resolve and validate release metadata"
        )["run"]
        self.assertIn('publish=true', metadata)
        self.assertIn('publish=false', metadata)
        self.assertNotIn("inputs.publish", content)
        self.assertFalse(
            (REPOSITORY_ROOT / ".github" / "workflows" / "release-readiness.yml").exists()
        )
        self.assertFalse(
            (REPOSITORY_ROOT / ".github" / "workflows" / "root-tag-promotion.yml").exists()
        )

    def test_development_push_does_not_publish_a_checkpoint_image(self):
        content = (
            REPOSITORY_ROOT / ".github" / "workflows" / "container.yml"
        ).read_text(encoding="utf-8")
        self.assertNotIn(
            '[[ "${GITHUB_EVENT_NAME}" == "push" ]]',
            content,
        )
        self.assertNotIn("verify_readiness_run.sh", content)
        self.assertNotIn("inputs.publish", content)

    def test_retired_release_evidence_helpers_are_absent(self):
        for relative_path in (
            "tools/release/verify_readiness_run.sh",
            "tools/release/release_readiness_attestation.py",
            "tools/release/release_phase_evidence.py",
            "tools/release/release_qualification_decision.py",
        ):
            with self.subTest(path=relative_path):
                self.assertFalse((REPOSITORY_ROOT / relative_path).exists())

    def test_root_release_has_no_published_version_default(self):
        content = (
            REPOSITORY_ROOT / ".github" / "workflows" / "release.yml"
        ).read_text(encoding="utf-8")
        self.assertNotIn("default: v1.0.0", content)


if __name__ == "__main__":
    unittest.main()
