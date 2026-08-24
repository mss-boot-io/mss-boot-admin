---
name: mss-add-workflow
description: Add a finite-state business workflow to an MSS management module, including states, transitions, transition permissions, optimistic concurrency, audit events, UI actions, and tests. Trigger for approval, review, publish, close, cancel, activate, or other state-machine requirements. Do not use for background task scheduling or an unstructured status field.
---

# Add an MSS domain workflow

Model state transitions explicitly. Do not permit arbitrary status updates through generic CRUD endpoints.

## Procedure

1. Identify the workflow field and define:
   - all valid states;
   - initial state;
   - terminal states;
   - each named transition;
   - allowed source states;
   - target state;
   - permission required;
   - validation and side effects;
   - retry and idempotency behavior.
2. Write a transition table:

   | Transition | From | To | Permission | Side effects |
   | --- | --- | --- | --- | --- |

3. Update `.mss/modules/<module>.yaml`:
   - ensure the workflow field is an enum;
   - declare matching enum values;
   - add transition permissions;
   - add `workflow.states`, `workflow.initial`, and `workflow.transitions`;
   - add workflow events when consumers require them.
4. Validate semantic consistency:

   ```shell
   mss spec validate .mss/modules/<module>.yaml --format json
   ```

5. Regenerate the deterministic baseline.
6. Implement transitions as explicit backend commands or service methods, not as unrestricted generic update payloads.
7. Protect each transition with:
   - authenticated actor;
   - transition permission;
   - current-state check;
   - business invariant validation;
   - optimistic concurrency or equivalent stale-write detection;
   - transaction around state, audit, and durable outbox writes when they form one logical operation.
8. Emit events only after the state change is durable. Consumers must tolerate duplicate delivery.
9. Generate or implement frontend action buttons based on current state and permission. Display server rejection accurately when state changed concurrently.
10. Test:
    - every valid transition;
    - each invalid source state;
    - missing permission;
    - stale version/concurrent transition;
    - side-effect failure and rollback behavior;
    - repeated idempotency key;
    - event/audit record content without sensitive payloads.
11. Run module generation drift and focused verification.

## Guardrails

- Never accept arbitrary target states from a generic update endpoint.
- Never perform irreversible external side effects before the database transaction is durable.
- Never rely on frontend button visibility to prevent transitions.
- Never silently skip failed side effects; record a durable operation/event status.
- Do not use the system scheduled-task model as a substitute for a domain workflow.

## Output

Provide the transition table, API/command contract, permission mapping, concurrency strategy, event semantics, migration impact, and executed tests.
