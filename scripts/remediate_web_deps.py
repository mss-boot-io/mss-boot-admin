#!/usr/bin/env python3
"""Converge the web/antd dependency graph and remove install-time side effects.

This script is intentionally deterministic and idempotent. It is used only by
the migration workflow and is removed after the generated manifests pass all
frontend and repository checks.
"""

from __future__ import annotations

import json
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1] / "web" / "antd"


def write_json(path: Path, value: object) -> None:
    path.write_text(
        json.dumps(value, ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )


def rewrite_package_json() -> None:
    path = ROOT / "package.json"
    package = json.loads(path.read_text(encoding="utf-8"))
    package.update(
        {
            "private": True,
            "packageManager": "pnpm@9.15.9",
            "engines": {
                "node": ">=22.0.0 <23",
                "pnpm": "9.15.9",
            },
        }
    )
    package["scripts"] = {
        "analyze": "cross-env ANALYZE=1 max build",
        "build": "max build",
        "build:alpha": "cross-env REACT_APP_ENV=alpha UMI_ENV=alpha max build",
        "build:beta": "cross-env REACT_APP_ENV=beta UMI_ENV=beta max build",
        "build:local": "cross-env REACT_APP_ENV=local UMI_ENV=local max build",
        "build:prod": "cross-env REACT_APP_ENV=prod UMI_ENV=prod max build",
        "deploy": "pnpm build && pnpm gh-pages",
        "dev": "pnpm start:dev",
        "format": 'prettier --write "**/*.{js,jsx,tsx,ts,less,md,json}"',
        "gh-pages": "gh-pages -d dist",
        "hooks:install": "node scripts/install-hooks.mjs",
        "jest": "jest",
        "lint": "pnpm lint:js && pnpm lint:prettier && pnpm tsc",
        "lint-staged": "lint-staged",
        "lint-staged:js": "eslint --cache --fix --max-warnings=0",
        "lint:fix": "eslint --fix --cache --ext .js,.jsx,.ts,.tsx --format=stylish ./src",
        "lint:js": "eslint --cache --ext .js,.jsx,.ts,.tsx --format=stylish ./src",
        "lint:prettier": 'prettier --check "**/*.{js,jsx,tsx,ts,less,md,json}"',
        "openapi": "max openapi",
        "playwright:install": "playwright install chromium",
        "preview": "pnpm build && max preview --port 8000",
        "setup": "max setup",
        "start": "cross-env UMI_ENV=dev max dev",
        "start:alpha": "cross-env REACT_APP_ENV=alpha MOCK=none UMI_ENV=alpha max dev",
        "start:beta": "cross-env REACT_APP_ENV=beta MOCK=none UMI_ENV=beta max dev",
        "start:dev": "cross-env REACT_APP_ENV=dev MOCK=none UMI_ENV=dev max dev",
        "start:no-mock": "cross-env MOCK=none UMI_ENV=dev max dev",
        "start:pre": "cross-env REACT_APP_ENV=pre UMI_ENV=dev max dev",
        "start:prod": "cross-env REACT_APP_ENV=prod MOCK=none UMI_ENV=prod max dev",
        "start:test": "cross-env REACT_APP_ENV=test MOCK=none UMI_ENV=dev max dev",
        "test": "cross-env MOCK=none jest",
        "test:ci": "cross-env MOCK=none jest --runInBand --detectOpenHandles",
        "test:coverage": "cross-env MOCK=none jest --coverage",
        "test:e2e": "playwright test",
        "test:e2e:debug": "playwright test --debug",
        "test:e2e:ui": "playwright test --ui",
        "test:update": "cross-env MOCK=none jest -u",
        "tsc": "tsc --noEmit",
    }
    package["lint-staged"] = {
        "**/*.{js,jsx,ts,tsx}": [
            "pnpm lint-staged:js",
            "prettier --write",
        ],
        "**/*.{less,md,json}": "prettier --write",
    }
    package["browserslist"] = [
        "defaults and supports es6-module",
        "not IE 11",
        "not dead",
    ]
    package["dependencies"] = {
        "@ant-design/charts": "2.6.7",
        "@ant-design/icons": "4.8.1",
        "@ant-design/pro-components": "2.8.10",
        "@ant-design/use-emotion-css": "1.0.4",
        "ahooks": "3.9.7",
        "antd": "5.12.1",
        "braft-editor": "2.3.9",
        "classnames": "2.5.1",
        "lodash": "4.18.1",
        "moment": "2.30.1",
        "rc-menu": "9.16.1",
        "rc-util": "5.44.4",
        "react": "18.2.0",
        "react-dom": "18.2.0",
        "uuid": "14.0.1",
    }
    package["devDependencies"] = {
        "@playwright/test": "1.62.1",
        "@testing-library/react": "15.0.7",
        "@types/express": "4.17.21",
        "@types/jest": "29.5.10",
        "@types/lodash": "4.17.25",
        "@types/node": "20.10.3",
        "@types/react": "18.2.41",
        "@types/react-dom": "18.2.17",
        "@umijs/lint": "4.7.0",
        "@umijs/max": "4.7.0",
        "@umijs/max-plugin-openapi": "2.0.3",
        "cross-env": "10.1.0",
        "dva": "2.5.0-beta.2",
        "eslint": "8.57.1",
        "express": "4.22.2",
        "gh-pages": "6.3.0",
        "husky": "9.1.7",
        "jest": "29.7.0",
        "jest-environment-jsdom": "29.7.0",
        "lint-staged": "17.3.0",
        "prettier": "2.8.8",
        "styled-jsx": "5.1.6",
        "ts-node": "10.9.2",
        "typescript": "4.9.5",
        "webpack": "5.109.2",
    }
    package["pnpm"] = {
        "peerDependencyRules": {
            "allowedVersions": {
                "dva-core": "2.0.4",
                "less-loader": "11.1.0",
                "react": "18.2.0",
                "react-dom": "18.2.0",
            }
        },
        "overrides": {
            "@babel/core": "7.29.7",
            "@babel/plugin-transform-modules-systemjs": "7.29.7",
            "@babel/runtime": "7.29.7",
            "@remix-run/router": "1.23.3",
            "@tootallnate/once": "2.0.1",
            "@vitejs/plugin-react@4.0.0": "4.7.0",
            "ajv@6.12.6": "6.15.0",
            "ajv@8.12.0": "8.20.0",
            "axios": "0.33.0",
            "body-parser": "1.20.6",
            "brace-expansion@1.1.15": "1.1.16",
            "brace-expansion@2.1.1": "2.1.2",
            "braces": "3.0.3",
            "braft-editor>immutable": "4.3.9",
            "braft-utils>immutable": "4.3.9",
            "cross-spawn": "7.0.6",
            "diff": "4.0.4",
            "draft-convert>immutable": "4.3.9",
            "draft-js>immutable": "4.3.9",
            "draftjs-utils>immutable": "4.3.9",
            "es5-ext": "0.10.63",
            "esbuild": "0.28.1",
            "fast-uri": "3.1.4",
            "flatted": "3.4.2",
            "form-data": "4.0.6",
            "hono": "4.12.27",
            "immer": "9.0.21",
            "isomorphic-fetch>node-fetch": "2.7.0",
            "js-cookie": "3.0.8",
            "js-yaml": "4.3.0",
            "micromatch": "4.0.8",
            "min-document@2.19.0": "2.19.2",
            "minimatch@3.1.2": "3.1.4",
            "minimatch@8.0.4": "8.0.6",
            "picomatch": "2.3.2",
            "piscina": "4.9.3",
            "postcss": "8.5.23",
            "qs": "6.15.2",
            "react-router@6.3.0": "6.30.4",
            "react-router@6.20.1": "6.30.4",
            "react-router-dom@6.3.0": "6.30.4",
            "react-router-dom@6.20.1": "6.30.4",
            "send": "0.19.2",
            "svgo": "2.8.3",
            "webpack": "5.109.2",
            "ws": "8.21.0",
            "yaml": "1.10.3",
        },
    }
    write_json(path, package)


def rewrite_umi_config() -> None:
    path = ROOT / "config" / "config.ts"
    text = path.read_text(encoding="utf-8")
    text = text.replace(
        "  presets: ['umi-presets-pro'],\n",
        "  plugins: ['@umijs/max-plugin-openapi'],\n",
    )
    text = text.replace(
        "path.resolve(__dirname, '../../mss-boot-admin/docs/swagger.json')",
        "path.resolve(__dirname, '../../../admin/docs/swagger.json')",
    )
    text = text.replace("  requestRecord: {},\n", "")
    path.write_text(text, encoding="utf-8")


def rewrite_login_test() -> None:
    path = ROOT / "src" / "pages" / "User" / "Login" / "login.test.tsx"
    text = path.read_text(encoding="utf-8-sig")
    text = text.replace(
        "// @ts-ignore\nimport { startMock } from '@@/requestRecordMock';\n\n",
        "",
    )
    server_start = text.find("let server: {")
    describe_start = text.find("describe('Login Page'")
    if server_start >= 0 and describe_start > server_start:
        text = text[:server_start] + text[describe_start:]
    mock = (
        "jest.mock('@/services/admin/appConfig', () => ({\n"
        "  getAppConfigsProfile: jest.fn().mockResolvedValue(undefined),\n"
        "}));\n\n"
    )
    marker = "import { resolveSafeRedirect } from './redirect';\n\n"
    if mock not in text:
        if marker not in text:
            raise RuntimeError("login test import marker is missing")
        text = text.replace(marker, marker + mock)
    path.write_text(text, encoding="utf-8")


def rewrite_types() -> None:
    typings_path = ROOT / "src" / "typings.d.ts"
    typings = typings_path.read_text(encoding="utf-8")
    typings = typings.replace("declare module 'omit.js';\n", "")
    typings = typings.replace("declare module 'uuid';\n", "")
    typings_path.write_text(typings, encoding="utf-8")

    intl_path = ROOT / "src" / "util" / "intl.ts"
    intl_path.write_text(
        "export type IntlFormatter = {\n"
        "  formatMessage(descriptor: { id: string; defaultMessage?: string }): string;\n"
        "};\n",
        encoding="utf-8",
    )
    for relative in (
        "src/util/addOption.tsx",
        "src/util/fieldIntl.tsx",
        "src/util/menuTransferTree.tsx",
    ):
        path = ROOT / relative
        text = path.read_text(encoding="utf-8-sig")
        text = text.replace(
            "// @ts-ignore\nimport { IntlShape } from 'react-intl';",
            "import type { IntlFormatter } from './intl';",
        )
        text = text.replace("IntlShape", "IntlFormatter")
        path.write_text(text, encoding="utf-8")


def rewrite_jest_config() -> None:
    path = ROOT / "jest.config.ts"
    text = path.read_text(encoding="utf-8")
    text = text.replace("  console.log();\n", "")
    path.write_text(text, encoding="utf-8")


def rewrite_hooks() -> None:
    scripts = ROOT / "scripts"
    scripts.mkdir(exist_ok=True)
    (scripts / "install-hooks.mjs").write_text(
        "import { execFileSync } from 'node:child_process';\n"
        "import { fileURLToPath } from 'node:url';\n"
        "import path from 'node:path';\n\n"
        "const packageRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');\n"
        "const repositoryRoot = path.resolve(packageRoot, '../..');\n"
        "const huskyBin = path.join(packageRoot, 'node_modules/husky/bin.js');\n"
        "execFileSync(process.execPath, [huskyBin, 'web/antd/.husky'], {\n"
        "  cwd: repositoryRoot,\n"
        "  stdio: 'inherit',\n"
        "});\n",
        encoding="utf-8",
    )
    pre_commit = ROOT / ".husky" / "pre-commit"
    pre_commit.write_text("pnpm --dir web/antd lint-staged\n", encoding="utf-8")
    pre_commit.chmod(0o755)


def rewrite_workspace() -> None:
    (ROOT / ".npmrc").write_text(
        "auto-install-peers=false\n"
        "engine-strict=true\n"
        "prefer-frozen-lockfile=true\n"
        "save-exact=true\n"
        "strict-peer-dependencies=true\n"
        "verify-store-integrity=true\n",
        encoding="utf-8",
    )
    (ROOT / "pnpm-workspace.yaml").write_text(
        "packages:\n  - workers-site\n",
        encoding="utf-8",
    )
    worker_path = ROOT / "workers-site" / "package.json"
    worker = json.loads(worker_path.read_text(encoding="utf-8"))
    worker.update(
        {
            "name": "@mss-boot-admin/workers-site",
            "version": "1.0.0",
            "private": True,
            "type": "module",
            "packageManager": "pnpm@9.15.9",
            "dependencies": {"@cloudflare/kv-asset-handler": "0.5.0"},
        }
    )
    write_json(worker_path, worker)


def remove_obsolete_files() -> None:
    for relative in (
        ".husky/commit-msg",
        "check_login.js",
        "check_login.mjs",
        "workers-site/pnpm-lock.yaml",
    ):
        path = ROOT / relative
        if path.exists():
            path.unlink()


def main() -> None:
    rewrite_package_json()
    rewrite_umi_config()
    rewrite_login_test()
    rewrite_types()
    rewrite_jest_config()
    rewrite_hooks()
    rewrite_workspace()
    remove_obsolete_files()


if __name__ == "__main__":
    main()
