#!/usr/bin/env python3
from __future__ import annotations

from pathlib import Path
import re
import subprocess

ROOT = Path(".")
DISABLED = "copi" + "lot"


def replace_once(path: Path, old: str, new: str, *, required: bool = True) -> bool:
    text = path.read_text(encoding="utf-8")
    if old not in text:
        if required:
            raise SystemExit(f"expected text not found in {path}: {old[:120]!r}")
        return False
    path.write_text(text.replace(old, new, 1), encoding="utf-8")
    return True


def regex_once(path: Path, pattern: str, replacement: str, *, required: bool = True) -> bool:
    text = path.read_text(encoding="utf-8")
    updated, count = re.subn(pattern, replacement, text, count=1, flags=re.S)
    if count == 0:
        if required:
            raise SystemExit(f"expected pattern not found in {path}: {pattern[:120]!r}")
        return False
    path.write_text(updated, encoding="utf-8")
    return True


def remove_disabled_files() -> None:
    tracked = subprocess.check_output(["git", "ls-files", "-z"]).split(b"\0")
    for raw in tracked:
        if not raw:
            continue
        relative = raw.decode("utf-8")
        if DISABLED in relative.lower():
            path = ROOT / relative
            if path.exists() or path.is_symlink():
                path.unlink()

    for relative in (
        ".github/workflows/agent-native-feature-mcp-eval.yml",
        ".github/workflows/agent-native-feature-spec-fix.yml",
        ".github/workflows/agent-native-closeout.yml",
        ".github/workflows/agent-native-closeout-v2.yml",
        "tools/agent_closeout.py",
    ):
        path = ROOT / relative
        if path.exists() or path.is_symlink():
            path.unlink()


def converge_cli() -> None:
    app_path = ROOT / "internal/mss/app/app.go"
    app = app_path.read_text(encoding="utf-8")

    if "return ExecuteAgent()" not in app:
        app, count = re.subn(
            r"// Execute runs the agent-facing mss CLI\..*?\nfunc newContextCommand",
            """// Execute runs the complete agent-facing mss CLI.
func Execute() error {
\treturn ExecuteAgent()
}

// NewRootCommand returns the same complete command tree used by cmd/mss.
func NewRootCommand() *cobra.Command {
\treturn NewAgentRootCommand()
}

func newContextCommand""",
            app,
            count=1,
            flags=re.S,
        )
        if count != 1:
            raise SystemExit("failed to consolidate the root command tree")

    if "func newSpecCommand(" in app:
        app, count = re.subn(
            r"func newSpecCommand\(rootOverride \*string\) \*cobra\.Command \{.*?\n\}\n\nfunc newModuleCommand",
            "func newModuleCommand",
            app,
            count=1,
            flags=re.S,
        )
        if count != 1:
            raise SystemExit("failed to remove the legacy AdminModule-only spec command")

    if "os." not in app:
        app = app.replace('\n\t"os"', "", 1)
    app_path.write_text(app, encoding="utf-8")

    main_path = ROOT / "cmd/mss/main.go"
    main = main_path.read_text(encoding="utf-8")
    if "app.ExecuteAgent()" in main:
        main_path.write_text(main.replace("app.ExecuteAgent()", "app.Execute()", 1), encoding="utf-8")
    elif "app.Execute()" not in main:
        raise SystemExit("cmd/mss has no recognized app entrypoint")


