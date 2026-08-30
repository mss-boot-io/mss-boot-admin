#!/usr/bin/env python3
"""Fail closed when current documentation drifts from reviewed release state."""

from __future__ import annotations

import re
import sys
from pathlib import Path
from typing import Iterable, NamedTuple
from urllib.parse import unquote


ROOT = Path(__file__).resolve().parents[2]
DOCS_ROOT = Path("docs/docs")
EXPECTED_DISTRIBUTION_VERSION = "v1.3.7"
CURRENT_RELEASE_PAGE = Path("docs/docs/releases/v1-3-7.md")
STOPPED_RELEASE_PAGES = (
    Path("docs/docs/releases/v1-3-5.md"),
    Path("docs/docs/releases/v1-3-6.md"),
)

CORE_CURRENT_FILES = (
    Path("README.md"),
    Path("README.zh-CN.md"),
    Path("CONTRIBUTING.md"),
    Path("MONOREPO.md"),
    Path("SECURITY.md"),
    Path("admin/README.md"),
    Path("mss-boot/README.md"),
    Path("mss-boot/README.Zh-cn.md"),
    Path("web/antd-v6/README.md"),
    Path("docs/README.md"),
    Path("docs/CONTRIBUTING.md"),
    Path("docs/docs/index.md"),
    Path("docs/docs/getting-started/index.md"),
    Path("docs/docs/getting-started/packages.md"),
    Path("docs/docs/getting-started/tooling.md"),
    Path("docs/docs/getting-started/mss-shop.md"),
    Path("docs/docs/releases/index.md"),
    CURRENT_RELEASE_PAGE,
)

CORE_LINK_FILES = CORE_CURRENT_FILES + (
    Path("CHANGELOG.md"),
    Path("mss-boot/CHANGELOG.md"),
)

ADOPTER_ONBOARDING_FILES = (
    Path("README.md"),
    Path("README.zh-CN.md"),
    Path("docs/docs/index.md"),
    Path("docs/docs/getting-started/index.md"),
    Path("docs/docs/getting-started/packages.md"),
    Path("docs/docs/getting-started/tooling.md"),
    Path("docs/docs/getting-started/mss-shop.md"),
    Path("docs/docs/releases/index.md"),
    CURRENT_RELEASE_PAGE,
)

PARTIAL_RELEASE_STATUS_FILES = ADOPTER_ONBOARDING_FILES + (
    Path("admin/README.md"),
    Path("web/antd-v6/README.md"),
    Path(
        "docs/docs/architecture/"
        "complete-admin-distribution-and-thin-business-host.zh-CN.md"
    ),
    Path("docs/docs/agent/blueprints-and-upgrades.md"),
    Path("docs/docs/admin/docker.md"),
    Path("docs/docs/admin/operations-guide.md"),
    Path("docs/docs/admin/index.md"),
    Path("docs/docs/guide/faq.md"),
    Path("docs/docs/agent/skills-and-mcp.md"),
    Path("docs/docs/architecture/agent-native-foundation.zh-CN.md"),
    Path("docs/docs/admin/mobile-h5-adaptation.md"),
    Path("docs/docs/admin/presentation-configuration.md"),
    Path("docs/docs/admin/login-troubleshooting.md"),
    Path("docs/docs/admin/local-debug.md"),
    Path("docs/docs/admin/theme-settings-precedence.md"),
    Path("docs/docs/admin/current-capabilities.md"),
    Path("docs/docs/agent/getting-started.md"),
    Path("docs/docs/admin/security-baseline.md"),
    Path("docs/docs/modules/supplier.md"),
    Path("docs/docs/guide/index.md"),
    Path("docs/docs/coding/first-contribution.md"),
    Path("mss-boot/README.md"),
    Path("mss-boot/README.Zh-cn.md"),
)

BOOTSTRAP_PASSWORD_FILES = (
    Path("README.md"),
    Path("README.zh-CN.md"),
    Path("admin/README.md"),
    Path("templates/application/README.md"),
    Path("docs/docs/getting-started/index.md"),
    Path("docs/docs/admin/local-debug.md"),
    Path("docs/docs/admin/docker.md"),
    Path("docs/docs/guide/faq.md"),
)

FIRST_LOGIN_FILES = (
    Path("README.md"),
    Path("README.zh-CN.md"),
    Path("templates/application/README.md"),
    Path("docs/docs/getting-started/index.md"),
    Path("docs/docs/admin/login-troubleshooting.md"),
    Path("docs/docs/guide/faq.md"),
)

UPGRADE_CONTRACT_FILES = (
    Path("docs/docs/getting-started/index.md"),
    Path("docs/docs/guide/faq.md"),
    Path("docs/docs/agent/blueprints-and-upgrades.md"),
    CURRENT_RELEASE_PAGE,
    Path("templates/application/README.md"),
)

MCP_CONTRACT_FILES = (
    Path("docs/docs/getting-started/tooling.md"),
    Path("docs/docs/agent/skills-and-mcp.md"),
)

PUBLIC_THIN_HOST_SKILLS = (
    "mss-thin-host",
    "mss-add-module",
    "mss-add-field",
    "mss-add-permission",
    "mss-debug-fullstack",
    "mss-review-change",
    "mss-upgrade-foundation",
)

FOUNDATION_MAINTAINER_SKILLS = (
    "mss-project-onboarding",
    "mss-new-application",
    "mss-add-module",
    "mss-add-field",
    "mss-add-permission",
    "mss-add-workflow",
    "mss-debug-fullstack",
    "mss-review-change",
    "mss-upgrade-foundation",
    "mss-release",
    "mss-update-release-docs",
)

AUDIENCE_BOUNDARY_MARKERS = {
    Path("docs/README.md"): (
        "## Audience and authority map",
        "`docs/docs/agent/`",
        "Foundation AI Agents",
        "Generated Thin Host AI Agents",
    ),
    Path("docs/docs/index.md"): (
        "## 人类文档与 Agent 合同",
        "Foundation AI Agent",
        "Thin Host AI Agent",
        "不是 Agent",
    ),
    Path("docs/docs/agent/index.md"): (
        "给人类看的公开说明",
        "Foundation 源码",
        "生成 Thin Host",
        "不是 Agent",
        "| --- | --- | --- |",
    ),
    Path("docs/.dumirc.ts"): (
        "Agent 协作",
        "外部 Supplier 示例",
    ),
}

ACTIVE_ADMIN_SCOPE_FILES = (
    Path("docs/docs/admin/config-cache-consistency.md"),
    Path("docs/docs/admin/configuration-guide.md"),
    Path("docs/docs/admin/governance-guide.md"),
    Path("docs/docs/admin/i18n-troubleshooting.md"),
    Path("docs/docs/admin/integration-test-guide.md"),
    Path("docs/docs/admin/legacy-capability-deprecation.md"),
)

EXACT_SECTION_CONTENT = {
    Path("docs/docs/getting-started"): {
        "index.md",
        "packages.md",
        "tooling.md",
        "mss-shop.md",
    },
    Path("docs/docs/guide"): {"index.md", "faq.md"},
    Path("docs/docs/coding"): {"first-contribution.md"},
    Path("docs/docs/devops"): {"security-policy-faq.md"},
    Path("docs/docs/operations"): set(),
}

