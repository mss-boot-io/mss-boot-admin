---
name: mss-debug-fullstack
description: Diagnose and fix a failing MSS Foundation or Thin Host backend, frontend, migration, generated module, CI job, or local integration path from reproducible evidence. Do not use to add a feature when no failure exists.
---

# Debug an MSS full-stack failure

Work from the smallest reproducible failure toward the root cause. Do not mask a defect by weakening tests, swallowing errors, or broadening security controls.

## Command and layout context

- In a Thin Host, use the installed `mss` v1.3.6 binary.
- In the Foundation repository, substitute `GOWORK=off go run ./cmd/mss` only when testing unpublished CLI changes.
- Always read `mss context --format json`; backend, frontend, module, and generated paths differ between the two layouts.

## Procedure

1. Capture the exact failing command, status code, stack trace, test name, workflow run, or browser symptom.
2. Record context and environment evidence:

   ```shell
   mss context --format json
   mss doctor --format json
   ```

3. Classify the failure as toolchain/setup, generated drift, Go compile/test, database migration, authentication/authorization, frontend type/build/runtime, proxy/network, optional integration, or CI-only behavior.
4. Reproduce with the narrowest command and preserve the first complete failure before editing.
5. Trace one request or operation end to end: frontend route/service, HTTP contract, middleware/controller, service/repository, database or adapter, response, and visible state.
6. For a generated module, compare the source contract with generated output:

   ```shell
   mss spec validate .mss/modules/<module>.yaml --format json
   mss module generate .mss/modules/<module>.yaml --check
   ```

   Remember that the public v1.3.6 profile generates only its documented simple CRUD surface; do not diagnose an unsupported relation, import, workflow, or row-scope request as ordinary generated drift.

7. Form a falsifiable hypothesis, test it with a focused assertion or regression test, and implement the smallest correct repair.
8. Run the focused test first, then:

   ```shell
   mss verify --changed
   ```

9. If CI differs from local behavior, compare working directory, environment variables, Go/Node/pnpm versions, lockfiles, Git base, generated files, and dependency readiness.

## Guardrails

- Do not delete or skip a test solely to make CI green.
- Do not add sleeps when readiness can prove state.
- Do not log passwords, tokens, sensitive request bodies, or production data.
- Do not hand-replace deterministic generated output.
- Do not broaden CORS, authentication, or authorization as a shortcut.
- Do not claim unrun browser, migration, or external-integration validation.

## Output

Report failing evidence, root cause, changed files, regression test, exact commands and outcomes, layout/environment assumptions, and remaining unverified paths.
