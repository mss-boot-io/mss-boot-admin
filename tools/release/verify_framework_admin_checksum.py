#!/usr/bin/env python3
"""Verify final Framework/Admin source sums before any v1.3.3 component tag."""

from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
import tempfile
import zipfile
from pathlib import Path


FRAMEWORK_MODULE = "github.com/mss-boot-io/mss-boot-admin/mss-boot"
ADMIN_MODULE = "github.com/mss-boot-io/mss-boot-admin/admin"
VERSION_RE = re.compile(r"^v(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)$")


class ChecksumContractError(ValueError):
    pass


def _run(
    argv: list[str],
    *,
    cwd: Path,
    environment: dict[str, str] | None = None,
) -> subprocess.CompletedProcess[str]:
    try:
        return subprocess.run(
            argv,
            cwd=cwd,
            env=environment,
            check=False,
            capture_output=True,
            text=True,
            encoding="utf-8",
            timeout=300,
        )
    except (OSError, subprocess.TimeoutExpired) as exc:
        raise ChecksumContractError(f"cannot execute {' '.join(argv)}: {exc}") from exc


def _tracked_module_files(
    repository_root: Path,
    source_root: Path,
    *,
    label: str = "Framework",
) -> list[Path]:
    relative_root = source_root.relative_to(repository_root).as_posix()
    tracked = _run(
        ["git", "ls-files", "-z", "--", relative_root],
        cwd=repository_root,
    )
    if tracked.returncode != 0:
        raise ChecksumContractError(
            f"cannot inventory tracked {label} files: {tracked.stderr.strip()}"
        )
    untracked = _run(
        ["git", "ls-files", "--others", "--exclude-standard", "--", relative_root],
        cwd=repository_root,
    )
    if untracked.returncode != 0:
        raise ChecksumContractError(
            f"cannot inventory untracked {label} files: {untracked.stderr.strip()}"
        )
    if untracked.stdout.strip():
        raise ChecksumContractError(
            f"untracked {label} files are excluded from the candidate module: "
            + ", ".join(untracked.stdout.splitlines())
        )

    files = [repository_root / value for value in tracked.stdout.split("\0") if value]
    if not files:
        raise ChecksumContractError(f"the {label} candidate contains no tracked files")
    return files


def _module_archive_files(source_root: Path, sources: list[Path]) -> list[Path]:
    nested_modules = {
        source.parent.relative_to(source_root)
        for source in sources
        if source.name == "go.mod" and source.parent != source_root
    }
    selected: list[Path] = []
    for source in sources:
        try:
            relative = source.relative_to(source_root)
        except ValueError as exc:
            raise ChecksumContractError(
                f"candidate file escapes the Framework root: {source}"
            ) from exc
        if source.is_symlink() or not source.is_file():
            continue
        if "vendor" in relative.parts:
            continue
        if any(part in {".git", ".hg", ".svn", ".bzr"} for part in relative.parts):
            continue
        if any(relative == nested or nested in relative.parents for nested in nested_modules):
            continue
        selected.append(source)
    if not any(path.name == "go.mod" and path.parent == source_root for path in selected):
        raise ChecksumContractError("the Framework candidate does not contain its root go.mod")
    return sorted(selected, key=lambda path: path.relative_to(source_root).as_posix())


def _write_file_module_proxy(
    proxy_root: Path,
    *,
    module: str,
    version: str,
    source_root: Path,
    sources: list[Path],
) -> int:
    version_root = proxy_root.joinpath(*module.split("/"), "@v")
    version_root.mkdir(parents=True, exist_ok=True)
    (version_root / "list").write_text(f"{version}\n", encoding="utf-8")
    (version_root / f"{version}.info").write_text(
        json.dumps({"Version": version, "Time": "1980-01-01T00:00:00Z"}) + "\n",
        encoding="utf-8",
    )
    (version_root / f"{version}.mod").write_bytes((source_root / "go.mod").read_bytes())

    selected = _module_archive_files(source_root, sources)
    prefix = f"{module}@{version}/"
    with zipfile.ZipFile(
        version_root / f"{version}.zip",
        "w",
        compression=zipfile.ZIP_DEFLATED,
    ) as archive:
        for source in selected:
            relative = source.relative_to(source_root).as_posix()
            archive.write(source, prefix + relative)
    return len(selected)