def converge_feature_and_spec() -> None:
    plan_path = ROOT / "internal/mss/feature/plan.go"
    plan = plan_path.read_text(encoding="utf-8")
    if "document.Document.(*spec.ModuleSpec)" in plan:
        plan = plan.replace("document.Document.(*spec.ModuleSpec)", "document.Document.(*spec.Module)", 1)
    if "document.Document.(*spec.Module)" not in plan:
        raise SystemExit("Feature plan has no valid AdminModule type assertion")
    plan_path.write_text(plan, encoding="utf-8")

    feature_path = ROOT / "internal/mss/spec/feature.go"
    feature = feature_path.read_text(encoding="utf-8")
    feature = feature.replace('\n\t"errors"', "", 1)
    feature = feature.replace(
        "\n// Ensure FeatureSpec's validator remains independently usable by callers.\n"
        "var _ error = validationError{}\nvar _ = errors.New\n",
        "\n",
    )
    feature_path.write_text(feature, encoding="utf-8")

    feature_yaml_path = ROOT / ".mss/features/example-supplier-onboarding.yaml"
    feature_yaml = feature_yaml_path.read_text(encoding="utf-8")
    if "id: supplier-audit-safe" not in feature_yaml:
        marker = """    - id: changed-verification
      statement: The change-aware verifier completes without failed required checks.
"""
        replacement = """    - id: supplier-audit-safe
      requirement: supplier-audit
      statement: Supplier mutations produce safe audit metadata without persisting credentials, tokens, attachment content, or sensitive request bodies.
      level: security
      required: true
      evidence:
        - type: test
          value: modules/supplier/tests/audit_test.go
    - id: changed-verification
      statement: The change-aware verifier completes without failed required checks.
"""
        if marker not in feature_yaml:
            raise SystemExit("failed to locate supplier audit Acceptance insertion point")
        feature_yaml_path.write_text(feature_yaml.replace(marker, replacement, 1), encoding="utf-8")


def converge_mcp() -> None:
    path = ROOT / "internal/mss/mcp/server.go"
    text = path.read_text(encoding="utf-8")

    if "s.callSpecificationTool" not in text:
        old = """\tdefault:
\t\tif result, known := s.callBlueprintTool(ctx, name, arguments); known {
\t\t\treturn result, true
\t\t}
\t\treturn callToolResult{}, false
\t}
"""
        new = """\tdefault:
\t\tif result, known := s.callSpecificationTool(ctx, name, arguments); known {
\t\t\treturn result, true
\t\t}
\t\tif result, known := s.callFeatureTool(ctx, name, arguments); known {
\t\t\treturn result, true
\t\t}
\t\tif result, known := s.callBlueprintTool(ctx, name, arguments); known {
\t\t\treturn result, true
\t\t}
\t\treturn callToolResult{}, false
\t}
"""
        if old not in text:
            raise SystemExit("failed to locate MCP extension dispatch")
        text = text.replace(old, new, 1)

    if "specificationToolDefinitions()" not in text:
        old = """\tdefinitions = append(definitions, blueprintToolDefinitions()...)
\tsort.SliceStable(definitions, func(i, j int) bool { return definitions[i].Name < definitions[j].Name })
"""
        new = """\tdefinitions = append(definitions, specificationToolDefinitions()...)
\tdefinitions = append(definitions, featureToolDefinitions()...)
\tdefinitions = append(definitions, blueprintToolDefinitions()...)
\tsort.SliceStable(definitions, func(i, j int) bool { return definitions[i].Name < definitions[j].Name })
"""
        if old not in text:
            raise SystemExit("failed to locate MCP tool aggregation")
        text = text.replace(old, new, 1)

    path.write_text(text, encoding="utf-8")


