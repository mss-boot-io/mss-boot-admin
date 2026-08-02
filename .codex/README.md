# Codex project integration

This directory contains repository-local Codex configuration without credentials.

## MCP

`.codex/config.toml` starts the checked-in stdio MCP server:

```shell
go run ./cmd/mss-mcp --root .
```

Read-only tools inspect project contracts, Skills, specifications, validation plans, downstream Blueprints, and upgrade status. Write-capable tools use `writes` approval mode and still enforce CLI-level dry-run, confirmation, path, and conflict checks.

## Cloud environment setup

Configure the Codex environment setup command as:

```shell
bash .codex/setup.sh
```

The script:

- runs from the repository root;
- uses the canonical `mss setup` implementation;
- does not require production credentials;
- performs required environment diagnostics;
- validates repository Skills;
- stores local JSON context under ignored `.mss/cache/`.

After setup, a task should begin with:

```shell
go run ./cmd/mss context --format json
go run ./cmd/mss verify --changed --plan --format json
```

Do not place API tokens, personal paths, kubeconfigs, production DSNs, or tool-specific architecture rules in `.codex/`.