def _download_candidate_sums(
    *,
    proxy_root: Path,
    module: str,
    version: str,
    go_command: str,
) -> tuple[str, str]:
    with tempfile.TemporaryDirectory(prefix="mss-framework-sum-go-") as directory:
        work = Path(directory)
        (work / "go.mod").write_text(
            "module example.com/mss-framework-checksum-probe\n\ngo 1.26.6\n",
            encoding="utf-8",
        )
        environment = os.environ.copy()
        environment.update(
            {
                "GOWORK": "off",
                "GOPROXY": proxy_root.as_uri(),
                "GONOPROXY": "none",
                "GOSUMDB": "off",
                "GONOSUMDB": "",
                "GOMODCACHE": str(work / "modcache"),
                "GOCACHE": str(work / "gocache"),
                "GOTOOLCHAIN": "local",
            }
        )
        result = _run(
            [go_command, "mod", "download", "-json", f"{module}@{version}"],
            cwd=work,
            environment=environment,
        )
        if result.returncode != 0:
            raise ChecksumContractError(
                f"cannot calculate the candidate checksum for {module}: "
                + (result.stderr.strip() or result.stdout.strip())
            )
        try:
            payload = json.loads(result.stdout)
            module_sum = payload["Sum"]
            go_mod_sum = payload["GoModSum"]
        except (json.JSONDecodeError, KeyError, TypeError) as exc:
            raise ChecksumContractError(
                "Go did not return canonical candidate Module and GoMod sums"
            ) from exc
        if not isinstance(module_sum, str) or not isinstance(go_mod_sum, str):
            raise ChecksumContractError("Go returned malformed candidate checksums")
        return module_sum, go_mod_sum


def _calculate_module_candidate_sums(
    repository_root: Path,
    *,
    module: str,
    source_directory: str,
    label: str,
    version: str,
    go_command: str = "go",
) -> tuple[str, str, int]:
    repository_root = repository_root.resolve()
    source_root = repository_root / source_directory
    sources = _tracked_module_files(repository_root, source_root, label=label)
    with tempfile.TemporaryDirectory(prefix=f"mss-{label.lower()}-file-proxy-") as directory:
        proxy_root = Path(directory)
        file_count = _write_file_module_proxy(
            proxy_root,
            module=module,
            version=version,
            source_root=source_root,
            sources=sources,
        )
        module_sum, go_mod_sum = _download_candidate_sums(
            proxy_root=proxy_root,
            module=module,
            version=version,
            go_command=go_command,
        )
    return module_sum, go_mod_sum, file_count


def calculate_candidate_sums(
    repository_root: Path,
    *,
    version: str,
    go_command: str = "go",
) -> tuple[str, str, int]:
    return _calculate_module_candidate_sums(
        repository_root,
        module=FRAMEWORK_MODULE,
        source_directory="mss-boot",
        label="Framework",
        version=version,
        go_command=go_command,
    )


def calculate_admin_candidate_sums(
    repository_root: Path,
    *,
    version: str,
    go_command: str = "go",
) -> tuple[str, str, int]:
    return _calculate_module_candidate_sums(
        repository_root,
        module=ADMIN_MODULE,
        source_directory="admin",
        label="Admin",
        version=version,
        go_command=go_command,
    )


def _go_edit_metadata(
    *,
    cwd: Path,
    argv: list[str],
    go_command: str,
    label: str,
) -> dict[str, object]:
    result = _run([go_command, *argv], cwd=cwd)
    if result.returncode != 0:
        raise ChecksumContractError(
            f"cannot parse {label}: {result.stderr.strip() or result.stdout.strip()}"
        )
    try:
        payload = json.loads(result.stdout)
    except (json.JSONDecodeError, TypeError) as exc:
        raise ChecksumContractError(f"Go returned malformed {label} metadata") from exc
    if not isinstance(payload, dict):
        raise ChecksumContractError(f"Go returned malformed {label} metadata")
    return payload


def _required_framework_version(admin_go_mod: Path, *, go_command: str) -> str:
    metadata = _go_edit_metadata(
        cwd=admin_go_mod.parent,
        argv=["mod", "edit", "-json"],
        go_command=go_command,
        label="admin/go.mod",
    )
    requirements = metadata.get("Require")
    if not isinstance(requirements, list):
        requirements = []
    matches = [
        entry.get("Version")
        for entry in requirements
        if isinstance(entry, dict) and entry.get("Path") == FRAMEWORK_MODULE
    ]
    if len(matches) != 1 or not isinstance(matches[0], str):
        raise ChecksumContractError(
            "admin/go.mod must require the Framework module exactly once"
        )
    replacements = metadata.get("Replace")
    if not isinstance(replacements, list):
        replacements = []
    if any(
        isinstance(entry, dict)
        and isinstance(entry.get("Old"), dict)
        and entry["Old"].get("Path") == FRAMEWORK_MODULE
        for entry in replacements
    ):
        raise ChecksumContractError("admin/go.mod must not replace the Framework module")
    return matches[0]


