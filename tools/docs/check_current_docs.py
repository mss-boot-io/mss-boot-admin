#!/usr/bin/env python3
"""Fail closed when current documentation drifts from the package-first release."""

from __future__ import annotations

import re
import sys
from pathlib import Path
from typing import Iterable
from urllib.parse import unquote


ROOT = Path(__file__).resolve().parents[2]
DOCS_ROOT = Path("docs/docs")

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
    Path("docs/docs/releases/v1-3-5.md"),
)

CORE_LINK_FILES = CORE_CURRENT_FILES + (
    Path("CHANGELOG.md"),
    Path("mss-boot/CHANGELOG.md"),
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
    Path("docs/docs/releases/v1-3-5.md"),
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

ALLOWED_RELEASE_ROOT = {"index.md", "v1-3-5.md"}
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
    r"curl\b|bash\b|Invoke-WebRequest\b|&\s+\.\\install-mss|docker\b)",
    re.IGNORECASE,
)
HISTORICAL_RELEASE_VERSION_REFERENCES = {
    Path("docs/docs/releases/index.md"): {"v1.3.4"},
    Path("docs/docs/releases/v1-3-5.md"): {"v1.3.4"},
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


def distribution_version(root: Path) -> str:
    policy = root / ".mss/release-policy.yaml"
    try:
        text = policy.read_text(encoding="utf-8")
    except OSError as exc:
        raise ValueError(f"cannot read {policy}: {exc}") from exc
    match = re.search(
        r"^\s*distributionVersion:\s*['\"]?(v\d+\.\d+\.\d+)['\"]?\s*$",
        text,
        re.MULTILINE,
    )
    if not match:
        raise ValueError(
            ".mss/release-policy.yaml must declare spec.distributionVersion"
        )
    return match.group(1)


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
    root: Path, version: str, paths: Iterable[Path]
) -> list[str]:
    errors: list[str] = []
    npm_version = version.removeprefix("v")
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
            if token != version and not (
                token in allowed_historical_versions
                and not in_fenced_code(match.start())
                and not on_operational_line(match.start())
            ):
                line = text.count("\n", 0, match.start()) + 1
                errors.append(
                    f"{path}:{line}: stale distribution token {token}; expected {version}"
                )
        for match in ADMIN_WEB_TOKEN.finditer(text):
            token = match.group(1)
            if token != npm_version and not (
                f"v{token}" in allowed_historical_versions
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


def bootstrap_password_errors(root: Path) -> list[str]:
    errors: list[str] = []
    for path in BOOTSTRAP_PASSWORD_FILES:
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


def first_login_errors(root: Path) -> list[str]:
    errors: list[str] = []
    for path in FIRST_LOGIN_FILES:
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


def upgrade_contract_errors(root: Path) -> list[str]:
    errors: list[str] = []
    required_markers = (
        "mss --version",
        "mss-mcp --version",
        ".mss/blueprint-manifest.json",
        "mss doctor --strict",
        "mss verify --all",
    )
    for path in UPGRADE_CONTRACT_FILES:
        absolute = root / path
        if not absolute.is_file():
            errors.append(f"missing upgrade documentation: {path}")
            continue
        text = absolute.read_text(encoding="utf-8")
        for marker in required_markers:
            if marker not in text:
                errors.append(f"{path}: upgrade contract is missing {marker}")
        upgrade_commands = re.findall(
            r"mss upgrade admin (?:v1\.3\.5|__MSS_DISTRIBUTION_VERSION__)",
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


def mcp_contract_errors(root: Path) -> list[str]:
    errors: list[str] = []
    for path in MCP_CONTRACT_FILES:
        absolute = root / path
        if not absolute.is_file():
            errors.append(f"missing MCP documentation: {path}")
            continue
        text = absolute.read_text(encoding="utf-8")
        for marker in ("stdio", "tools/list", "mss_plan_application", ".mss/project.yaml"):
            if marker not in text:
                errors.append(f"{path}: MCP contract is missing {marker}")
    tooling = root / "docs/docs/getting-started/tooling.md"
    if tooling.is_file():
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
    errors = [
        f"{path}: public Skill list is missing {name}"
        for name in PUBLIC_THIN_HOST_SKILLS
        if f"`{name}`" not in text
    ]
    for forbidden in ("mss-add-workflow", "mss-release", "mss-update-release-docs"):
        if f"`{forbidden}`" in text:
            errors.append(f"{path}: Foundation-only or unsupported Skill is public: {forbidden}")
    return errors


def package_and_container_contract_errors(root: Path) -> list[str]:
    errors: list[str] = []
    packages = root / "docs/docs/getting-started/packages.md"
    if packages.is_file():
        text = packages.read_text(encoding="utf-8")
        for marker in (
            "go get github.com/mss-boot-io/mss-boot-admin/admin@v1.3.5",
            "go get github.com/mss-boot-io/mss-boot-admin/mss-boot@v1.3.5",
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
                    f"v1.3.5-qualified tag and immutable digest ({pattern})"
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


def repository_context_errors(root: Path) -> list[str]:
    """Reject stale monorepo, contributor, release, and user-visible context."""

    errors: list[str] = []
    contributor = root / "CONTRIBUTING.md"
    if not contributor.is_file():
        errors.append("missing Foundation contributor contract: CONTRIBUTING.md")
    else:
        text = contributor.read_text(encoding="utf-8")
        for marker in (
            "v1.3.5 快速开始",
            "本文只适用于修改 Foundation 本身的贡献者",
            "go run ./cmd/mss context",
            "go run ./cmd/mss verify --changed",
            "corepack pnpm@10.34.5 --dir web/antd-v6 run start:dev",
        ):
            if marker not in text:
                errors.append(f"CONTRIBUTING.md: missing contributor boundary {marker}")
        for label, pattern in {
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
        expected_order = (
            "The fail-closed v1.3.5 publication order is Framework, Admin, "
            "Admin Web, protected Root tag promotion, Root release, Docs, and "
            "finally npm Trusted Publishing."
        )
        if expected_order not in normalized:
            errors.append("MONOREPO.md: v1.3.5 publication order is incomplete or stale")
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
        version = distribution_version(root)
    except ValueError as exc:
        return [str(exc)]

    if version != "v1.3.5":
        errors.append(
            f"documentation contract is for v1.3.5, release policy declares {version}"
        )

    for path in CORE_CURRENT_FILES:
        absolute = root / path
        if not absolute.is_file():
            errors.append(f"missing current documentation file: {path}")
            continue
        if version not in absolute.read_text(encoding="utf-8"):
            errors.append(f"{path}: must name current distribution {version}")

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
    errors.extend(forbidden_content_errors(root, version, active_paths))
    errors.extend(bootstrap_password_errors(root))
    errors.extend(first_login_errors(root))
    errors.extend(upgrade_contract_errors(root))
    errors.extend(mcp_contract_errors(root))
    errors.extend(public_skill_documentation_errors(root))
    errors.extend(package_and_container_contract_errors(root))
    errors.extend(repository_context_errors(root))

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
    if quick_start_titles != [Path("docs/docs/getting-started/index.md")]:
        errors.append(
            "exactly one quick-start page is allowed; found "
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
            if "v1.3.5" not in text or "/getting-started" not in text:
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
        "install-mss.sh",
        "install-mss.ps1",
        "$env:Path",
        "mss new app",
        "mss upgrade admin v1.3.5",
        "mss-mcp",
        "@mss-boot-io/admin-web@1.3.5",
        "MSS_ADMIN_INITIAL_PASSWORD",
    )
    for marker in aligned_markers:
        if marker not in root_readme or marker not in zh_readme:
            errors.append(f"root README language pair is missing shared marker: {marker}")

    errors.extend(internal_link_errors(root))
    return sorted(set(errors))


def main() -> int:
    errors = collect_errors()
    if errors:
        print("current documentation contract failed:", file=sys.stderr)
        for error in errors:
            print(f"- {error}", file=sys.stderr)
        return 1
    print("current documentation contract OK: v1.3.5 package-first, links and archive")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
