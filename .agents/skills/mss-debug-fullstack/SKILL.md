---
name: mss-debug-fullstack
description: Diagnose and fix a failing mss-boot-admin backend, frontend, migration, generated module, CI job, or local integration path using reproducible evidence. Trigger for build failures, tests, 4xx/5xx errors, blank pages, proxy issues, migration failures, or inconsistent generated output. Do not use for adding a new feature when no failure exists.
---

# Debug an MSS full-stack failure

Work from the smallest reproducible failure toward the root cause. Do not mask a defect by weakening tests or swallowing errors.

## Procedure

1. Capture the exact failing command, status code, stack trace, test name, workflow run, or browser symptom.
2. Run context and environment checks:

   ```shell
   go run ./cmd/mss context --format json
   go run ./cmd/mss doctor --format json
   ```

3. Classify the failure:
   - toolchain/setup;
   - generated contract drift;
   - Go compile/test;
   - database migration/data;
   - authentication/authorization;
   - frontend type/build/runtime;
   - proxy/network;
   - optional integration;
   - CI/workflow-only behavior.
4. Reproduce with the narrowest command. Preserve the complete first failure before changing code.
5. Trace one request or operation end to end:
   - frontend route and service;
   - HTTP method/path/body;
   - router/controller/middleware;
   - service/repository;
   - database or external adapter;
   - response and frontend state.
6. Compare machine-readable contracts and generated output:

   ```shell
   go run ./cmd/mss spec validate .mss/modules/<module>.yaml
   go run ./cmd/mss module generate .mss/modules/<module>.yaml --check
   ```

7. Form a falsifiable root-cause hypothesis and test it with logging, a focused unit test, a temporary assertion, or a minimal reproducer.
8. Implement the smallest correct fix. Add a regression test that fails before the fix.
9. Commit the fix before broad validation when the workspace may be lost.
10. Run focused checks, then:

    ```shell
    go run ./cmd/mss verify --changed
    ```

11. When CI differs from local behavior, compare:
    - working directory;
    - environment variables;
    - Go/Node/pnpm versions;
    - lockfile and cache keys;
    - Git base ref and generated files;
    - service availability and timeouts.

## Guardrails

- Do not delete or skip a test solely to make CI green.
- Do not add sleeps when a health check or readiness condition can prove state.
- Do not log passwords, tokens, request bodies containing secrets, or production data.
- Do not call `os.Exit` from reusable packages to hide error propagation.
- Do not replace a deterministic generated file with a handwritten copy.
- Do not broaden CORS, authentication, or authorization as a debugging shortcut.

## Output

Report the failing evidence, root cause, changed files, regression test, commands and outcomes, environmental assumptions, and any remaining unverified path.
