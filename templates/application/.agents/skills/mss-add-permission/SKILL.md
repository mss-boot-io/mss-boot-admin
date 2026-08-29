---
name: mss-add-permission
description: Add or change coarse backend RBAC, API, menu, or action permissions in a Thin Host v1.3.7 module. Trigger for role access or authorization defects. Row-level ownership and department scope require separate handwritten enforcement.
---

# Add a coarse module permission

The generated v1.3.7 profile projects module permission codes into backend API/RBAC registration and frontend menu/action visibility while `ownership.mode` remains `none`.

1. Write an allow/deny matrix for resource, action, and role.
2. Inspect `.mss/modules/<module>.yaml`, current routes, generated authorization migration, middleware, and frontend access checks.
3. Update `spec.permissions` and keep actions aligned with the declared API operations.
4. Validate, inspect, write, and check:

   ```shell
   mss spec validate .mss/modules/<module>.yaml --format json
   mss module generate .mss/modules/<module>.yaml --format json
   mss module generate .mss/modules/<module>.yaml --write
   mss module generate .mss/modules/<module>.yaml --check
   ```

5. Confirm backend enforcement is authoritative and add positive/negative tests, including missing identity and a forged direct-resource request where applicable.
6. Run `mss verify --module <module>`.

Row-level ownership, department or tenant scope, state-transition policy, and resource-dependent decisions are not generated. Fail closed, create a separate Feature specification, and implement backend list and direct-resource enforcement in handwritten business code. Frontend hiding never replaces authorization.
