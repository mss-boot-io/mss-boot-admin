---
name: mss-add-permission
description: Add or change RBAC, API, menu, action, or row-level access rules in an MSS module, with backend enforcement and positive/negative tests. Trigger for role access, “only own records”, department scope, new actions, menu visibility, or authorization defects. Do not use for authentication protocol changes or frontend-only visibility tweaks.
---

# Add or change MSS authorization

Authorization is a backend security contract. UI visibility is a secondary projection of that contract.

## Procedure

1. Identify the protected resource, action, actors, scope, and exceptions.
2. Inspect the current capability and permission model:
   - module `permissions` in `module.yaml`;
   - Casbin role/menu/API rules;
   - route middleware and controller authorization;
   - ownership or department scope;
   - frontend access checks.
3. Write a permission matrix before implementation:

   | Actor/role | Resource scope | Action | Expected |
   | --- | --- | --- | --- |
   | admin | all | update | allow |
   | operator | own | update | allow |
   | operator | other | update | deny |
   | viewer | any | update | deny |

4. For a generated module, add or update a permission entry in `.mss/modules/<module>.yaml` and reference it from workflows or actions.
5. Validate and regenerate:

   ```shell
   go run ./cmd/mss spec validate .mss/modules/<module>.yaml
   go run ./cmd/mss module generate .mss/modules/<module>.yaml --write
   ```

6. Implement backend enforcement at the narrowest reliable boundary:
   - route/action permission for coarse resource operations;
   - service/repository scope for row-level ownership;
   - transition-specific permission for workflows;
   - explicit admin bypass only when defined by contract.
7. Ensure list/search queries apply the same scope as get/update/delete. Prevent ID enumeration from bypassing list filters.
8. Project permissions into frontend access checks and action visibility, but do not rely on them for enforcement.
9. Add tests for:
   - every allow row in the matrix;
   - every deny row;
   - missing/expired identity;
   - forged resource ID;
   - list and direct-resource consistency;
   - cache or policy reload behavior when rules change.
10. Verify:

   ```shell
   go run ./cmd/mss verify --module <module>
   go run ./cmd/mss verify --changed
   ```

## Guardrails

- Default to deny when identity, policy, ownership, or resource state is missing.
- Never expose a resource existence difference when the actor is not permitted to know it exists.
- Never use client-supplied owner or department IDs without server-side validation.
- Never make state-changing authorization decisions only in JavaScript.
- Do not broaden an existing default role without documenting compatibility and security impact.
- Audit sensitive permission changes without recording secrets or sensitive request bodies.

## Output

Include the permission matrix, fully qualified permission codes, enforcement locations, policy/migration impact, frontend projection, negative tests, and any backward-compatibility behavior.
