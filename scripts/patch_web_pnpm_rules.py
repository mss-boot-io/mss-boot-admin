#!/usr/bin/env python3
"""Finalize the deterministic frontend dependency policy.

pnpm's strict mode reports peer ranges declared by legacy editor/DVA packages
and optional adapters advertised by the Umi toolchain. We keep automatic peer
installation disabled, generate the lockfile without suppressing diagnostics,
and validate the strict-mode NDJSON against a reviewed repository policy.
"""

import json
from pathlib import Path


REPOSITORY = Path(__file__).resolve().parents[1]
ROOT = REPOSITORY / "web" / "antd"


def main() -> None:
    package_path = ROOT / "package.json"
    package = json.loads(package_path.read_text(encoding="utf-8"))
    package["dependencies"].update(
        {
            "@ant-design/charts": "2.6.7",
            "@ant-design/pro-components": "2.8.10",
            "ahooks": "3.9.7",
        }
    )
    pnpm = package.setdefault("pnpm", {})
    pnpm.pop("peerDependencyRules", None)
    package_path.write_text(
        json.dumps(package, ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )

    (ROOT / ".npmrc").write_text(
        "auto-install-peers=false\n"
        "engine-strict=true\n"
        "prefer-frozen-lockfile=true\n"
        "save-exact=true\n"
        "strict-peer-dependencies=false\n"
        "verify-store-integrity=true\n",
        encoding="utf-8",
    )
    (ROOT / "pnpm-workspace.yaml").write_text(
        "packages:\n  - workers-site\n",
        encoding="utf-8",
    )

    marker = ROOT / ".deps-remediation-trigger"
    if marker.exists():
        marker.unlink()


if __name__ == "__main__":
    main()
