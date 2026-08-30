#!/usr/bin/env python3
"""Verify the generated Swagger contract for runtime presentation APIs."""

from __future__ import annotations

import json
import sys
from pathlib import Path
from typing import Any


GENERIC_RESPONSE_REF = "#/definitions/response.Response"
CONFLICT_RESPONSE_REF = "#/definitions/dto.PresentationConflictResponse"


def _parameter(
    name: str,
    location: str,
    *,
    required: bool,
    primitive_type: str | None = None,
    schema_ref: str | None = None,
) -> dict[str, Any]:
    return {
        "name": name,
        "in": location,
        "required": required,
        "type": primitive_type,
        "$ref": schema_ref,
    }


def _operation(
    success_status: str,
    success_ref: str,
    *,
    parameters: tuple[dict[str, Any], ...] = (),
    failures: tuple[str, ...] = (),
    conflict_statuses: tuple[str, ...] = (),
    etag_statuses: tuple[str, ...] = (),
) -> dict[str, Any]:
    conflict = set(conflict_statuses)
    return {
        "success_status": success_status,
        "success_ref": success_ref,
        "parameters": parameters,
        "responses": {
            status: (
                CONFLICT_RESPONSE_REF if status in conflict else GENERIC_RESPONSE_REF
            )
            for status in failures
        },
        "etag_statuses": set(etag_statuses),
    }


ID = _parameter("id", "path", required=True, primitive_type="string")
REVISION = _parameter("revision", "path", required=True, primitive_type="integer")
PAGE = _parameter("page", "query", required=False, primitive_type="integer")
PAGE_SIZE = _parameter("pageSize", "query", required=False, primitive_type="integer")
SCOPE = _parameter("scope", "query", required=False, primitive_type="string")
PAGE_KEY_QUERY = _parameter(
    "pageKey", "query", required=False, primitive_type="string"
)
PAGE_KEY_PATH = _parameter("pageKey", "path", required=True, primitive_type="string")
IF_NONE_MATCH = _parameter(
    "If-None-Match", "header", required=True, primitive_type="string"
)
IF_MATCH = _parameter("If-Match", "header", required=True, primitive_type="string")
IDEMPOTENCY_KEY = _parameter(
    "Idempotency-Key", "header", required=True, primitive_type="string"
)


EXPECTED_OPERATIONS: dict[tuple[str, str], dict[str, Any]] = {
    ("/admin/api/presentation-capabilities", "get"): _operation(
        "200",
        "#/definitions/dto.PresentationCapabilityListResponse",
        failures=("401", "403"),
    ),
    ("/admin/api/presentation-profiles/validate", "post"): _operation(
        "200",
        "#/definitions/dto.PresentationValidationResponse",
        parameters=(
            _parameter(
                "data",
                "body",
                required=True,
                schema_ref="#/definitions/dto.PresentationValidationRequest",
            ),
        ),
        failures=("401", "403", "413", "422"),
    ),
    ("/admin/api/presentation-profiles", "get"): _operation(
        "200",
        "#/definitions/dto.PresentationProfileListResponse",
        parameters=(PAGE, PAGE_SIZE, SCOPE, PAGE_KEY_QUERY),
        failures=("401", "403", "422", "503"),
    ),
    ("/admin/api/presentation-profiles", "post"): _operation(
        "201",
        "#/definitions/dto.PresentationProfileResource",
        parameters=(
            IF_NONE_MATCH,
            _parameter(
                "data",
                "body",
                required=True,
                schema_ref="#/definitions/dto.PresentationProfileCreateRequest",
            ),
        ),
        failures=("400", "401", "403", "409", "413", "422", "428", "503"),
        etag_statuses=("201",),
    ),
    ("/admin/api/presentation-profiles/{id}", "get"): _operation(
        "200",
        "#/definitions/dto.PresentationProfileResource",
        parameters=(ID,),
        failures=("401", "403", "404", "422", "503"),
        etag_statuses=("200",),
    ),
    ("/admin/api/presentation-profiles/{id}/draft", "put"): _operation(
        "200",
        "#/definitions/dto.PresentationProfileResource",
        parameters=(
            ID,
            IF_MATCH,
            _parameter(
                "data",
                "body",
                required=True,
                schema_ref="#/definitions/dto.PresentationDraftReplaceRequest",
            ),
        ),
        failures=(
            "400",
            "401",
            "403",
            "404",
            "409",
            "412",
            "413",
            "422",
            "428",
            "503",
        ),
        conflict_statuses=("412",),
        etag_statuses=("200", "412"),
    ),
    ("/admin/api/presentation-profiles/{id}/publish", "post"): _operation(
        "200",
        "#/definitions/dto.PresentationTransitionResponse",
        parameters=(ID, IF_MATCH, IDEMPOTENCY_KEY),
        failures=("400", "401", "403", "404", "409", "412", "422", "428", "503"),
        conflict_statuses=("412",),
        etag_statuses=("200", "412"),
    ),
    ("/admin/api/presentation-profiles/{id}/rollback", "post"): _operation(
        "200",
        "#/definitions/dto.PresentationTransitionResponse",
        parameters=(
            ID,
            IF_MATCH,
            IDEMPOTENCY_KEY,
            _parameter(
                "data",
                "body",
                required=True,
                schema_ref="#/definitions/dto.PresentationRollbackRequest",
            ),
        ),
        failures=(
            "400",
            "401",
            "403",
            "404",
            "409",
            "412",
            "413",
            "422",
            "428",
            "503",
        ),
        conflict_statuses=("412",),
        etag_statuses=("200", "412"),
    ),
    ("/admin/api/presentation-profiles/{id}/revisions", "get"): _operation(
        "200",
        "#/definitions/dto.PresentationRevisionListResponse",
        parameters=(ID, PAGE, PAGE_SIZE),
        failures=("401", "403", "404", "422", "503"),
    ),
    (
        "/admin/api/presentation-profiles/{id}/revisions/{revision}",
        "get",
    ): _operation(
        "200",
        "#/definitions/dto.PresentationRevisionResource",
        parameters=(ID, REVISION),
        failures=("401", "403", "404", "422", "503"),
    ),
    ("/admin/api/presentation/effective/{pageKey}", "get"): _operation(
        "200",
        "#/definitions/dto.EffectivePresentationResponse",
        parameters=(PAGE_KEY_PATH,),
        failures=("401", "403", "422", "503"),
    ),
}


