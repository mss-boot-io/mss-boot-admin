from __future__ import annotations

import importlib.util
import unittest
from copy import deepcopy
from pathlib import Path
from unittest.mock import patch


SCRIPT = Path(__file__).with_name("check_presentation_openapi.py")
SPEC = importlib.util.spec_from_file_location("check_presentation_openapi", SCRIPT)
assert SPEC and SPEC.loader
check_presentation_openapi = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(check_presentation_openapi)


def _swagger_parameter(expected: dict) -> dict:
    parameter = {
        "name": expected["name"],
        "in": expected["in"],
        "required": expected["required"],
    }
    if expected["type"] is not None:
        parameter["type"] = expected["type"]
    if expected["$ref"] is not None:
        parameter["schema"] = {"$ref": expected["$ref"]}
    parameter.update(expected["constraints"])
    return parameter


def _definition_name(reference: str) -> str:
    return reference.removeprefix("#/definitions/")


def valid_document() -> dict:
    paths: dict = {}
    definitions: dict = {}
    for (path, method), expected in check_presentation_openapi.EXPECTED_OPERATIONS.items():
        responses = {
            expected["success_status"]: {
                "description": "success",
                "schema": {"$ref": expected["success_ref"]},
            }
        }
        definitions[_definition_name(expected["success_ref"])] = {
            "type": "object",
            "properties": {},
        }
        for status, reference in expected["responses"].items():
            responses[status] = {
                "description": "failure",
                "schema": {"$ref": reference},
            }
            definitions[_definition_name(reference)] = {
                "type": "object",
                "properties": {},
            }
        for status in expected["etag_statuses"]:
            responses[status]["headers"] = {
                "ETag": {"description": "strong profile ETag", "type": "string"}
            }
        parameters = [_swagger_parameter(item) for item in expected["parameters"]]
        for item in expected["parameters"]:
            if item["$ref"] is not None:
                definitions[_definition_name(item["$ref"])] = {
                    "type": "object",
                    "properties": {},
                }
        paths.setdefault(path, {})[method] = {
            "tags": ["presentation"],
            "security": [{"Bearer": []}],
            "parameters": parameters,
            "responses": responses,
        }
    for definition_name, properties in (
        check_presentation_openapi.RAW_JSON_OBJECT_PROPERTIES.items()
    ):
        definition = definitions.setdefault(
            definition_name, {"type": "object", "properties": {}}
        )
        definition["properties"].update(
            {property_name: {"type": "object"} for property_name in properties}
        )
    for definition_name, properties in (
        check_presentation_openapi.DEFINITION_PROPERTY_CONTRACTS.items()
    ):
        definition = definitions.setdefault(
            definition_name, {"type": "object", "properties": {}}
        )
        definition["type"] = "object"
        definition.setdefault("properties", {}).update(deepcopy(properties))
    for definition_name, required in (
        check_presentation_openapi.REQUIRED_DEFINITION_PROPERTIES.items()
    ):
        definitions[definition_name]["required"] = sorted(required)
    for definition_name, properties in (
        check_presentation_openapi.OPAQUE_JSON_PROPERTIES.items()
    ):
        definition = definitions.setdefault(
            definition_name, {"type": "object", "properties": {}}
        )
        definition.setdefault("properties", {}).update(
            {property_name: {} for property_name in properties}
        )
    while True:
        referenced = set()
        for definition in definitions.values():
            referenced.update(
                check_presentation_openapi._collect_reference_names(definition)
            )
        missing = referenced - set(definitions)
        if not missing:
            break
        for definition_name in missing:
            definitions[definition_name] = {"type": "object", "properties": {}}
    return {
        "swagger": "2.0",
        "securityDefinitions": {
            "Bearer": {
                "type": "apiKey",
                "name": "Authorization",
                "in": "header",
            }
        },
        "paths": paths,
        "definitions": definitions,
    }


def _reachable_definition_names(document: dict) -> set[str]:
    names: set[str] = set()
    for path, method in check_presentation_openapi.EXPECTED_OPERATIONS:
        path_item = document["paths"].get(path)
        operation = path_item.get(method) if isinstance(path_item, dict) else None
        if isinstance(operation, dict):
            names.update(check_presentation_openapi._collect_reference_names(operation))
    definitions = document["definitions"]
    pending = list(names)
    checked: set[str] = set()
    while pending:
        name = pending.pop()
        if name in checked:
            continue
        checked.add(name)
        definition = definitions.get(name)
        if isinstance(definition, dict):
            pending.extend(
                nested
                for nested in check_presentation_openapi._collect_reference_names(
                    definition
                )
                if nested not in checked
            )
    return checked


