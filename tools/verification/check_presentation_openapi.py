#!/usr/bin/env python3
"""Verify the generated Swagger contract for runtime presentation APIs."""

from __future__ import annotations

import json
import sys
from pathlib import Path
from typing import Any


EXPECTED_OPERATIONS = {
    ("/admin/api/presentation-capabilities", "get"): "200",
    ("/admin/api/presentation-profiles/validate", "post"): "200",
    ("/admin/api/presentation-profiles", "get"): "200",
    ("/admin/api/presentation-profiles", "post"): "201",
    ("/admin/api/presentation-profiles/{id}", "get"): "200",
    ("/admin/api/presentation-profiles/{id}/draft", "put"): "200",
    ("/admin/api/presentation-profiles/{id}/publish", "post"): "200",
    ("/admin/api/presentation-profiles/{id}/rollback", "post"): "200",
    ("/admin/api/presentation-profiles/{id}/revisions", "get"): "200",
    (
        "/admin/api/presentation-profiles/{id}/revisions/{revision}",
        "get",
    ): "200",
    ("/admin/api/presentation/effective/{pageKey}", "get"): "200",
}


def collect_errors(document: dict[str, Any]) -> list[str]:
    errors: list[str] = []
    paths = document.get("paths")
    if not isinstance(paths, dict):
        return ["Swagger document has no paths object"]

    tagged_operations: set[tuple[str, str]] = set()
    for path, path_item in paths.items():
        if not isinstance(path_item, dict):
            continue
        for method, operation in path_item.items():
            if not isinstance(operation, dict):
                continue
            tags = operation.get("tags")
            if isinstance(tags, list) and "presentation" in tags:
                tagged_operations.add((path, method.lower()))

    expected_keys = set(EXPECTED_OPERATIONS)
    for path, method in sorted(expected_keys - tagged_operations):
        errors.append(f"missing presentation operation: {method.upper()} {path}")
    for path, method in sorted(tagged_operations - expected_keys):
        errors.append(f"unexpected presentation operation: {method.upper()} {path}")

    for (path, method), success_status in sorted(EXPECTED_OPERATIONS.items()):
        path_item = paths.get(path)
        operation = path_item.get(method) if isinstance(path_item, dict) else None
        if not isinstance(operation, dict):
            continue
        security = operation.get("security")
        if not isinstance(security, list) or not any(
            isinstance(entry, dict) and "Bearer" in entry for entry in security
        ):
            errors.append(
                f"presentation operation lacks Bearer security: {method.upper()} {path}"
            )
        responses = operation.get("responses")
        response = responses.get(success_status) if isinstance(responses, dict) else None
        if not isinstance(response, dict) or not isinstance(response.get("schema"), dict):
            errors.append(
                "presentation operation lacks typed "
                f"{success_status} response: {method.upper()} {path}"
            )

    return errors


def main(argv: list[str]) -> int:
    if len(argv) != 2:
        print("usage: check_presentation_openapi.py <swagger.json>", file=sys.stderr)
        return 2
    swagger_path = Path(argv[1])
    try:
        document = json.loads(swagger_path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as error:
        print(f"cannot read Swagger document {swagger_path}: {error}", file=sys.stderr)
        return 2
    errors = collect_errors(document)
    if errors:
        for error in errors:
            print(error, file=sys.stderr)
        return 1
    print(
        "verified runtime presentation OpenAPI contract: "
        f"{len(EXPECTED_OPERATIONS)} operations"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