REMOVED_TREES = (
    Path("aigc/prompts"),
    Path("docs/aigc"),
    Path("docs/.github"),
    Path("mss-boot/aigc"),
    Path("mss-boot/.github"),
    Path("docs/docs/aigc"),
)

REMOVED_PATHS = (
    Path("mss-boot/core/README.md"),
)

REMOVED_ADMIN_PAGES = {
    "ai-annotation-spec.md",
    "ai-annotation-templates.md",
    "ant-design-v6-migration-plan.md",
    "comprehensive-test-report.md",
    "e2e-test-plan.md",
    "extension-guardrails.md",
    "hotgo-comparison.md",
    "operations-planning.md",
    "phase-3-roadmap.md",
    "phase-4-roadmap.md",
    "phase-5-roadmap.md",
    "pre-release-checklist.md",
    "product-direction.md",
    "product-polish-governance-plan.md",
    "product-polish-remediation-round-1.md",
    "production-standardization.md",
    "quickly.md",
    "release-readiness-report.md",
    "test-cases-full.md",
    "test-execution-report.md",
    "tutorials.md",
    "ui-experience-and-static-delivery.md",
}

ALLOWED_RELEASE_ROOT = {
    "index.md",
    CURRENT_RELEASE_PAGE.name,
    *(path.name for path in STOPPED_RELEASE_PAGES),
}
REQUIRED_ARCHIVE_PAGES = {
    "index.md",
    "v1-0-0.md",
    "v1-0-0-compatibility.md",
    "v1-0-0-upgrade.md",
    "v1-0-0-rollback.md",
    "v1-2-0.md",
    "v1-2-1.md",
    "v1-2-2.md",
    "v1-2-3.md",
    "v1-3-0.md",
    "v1-3-1.md",
    "v1-3-2.md",
    "v1-3-3.md",
    "v1-3-4.md",
}
REQUIRED_ADRS = {
    "2026-08-04-admin-module-boundary-and-coverage.md",
    "2026-08-04-component-scoped-ci.md",
    "2026-08-06-admin-secret-lifecycle.md",
    "2026-08-06-remove-runtime-developer-tools-and-sample-monitoring.md",
    "2026-08-07-layered-theme-settings-precedence.md",
    "2026-08-09-storage-runtime-v2.md",
    "2026-08-15-browser-session-and-websocket-ticket.md",
    "2026-08-15-independent-ant-design-v6-application.md",
    "2026-08-16-account-reauthentication-and-credential-self-service.md",
    "2026-08-17-ant-design-v6-default-cutover.md",
    "2026-08-19-complete-admin-distribution-and-thin-business-host.md",
    "2026-08-22-admin-distribution-contract-hardening.md",
    "2026-08-24-admin-presentation-publication-workflow.md",
    "2026-08-24-governed-admin-presentation-configuration.md",
}
CONTRIBUTOR_PAGE = Path("docs/docs/coding/first-contribution.md")
ARCHIVE_PREFIX = Path("docs/docs/releases/archive")

MARKDOWN_LINK = re.compile(r"(?<!!)\[[^\]]*\]\(([^)]+)\)")
NAV_LINK = re.compile(r"\blink:\s*['\"](/[^'\"]*)['\"]")
FRONTMATTER_TITLE = re.compile(
    r"\A---\s*\n(?P<body>.*?)\n---", re.DOTALL
)
VERSION_TOKEN = re.compile(r"(?<![A-Za-z0-9])v\d+\.\d+\.\d+(?:[-+][A-Za-z0-9.-]+)?")
ADMIN_WEB_TOKEN = re.compile(r"@mss-boot-io/admin-web@(\d+\.\d+\.\d+)")
FENCED_CODE_BLOCK = re.compile(r"```[^\n]*\n.*?```", re.DOTALL)
OPERATIONAL_VERSION_LINE = re.compile(
    r"^\s*(?:[$>]\s*)?(?:mss\b|go\s+(?:get|install|mod)\b|corepack\b|"
    r"npm\b|pnpm\b|yarn\b|curl\b|bash\b|Invoke-WebRequest\b|"
    r"&\s+\.\\install-mss|docker\b)",
    re.IGNORECASE,
)
HISTORICAL_RELEASE_VERSION_REFERENCES = {
    Path("docs/docs/releases/index.md"): {"v1.3.4"},
    CURRENT_RELEASE_PAGE: {"v1.3.4"},
    STOPPED_RELEASE_PAGES[0]: {"v1.3.4"},
}
FORBIDDEN_SOURCE_COMMANDS = {
    "Foundation clone command": re.compile(r"\bgit\s+clone\b"),
    "source-only mss invocation": re.compile(
        r"\bgo\s+run(?:\s+-[^\s]+)*\s+\./cmd/mss(?:-mcp)?\b"
    ),
    "checkout-dependent upgrade": re.compile(r"(?<![A-Za-z0-9_])--foundation\b"),
    "POSIX sh invocation for Bash installer": re.compile(
        r"\bsh\s+(?:\./)?install-mss\.sh\b"
    ),
    "manual shell bootstrap password prompt": re.compile(
        r"\bread\s+-[^\n]*\bMSS_ADMIN_INITIAL_PASSWORD\b"
    ),
    "manual PowerShell bootstrap password conversion": re.compile(
        r"\[System\.Net\.NetworkCredential\]"
    ),
    "retired monolithic application config": re.compile(
        r"(?<![A-Za-z0-9_.-])config/application\.ya?ml\b"
    ),
    "obsolete HotGo comparison context": re.compile(r"\bHotGo\b", re.IGNORECASE),
    "direct SQL mutation example": re.compile(
        r"^\s*(?:INSERT\s+INTO|UPDATE\s+\S+\s+SET|DELETE\s+FROM)\b",
        re.IGNORECASE | re.MULTILINE,
    ),
    "literal credential example": re.compile(
        r"^\s*(?:password|token|secret|smtpPassword|webhookToken)\s*:\s*"
        r"(?!$|#|(?:password|token|secret)?Ref\b|env://|\$\{|\[REDACTED\]|<redacted>)"
        r"\S+",
        re.IGNORECASE | re.MULTILINE,
    ),
}

def unpublished_operational_commands(version: str) -> dict[str, re.Pattern[str]]:
    escaped_version = re.escape(version)
    escaped_npm_version = re.escape(version.removeprefix("v"))
    return {
        f"unpublished {version} installer URL": re.compile(
            r"https://github\.com/mss-boot-io/mss-boot-admin/releases/download/"
            rf"{escaped_version}/install-mss\.(?:sh|ps1)",
            re.IGNORECASE,
        ),
        "unpublished shell installer invocation": re.compile(
            rf"(?im)^(?=[^\n]*{escaped_version})[^\n]*"
            r"\b(?:bash|sh)\s+(?:\./)?install-mss\.sh\b[^\n]*$",
        ),
        "unpublished PowerShell installer invocation": re.compile(
            rf"(?im)^(?=[^\n]*{escaped_version})[^\n]*"
            r"(?:Invoke-WebRequest[^\n]*install-mss\.ps1|"
            r"&\s+\.\\install-mss\.ps1\b)[^\n]*$",
        ),
        f"unpublished {version} Admin upgrade": re.compile(
            rf"(?<![A-Za-z0-9_-])mss\s+upgrade\s+admin\s+{escaped_version}\b",
            re.IGNORECASE,
        ),
        "unpublished official npmjs install": re.compile(
            r"(?im)^\s*(?:[$>]\s*)?(?:corepack\s+)?(?:npm|pnpm|yarn|bun)\b"
            rf"[^\n]*@mss-boot-io/admin-web@{escaped_npm_version}\b",
        ),
        "unpublished Root image command": re.compile(
            r"(?im)^\s*(?:[$>]\s*)?(?:docker|podman|nerdctl)\s+(?:pull|run)\b"
            rf"[^\n]*ghcr\.io/mss-boot-io/mss-boot-admin:{escaped_version}\b",
        ),
        f"source-built {version} Root tool": re.compile(
            r"(?im)^\s*(?:[$>]\s*)?go\s+install\b[^\n]*/cmd/mss(?:-mcp)?@"
            rf"{escaped_version}\b",
        ),
    }

