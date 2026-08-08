#!/usr/bin/env python3
"""Create a disposable migration config without logging its database DSN."""

from __future__ import annotations

import argparse
import json
from pathlib import Path


def without_redis_cache(lines: list[str]) -> list[str]:
    """Remove the optional Redis subtree from a database-only rehearsal config."""

    rendered: list[str] = []
    in_cache = False
    skipping_redis = False

    for line in lines:
        if line and not line.startswith((" ", "\t")):
            in_cache = line.strip().removesuffix(":") == "cache"
            skipping_redis = False

        if in_cache and line.startswith("  redis:"):
            skipping_redis = True
            continue
        if skipping_redis:
            if line.startswith(("    ", "\t\t")) or not line.strip():
                continue
            skipping_redis = False

        rendered.append(line)

    return rendered


def replace_database_fields(text: str, driver: str, dsn: str, name: str) -> str:
    lines = without_redis_cache(text.splitlines())
    section = ""
    replaced: set[str] = set()
    logger_stdout_replaced = False
    logger_level_replaced = False
    query_cache_disabled = False

    for index, line in enumerate(lines):
        if line and not line.startswith((" ", "\t")):
            section = line.strip().removesuffix(":")
            continue
        if not line.startswith("  ") or line.startswith("    "):
            continue

        stripped = line.strip()
        if section == "database":
            for field, value in (("driver", driver), ("source", dsn), ("name", name)):
                if stripped.startswith(field + ":"):
                    lines[index] = f"  {field}: {json.dumps(value)}"
                    replaced.add(field)
                    break
        elif section == "logger" and stripped.startswith("stdout:"):
            # Migration diagnostics must reach the workflow log instead of a
            # temporary file that is deleted with the disposable runtime.
            lines[index] = '  stdout: "stderr"'
            logger_stdout_replaced = True
        elif section == "logger" and stripped.startswith("level:"):
            lines[index] = '  level: "error"'
            logger_level_replaced = True
        elif section == "cache" and stripped.startswith("queryCache:"):
            # Migration evidence exercises the authoritative database only.
            lines[index] = "  queryCache: false"
            query_cache_disabled = True

    missing = {"driver", "source", "name"} - replaced
    if missing:
        raise ValueError(f"database config is missing fields: {', '.join(sorted(missing))}")
    if not logger_stdout_replaced:
        raise ValueError("logger config is missing stdout")
    if not logger_level_replaced:
        raise ValueError("logger config is missing level")
    if not query_cache_disabled:
        raise ValueError("cache config is missing queryCache")
    return "\n".join(lines) + "\n"


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--input", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--driver", required=True, choices=("mysql", "postgres", "sqlite"))
    parser.add_argument("--dsn", required=True)
    parser.add_argument("--database", required=True)
    args = parser.parse_args()

    rendered = replace_database_fields(
        args.input.read_text(encoding="utf-8"),
        args.driver,
        args.dsn,
        args.database,
    )
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(rendered, encoding="utf-8")


if __name__ == "__main__":
    main()