RAW_JSON_OBJECT_PROPERTIES = {
    "dto.PresentationProfileCreateRequest": ("document",),
    "dto.PresentationDraftReplaceRequest": ("document",),
    "dto.PresentationValidationRequest": ("document",),
    "dto.PresentationDraftResource": ("document",),
    "dto.PresentationRevisionResource": ("document",),
    "dto.PresentationValidationResponse": ("canonicalDocument",),
    "dto.EffectivePresentationLayers": ("application", "role", "user"),
}


def _parameter_key(parameter: dict[str, Any]) -> tuple[Any, Any]:
    return parameter.get("name"), parameter.get("in")


def _definition_name(reference: Any) -> str | None:
    prefix = "#/definitions/"
    if isinstance(reference, str) and reference.startswith(prefix):
        return reference[len(prefix) :]
    return None


def _check_parameter(
    actual: dict[str, Any], expected: dict[str, Any], operation_label: str
) -> list[str]:
    errors: list[str] = []
    key = _parameter_key(expected)
    label = f"parameter {key[1]}:{key[0]} on {operation_label}"
    if bool(actual.get("required", False)) != expected["required"]:
        errors.append(f"{label} has wrong required flag")
    if expected["type"] is not None and actual.get("type") != expected["type"]:
        errors.append(f"{label} has wrong type; expected {expected['type']}")
    if expected["$ref"] is not None:
        schema = actual.get("schema")
        reference = schema.get("$ref") if isinstance(schema, dict) else None
        if reference != expected["$ref"]:
            errors.append(f"{label} has wrong schema; expected {expected['$ref']}")
    return errors


def _check_response(
    response: Any,
    expected_ref: str,
    status: str,
    operation_label: str,
    *,
    etag_required: bool,
) -> list[str]:
    errors: list[str] = []
    if not isinstance(response, dict):
        return [f"missing response {status} on {operation_label}"]
    schema = response.get("schema")
    reference = schema.get("$ref") if isinstance(schema, dict) else None
    if reference != expected_ref:
        errors.append(
            f"response {status} on {operation_label} has wrong schema; "
            f"expected {expected_ref}"
        )
    headers = response.get("headers")
    etag = headers.get("ETag") if isinstance(headers, dict) else None
    if etag_required:
        if not isinstance(etag, dict) or etag.get("type") != "string":
            errors.append(f"response {status} on {operation_label} lacks string ETag")
    elif isinstance(headers, dict) and "ETag" in headers:
        errors.append(f"response {status} on {operation_label} has unexpected ETag")
    return errors