PARTIAL_RELEASE_STATUS_MARKER = re.compile(
    r"(?:permanently stopped|immutable[- ]partial|永久停止|不可变(?:的)?部分发布)",
    re.IGNORECASE,
)
ACTIVE_TARGET_STATUS_MARKER = re.compile(
    r"(?:release[- ]candidate|candidate|pre[- ]publication|unpublished|"
    r"active target|候选|发布前|尚未发布|未发布|当前目标)",
    re.IGNORECASE,
)
ABSOLUTE_UNPUBLISHED_CLAIMS = (
    re.compile(
        r"{version}[\s\S]{{0,120}}(?:remains?\s+unpublished|"
        r"is\s+not\s+(?:yet\s+)?published|has\s+not\s+been\s+published)",
        re.IGNORECASE,
    ),
    re.compile(
        r"(?:remains?\s+unpublished|is\s+not\s+(?:yet\s+)?published|"
        r"has\s+not\s+been\s+published)[\s\S]{{0,120}}{version}",
        re.IGNORECASE,
    ),
    re.compile(r"{version}[\s\S]{{0,80}}(?:尚未发布|还未发布|未发布|未公开)"),
    re.compile(r"(?:尚未发布|还未发布|未发布|未公开)[\s\S]{{0,80}}{version}"),
)
PARTIAL_RELEASE_BOUNDARY_MARKER = re.compile(
    r"(?:source-only|source (?:checkout|contract|development)|"
    r"future (?:complete|release|contract)|"
    r"not (?:a |an )?(?:complete|installable|adoptable|supported)|"
    r"源码(?:专用|工作区|贡献|合同|开发)|未来(?:未使用|完整|发行|合同)|"
    r"不是(?:可安装|可采用|完整|当前)|不可(?:安装|采用|升级|运行)|"
    r"不能[^\n]{0,80}(?:采用|安装|升级|发行|Thin Host))",
    re.IGNORECASE,
)
def unreconciled_adoption_claims(version: str) -> dict[str, re.Pattern[str]]:
    escaped_version = re.escape(version)
    return {
        f"{version} tool availability claim": re.compile(
            rf"(?:公共对账完成后[,，]?\s*)?{escaped_version}[^\n]{{0,100}}"
            r"(?:对外工具|工具包并报告|工具和包生成)",
            re.IGNORECASE,
        ),
        f"{version} generated Thin Host claim": re.compile(
            rf"(?:生成的|通过公开|从公开|由公开)[^\n]{{0,50}}{escaped_version}"
            r"[^\n]{0,100}(?:Thin Host|生成|mss-shop)",
            re.IGNORECASE,
        ),
        f"{version} current-capabilities heading": re.compile(
            rf"(?m)^#\s+{escaped_version}\s+当前能力(?:与边界)?\s*$",
            re.IGNORECASE,
        ),
        f"{version} embedded-tool availability claim": re.compile(
            rf"{escaped_version}\s+的\s+`?mss`?\s+二进制内置",
            re.IGNORECASE,
        ),
        f"English {version} candidate publication claim": re.compile(
            rf"\bThe\s+{escaped_version}\s+candidate\s+publishes\b",
            re.IGNORECASE,
        ),
        f"unreconciled {version} current-stable claim": re.compile(
            rf"(?:\bcurrent(?:\s+coordinated)?\s+stable(?:\s+(?:distribution|version))?"
            rf"\s*(?:is|:)\s*[*_`]*{escaped_version}\b|"
            rf"\b{escaped_version}\s+is\s+(?:the\s+)?current\s+stable\b|"
            rf"当前(?:协调)?稳定(?:版本|版|基线)?\s*(?:是|为|：|:)\s*[*_`]*{escaped_version}\b|"
            rf"{escaped_version}\s*(?:是|为)\s*当前(?:协调)?稳定(?:版本|版|基线)?)",
            re.IGNORECASE,
        ),
    }


class ReleaseDocumentationState(NamedTuple):
    distribution_version: str
    current_stable_version: str
    immutable_stopped_versions: tuple[str, ...]
    publication_workflows_ready: bool
    release_status: str

    @property
    def operational_onboarding_allowed(self) -> bool:
        return self.publication_workflows_ready and self.release_status == "stable"


def release_documentation_state(root: Path) -> ReleaseDocumentationState:
    policy = root / ".mss/release-policy.yaml"
    try:
        text = policy.read_text(encoding="utf-8")
    except OSError as exc:
        raise ValueError(f"cannot read {policy}: {exc}") from exc

    def required_value(name: str, value_pattern: str) -> str:
        match = re.search(
            rf"^\s*{re.escape(name)}:\s*['\"]?({value_pattern})['\"]?\s*$",
            text,
            re.MULTILINE,
        )
        if not match:
            raise ValueError(
                f".mss/release-policy.yaml must declare spec.{name}"
            )
        return match.group(1)

    distribution = required_value(
        "distributionVersion", r"v\d+\.\d+\.\d+"
    )
    stable = required_value(
        "currentStableVersion", r"v\d+\.\d+\.\d+"
    )

    stopped_headers = list(
        re.finditer(
            r"^(?P<indent> *)immutableStoppedTrains:\s*$",
            text,
            re.MULTILINE,
        )
    )
    if len(stopped_headers) != 1:
        raise ValueError(
            ".mss/release-policy.yaml must declare exactly one "
            "spec.immutableStoppedTrains list"
        )
    stopped_header = stopped_headers[0]
    header_indent = len(stopped_header.group("indent"))
    lines = text[stopped_header.end() :].splitlines()
    stopped_versions: list[str] = []
    item_pattern = re.compile(
        rf"^ {{{header_indent + 2}}}- version:\s*(?P<quote>['\"]?)"
        r"(?P<version>v\d+\.\d+\.\d+)(?P=quote)\s*$"
    )
    direct_item_pattern = re.compile(rf"^ {{{header_indent + 2}}}-\s")
    for line in lines:
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        if "\t" in line[: len(line) - len(line.lstrip())]:
            raise ValueError(
                ".mss/release-policy.yaml spec.immutableStoppedTrains "
                "must use space indentation"
            )
        indent = len(line) - len(line.lstrip(" "))
        if indent <= header_indent:
            break
        if indent == header_indent + 2 and not direct_item_pattern.match(line):
            raise ValueError(
                ".mss/release-policy.yaml spec.immutableStoppedTrains must be "
                "a list of version records"
            )
        if not direct_item_pattern.match(line):
            continue
        item = item_pattern.fullmatch(line)
        if not item:
            raise ValueError(
                ".mss/release-policy.yaml spec.immutableStoppedTrains entries "
                "must start with '- version: vX.Y.Z'"
            )
        stopped_versions.append(item.group("version"))
    if not stopped_versions:
        raise ValueError(
            ".mss/release-policy.yaml spec.immutableStoppedTrains must contain "
            "at least one version"
        )
    if len(set(stopped_versions)) != len(stopped_versions):
        raise ValueError(
            ".mss/release-policy.yaml spec.immutableStoppedTrains contains "
            "duplicate versions"
        )
    publication_ready_text = required_value(
        "publicationWorkflowsReady", r"true|false"
    )

    matching_features: list[tuple[Path, str]] = []
    feature_root = root / ".mss/features"
    for feature in sorted(feature_root.glob("*.yaml")):
        try:
            feature_text = feature.read_text(encoding="utf-8")
        except OSError as exc:
            raise ValueError(f"cannot read {feature}: {exc}") from exc
        target = re.search(
            r"^\s*target-version:\s*['\"]?(v\d+\.\d+\.\d+)['\"]?\s*$",
            feature_text,
            re.MULTILINE,
        )
        if not target or target.group(1) != distribution:
            continue
        status = re.search(
            r"^\s*release-status:\s*['\"]?([a-z0-9-]+)['\"]?\s*$",
            feature_text,
            re.MULTILINE,
        )
        if not status:
            raise ValueError(
                f"{feature.relative_to(root)} must declare metadata.labels.release-status"
            )
        matching_features.append((feature, status.group(1)))

    if len(matching_features) != 1:
        names = ", ".join(str(path.relative_to(root)) for path, _ in matching_features)
        raise ValueError(
            f"expected exactly one Feature for {distribution}; found "
            f"{len(matching_features)} ({names})"
        )

    return ReleaseDocumentationState(
        distribution_version=distribution,
        current_stable_version=stable,
        immutable_stopped_versions=tuple(stopped_versions),
        publication_workflows_ready=publication_ready_text == "true",
        release_status=matching_features[0][1],
    )