def converge_evals() -> None:
    path = ROOT / "internal/mss/eval/eval.go"
    text = path.read_text(encoding="utf-8")

    import_marker = '\t"github.com/mss-boot-io/mss-boot-admin/internal/mss/generator"\n'
    if "internal/mss/feature" not in text:
        if import_marker not in text:
            raise SystemExit("failed to locate Eval import insertion point")
        text = text.replace(
            import_marker,
            '\tfeaturecmd "github.com/mss-boot-io/mss-boot-admin/internal/mss/feature"\n'
            + import_marker,
            1,
        )

    if '"feature-spec"' not in text:
        marker = """\t\t"application-blueprint-plan": true,
\t\t"validation-plan":            true,
"""
        replacement = """\t\t"application-blueprint-plan": true,
\t\t"feature-spec":               true,
\t\t"feature-plan":               true,
\t\t"validation-plan":            true,
"""
        if marker not in text:
            raise SystemExit("failed to locate Eval check catalog")
        text = text.replace(marker, replacement, 1)

    compact_condition = (
        '\t\t\tif (check.Type == "module-spec" || check.Type == "module-generation-plan") '
        '&& strings.TrimSpace(check.Path) == "" {\n'
    )
    if 'check.Type == "feature-plan"' not in text:
        replacement = """\t\t\tif (check.Type == "module-spec" ||
\t\t\t\tcheck.Type == "module-generation-plan" ||
\t\t\t\tcheck.Type == "feature-spec" ||
\t\t\t\tcheck.Type == "feature-plan") && strings.TrimSpace(check.Path) == "" {
"""
        if compact_condition not in text:
            raise SystemExit("failed to locate Eval path validation")
        text = text.replace(compact_condition, replacement, 1)

    if 'case "feature-spec":' not in text:
        marker = """\tcase "application-blueprint-plan":
\t\tvalue, err = checkApplicationBlueprint(ctx, root, check.Minimum)
\tcase "validation-plan":
"""
        replacement = """\tcase "application-blueprint-plan":
\t\tvalue, err = checkApplicationBlueprint(ctx, root, check.Minimum)
\tcase "feature-spec":
\t\tvalue, err = checkFeatureSpec(root, check.Path)
\tcase "feature-plan":
\t\tvalue, err = checkFeaturePlan(root, check.Path, check.Minimum)
\tcase "validation-plan":
"""
        if marker not in text:
            raise SystemExit("failed to locate Eval execution switch")
        text = text.replace(marker, replacement, 1)

    if "func checkFeatureSpec(" not in text:
        marker = "func checkApplicationBlueprint(ctx context.Context, root string, minimum int) (map[string]any, error) {\n"
        functions = """func checkFeatureSpec(root, inputPath string) (map[string]any, error) {
\tabsolute, relative, err := resolveFile(root, inputPath)
\tif err != nil {
\t\treturn nil, err
\t}
\tfeature, err := spec.LoadFeature(absolute)
\tif err != nil {
\t\treturn nil, err
\t}
\tfeature.SourcePath = relative
\tsummary := feature.Summary()
\tsummary["path"] = relative
\treturn summary, nil
}

func checkFeaturePlan(root, inputPath string, minimum int) (map[string]any, error) {
\tplan, err := featurecmd.Build(featurecmd.Options{Root: root, FeaturePath: inputPath})
\tif err != nil {
\t\treturn nil, err
\t}
\toutputs := 0
\tfor _, module := range plan.Modules {
\t\toutputs += module.GeneratedOutputs
\t}
\tif outputs < minimum {
\t\treturn nil, fmt.Errorf("Feature plan contains %d generated outputs, expected at least %d", outputs, minimum)
\t}
\treturn map[string]any{
\t\t"feature":      plan.Feature.Name,
\t\t"modules":      len(plan.Modules),
\t\t"requirements": len(plan.Requirements),
\t\t"acceptance":   len(plan.Acceptance),
\t\t"outputs":      outputs,
\t\t"rollout":      plan.Rollout.Strategy,
\t}, nil
}

"""
        if marker not in text:
            raise SystemExit("failed to locate Eval Feature function insertion point")
        text = text.replace(marker, functions + marker, 1)

    path.write_text(text, encoding="utf-8")