def collect_errors(document: dict[str, Any]) -> list[str]:
    errors: list[str] = []
    if document.get("swagger") != "2.0":
        errors.append("document must declare Swagger 2.0")

    security_definitions = document.get("securityDefinitions")
    bearer = (
        security_definitions.get("Bearer")
        if isinstance(security_definitions, dict)
        else None
    )
    expected_bearer = {
        "type": "apiKey",
        "name": "Authorization",
        "in": "header",
    }
    if bearer != expected_bearer:
        errors.append("global Bearer apiKey security definition is missing or malformed")

    paths = document.get("paths")
    if not isinstance(paths, dict):
        return errors + ["Swagger document has no paths object"]

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

    referenced_definitions: set[str] = set()
    for (path, method), expected in sorted(EXPECTED_OPERATIONS.items()):
        operation_label = f"{method.upper()} {path}"
        path_item = paths.get(path)
        operation = path_item.get(method) if isinstance(path_item, dict) else None
        if not isinstance(operation, dict):
            continue

        if operation.get("security") != [{"Bearer": []}]:
            errors.append(f"{operation_label} must require exact Bearer security")

        actual_parameters = operation.get("parameters", [])
        if not isinstance(actual_parameters, list) or not all(
            isinstance(parameter, dict) for parameter in actual_parameters
        ):
            errors.append(f"{operation_label} has malformed parameters")
            actual_parameters = []
        expected_parameters = {
            _parameter_key(parameter): parameter
            for parameter in expected["parameters"]
        }
        actual_parameter_map = {
            _parameter_key(parameter): parameter for parameter in actual_parameters
        }
        for key in sorted(expected_parameters.keys() - actual_parameter_map.keys()):
            errors.append(f"missing parameter {key[1]}:{key[0]} on {operation_label}")
        for key in sorted(actual_parameter_map.keys() - expected_parameters.keys()):
            errors.append(f"unexpected parameter {key[1]}:{key[0]} on {operation_label}")
        for key in sorted(expected_parameters.keys() & actual_parameter_map.keys()):
            errors.extend(
                _check_parameter(
                    actual_parameter_map[key], expected_parameters[key], operation_label
                )
            )
            reference = expected_parameters[key]["$ref"]
            definition_name = _definition_name(reference)
            if definition_name is not None:
                referenced_definitions.add(definition_name)

        responses = operation.get("responses")
        if not isinstance(responses, dict):
            errors.append(f"{operation_label} has no responses object")
            continue
        expected_responses = {
            expected["success_status"]: expected["success_ref"],
            **expected["responses"],
        }
        actual_statuses = set(responses)
        expected_statuses = set(expected_responses)
        for status in sorted(expected_statuses - actual_statuses):
            errors.append(f"missing response {status} on {operation_label}")
        for status in sorted(actual_statuses - expected_statuses):
            errors.append(f"unexpected response {status} on {operation_label}")
        for status, reference in sorted(expected_responses.items()):
            errors.extend(
                _check_response(
                    responses.get(status),
                    reference,
                    status,
                    operation_label,
                    etag_required=status in expected["etag_statuses"],
                )
            )
            definition_name = _definition_name(reference)
            if definition_name is not None:
                referenced_definitions.add(definition_name)

    definitions = document.get("definitions")
    if not isinstance(definitions, dict):
        errors.append("Swagger document has no definitions object")
        definitions = {}
    for name in sorted(referenced_definitions - set(definitions)):
        errors.append(f"referenced definition is missing: {name}")

    for definition_name, property_names in RAW_JSON_OBJECT_PROPERTIES.items():
        definition = definitions.get(definition_name)
        properties = definition.get("properties") if isinstance(definition, dict) else None
        if not isinstance(properties, dict):
            errors.append(f"definition is missing object properties: {definition_name}")
            continue
        for property_name in property_names:
            property_schema = properties.get(property_name)
            if not isinstance(property_schema, dict) or property_schema.get("type") != "object":
                errors.append(
                    f"{definition_name}.{property_name} must be documented as a JSON object"
                )
            elif "items" in property_schema:
                errors.append(
                    f"{definition_name}.{property_name} must not be documented as an array"
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
