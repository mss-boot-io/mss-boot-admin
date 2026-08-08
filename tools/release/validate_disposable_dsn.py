#!/usr/bin/env python3
"""Validate that a release migration DSN targets an exact local disposable DB."""

from __future__ import annotations

import argparse
import ipaddress
import re
from urllib.parse import unquote, urlsplit


MYSQL_DSN = re.compile(r"^.+@tcp\((?P<address>[^)]+)\)/(?P<database>[^?]+)(?:\?.*)?$")


def require_loopback(host: str) -> None:
    if host.lower() == "localhost":
        return
    try:
        address = ipaddress.ip_address(host)
    except ValueError as error:
        raise ValueError("database host must be localhost or a loopback address") from error
    if not address.is_loopback:
        raise ValueError("database host must be localhost or a loopback address")


def split_host_port(address: str) -> tuple[str, int]:
    if address.startswith("["):
        closing = address.find("]")
        if closing < 0 or closing + 1 >= len(address) or address[closing + 1] != ":":
            raise ValueError("database address must include a port")
        host = address[1:closing]
        port_text = address[closing + 2 :]
    else:
        try:
            host, port_text = address.rsplit(":", 1)
        except ValueError as error:
            raise ValueError("database address must include a port") from error
    try:
        port = int(port_text)
    except ValueError as error:
        raise ValueError("database port must be numeric") from error
    if port < 1 or port > 65535:
        raise ValueError("database port is out of range")
    return host, port


def validate_mysql(dsn: str, expected_database: str) -> None:
    match = MYSQL_DSN.fullmatch(dsn)
    if match is None:
        raise ValueError("MySQL DSN must use the tcp(host:port)/database form")
    host, _ = split_host_port(match.group("address"))
    require_loopback(host)
    if unquote(match.group("database")) != expected_database:
        raise ValueError("MySQL DSN targets an unexpected database")


def validate_postgres(dsn: str, expected_database: str) -> None:
    parsed = urlsplit(dsn)
    if parsed.scheme not in {"postgres", "postgresql"}:
        raise ValueError("PostgreSQL DSN must use the postgres URL form")
    if parsed.hostname is None or parsed.port is None:
        raise ValueError("PostgreSQL DSN must include host and port")
    require_loopback(parsed.hostname)
    if unquote(parsed.path.removeprefix("/")) != expected_database or "/" in parsed.path.removeprefix("/"):
        raise ValueError("PostgreSQL DSN targets an unexpected database")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--driver", required=True, choices=("mysql", "postgres"))
    parser.add_argument("--dsn", required=True)
    parser.add_argument("--database", required=True)
    args = parser.parse_args()

    validator = validate_mysql if args.driver == "mysql" else validate_postgres
    validator(args.dsn, args.database)


if __name__ == "__main__":
    main()