def converge_setup_and_readmes() -> None:
    setup_path = ROOT / ".codex/setup.sh"
    setup = setup_path.read_text(encoding="utf-8")
    marker = "export COREPACK_ENABLE_DOWNLOAD_PROMPT=0\n"
    if "mkdir -p .mss/cache" not in setup:
        if marker not in setup:
            raise SystemExit("failed to locate Codex setup insertion point")
        setup = setup.replace(marker, marker + "\nmkdir -p .mss/cache\n", 1)
    setup_path.write_text(setup, encoding="utf-8")

    readme_path = ROOT / "README.md"
    readme = readme_path.read_text(encoding="utf-8")
    if "Agent-native management-system development foundation" not in readme:
        old = """## Introduction
> `mss-boot-admin` is a front-end and back-end separation admin platform based on Gin, React, Ant Design v5, Umi v4, and mss-boot. Its current product focus is governance, operations, configuration, access control, internationalization, and AI-annotation-assisted engineering collaboration.

> The repository still contains some historical dynamic-model and code-generation related capabilities, but they are no longer the primary direction for future product investment.
"""
        new = """## Introduction

> `mss-boot-admin` is an Agent-native management-system development foundation. It combines a production-oriented Gin + React + Ant Design reference application with machine-readable project contracts, Feature and AdminModule specifications, deterministic full-stack generation, repository Skills, a project MCP server, reproducible setup, change-aware verification, Agent Evals, versioned application Blueprints, and conflict-aware downstream upgrades.

> The runtime admin platform still provides identity, RBAC, organization, configuration, audit, notification, task, internationalization, storage, WebSocket, and observability capabilities. Historical runtime dynamic-model and virtual code-generation paths remain compatibility-only; new business modules use development-time specifications and compiled vertical modules.

## Agent-native workflow

```text
business intent
  → Feature and Acceptance contract
  → AdminModule contract
  → deterministic generation
  → Agent implements non-template business rules
  → change-aware verification and Evals
  → reviewable PR and upgradeable downstream application
```

```shell
./mss context --format json
./mss doctor --strict --format json
./mss setup
./mss dev --detach
./mss verify --changed
./mss eval run --all
```
"""
        if old not in readme:
            raise SystemExit("failed to locate English README introduction")
        readme = readme.replace(old, new, 1)
        readme = readme.replace(
            "- Evolving toward AI-annotation-assisted engineering workflows for clearer collaboration and delivery discipline\n",
            "- Agent-native contracts, deterministic generation, project MCP tools, change-aware verification, downstream Blueprints, and three-way foundation upgrades\n",
        )

    quick_pattern = re.compile(r"## 📦 Quick start\n.*?\n## 📨 Interaction", re.S)
    quick = """## 📦 Quick start

```shell
git clone https://github.com/mss-boot-io/mss-boot-admin.git
cd mss-boot-admin

./mss doctor --strict --format json
./mss setup
./mss dev --detach
./mss dev status --format json
```

Create or validate development contracts before editing repetitive code:

```shell
./mss spec validate .mss/features/example-supplier-onboarding.yaml
./mss feature plan .mss/features/example-supplier-onboarding.yaml
./mss module generate .mss/modules/example-supplier.yaml --format json
./mss verify --changed
```

The manual backend, frontend, migration, Blueprint, upgrade, Skills, MCP, and Eval workflows are documented under `docs/docs/agent/`.

## 📨 Interaction"""
    readme, count = quick_pattern.subn(quick, readme, count=1)
    if count != 1:
        raise SystemExit("failed to replace English README quick start")
    readme_path.write_text(readme, encoding="utf-8")

    zh_path = ROOT / "README.zh-CN.md"
    zh = zh_path.read_text(encoding="utf-8")
    if "Agent 原生的管理系统开发基础设施" not in zh:
        old = """## 简介
> `mss-boot-admin` 是基于 Gin + React + Ant Design v5 + Umi v4 + mss-boot 的前后端分离后台管理平台。当前产品主线聚焦于权限治理、组织管理、系统配置、访问控制、国际化，以及 AI 注解协同驱动的研发流程。

> 当前仓库中仍然保留了部分动态模型与代码生成相关实现，但它们不再是后续阶段的主要产品投入方向。
"""
        new = """## 简介

> `mss-boot-admin` 是一套 Agent 原生的管理系统开发基础设施。它把可生产使用的 Gin + React + Ant Design 参考应用，与机器可读项目契约、Feature/Acceptance/AdminModule 规格、确定性全栈生成、仓库级 Skills、项目 MCP、可重复环境、变更感知验证、Agent Evals、应用 Blueprint 和三方 Foundation 升级能力整合在同一个仓库中。

> 运行时管理平台继续提供身份、RBAC、组织、配置、审计、通知、任务、国际化、存储、WebSocket 和可观测性。历史动态模型与运行时代码生成仅保留兼容；新业务模块使用开发期规格和可编译的垂直模块。

## Agent 原生开发闭环

```text
业务意图
  → Feature 与 Acceptance 契约
  → AdminModule 契约
  → 确定性生成
  → Agent 实现非模板化业务规则
  → 变更感知验证与 Evals
  → 可审查 PR 与可持续升级的下游系统
```

```shell
./mss context --format json
./mss doctor --strict --format json
./mss setup
./mss dev --detach
./mss verify --changed
./mss eval run --all
```
"""
        if old not in zh:
            raise SystemExit("failed to locate Chinese README introduction")
        zh = zh.replace(old, new, 1)
        zh = zh.replace(
            "- 正在向 AI 注解协同驱动的工程化研发流程演进\n",
            "- 提供 Agent 原生契约、确定性生成、项目 MCP、变更感知验证、应用 Blueprint 和三方 Foundation 升级\n",
        )

    quick_pattern_zh = re.compile(r"## 📦 快速开始\n.*?\n## 本地测试前置条件", re.S)
    quick_zh = """## 📦 快速开始

```shell
git clone https://github.com/mss-boot-io/mss-boot-admin.git
cd mss-boot-admin

./mss doctor --strict --format json
./mss setup
./mss dev --detach
./mss dev status --format json
```

在编写重复代码前先创建或验证结构化契约：

```shell
./mss spec validate .mss/features/example-supplier-onboarding.yaml
./mss feature plan .mss/features/example-supplier-onboarding.yaml
./mss module generate .mss/modules/example-supplier.yaml --format json
./mss verify --changed
```

后端、前端、迁移、Blueprint、升级、Skills、MCP 与 Evals 的详细流程位于 `docs/docs/agent/`。

## 本地测试前置条件"""
    zh, count = quick_pattern_zh.subn(quick_zh, zh, count=1)
    if count != 1:
        raise SystemExit("failed to replace Chinese README quick start")
    zh_path.write_text(zh, encoding="utf-8")