def distribution_version(root: Path) -> str:
    return release_documentation_state(root).distribution_version


def active_markdown_paths(root: Path) -> list[Path]:
    paths = [
        Path("README.md"),
        Path("README.zh-CN.md"),
        Path("SECURITY.md"),
        Path("admin/README.md"),
        Path("mss-boot/README.md"),
        Path("mss-boot/README.Zh-cn.md"),
        Path("web/antd-v6/README.md"),
        Path("docs/README.md"),
    ]
    docs_root = root / DOCS_ROOT
    if docs_root.is_dir():
        for absolute in sorted(docs_root.rglob("*.md")):
            relative = absolute.relative_to(root)
            if relative == CONTRIBUTOR_PAGE:
                continue
            if ARCHIVE_PREFIX in relative.parents:
                continue
            paths.append(relative)
    application_template = root / "templates/application"
    if application_template.is_dir():
        paths.extend(
            absolute.relative_to(root)
            for absolute in sorted(application_template.rglob("*.md"))
        )
    return paths


def forbidden_content_errors(
    root: Path,
    version: str,
    paths: Iterable[Path],
    *,
    current_stable_version: str | None = None,
    immutable_stopped_versions: tuple[str, ...] = (),
) -> list[str]:
    errors: list[str] = []
    npm_version = version.removeprefix("v")
    stopped_versions = frozenset(immutable_stopped_versions)
    for path in paths:
        absolute = root / path
        if not absolute.is_file():
            continue
        text = absolute.read_text(encoding="utf-8")
        fenced_ranges = [
            (match.start(), match.end())
            for match in FENCED_CODE_BLOCK.finditer(text)
        ]
        allowed_historical_versions = HISTORICAL_RELEASE_VERSION_REFERENCES.get(
            path, set()
        )

        def in_fenced_code(position: int) -> bool:
            return any(start <= position < end for start, end in fenced_ranges)

        def on_operational_line(position: int) -> bool:
            line_start = text.rfind("\n", 0, position) + 1
            line_end = text.find("\n", position)
            if line_end < 0:
                line_end = len(text)
            return OPERATIONAL_VERSION_LINE.search(text[line_start:line_end]) is not None

        for label, pattern in FORBIDDEN_SOURCE_COMMANDS.items():
            for match in pattern.finditer(text):
                line = text.count("\n", 0, match.start()) + 1
                errors.append(f"{path}:{line}: {label} is not allowed in current docs")
        for match in VERSION_TOKEN.finditer(text):
            token = match.group(0)
            if (
                token != version
                and token != current_stable_version
                and not (
                    (
                        token in allowed_historical_versions
                        or token in stopped_versions
                    )
                    and not in_fenced_code(match.start())
                    and not on_operational_line(match.start())
                )
            ):
                line = text.count("\n", 0, match.start()) + 1
                errors.append(
                    f"{path}:{line}: stale distribution token {token}; expected {version}"
                )
        for match in ADMIN_WEB_TOKEN.finditer(text):
            token = match.group(1)
            if token != npm_version and not (
                (
                    f"v{token}" in allowed_historical_versions
                    or f"v{token}" in stopped_versions
                )
                and not in_fenced_code(match.start())
                and not on_operational_line(match.start())
            ):
                line = text.count("\n", 0, match.start()) + 1
                errors.append(
                    f"{path}:{line}: stale Admin Web version {token}; expected {npm_version}"
                )
        if re.search(r"(?<![A-Za-z0-9-])mss-pr(?![A-Za-z0-9-])", text):
            errors.append(f"{path}: internal mss-pr must not be documented publicly")
    return errors


def partial_release_operational_errors(
    root: Path, state: ReleaseDocumentationState
) -> list[str]:
    if state.operational_onboarding_allowed:
        return []

    errors: list[str] = []
    for path in active_markdown_paths(root):
        absolute = root / path
        if not absolute.is_file():
            continue
        text = absolute.read_text(encoding="utf-8")
        for label, pattern in unpublished_operational_commands(
            state.distribution_version
        ).items():
            for match in pattern.finditer(text):
                line = text.count("\n", 0, match.start()) + 1
                errors.append(
                    f"{path}:{line}: {label} is forbidden while "
                    f"publicationWorkflowsReady={str(state.publication_workflows_ready).lower()} "
                    f"and release-status={state.release_status}"
                )
    return errors


def absolute_unpublished_claim_errors(
    root: Path,
    version: str,
    paths: Iterable[Path],
) -> list[str]:
    """Reject stage-sensitive claims that become false during candidate rollout."""

    errors: list[str] = []
    escaped_version = re.escape(version)
    patterns = tuple(
        re.compile(pattern.pattern.format(version=escaped_version), pattern.flags)
        for pattern in ABSOLUTE_UNPUBLISHED_CLAIMS
    )
    for path in paths:
        absolute = root / path
        if not absolute.is_file():
            continue
        text = absolute.read_text(encoding="utf-8")
        offset = 0
        for paragraph in re.split(r"(\n\s*\n)", text):
            if not paragraph or re.fullmatch(r"\n\s*\n", paragraph):
                offset += len(paragraph)
                continue
            for pattern in patterns:
                for match in pattern.finditer(paragraph):
                    line = text.count("\n", 0, offset + match.start()) + 1
                    errors.append(
                        f"{path}:{line}: {version} publication status must be stage-neutral; "
                        "candidate surfaces can become public before stable reconciliation"
                    )
            offset += len(paragraph)
    return errors


