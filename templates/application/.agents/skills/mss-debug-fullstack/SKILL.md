---
name: mss-debug-fullstack
description: Diagnose and fix a Thin Host backend, frontend, migration, generated module, CI, or local integration failure from reproducible evidence. Do not use to add a feature when no failure exists.
---

# Debug a Thin Host failure

1. Capture the exact failing command, status, stack trace, test, CI run, or browser symptom.
2. Record project evidence with the installed binary:

   ```shell
   mss context --format json
   mss doctor --format json
   ```

3. Reproduce with the narrowest command and trace one request through `web/`, HTTP middleware/handler, `internal/modules/`, database or adapter, and the visible response.
4. For generated output, compare the source contract:

   ```shell
   mss spec validate .mss/modules/<module>.yaml --format json
   mss module generate .mss/modules/<module>.yaml --check
   ```

5. Treat relation, import, workflow, or row-scope requests as unsupported profile work, not ordinary generated drift.
6. Test a falsifiable root-cause hypothesis, implement the smallest repair, add a regression test, then run `mss verify --changed`.

Do not skip tests, add blind sleeps, expose secrets, hand-replace generated files, or broaden CORS/authentication/authorization. Report evidence, root cause, changed files, exact validation, and remaining unverified paths.