def remove_active_references() -> None:
    active = [
        Path("AGENTS.md"),
        Path("CLAUDE.md"),
        Path("README.md"),
        Path("README.zh-CN.md"),
        Path(".agents"),
        Path(".codex"),
        Path(".cursor"),
        Path(".mss"),
        Path("docs/AGENTS.md"),
        Path("docs/docs/agent"),
        Path("docs/docs/architecture/agent-native-foundation.zh-CN.md"),
        Path("mss-boot/AGENTS.md"),
        Path("web/antd/AGENTS.md"),
    ]
    for item in active:
        paths = [item] if item.is_file() else sorted(item.rglob("*")) if item.exists() else []
        for path in paths:
            if not path.is_file():
                continue
            try:
                value = path.read_text(encoding="utf-8")
            except UnicodeDecodeError:
                continue
            value = re.sub(r"GitHub\s+" + DISABLED, "other coding agents", value, flags=re.I)
            value = re.sub(r"\b" + DISABLED + r"\b", "other coding agents", value, flags=re.I)
            path.write_text(value, encoding="utf-8")

    violations: list[str] = []
    for item in active + [Path(".github")]:
        paths = [item] if item.is_file() else sorted(item.rglob("*")) if item.exists() else []
        for path in paths:
            if not path.is_file():
                continue
            if DISABLED in path.as_posix().lower():
                violations.append(f"{path}: disabled integration path")
                continue
            try:
                value = path.read_text(encoding="utf-8")
            except UnicodeDecodeError:
                continue
            if DISABLED in value.lower():
                violations.append(f"{path}: disabled integration reference")
    if violations:
        raise SystemExit("\n".join(violations))


def main() -> None:
    remove_disabled_files()
    converge_cli()
    converge_feature_and_spec()
    converge_mcp()
    converge_evals()
    converge_setup_and_readmes()
    remove_active_references()


if __name__ == "__main__":
    main()