def partial_release_semantic_errors(
    root: Path,
    state: ReleaseDocumentationState,
    *,
    status_paths: Iterable[Path] = PARTIAL_RELEASE_STATUS_FILES,
    claim_paths: Iterable[Path] | None = None,
) -> list[str]:
    if state.operational_onboarding_allowed:
        return []

    errors: list[str] = []
    checked_status_paths = tuple(status_paths)
    status_marker = (
        PARTIAL_RELEASE_STATUS_MARKER
        if state.release_status == "immutable-partial"
        else ACTIVE_TARGET_STATUS_MARKER
    )
    for path in checked_status_paths:
        absolute = root / path
        if not absolute.is_file():
            errors.append(f"missing partial-release status documentation: {path}")
            continue
        text = absolute.read_text(encoding="utf-8")
        if state.distribution_version not in text:
            errors.append(
                f"{path}: partial-release status page must name "
                f"{state.distribution_version}"
            )
        if state.current_stable_version not in text:
            errors.append(
                f"{path}: partial-release status page must name current stable "
                f"{state.current_stable_version}"
            )
        for stopped_version in state.immutable_stopped_versions:
            if (
                stopped_version != state.distribution_version
                and stopped_version not in text
            ):
                errors.append(
                    f"{path}: pre-publication status page must name immutable "
                    f"stopped {stopped_version} separately from active target "
                    f"{state.distribution_version}"
                )
        if not status_marker.search(text):
            errors.append(
                f"{path}: must explicitly label {state.distribution_version} "
                f"release-status={state.release_status}"
            )
        if not PARTIAL_RELEASE_BOUNDARY_MARKER.search(text):
            errors.append(
                f"{path}: must distinguish source-only or future-contract content "
                "from current adoption"
            )
        expected_npm_version = state.distribution_version.removeprefix("v")
        for line_number, line in enumerate(text.splitlines(), start=1):
            if "candidate" not in line.casefold() and "候选" not in line:
                continue
            for match in ADMIN_WEB_TOKEN.finditer(line):
                if match.group(1) != expected_npm_version:
                    errors.append(
                        f"{path}:{line_number}: stale candidate Admin Web identity "
                        f"{match.group(1)}; expected {expected_npm_version}"
                    )

    if claim_paths is None:
        scanned_claim_paths = active_markdown_paths(root)
        if (root / CONTRIBUTOR_PAGE).is_file():
            scanned_claim_paths.append(CONTRIBUTOR_PAGE)
    else:
        scanned_claim_paths = list(claim_paths)
    for path in scanned_claim_paths:
        absolute = root / path
        if not absolute.is_file():
            continue
        text = absolute.read_text(encoding="utf-8")
        for label, pattern in unreconciled_adoption_claims(
            state.distribution_version
        ).items():
            for match in pattern.finditer(text):
                line = text.count("\n", 0, match.start()) + 1
                errors.append(
                    f"{path}:{line}: {label} is incompatible with "
                    f"release-status={state.release_status}"
                )
    return errors


def stopped_release_history_errors(
    root: Path, state: ReleaseDocumentationState
) -> list[str]:
    errors: list[str] = []
    for stopped_version in state.immutable_stopped_versions:
        path = Path(
            "docs/docs/releases/"
            f"{stopped_version.replace('.', '-')}.md"
        )
        absolute = root / path
        if not absolute.is_file():
            errors.append(f"missing immutable stopped release record: {path}")
            continue
        text = absolute.read_text(encoding="utf-8")
        if stopped_version not in text:
            errors.append(f"{path}: must name immutable stopped {stopped_version}")
        if state.current_stable_version not in text:
            errors.append(
                f"{path}: must retain rollback baseline "
                f"{state.current_stable_version}"
            )
        if not PARTIAL_RELEASE_STATUS_MARKER.search(text):
            errors.append(
                f"{path}: must retain the immutable-partial or permanently "
                "stopped boundary"
            )
    return errors


def bootstrap_password_errors(
    root: Path, paths: Iterable[Path] = BOOTSTRAP_PASSWORD_FILES
) -> list[str]:
    errors: list[str] = []
    for path in paths:
        absolute = root / path
        if not absolute.is_file():
            errors.append(f"missing bootstrap password documentation: {path}")
            continue
        text = absolute.read_text(encoding="utf-8")
        if "MSS_ADMIN_INITIAL_PASSWORD" not in text:
            errors.append(
                f"{path}: must document the one-use MSS_ADMIN_INITIAL_PASSWORD contract"
            )
        if re.search(r"(?s)\bmigrate\b.{0,240}?--password\b", text):
            errors.append(
                f"{path}: initial administrator passwords must not be passed in command arguments"
            )
    return errors


def first_login_errors(
    root: Path, paths: Iterable[Path] = FIRST_LOGIN_FILES
) -> list[str]:
    errors: list[str] = []
    for path in paths:
        absolute = root / path
        if not absolute.is_file():
            errors.append(f"missing first-login documentation: {path}")
            continue
        text = absolute.read_text(encoding="utf-8")
        if not re.search(r"(?<![A-Za-z0-9_-])`?admin`?(?![A-Za-z0-9_-])", text):
            errors.append(f"{path}: must document the initial admin username")
        if "127.0.0.1:8001" not in text:
            errors.append(f"{path}: must document the local Admin Web address")
    return errors


def upgrade_contract_errors(
    root: Path,
    paths: Iterable[Path] = UPGRADE_CONTRACT_FILES,
    *,
    version: str = EXPECTED_DISTRIBUTION_VERSION,
) -> list[str]:
    errors: list[str] = []
    required_markers = (
        "mss --version",
        "mss-mcp --version",
        ".mss/blueprint-manifest.json",
        "mss doctor --strict",
        "mss verify --all",
    )
    for path in paths:
        absolute = root / path
        if not absolute.is_file():
            errors.append(f"missing upgrade documentation: {path}")
            continue
        text = absolute.read_text(encoding="utf-8")
        for marker in required_markers:
            if marker not in text:
                errors.append(f"{path}: upgrade contract is missing {marker}")
        upgrade_commands = re.findall(
            rf"mss upgrade admin (?:{re.escape(version)}|__MSS_DISTRIBUTION_VERSION__)",
            text,
        )
        if len(upgrade_commands) < 3:
            errors.append(
                f"{path}: must show plan, apply, and final no-op upgrade commands"
            )
        if not re.search(r"(?:back\s*up|backup|备份)", text, re.IGNORECASE):
            errors.append(f"{path}: upgrade contract must require a backup")
        if not re.search(
            r"(?:hand-assembled|missing (?:its )?manifest|手工拼装|丢失 manifest|缺失)",
            text,
            re.IGNORECASE,
        ):
            errors.append(
                f"{path}: must explain the manifest-less adoption path"
            )
    return errors


def mcp_contract_errors(
    root: Path,
    paths: Iterable[Path] = MCP_CONTRACT_FILES,
    *,
    require_client_example: bool = True,
) -> list[str]:
    errors: list[str] = []
    for path in paths:
        absolute = root / path
        if not absolute.is_file():
            errors.append(f"missing MCP documentation: {path}")
            continue
        text = absolute.read_text(encoding="utf-8")
        for marker in ("stdio", "tools/list", "mss_plan_application", ".mss/project.yaml"):
            if marker not in text:
                errors.append(f"{path}: MCP contract is missing {marker}")
    tooling = root / "docs/docs/getting-started/tooling.md"
    if require_client_example and tooling.is_file():
        text = tooling.read_text(encoding="utf-8")
        for marker in ('"mcpServers"', '"command"', '"args"'):
            if marker not in text:
                errors.append(f"{tooling.relative_to(root)}: MCP client example is missing {marker}")
    return errors


