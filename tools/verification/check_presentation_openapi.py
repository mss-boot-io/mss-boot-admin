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
    minimum: int | None = None,
    maximum: int | None = None,
    min_length: int | None = None,
    max_length: int | None = None,
    enum: tuple[str, ...] | None = None,
) -> dict[str, Any]:
    constraints = {
        key: value
        for key, value in {
            "minimum": minimum,
            "maximum": maximum,
            "minLength": min_length,
            "maxLength": max_length,
            "enum": list(enum) if enum is not None else None,
        }.items()
        if value is not None
    }
    return {
        "name": name,
        "in": location,
        "required": required,
        "type": primitive_type,
        "$ref": schema_ref,
        "constraints": constraints,
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


ID = _parameter(
    "id", "path", required=True, primitive_type="string", max_length=64
)
REVISION = _parameter(
    "revision", "path", required=True, primitive_type="integer", minimum=1
)
PAGE = _parameter(
    "page",
    "query",
    required=False,
    primitive_type="integer",
    minimum=1,
    maximum=1_000_000,
)
PAGE_SIZE = _parameter(
    "pageSize",
    "query",
    required=False,
    primitive_type="integer",
    minimum=1,
    maximum=100,
)
SCOPE = _parameter(
    "scope",
    "query",
    required=False,
    primitive_type="string",
    enum=("application", "role", "user"),
)
PAGE_KEY_QUERY = _parameter(
    "pageKey",
    "query",
    required=False,
    primitive_type="string",
    max_length=120,
)
PAGE_KEY_PATH = _parameter(
    "pageKey", "path", required=True, primitive_type="string", max_length=120
)
IF_NONE_MATCH = _parameter(
    "If-None-Match",
    "header",
    required=True,
    primitive_type="string",
    enum=("*",),
)
IF_MATCH = _parameter("If-Match", "header", required=True, primitive_type="string")
IDEMPOTENCY_KEY = _parameter(
    "Idempotency-Key",
    "header",
    required=True,
    primitive_type="string",
    min_length=8,
    max_length=200,
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

OPAQUE_JSON_PROPERTIES = {
    "presentation.Condition": ("value",),
}


def _type_schema(schema_type: str) -> dict[str, Any]:
    return {"type": schema_type}


def _ref_schema(reference: str) -> dict[str, Any]:
    return {"$ref": reference}


def _array_schema(item_schema: dict[str, Any]) -> dict[str, Any]:
    return {"type": "array", "items": item_schema}


DEFINITION_PROPERTY_CONTRACTS: dict[str, dict[str, dict[str, Any]]] = {
    "response.Response": {
        "success": _type_schema("boolean"),
        "status": _type_schema("string"),
        "code": _type_schema("integer"),
        "errorCode": _type_schema("string"),
        "errorMessage": _type_schema("string"),
        "traceId": _type_schema("string"),
    },
    "dto.PresentationConflictResponse": {
        "success": _type_schema("boolean"),
        "status": _type_schema("string"),
        "code": _type_schema("integer"),
        "errorCode": _type_schema("string"),
        "errorMessage": _type_schema("string"),
        "traceID": _type_schema("string"),
        "data": _ref_schema(
            "#/definitions/dto.PresentationConflictResponseData"
        ),
    },
    "dto.PresentationCapabilityListResponse": {
        "items": _array_schema(
            _ref_schema("#/definitions/presentation.CapabilityDefinition")
        ),
        "recoveryMode": _type_schema("boolean"),
        "adoptionMode": _ref_schema("#/definitions/presentation.AdoptionMode"),
        "activePages": _array_schema(_type_schema("string")),
    },
    "dto.PresentationValidationResponse": {
        "structurallyValid": _type_schema("boolean"),
        "semanticallyValid": _type_schema("boolean"),
        "canonicalDocument": _type_schema("object"),
        "digest": _type_schema("string"),
        "currentDefinition": _type_schema("string"),
        "issues": _array_schema(_ref_schema("#/definitions/presentation.Issue")),
    },
    "dto.PresentationProfileListResponse": {
        "items": _array_schema(
            _ref_schema("#/definitions/dto.PresentationProfileSummary")
        ),
        "page": _type_schema("integer"),
        "pageSize": _type_schema("integer"),
        "total": _type_schema("integer"),
    },
    "dto.PresentationProfileResource": {
        "id": _type_schema("string"),
        "scope": _ref_schema("#/definitions/presentation.ScopeKind"),
        "pageKey": _type_schema("string"),
        "state": _type_schema("string"),
        "version": _type_schema("integer"),
        "draft": _ref_schema("#/definitions/dto.PresentationDraftResource"),
        "published": _ref_schema(
            "#/definitions/dto.PresentationRevisionSummary"
        ),
    },
    "dto.PresentationTransitionResponse": {
        "profile": _ref_schema("#/definitions/dto.PresentationProfileResource"),
        "revision": _ref_schema("#/definitions/dto.PresentationRevisionResource"),
        "replayed": _type_schema("boolean"),
    },
    "dto.PresentationRevisionListResponse": {
        "items": _array_schema(
            _ref_schema("#/definitions/dto.PresentationRevisionSummary")
        ),
        "page": _type_schema("integer"),
        "pageSize": _type_schema("integer"),
        "total": _type_schema("integer"),
    },
    "dto.PresentationRevisionResource": {
        "profileID": _type_schema("string"),
        "revision": _type_schema("integer"),
        "aggregateVersion": _type_schema("integer"),
        "contentDigest": _type_schema("string"),
        "definitionHash": _type_schema("string"),
        "document": _type_schema("object"),
    },
    "dto.EffectivePresentationResponse": {
        "pageKey": _type_schema("string"),
        "definitionHash": _type_schema("string"),
        "recoveryMode": _type_schema("boolean"),
        "fallback": _type_schema("boolean"),
        "adoption": _ref_schema("#/definitions/dto.PresentationAdoptionResource"),
        "layers": _ref_schema("#/definitions/dto.EffectivePresentationLayers"),
        "diagnostics": _array_schema(
            _ref_schema("#/definitions/dto.EffectivePresentationDiagnostic")
        ),
    },
    "dto.PresentationProfileCreateRequest": {
        "scope": _ref_schema("#/definitions/presentation.ScopeKind"),
        "pageKey": _type_schema("string"),
        "subjectID": _type_schema("string"),
        "document": _type_schema("object"),
    },
    "dto.PresentationDraftReplaceRequest": {
        "document": _type_schema("object"),
    },
    "dto.PresentationValidationRequest": {
        "document": _type_schema("object"),
    },
    "dto.PresentationRollbackRequest": {
        "revision": _type_schema("integer"),
    },
    "dto.EffectivePresentationLayers": {
        "application": _type_schema("object"),
        "role": _type_schema("object"),
        "user": _type_schema("object"),
    },
}


REQUIRED_DEFINITION_PROPERTIES = {
    "dto.PresentationProfileCreateRequest": {"document", "pageKey", "scope"},
    "dto.PresentationDraftReplaceRequest": {"document"},
    "dto.PresentationValidationRequest": {"document"},
    "dto.PresentationRollbackRequest": {"revision"},
}


def _parameter_key(parameter: dict[str, Any]) -> tuple[Any, Any]:
    return parameter.get("name"), parameter.get("in")


def _definition_name(reference: Any) -> str | None:
    prefix = "#/definitions/"
    if isinstance(reference, str) and reference.startswith(prefix):
        return reference[len(prefix) :]
    return None


def _collect_reference_names(value: Any) -> set[str]:
    names: set[str] = set()
    if isinstance(value, dict):
        definition_name = _definition_name(value.get("$ref"))
        if definition_name is not None:
            names.add(definition_name)
        for child in value.values():
            names.update(_collect_reference_names(child))
    elif isinstance(value, list):
        for child in value:
            names.update(_collect_reference_names(child))
    return names


def _schema_contains(actual: Any, expected: Any) -> bool:
    if isinstance(expected, dict):
        return isinstance(actual, dict) and all(
            key in actual and _schema_contains(actual[key], value)
            for key, value in expected.items()
        )
    if isinstance(expected, list):
        return isinstance(actual, list) and actual == expected
    return actual == expected


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
    constraint_keys = ("minimum", "maximum", "minLength", "maxLength", "enum")
    actual_constraints = {
        key: actual[key] for key in constraint_keys if key in actual
    }
    if actual_constraints != expected["constraints"]:
        errors.append(
            f"{label} has wrong validation constraints; "
            f"expected {expected['constraints']}"
        )
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
        actual_parameter_keys = [
            _parameter_key(parameter) for parameter in actual_parameters
        ]
        duplicate_parameter_keys = {
            key for key in actual_parameter_keys if actual_parameter_keys.count(key) > 1
        }
        for key in sorted(duplicate_parameter_keys):
            errors.append(f"duplicate parameter {key[1]}:{key[0]} on {operation_label}")
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

    pending_definitions = list(referenced_definitions)
    checked_definitions: set[str] = set()
    while pending_definitions:
        name = pending_definitions.pop()
        if name in checked_definitions:
            continue
        checked_definitions.add(name)
        definition = definitions.get(name)
        if not isinstance(definition, dict):
            errors.append(f"referenced definition is missing: {name}")
            continue
        for nested_name in _collect_reference_names(definition):
            if nested_name not in checked_definitions:
                pending_definitions.append(nested_name)

    for definition_name, property_contracts in DEFINITION_PROPERTY_CONTRACTS.items():
        definition = definitions.get(definition_name)
        properties = definition.get("properties") if isinstance(definition, dict) else None
        if not isinstance(definition, dict) or definition.get("type") != "object":
            errors.append(f"definition must be an object: {definition_name}")
            continue
        if not isinstance(properties, dict):
            errors.append(f"definition is missing object properties: {definition_name}")
            continue
        for property_name, expected_schema in property_contracts.items():
            if not _schema_contains(properties.get(property_name), expected_schema):
                errors.append(
                    f"{definition_name}.{property_name} has wrong or missing schema"
                )

    for definition_name, expected_required in REQUIRED_DEFINITION_PROPERTIES.items():
        definition = definitions.get(definition_name)
        actual_required = (
            definition.get("required") if isinstance(definition, dict) else None
        )
        if not isinstance(actual_required, list) or set(actual_required) != expected_required:
            errors.append(
                f"{definition_name} has wrong required properties; "
                f"expected {sorted(expected_required)}"
            )

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

    for definition_name, property_names in OPAQUE_JSON_PROPERTIES.items():
        definition = definitions.get(definition_name)
        properties = definition.get("properties") if isinstance(definition, dict) else None
        if not isinstance(properties, dict):
            errors.append(f"definition is missing object properties: {definition_name}")
            continue
        for property_name in property_names:
            property_schema = properties.get(property_name)
            if not isinstance(property_schema, dict):
                errors.append(
                    f"{definition_name}.{property_name} must be documented as opaque JSON"
                )
                continue
            forbidden_keys = {"type", "$ref", "items", "allOf", "properties"}
            if forbidden_keys & property_schema.keys():
                errors.append(
                    f"{definition_name}.{property_name} must be documented as opaque JSON"
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