class PresentationOpenAPIContractTest(unittest.TestCase):
    def setUp(self) -> None:
        baseline = valid_document()
        expected_hashes = check_presentation_openapi.definition_shape_hashes(
            baseline, _reachable_definition_names(baseline)
        )
        hash_patch = patch.object(
            check_presentation_openapi,
            "EXPECTED_DEFINITION_SHAPE_HASHES",
            expected_hashes,
        )
        hash_patch.start()
        self.addCleanup(hash_patch.stop)

    def test_accepts_the_complete_typed_authenticated_contract(self) -> None:
        self.assertEqual(check_presentation_openapi.collect_errors(valid_document()), [])

    def test_rejects_missing_and_unexpected_operations(self) -> None:
        document = valid_document()
        del document["paths"]["/admin/api/presentation-capabilities"]["get"]
        document["paths"]["/admin/api/presentation-extra"] = {
            "get": {
                "tags": ["presentation"],
                "security": [{"Bearer": []}],
                "parameters": [],
                "responses": {},
            }
        }
        errors = check_presentation_openapi.collect_errors(document)
        self.assertIn(
            "missing presentation operation: GET /admin/api/presentation-capabilities",
            errors,
        )
        self.assertIn(
            "unexpected presentation operation: GET /admin/api/presentation-extra",
            errors,
        )

    def test_rejects_missing_global_security_parameters_and_typed_success(self) -> None:
        document = deepcopy(valid_document())
        del document["securityDefinitions"]
        operation = document["paths"]["/admin/api/presentation-profiles"]["post"]
        operation["security"] = [{"Bearer": ["bogus"]}]
        operation["parameters"] = []
        operation["responses"]["201"]["schema"] = {}
        errors = check_presentation_openapi.collect_errors(document)
        self.assertIn(
            "global Bearer apiKey security definition is missing or malformed", errors
        )
        self.assertIn(
            "POST /admin/api/presentation-profiles must require exact Bearer security",
            errors,
        )
        self.assertTrue(
            any("missing parameter" in error and "If-None-Match" in error for error in errors)
        )
        self.assertTrue(
            any("response 201" in error and "wrong schema" in error for error in errors)
        )

    def test_rejects_optional_idempotency_key_and_wrong_conflict_schema(self) -> None:
        document = deepcopy(valid_document())
        operation = document["paths"][
            "/admin/api/presentation-profiles/{id}/publish"
        ]["post"]
        idempotency_key = next(
            parameter
            for parameter in operation["parameters"]
            if parameter["name"] == "Idempotency-Key"
        )
        idempotency_key["required"] = False
        del idempotency_key["minLength"]
        operation["responses"]["412"]["schema"]["$ref"] = (
            check_presentation_openapi.GENERIC_RESPONSE_REF
        )
        errors = check_presentation_openapi.collect_errors(document)
        self.assertTrue(
            any("Idempotency-Key" in error and "required flag" in error for error in errors)
        )
        self.assertTrue(
            any(
                "Idempotency-Key" in error and "validation constraints" in error
                for error in errors
            )
        )
        self.assertTrue(
            any("response 412" in error and "wrong schema" in error for error in errors)
        )

    def test_rejects_byte_array_json_and_missing_precondition_response(self) -> None:
        document = deepcopy(valid_document())
        document["definitions"]["dto.PresentationProfileCreateRequest"]["properties"][
            "document"
        ] = {"type": "array", "items": {"type": "integer"}}
        del document["paths"]["/admin/api/presentation-profiles"]["post"]["responses"][
            "428"
        ]
        errors = check_presentation_openapi.collect_errors(document)
        self.assertIn(
            "dto.PresentationProfileCreateRequest.document must be documented as a JSON object",
            errors,
        )
        self.assertIn(
            "missing response 428 on POST /admin/api/presentation-profiles", errors
        )

    def test_rejects_duplicate_parameters_and_removed_pagination_bounds(self) -> None:
        document = deepcopy(valid_document())
        operation = document["paths"]["/admin/api/presentation-profiles"]["get"]
        page = next(
            parameter
            for parameter in operation["parameters"]
            if parameter["name"] == "page"
        )
        del page["minimum"]
        operation["parameters"].append(deepcopy(page))
        errors = check_presentation_openapi.collect_errors(document)
        self.assertIn(
            "duplicate parameter query:page on GET /admin/api/presentation-profiles",
            errors,
        )
        self.assertTrue(
            any(
                "query:page" in error and "validation constraints" in error
                for error in errors
            )
        )

    def test_rejects_empty_success_definition_and_missing_body_requirement(self) -> None:
        document = deepcopy(valid_document())
        document["definitions"]["dto.EffectivePresentationResponse"] = {
            "type": "object",
            "properties": {},
        }
        required = document["definitions"][
            "dto.PresentationProfileCreateRequest"
        ]["required"]
        required.remove("document")
        errors = check_presentation_openapi.collect_errors(document)
        self.assertIn(
            "dto.EffectivePresentationResponse.pageKey has wrong or missing schema",
            errors,
        )
        self.assertIn(
            "dto.PresentationProfileCreateRequest has wrong required properties; "
            "expected ['document', 'pageKey', 'scope']",
            errors,
        )

    def test_rejects_condition_value_as_a_byte_array(self) -> None:
        document = deepcopy(valid_document())
        document["definitions"]["presentation.Condition"]["properties"]["value"] = {
            "type": "array",
            "items": {"type": "integer"},
        }
        errors = check_presentation_openapi.collect_errors(document)
        self.assertIn(
            "presentation.Condition.value must be documented as opaque JSON", errors
        )

    def test_rejects_recursive_definition_shape_drift(self) -> None:
        document = deepcopy(valid_document())
        document["definitions"]["dto.PresentationProfileSummary"]["properties"][
            "unexpected"
        ] = {"type": "string"}
        errors = check_presentation_openapi.collect_errors(document)
        self.assertIn(
            "presentation definition shape drift: dto.PresentationProfileSummary",
            errors,
        )

    def test_rejects_rollback_revision_without_minimum(self) -> None:
        document = deepcopy(valid_document())
        del document["definitions"]["dto.PresentationRollbackRequest"]["properties"][
            "revision"
        ]["minimum"]
        errors = check_presentation_openapi.collect_errors(document)
        self.assertIn(
            "dto.PresentationRollbackRequest.revision has wrong or missing schema",
            errors,
        )
        self.assertIn(
            "presentation definition shape drift: dto.PresentationRollbackRequest",
            errors,
        )


if __name__ == "__main__":
    unittest.main()