def public_skill_documentation_errors(root: Path) -> list[str]:
    path = Path("docs/docs/agent/skills-and-mcp.md")
    absolute = root / path
    if not absolute.is_file():
        return [f"missing public Skill documentation: {path}"]
    text = absolute.read_text(encoding="utf-8")
    errors: list[str] = []
    foundation_heading = "## Foundation 维护者 Skills"
    thin_host_heading = "## Thin Host 分发 Skills"
    if foundation_heading not in text or thin_host_heading not in text:
        return [f"{path}: Skill documentation must separate Foundation and Thin Host sections"]
    foundation = text.split(foundation_heading, 1)[1].split(thin_host_heading, 1)[0]
    thin_host = text.split(thin_host_heading, 1)[1]
    errors.extend(
        f"{path}: Foundation maintainer Skill list is missing {name}"
        for name in FOUNDATION_MAINTAINER_SKILLS
        if f"`{name}`" not in foundation
    )
    errors.extend(
        f"{path}: Thin Host Skill list is missing {name}"
        for name in PUBLIC_THIN_HOST_SKILLS
        if f"`{name}`" not in thin_host
    )
    for forbidden in (
        "mss-project-onboarding",
        "mss-new-application",
        "mss-add-workflow",
        "mss-release",
        "mss-update-release-docs",
    ):
        if f"`{forbidden}`" in thin_host:
            errors.append(
                f"{path}: Foundation-only or unsupported Skill is listed in the Thin Host section: {forbidden}"
            )
    return errors


def documentation_audience_errors(root: Path) -> list[str]:
    errors: list[str] = []
    for relative, markers in AUDIENCE_BOUNDARY_MARKERS.items():
        path = root / relative
        if not path.is_file():
            errors.append(f"missing documentation audience boundary: {relative}")
            continue
        text = path.read_text(encoding="utf-8")
        for marker in markers:
            if marker not in text:
                errors.append(f"{relative}: audience boundary is missing {marker}")
    if (root / "docs/docs/agent/architecture.md").exists():
        errors.append(
            "docs/docs/agent/architecture.md: duplicate Agent architecture summary must remain absent"
        )
    return errors


def active_admin_scope_errors(
    root: Path, immutable_stopped_versions: tuple[str, ...]
) -> list[str]:
    errors: list[str] = []
    for relative in ACTIVE_ADMIN_SCOPE_FILES:
        path = root / relative
        if not path.is_file():
            errors.append(f"missing active Admin guide: {relative}")
            continue
        text = path.read_text(encoding="utf-8")
        header_and_scope = "\n".join(text.splitlines()[:24])
        for version in immutable_stopped_versions:
            if version in header_and_scope:
                errors.append(
                    f"{relative}: stopped {version} must not define an active title, description, keywords, or scope"
                )
    return errors


def package_and_container_contract_errors(
    root: Path,
    *,
    require_adopter_packages: bool = True,
    version: str = EXPECTED_DISTRIBUTION_VERSION,
) -> list[str]:
    errors: list[str] = []
    packages = root / "docs/docs/getting-started/packages.md"
    if require_adopter_packages and packages.is_file():
        text = packages.read_text(encoding="utf-8")
        for marker in (
            f"go get github.com/mss-boot-io/mss-boot-admin/admin@{version}",
            f"go get github.com/mss-boot-io/mss-boot-admin/mss-boot@{version}",
            "$previousGowork",
            "Remove-Item Env:GOWORK",
        ):
            if marker not in text:
                errors.append(f"{packages.relative_to(root)}: package contract is missing {marker}")
    docker = root / "docs/docs/admin/docker.md"
    if docker.is_file():
        text = docker.read_text(encoding="utf-8")
        for marker in ("不能直接部署", "docker buildx build", "OCI index digest", "业务镜像"):
            if marker not in text:
                errors.append(f"{docker.relative_to(root)}: Thin Host image contract is missing {marker}")
    dockerfile = root / "templates/application/Dockerfile"
    if dockerfile.is_file():
        text = dockerfile.read_text(encoding="utf-8")
        required_bases = (
            r"^FROM --platform=\$BUILDPLATFORM golang:1\.26\.6-alpine@sha256:[a-f0-9]{64} AS backend$",
            r"^FROM --platform=\$BUILDPLATFORM node:24\.19\.0-bookworm-slim@sha256:[a-f0-9]{64} AS frontend$",
            r"^FROM alpine:3\.24\.1@sha256:[a-f0-9]{64}$",
        )
        for pattern in required_bases:
            if not re.search(pattern, text, re.MULTILINE):
                errors.append(
                    "templates/application/Dockerfile: every base image must use the "
                    f"{version}-qualified tag and immutable digest ({pattern})"
                )
    else:
        errors.append("missing Thin Host templates/application/Dockerfile")
    dockerignore = root / "templates/application/.dockerignore"
    if dockerignore.is_file():
        ignored = set(dockerignore.read_text(encoding="utf-8").splitlines())
        for required in (".git", ".env", ".mss/logs", ".mss/reports", "*.db", "web/node_modules"):
            if required not in ignored:
                errors.append(
                    f"templates/application/.dockerignore: missing sensitive build-context exclusion {required}"
                )
    else:
        errors.append("missing Thin Host templates/application/.dockerignore")
    return errors


