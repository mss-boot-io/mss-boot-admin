# Copilot Repository Instructions

## 项目简称
- `mss-boot` 简称 `mss`
- `mss-boot-admin` 简称 `admin`
- `mss-boot-admin-antd` 简称 `antd`
- `mss-boot-docs` 简称 `docs`

## Startup Reuse Constraint

When the user explicitly asks to "start the project" (including equivalent intents such as starting both backend and frontend), execute this default flow without extra confirmation:

1. Start `admin` backend: run `go run . server` in `mss-boot-admin` (background).
2. Start `antd` frontend: run `pnpm dev` in `mss-boot-admin-antd` (background).
3. Use repository config as source of truth: backend follows `config/application.yml` `server.addr` (baseline usually `0.0.0.0:8080`), frontend dev port is usually `8000`.
4. After start, provide verifiable status (for example: reachable ports and no fatal terminal errors).
5. Keep services running unless the user explicitly asks to stop them.

In this repository, follow these mandatory rules when generating any prompts or documentation files:

1. Always create or update prompt/document files only under `aigc/prompts/` (or its subfolders).
2. Never create prompt/document files in the repository root.
3. If a requested path is outside `aigc/prompts/`, automatically redirect it to `aigc/prompts/`.
4. Use lowercase kebab-case filenames, and use `.zh-CN.md` suffix for Chinese variants when needed.
5. After writing files, return the actual written paths.

## Open-source collaboration constraints

This repository is open source. When generating comments, documentation, or prompts, follow these rules:

1. Use neutral, discussable, and verifiable language; avoid absolute wording.
2. Base conclusions on currently visible repository code/docs, and state scope/assumptions when needed.
3. Never include sensitive information (keys, credentials, private addresses, personal data, internal system details).
4. Prefer reproducible steps and validation methods so contributors can verify results.
5. Keep notes understandable to external contributors without relying on internal context.
