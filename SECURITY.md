# Security Policy

## Reporting a vulnerability

Do not open a public issue for suspected vulnerabilities.

Use GitHub Security Advisories for this repository when private vulnerability
reporting is enabled. If that setting is not available yet, contact the
maintainer privately and include the affected commit/tag, impact, reproduction
steps, and any proof of concept.

The organization still needs a final public security contact. Until then,
private GitHub advisories are the preferred intake path.

## Supported versions

The active `main` branch and the current stable Complete Admin Distribution are
supported by default. During v1.3.0 preparation the reconciled stable remains
v1.2.3; after all v1.3.0 public artifacts and the follow-up reconciliation are
complete, the machine-readable policy advances the stable line to v1.3.0.
Preview and release-candidate refs remain immutable evidence but do not receive
the stable support commitment. Older stable versions are handled case by case.

## Response expectations

- Acknowledge valid private reports within 7 days when possible.
- Triage severity, affected versions, and exploitability before public
  disclosure.
- Prepare a fix, release note, image tag, and upgrade guidance before disclosing
  details.