def repository_context_errors(
    root: Path, *, immutable_stopped_versions: tuple[str, ...]
) -> list[str]:
    """Reject stale monorepo, contributor, release, and user-visible context."""

    errors: list[str] = []
    contributor = root / "CONTRIBUTING.md"
    if not contributor.is_file():
        errors.append("missing Foundation contributor contract: CONTRIBUTING.md")
    else:
        text = contributor.read_text(encoding="utf-8")
        for marker in (
            *(f"{version} 已永久停止" for version in immutable_stopped_versions),
            "v1.3.2 稳定记录",
            "本文只适用于修改 Foundation 本身的贡献者",
            "go run ./cmd/mss context",
            "go run ./cmd/mss verify --changed",
            "corepack pnpm@10.34.5 --dir web/antd-v6 run start:dev",
        ):
            if marker not in text:
                errors.append(f"CONTRIBUTING.md: missing contributor boundary {marker}")
        stopped_pattern = "|".join(
            re.escape(version) for version in immutable_stopped_versions
        )
        for label, pattern in {
            "stopped-version adopter quick start": (
                rf"(?:{stopped_pattern})\s+快速开始"
            ),
            "repository-wide gofmt": r"(?m)^\s*gofmt\s+-w\s+\.\s*$",
            "uncontracted log file": r"(?m)^\s*tail\s+-f\s+logs/app\.log\s*$",
        }.items():
            if re.search(pattern, text):
                errors.append(f"CONTRIBUTING.md: obsolete {label} command is not allowed")

    monorepo = root / "MONOREPO.md"
    if not monorepo.is_file():
        errors.append("missing monorepo release contract: MONOREPO.md")
    else:
        normalized = " ".join(monorepo.read_text(encoding="utf-8").split())
        for marker in (
            *(
                f"{version} is permanently stopped as an immutable partial release"
                for version in immutable_stopped_versions
            ),
            "one non-publishing Root preview",
            "Framework, Admin, and Admin Web tags in order",
            "Root tag starts only the Root Release and backend-image candidate",
            "GitHub Latest and npmjs `latest` remain v1.3.2",
            "Stable promotion is a separate reviewed policy decision",
            "manually dispatch `npm-release.yml` from the exact `v1.3.7` Root tag",
            "npm publish --tag latest --provenance",
            "promote the exact Root Release to GitHub Latest",
            "final stable-policy and human-documentation reconciliation follows "
            "through another PR",
            "do not repeat expensive qualification",
        ):
            if marker not in normalized:
                errors.append(
                    f"MONOREPO.md: missing simplified release contract {marker}"
                )
        for obsolete in (
            "protected Root tag promotion",
            "finally npm Trusted Publishing",
        ):
            if obsolete in normalized:
                errors.append(
                    f"MONOREPO.md: obsolete release mechanism is not allowed: {obsolete}"
                )
        if "After this migration is merged" in normalized:
            errors.append("MONOREPO.md: completed import must not remain future work")

    layout = root / "web/antd-v6/src/shared/layout/LayoutChrome.tsx"
    if not layout.is_file():
        errors.append(f"missing Admin Web layout: {layout.relative_to(root)}")
    elif 'href="https://github.com/mss-boot-io/mss-boot"' in layout.read_text(
        encoding="utf-8"
    ):
        errors.append(
            "web/antd-v6/src/shared/layout/LayoutChrome.tsx: retired Framework "
            "repository must not be exposed in the shipped UI"
        )

    changelog = root / "CHANGELOG.md"
    if not changelog.is_file():
        errors.append("missing release history: CHANGELOG.md")
    else:
        match = re.search(
            r"(?ms)^## \[Unreleased\]\s*(.*?)(?=^## \[)",
            changelog.read_text(encoding="utf-8"),
        )
        if not match:
            errors.append("CHANGELOG.md: missing bounded Unreleased section")
        else:
            body = match.group(1)
            has_empty_declaration = "No unreleased changes are recorded." in body
            has_entries = bool(re.search(r"(?m)^\s*-\s+", body))
            if has_empty_declaration and has_entries:
                errors.append(
                    "CHANGELOG.md: Unreleased cannot list changes while declaring none"
                )
            elif not has_empty_declaration and not has_entries:
                errors.append(
                    "CHANGELOG.md: Unreleased must contain entries or an explicit empty state"
                )
    return errors


def route_exists(root: Path, route: str) -> bool:
    clean = unquote(route.split("#", 1)[0].split("?", 1)[0]).strip("/")
    if not clean:
        return (root / DOCS_ROOT / "index.md").is_file()
    base = root / DOCS_ROOT / clean
    localized = base.parent / f"{base.name}.zh-CN.md"
    return (
        base.with_suffix(".md").is_file()
        or localized.is_file()
        or (base / "index.md").is_file()
    )


def internal_link_errors(root: Path) -> list[str]:
    errors: list[str] = []
    docs_root = root / DOCS_ROOT
    if not docs_root.is_dir():
        return [f"missing {DOCS_ROOT}"]
    checked = set(docs_root.rglob("*.md"))
    checked.update(
        root / relative for relative in CORE_LINK_FILES if (root / relative).is_file()
    )
    for absolute in sorted(checked):
        relative = absolute.relative_to(root)
        text = absolute.read_text(encoding="utf-8")
        for match in MARKDOWN_LINK.finditer(text):
            raw = match.group(1).strip()
            if not raw or raw.startswith(("#", "http://", "https://", "mailto:")):
                continue
            target = raw.split()[0].strip("<>")
            target_no_fragment = unquote(
                target.split("#", 1)[0].split("?", 1)[0]
            )
            if not target_no_fragment:
                continue
            if target_no_fragment.startswith("/"):
                suffix = Path(target_no_fragment).suffix
                if suffix and suffix != ".md":
                    continue
                route = target_no_fragment.removesuffix(".md")
                if not route_exists(root, route):
                    line = text.count("\n", 0, match.start()) + 1
                    errors.append(
                        f"{relative}:{line}: broken site route {target_no_fragment}"
                    )
                continue
            if target_no_fragment.endswith(".md"):
                resolved = (absolute.parent / target_no_fragment).resolve()
                try:
                    resolved.relative_to(root.resolve())
                except ValueError:
                    line = text.count("\n", 0, match.start()) + 1
                    errors.append(f"{relative}:{line}: link escapes repository: {target}")
                    continue
                if not resolved.is_file():
                    line = text.count("\n", 0, match.start()) + 1
                    errors.append(f"{relative}:{line}: broken Markdown link {target}")
    return errors


