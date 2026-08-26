# Codex integration for Foundation contributors

This directory configures Codex only for contributors working inside the
`mss-boot-admin` Foundation source checkout. It is not the v1.3.5 application
quick start and must not be copied into a Thin Host. Generated applications use
the installed `mss` and `mss-mcp` commands instead.

## MCP

`.codex/config.toml` deliberately starts the checked-in stdio MCP server in
Foundation contributor mode:

```shell
go run ./cmd/mss-mcp --root .
```

Read-only tools inspect project contracts, Skills, specifications, validation plans, downstream Blueprints, and upgrade status. Write-capable tools use `writes` approval mode and still enforce CLI-level dry-run, confirmation, path, and conflict checks.

## Cloud environment setup

Configure the Codex environment setup command as:

```shell
bash .codex/setup.sh
```

The contributor setup script:

- runs from the repository root;
- uses the canonical `mss setup` implementation;
- does not require production credentials;
- performs required environment diagnostics;
- validates repository Skills;
- stores local JSON context under ignored `.mss/cache/`.

After setup, a Foundation task should begin with:

```shell
go run ./cmd/mss context --format json
go run ./cmd/mss verify --changed --plan --format json
```

Do not place API tokens, personal paths, kubeconfigs, production DSNs, or tool-specific architecture rules in `.codex/`.