def _verify_workspace_replacement(
    repository_root: Path,
    *,
    version: str,
    go_command: str,
) -> None:
    metadata = _go_edit_metadata(
        cwd=repository_root,
        argv=["work", "edit", "-json"],
        go_command=go_command,
        label="go.work",
    )
    replacements = metadata.get("Replace")
    if not isinstance(replacements, list):
        replacements = []
    framework_replacements = [
        entry
        for entry in replacements
        if isinstance(entry, dict)
        and isinstance(entry.get("Old"), dict)
        and entry["Old"].get("Path") == FRAMEWORK_MODULE
    ]
    if len(framework_replacements) != 1:
        raise ChecksumContractError(
            "go.work must contain exactly one Framework replacement"
        )
    replacement = framework_replacements[0]
    old = replacement.get("Old")
    new = replacement.get("New")
    if not isinstance(old, dict) or not isinstance(new, dict):
        raise ChecksumContractError("go.work contains a malformed Framework replacement")
    if (
        old.get("Version") != version
        or new.get("Path") != "./mss-boot"
        or new.get("Version") not in (None, "")
    ):
        raise ChecksumContractError(
            "go.work does not contain the exact candidate-only Framework replacement"
        )


def _recorded_module_sums(
    go_sum: Path,
    *,
    module: str,
    version: str,
    label: str,
) -> tuple[str, str]:
    entries: dict[str, list[str]] = {version: [], version + "/go.mod": []}
    for line in go_sum.read_text(encoding="utf-8").splitlines():
        fields = line.split()
        if len(fields) != 3 or fields[0] != module:
            continue
        if fields[1] in entries:
            entries[fields[1]].append(fields[2])
    for key, values in entries.items():
        if len(values) != 1:
            raise ChecksumContractError(
                f"{label} must contain exactly one {module} {key} entry"
            )
    return entries[version][0], entries[version + "/go.mod"][0]


def _recorded_sums(admin_go_sum: Path, *, version: str) -> tuple[str, str]:
    return _recorded_module_sums(
        admin_go_sum,
        module=FRAMEWORK_MODULE,
        version=version,
        label="admin/go.sum",
    )


def verify_repository(
    repository_root: Path,
    *,
    version: str,
    go_command: str = "go",
) -> dict[str, object]:
    if not VERSION_RE.fullmatch(version):
        raise ChecksumContractError("version must be an exact stable vX.Y.Z value")
    repository_root = repository_root.resolve()
    required_version = _required_framework_version(
        repository_root / "admin" / "go.mod",
        go_command=go_command,
    )
    if required_version != version:
        raise ChecksumContractError(
            f"Admin requires Framework {required_version}, not candidate {version}"
        )
    _verify_workspace_replacement(
        repository_root,
        version=version,
        go_command=go_command,
    )
    recorded_sum, recorded_go_mod_sum = _recorded_sums(
        repository_root / "admin" / "go.sum", version=version
    )
    candidate_sum, candidate_go_mod_sum, file_count = calculate_candidate_sums(
        repository_root,
        version=version,
        go_command=go_command,
    )
    if recorded_sum != candidate_sum:
        raise ChecksumContractError(
            "Admin Framework Module checksum does not match the final candidate tree: "
            f"recorded {recorded_sum}, calculated {candidate_sum}"
        )
    if recorded_go_mod_sum != candidate_go_mod_sum:
        raise ChecksumContractError(
            "Admin Framework go.mod checksum does not match the final candidate tree: "
            f"recorded {recorded_go_mod_sum}, calculated {candidate_go_mod_sum}"
        )
    template_admin_sum, template_admin_go_mod_sum = _recorded_module_sums(
        repository_root / "templates" / "application" / "go.sum",
        module=ADMIN_MODULE,
        version=version,
        label="templates/application/go.sum",
    )
    candidate_admin_sum, candidate_admin_go_mod_sum, admin_file_count = (
        calculate_admin_candidate_sums(
            repository_root,
            version=version,
            go_command=go_command,
        )
    )
    if template_admin_sum != candidate_admin_sum:
        raise ChecksumContractError(
            "Thin Host Admin Module checksum does not match the final candidate tree: "
            f"recorded {template_admin_sum}, calculated {candidate_admin_sum}"
        )
    if template_admin_go_mod_sum != candidate_admin_go_mod_sum:
        raise ChecksumContractError(
            "Thin Host Admin go.mod checksum does not match the final candidate tree: "
            f"recorded {template_admin_go_mod_sum}, calculated {candidate_admin_go_mod_sum}"
        )
    return {
        "success": True,
        "module": FRAMEWORK_MODULE,
        "version": version,
        "sum": candidate_sum,
        "goModSum": candidate_go_mod_sum,
        "candidateFiles": file_count,
        "adminSum": candidate_admin_sum,
        "adminGoModSum": candidate_admin_go_mod_sum,
        "adminCandidateFiles": admin_file_count,
        "dependencyMode": "replace-free-file-proxy",
        "checksumDatabaseMode": "explicit-candidate-parity",
    }


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path("."))
    parser.add_argument("--version", required=True)
    parser.add_argument("--go", default="go", dest="go_command")
    return parser


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    try:
        result = verify_repository(
            args.root,
            version=args.version,
            go_command=args.go_command,
        )
    except (ChecksumContractError, OSError) as exc:
        print(f"Framework/Admin checksum contract failed: {exc}", file=sys.stderr)
        return 1
    json.dump(result, sys.stdout, sort_keys=True)
    sys.stdout.write("\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