def collect_errors(root: Path = ROOT) -> list[str]:
    errors: list[str] = []
    try:
        state = release_documentation_state(root)
    except ValueError as exc:
        return [str(exc)]
    version = state.distribution_version

    if version != EXPECTED_DISTRIBUTION_VERSION:
        errors.append(
            "documentation contract is for "
            f"{EXPECTED_DISTRIBUTION_VERSION}, release policy declares {version}"
        )

    for path in CORE_CURRENT_FILES:
        absolute = root / path
        if not absolute.is_file():
            errors.append(f"missing current documentation file: {path}")
            continue
        text = absolute.read_text(encoding="utf-8")
        if version not in text:
            errors.append(f"{path}: must name current distribution {version}")
        for stopped_version in state.immutable_stopped_versions:
            if stopped_version not in text:
                errors.append(
                    f"{path}: must distinguish immutable stopped "
                    f"{stopped_version} from current distribution {version}"
                )

    for tree in REMOVED_TREES:
        absolute = root / tree
        if absolute.is_dir() and any(path.is_file() for path in absolute.rglob("*")):
            errors.append(f"obsolete context tree must remain absent: {tree}")

    for path in REMOVED_PATHS:
        if (root / path).exists():
            errors.append(f"obsolete context path must remain absent: {path}")

    admin_root = root / "docs/docs/admin"
    for name in sorted(REMOVED_ADMIN_PAGES):
        if (admin_root / name).exists():
            errors.append(f"obsolete Admin page must remain absent: {admin_root / name}")

    for section, expected in EXACT_SECTION_CONTENT.items():
        absolute = root / section
        actual = (
            {path.name for path in absolute.glob("*.md")} if absolute.is_dir() else set()
        )
        if actual != expected:
            errors.append(
                f"{section}: expected {sorted(expected)}, found {sorted(actual)}"
            )

    release_root = root / "docs/docs/releases"
    actual_release_root = {path.name for path in release_root.glob("*.md")}
    if actual_release_root != ALLOWED_RELEASE_ROOT:
        errors.append(
            "docs/docs/releases: current root must contain only "
            f"{sorted(ALLOWED_RELEASE_ROOT)}, found {sorted(actual_release_root)}"
        )

    active_paths = active_markdown_paths(root)
    errors.extend(
        forbidden_content_errors(
            root,
            version,
            active_paths,
            current_stable_version=state.current_stable_version,
            immutable_stopped_versions=state.immutable_stopped_versions,
        )
    )
    stage_claim_paths = list(active_paths)
    stage_claim_paths.extend((Path("MONOREPO.md"), Path("docs/CONTRIBUTING.md")))
    errors.extend(
        absolute_unpublished_claim_errors(root, version, stage_claim_paths)
    )
    errors.extend(stopped_release_history_errors(root, state))

    partial_status_set = frozenset(PARTIAL_RELEASE_STATUS_FILES)
    if state.operational_onboarding_allowed:
        errors.extend(bootstrap_password_errors(root))
        errors.extend(first_login_errors(root))
        errors.extend(upgrade_contract_errors(root, version=version))
        errors.extend(mcp_contract_errors(root))
    else:
        errors.extend(partial_release_semantic_errors(root, state))
        errors.extend(partial_release_operational_errors(root, state))
        errors.extend(
            bootstrap_password_errors(
                root,
                (path for path in BOOTSTRAP_PASSWORD_FILES if path not in partial_status_set),
            )
        )
        errors.extend(
            first_login_errors(
                root,
                (path for path in FIRST_LOGIN_FILES if path not in partial_status_set),
            )
        )
        errors.extend(
            upgrade_contract_errors(
                root,
                (path for path in UPGRADE_CONTRACT_FILES if path not in partial_status_set),
                version=version,
            )
        )
        errors.extend(
            mcp_contract_errors(
                root,
                (path for path in MCP_CONTRACT_FILES if path not in partial_status_set),
                require_client_example=False,
            )
        )
    errors.extend(public_skill_documentation_errors(root))
    errors.extend(documentation_audience_errors(root))
    errors.extend(
        active_admin_scope_errors(root, state.immutable_stopped_versions)
    )
    errors.extend(
        package_and_container_contract_errors(
            root,
            require_adopter_packages=state.operational_onboarding_allowed,
            version=version,
        )
    )
    errors.extend(
        repository_context_errors(
            root,
            immutable_stopped_versions=state.immutable_stopped_versions,
        )
    )

    quick_start_titles: list[Path] = []
    for path in sorted((root / DOCS_ROOT).rglob("*.md")):
        text = path.read_text(encoding="utf-8")
        frontmatter = FRONTMATTER_TITLE.search(text)
        if not frontmatter:
            continue
        title_match = re.search(
            r"^title:\s*(.+?)\s*$", frontmatter.group("body"), re.MULTILINE
        )
        if title_match and re.search(
            r"(?:快速开始|quick\s*start)", title_match.group(1), re.IGNORECASE
        ):
            quick_start_titles.append(path.relative_to(root))
    expected_quick_start_titles = (
        [Path("docs/docs/getting-started/index.md")]
        if state.operational_onboarding_allowed
        else []
    )
    if quick_start_titles != expected_quick_start_titles:
        errors.append(
            "quick-start page set does not match publication state; found "
            + ", ".join(map(str, quick_start_titles))
        )

    nav = root / "docs/.dumirc.ts"
    if not nav.is_file():
        errors.append("missing docs/.dumirc.ts")
    else:
        nav_text = nav.read_text(encoding="utf-8")
        nav_routes = NAV_LINK.findall(nav_text)
        if nav_routes.count("/getting-started") != 1:
            errors.append("navigation must expose exactly one /getting-started entry")
        if (
            not state.operational_onboarding_allowed
            and not re.search(
                r"title:\s*['\"]采用状态['\"][\s\S]{0,160}?"
                r"link:\s*['\"]/getting-started['\"]",
                nav_text,
            )
        ):
            errors.append(
                "partial-release navigation must label /getting-started as 采用状态"
            )
        for route in nav_routes:
            if not route_exists(root, route):
                errors.append(f"docs/.dumirc.ts: navigation target does not exist: {route}")

    archive = root / ARCHIVE_PREFIX
    if not archive.is_dir():
        errors.append(f"missing release archive: {ARCHIVE_PREFIX}")
    else:
        actual_archive_pages = {page.name for page in archive.glob("*.md")}
        if actual_archive_pages != REQUIRED_ARCHIVE_PAGES:
            errors.append(
                f"{ARCHIVE_PREFIX}: expected {sorted(REQUIRED_ARCHIVE_PAGES)}, "
                f"found {sorted(actual_archive_pages)}"
            )
        for page in sorted(archive.glob("v*.md")):
            text = page.read_text(encoding="utf-8")
            if version not in text or "/getting-started" not in text:
                errors.append(
                    f"{page.relative_to(root)}: missing read-only historical banner"
                )

    adr_root = root / "docs/adr"
    adr_pages = sorted(adr_root.glob("*.md"))
    if not adr_pages:
        errors.append("docs/adr: no architecture decisions found")
    missing_adrs = REQUIRED_ADRS - {page.name for page in adr_pages}
    if missing_adrs:
        errors.append(f"docs/adr: missing retained decisions {sorted(missing_adrs)}")
    for page in adr_pages:
        head = "\n".join(page.read_text(encoding="utf-8").splitlines()[:24])
        if not re.search(r"(?im)^\s*-?\s*Status:\s*\S", head):
            errors.append(f"{page.relative_to(root)}: ADR must declare Status")

    root_readme_path = root / "README.md"
    zh_readme_path = root / "README.zh-CN.md"
    root_readme = (
        root_readme_path.read_text(encoding="utf-8")
        if root_readme_path.is_file()
        else ""
    )
    zh_readme = (
        zh_readme_path.read_text(encoding="utf-8")
        if zh_readme_path.is_file()
        else ""
    )
    aligned_markers = (
        (
            "install-mss.sh",
            "install-mss.ps1",
            "$env:Path",
            "mss new app",
            f"mss upgrade admin {version}",
            "mss-mcp",
            f"@mss-boot-io/admin-web@{version.removeprefix('v')}",
            "MSS_ADMIN_INITIAL_PASSWORD",
        )
        if state.operational_onboarding_allowed
        else (
            state.current_stable_version,
            state.distribution_version,
            *(
                marker
                for stopped_version in state.immutable_stopped_versions
                for marker in (
                    "github.com/mss-boot-io/mss-boot-admin/mss-boot@"
                    f"{stopped_version}",
                    "github.com/mss-boot-io/mss-boot-admin/admin@"
                    f"{stopped_version}",
                    "@mss-boot-io/admin-web@"
                    f"{stopped_version.removeprefix('v')}",
                    f"docs/{stopped_version}",
                )
            ),
            "Root Release",
        )
    )
    for marker in aligned_markers:
        if marker not in root_readme or marker not in zh_readme:
            errors.append(f"root README language pair is missing shared marker: {marker}")

    errors.extend(internal_link_errors(root))
    return sorted(set(errors))


def success_message(state: ReleaseDocumentationState) -> str:
    if state.operational_onboarding_allowed:
        return (
            f"current documentation contract OK: {state.distribution_version} "
            "stable operational onboarding, links and archive"
        )
    return (
        f"current documentation contract OK: {state.distribution_version} "
        f"{state.release_status}; current stable {state.current_stable_version}; "
        f"immutable stopped {', '.join(state.immutable_stopped_versions)}; "
        "operational onboarding disabled"
    )


def main() -> int:
    try:
        state = release_documentation_state(ROOT)
    except ValueError as exc:
        print("current documentation contract failed:", file=sys.stderr)
        print(f"- {exc}", file=sys.stderr)
        return 1
    errors = collect_errors()
    if errors:
        print("current documentation contract failed:", file=sys.stderr)
        for error in errors:
            print(f"- {error}", file=sys.stderr)
        return 1
    print(success_message(state))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
