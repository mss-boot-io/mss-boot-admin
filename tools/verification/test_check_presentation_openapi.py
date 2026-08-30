from __future__ import annotations

import importlib.util
import unittest
from copy import deepcopy
from pathlib import Path


SCRIPT = Path(__file__).with_name("check_presentation_openapi.py")
SPEC = importlib.util.spec_from_file_location("check_presentation_openapi", SCRIPT)
assert SPEC and SPEC.loader
check_presentation_openapi = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(check_presentation_openapi)


def valid_document() -> dict:
    paths: dict = {}
    for (path, method), status in check_presentation_openapi.EXPECTED_OPERATIONS.items():
        paths.setdefault(path, {})[method] = {
            "tags": ["presentation"],
            "security": [{"Bearer": []}],
            "responses": {status: {"schema": {"$ref": "#/definitions/example"}}},
        }
    return {"swagger": "2.0", "paths": paths}


class PresentationOpenAPIContractTest(unittest.TestCase):
    def test_accepts_the_complete_typed_authenticated_contract(self) -> None:
        self.assertEqual(check_presentation_openapi.collect_errors(valid_document()), [])

    def test_rejects_missing_operation(self) -> None:
        document = valid_document()
        del document["paths"]["/admin/api/presentation-capabilities"]["get"]
        errors = check_presentation_openapi.collect_errors(document)
        self.assertIn(
            "missing presentation operation: GET /admin/api/presentation-capabilities",
            errors,
        )

    def test_rejects_untagged_and_unexpected_operations(self) -> None:
        document = valid_document()
        document["paths"]["/admin/api/presentation-capabilities"]["get"]["tags"] = []
        document["paths"]["/admin/api/presentation-extra"] = {
            "get": {
                "tags": ["presentation"],
                "security": [{"Bearer": []}],
                "responses": {"200": {"schema": {"type": "object"}}},
            }
        }
        errors = check_presentation_openapi.collect_errors(document)
        self.assertTrue(any("missing presentation operation" in error for error in errors))
        self.assertTrue(any("unexpected presentation operation" in error for error in errors))

    def test_rejects_missing_security_and_untyped_success(self) -> None:
        document = deepcopy(valid_document())
        operation = document["paths"]["/admin/api/presentation-profiles"]["post"]
        operation["security"] = []
        operation["responses"]["201"] = {"description": "created"}
        errors = check_presentation_openapi.collect_errors(document)
        self.assertIn(
            "presentation operation lacks Bearer security: "
            "POST /admin/api/presentation-profiles",
            errors,
        )
        self.assertIn(
            "presentation operation lacks typed 201 response: "
            "POST /admin/api/presentation-profiles",
            errors,
        )


if __name__ == "__main__":
    unittest.main()
