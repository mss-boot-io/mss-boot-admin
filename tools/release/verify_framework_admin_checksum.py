#!/usr/bin/env python3
"""Verify final Framework/Admin source sums before coordinated component tags."""

from __future__ import annotations

import argparse
import importlib.util
import io
import json
import os
import re
import stat
import subprocess
import sys
import tarfile
import tempfile
import zipfile
from pathlib import Path, PurePosixPath
from typing import NamedTuple


FRAMEWORK_MODULE = "github.com/mss-boot-io/mss-boot-admin/mss-boot"
ADMIN_MODULE = "github.com/mss-boot-io/mss-boot-admin/admin"
VERSION_RE = re.compile(r"^v(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)$")


class ChecksumContractError(ValueError):
    pass


class GitCandidateFile(NamedTuple):
    """One canonical module file read from one exact Git tree."""

    relative: PurePosixPath
    content: bytes
    mode: int


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


def _run_bytes(argv: list[str], *, cwd: Path) -> subprocess.CompletedProcess[bytes]:
    try:
        return subprocess.run(
            argv,
            cwd=cwd,
            check=False,
            capture_output=True,
            timeout=300,
        )
    except (OSError, subprocess.TimeoutExpired) as exc:
        raise ChecksumContractError(f"cannot execute {' '.join(argv)}: {exc}") from exc


def _ensure_clean_paths(
    repository_root: Path,
    paths: list[str],
    *,
    label: str,
    reference: str = "HEAD",
) -> None:
    commands = [
        [
            "git",
            "diff",
            "--cached",
            "--name-only",
            "-z",
            reference,
            "--",
            *paths,
        ],
        ["git", "diff", "--name-only", "-z", "--ignore-cr-at-eol", "--", *paths],
        ["git", "ls-files", "--others", "--exclude-standard", "-z", "--", *paths],
    ]
    changed: list[str] = []
    for argv in commands:
        result = _run(argv, cwd=repository_root)
        if result.returncode != 0:
            raise ChecksumContractError(
                f"cannot verify the {label} Git candidate: {result.stderr.strip()}"
            )
        changed.extend(value for value in result.stdout.split("\0") if value)
    if changed:
        raise ChecksumContractError(
            f"uncommitted {label} candidate drift is not allowed: "
            + ", ".join(sorted(set(changed)))
        )


def _git_module_candidate(
    repository_root: Path,
    *,
    source_directory: str,
    label: str,
    source_commit: str = "HEAD",
    verify_clean: bool = True,
) -> list[GitCandidateFile]:
    """Read module inputs from an exact Git tree.

    Candidate qualification uses HEAD and proves that the index/worktree is
    equivalent. Published-stable reconciliation supplies a policy-bound
    immutable commit and deliberately ignores later component-source changes.
    """

    if verify_clean:
        _ensure_clean_paths(
            repository_root,
            [source_directory],
            label=label,
            reference=source_commit,
        )
    archive_result = _run_bytes(
        ["git", "archive", "--format=tar", source_commit, "--", source_directory],
        cwd=repository_root,
    )
    if archive_result.returncode != 0:
        raise ChecksumContractError(
            f"cannot read the exact {source_commit} {label} tree: "
            + archive_result.stderr.decode("utf-8", errors="replace").strip()
        )
    prefix = source_directory.rstrip("/") + "/"
    try:
        with tarfile.open(fileobj=io.BytesIO(archive_result.stdout), mode="r:") as archive:
            result: list[GitCandidateFile] = []
            for member in archive.getmembers():
                if member.isdir() or not member.name.startswith(prefix):
                    continue
                relative = PurePosixPath(member.name[len(prefix) :])
                if member.issym():
                    content = b""
                    mode = stat.S_IFLNK | member.mode
                elif member.isfile():
                    extracted = archive.extractfile(member)
                    if extracted is None:
                        raise ChecksumContractError(
                            f"cannot read archived {label} file: {member.name}"
                        )
                    content = extracted.read()
                    mode = stat.S_IFREG | member.mode
                else:
                    continue
                result.append(
                    GitCandidateFile(
                        relative=relative,
                        content=content,
                        mode=mode,
                    )
                )
    except (OSError, tarfile.TarError) as exc:
        raise ChecksumContractError(
            f"cannot decode the exact {source_commit} {label} tree: {exc}"
        ) from exc
    if not result:
        raise ChecksumContractError(f"the {label} candidate contains no tracked files")

    # cmd/go copies the repository-root LICENSE into a module stored in a
    # subdirectory when that module has no LICENSE of its own. Reproduce that
    # behavior before calculating the candidate h1 sum, otherwise a local
    # file proxy can disagree with the public Go proxy for the exact same tag.
    if not any(source.relative == PurePosixPath("LICENSE") for source in result):
        if verify_clean:
            _ensure_clean_paths(
                repository_root,
                ["LICENSE"],
                label=f"{label} repository LICENSE",
                reference=source_commit,
            )
        root_license = _repository_root_license_candidate(
            repository_root,
            source_commit=source_commit,
        )
        if root_license is not None:
            result.append(root_license)
    return result


def _repository_root_license_candidate(
    repository_root: Path,
    *,
    source_commit: str = "HEAD",
) -> GitCandidateFile | None:
    tree = _run_bytes(
        ["git", "ls-tree", "-z", source_commit, "--", "LICENSE"],
        cwd=repository_root,
    )
    if tree.returncode != 0:
        raise ChecksumContractError(
            f"cannot inspect the exact {source_commit} repository LICENSE: "
            + tree.stderr.decode("utf-8", errors="replace").strip()
        )
    entry = tree.stdout.removesuffix(b"\0")
    if not entry:
        return None
    try:
        metadata, name = entry.split(b"\t", 1)
        _mode, object_type, object_id = metadata.split(b" ", 2)
    except ValueError as exc:
        raise ChecksumContractError(
            "Git returned malformed repository LICENSE metadata"
        ) from exc
    if name != b"LICENSE" or object_type != b"blob":
        raise ChecksumContractError(
            "the repository LICENSE must be one regular Git blob"
        )
    content = _run_bytes(
        ["git", "cat-file", "blob", object_id.decode("ascii")],
        cwd=repository_root,
    )
    if content.returncode != 0:
        raise ChecksumContractError(
            f"cannot read the exact {source_commit} repository LICENSE: "
            + content.stderr.decode("utf-8", errors="replace").strip()
        )
    # cmd/go's inherited dataFile always exposes mode 0644, independently of
    # the repository-root blob's executable bit.
    return GitCandidateFile(
        relative=PurePosixPath("LICENSE"),
        content=content.stdout,
        mode=stat.S_IFREG | 0o644,
    )


def _module_archive_files(
    sources: list[GitCandidateFile],
) -> list[GitCandidateFile]:
    nested_modules = {
        source.relative.parent
        for source in sources
        if stat.S_ISREG(source.mode)
        and source.relative.name == "go.mod"
        and source.relative.parent != PurePosixPath(".")
    }
    selected: list[GitCandidateFile] = []
    for source in sources:
        relative = source.relative
        if relative.is_absolute() or ".." in relative.parts:
            raise ChecksumContractError(
                f"candidate file escapes the module root: {relative}"
            )
        if stat.S_ISLNK(source.mode):
            continue
        if "vendor" in relative.parts:
            continue
        if any(part in {".git", ".hg", ".svn", ".bzr"} for part in relative.parts):
            continue
        if any(relative == nested or nested in relative.parents for nested in nested_modules):
            continue
        selected.append(source)
    if not any(source.relative == PurePosixPath("go.mod") for source in selected):
        raise ChecksumContractError("the candidate does not contain its root go.mod")
    return sorted(selected, key=lambda source: source.relative.as_posix())


def _write_file_module_proxy(
    proxy_root: Path,
    *,
    module: str,
    version: str,
    sources: list[GitCandidateFile],
) -> int:
    version_root = proxy_root.joinpath(*module.split("/"), "@v")
    version_root.mkdir(parents=True, exist_ok=True)
    (version_root / "list").write_text(f"{version}\n", encoding="utf-8")
    (version_root / f"{version}.info").write_text(
        json.dumps({"Version": version, "Time": "1980-01-01T00:00:00Z"}) + "\n",
        encoding="utf-8",
    )
    selected = _module_archive_files(sources)
    root_go_mod = next(
        source.content
        for source in selected
        if source.relative == PurePosixPath("go.mod")
    )
    (version_root / f"{version}.mod").write_bytes(root_go_mod)
    prefix = f"{module}@{version}/"
    with zipfile.ZipFile(
        version_root / f"{version}.zip",
        "w",
        compression=zipfile.ZIP_DEFLATED,
    ) as archive:
        for source in selected:
            info = zipfile.ZipInfo(prefix + source.relative.as_posix())
            info.date_time = (1980, 1, 1, 0, 0, 0)
            info.compress_type = zipfile.ZIP_DEFLATED
            executable = bool(source.mode & 0o111)
            info.external_attr = ((0o100755 if executable else 0o100644) << 16)
            archive.writestr(info, source.content)
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
    sources: list[GitCandidateFile] | None = None,
    source_commit: str = "HEAD",
) -> tuple[str, str, int]:
    repository_root = repository_root.resolve()
    if sources is None:
        sources = _git_module_candidate(
            repository_root,
            source_directory=source_directory,
            label=label,
            source_commit=source_commit,
        )
    with tempfile.TemporaryDirectory(prefix=f"mss-{label.lower()}-file-proxy-") as directory:
        proxy_root = Path(directory)
        file_count = _write_file_module_proxy(
            proxy_root,
            module=module,
            version=version,
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
    source_commit: str = "HEAD",
) -> tuple[str, str, int]:
    return _calculate_module_candidate_sums(
        repository_root,
        module=FRAMEWORK_MODULE,
        source_directory="mss-boot",
        label="Framework",
        version=version,
        go_command=go_command,
        source_commit=source_commit,
    )


def calculate_admin_candidate_sums(
    repository_root: Path,
    *,
    version: str,
    go_command: str = "go",
    source_commit: str = "HEAD",
) -> tuple[str, str, int]:
    return _calculate_module_candidate_sums(
        repository_root,
        module=ADMIN_MODULE,
        source_directory="admin",
        label="Admin",
        version=version,
        go_command=go_command,
        source_commit=source_commit,
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
        argv=["work", "edit", "-json", str(repository_root / "go.work")],
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


def _updated_go_sum(
    original: bytes,
    *,
    module: str,
    version: str,
    module_sum: str,
    go_mod_sum: str,
    label: str,
) -> bytes:
    try:
        lines = original.decode("utf-8").splitlines()
    except UnicodeDecodeError as exc:
        raise ChecksumContractError(f"{label} is not UTF-8") from exc
    replacements = {
        version: module_sum,
        version + "/go.mod": go_mod_sum,
    }
    matches = {key: 0 for key in replacements}
    updated: list[str] = []
    for line in lines:
        fields = line.split()
        if len(fields) == 3 and fields[0] == module and fields[1] in replacements:
            key = fields[1]
            matches[key] += 1
            updated.append(f"{module} {key} {replacements[key]}")
        else:
            updated.append(line)
    for key, count in matches.items():
        if count != 1:
            raise ChecksumContractError(
                f"{label} must contain exactly one {module} {key} entry"
            )
    return ("\n".join(updated) + "\n").encode("utf-8")


def _updated_test_constants(
    original: bytes,
    *,
    framework_sum: str,
    framework_go_mod_sum: str,
    admin_sum: str,
    admin_go_mod_sum: str,
) -> bytes:
    text = original.decode("utf-8")
    values = {
        "EXPECTED_FRAMEWORK_SUM": framework_sum,
        "EXPECTED_FRAMEWORK_GO_MOD_SUM": framework_go_mod_sum,
        "EXPECTED_ADMIN_SUM": admin_sum,
        "EXPECTED_ADMIN_GO_MOD_SUM": admin_go_mod_sum,
    }
    for name, value in values.items():
        text, count = re.subn(
            rf'^{name} = "[^"]+"$',
            f'{name} = "{value}"',
            text,
            count=1,
            flags=re.MULTILINE,
        )
        if count != 1:
            raise ChecksumContractError(
                f"checksum test must define {name} exactly once"
            )
    return text.encode("utf-8")


def _head_file_bytes(
    repository_root: Path,
    relative: str,
    *,
    source_commit: str = "HEAD",
) -> bytes:
    result = _run_bytes(
        ["git", "show", f"{source_commit}:{relative}"],
        cwd=repository_root,
    )
    if result.returncode != 0:
        raise ChecksumContractError(
            f"cannot read exact {source_commit} metadata {relative}: "
            + result.stderr.decode("utf-8", errors="replace").strip()
        )
    return result.stdout


def _write_if_changed(path: Path, content: bytes) -> bool:
    if path.read_bytes() == content:
        return False
    mode = path.stat().st_mode
    with tempfile.NamedTemporaryFile(dir=path.parent, delete=False) as temporary:
        temporary.write(content)
        temporary_path = Path(temporary.name)
    try:
        os.chmod(temporary_path, stat.S_IMODE(mode))
        os.replace(temporary_path, path)
    finally:
        temporary_path.unlink(missing_ok=True)
    return True


def _verify_update_target_state(
    repository_root: Path,
    *,
    relative: str,
    expected: bytes,
    source_commit: str = "HEAD",
) -> None:
    current = (repository_root / relative).read_bytes()
    committed = _head_file_bytes(
        repository_root,
        relative,
        source_commit=source_commit,
    )
    # The approved targets are text files governed by the repository's LF
    # attributes. A Windows checkout may still expose CRLF bytes, so compare
    # their Git-equivalent text before deciding that the user changed them.
    normalized_current = current.replace(b"\r\n", b"\n")
    if normalized_current not in (
        committed.replace(b"\r\n", b"\n"),
        expected.replace(b"\r\n", b"\n"),
    ):
        raise ChecksumContractError(
            f"refusing to overwrite unrelated drift in updater target {relative}"
        )


def _ensure_refresh_inputs_clean(
    repository_root: Path,
    *,
    reference: str,
) -> None:
    _ensure_clean_paths(
        repository_root,
        ["mss-boot"],
        label="Framework",
        reference=reference,
    )
    _ensure_clean_paths(
        repository_root,
        ["admin", ":(exclude)admin/go.sum"],
        label="Admin source",
        reference=reference,
    )
    _ensure_clean_paths(
        repository_root,
        ["templates/application", ":(exclude)templates/application/go.sum"],
        label="Thin Host source",
        reference=reference,
    )
    _ensure_clean_paths(
        repository_root,
        ["LICENSE"],
        label="repository LICENSE",
        reference=reference,
    )
    _ensure_clean_paths(
        repository_root,
        ["go.work"],
        label="workspace",
        reference=reference,
    )
    _ensure_clean_paths(
        repository_root,
        [".mss/release-policy.yaml"],
        label="release policy evidence",
        reference=reference,
    )


def refresh_repository_metadata(
    repository_root: Path,
    *,
    version: str,
    go_command: str = "go",
    expected_head_commit: str | None = None,
) -> dict[str, object]:
    """Deterministically refresh the four checksum layers from exact HEAD blobs."""

    if not VERSION_RE.fullmatch(version):
        raise ChecksumContractError("version must be an exact stable vX.Y.Z value")
    repository_root = repository_root.resolve()
    head_commit = _head_commit(repository_root)
    if expected_head_commit is not None and head_commit != expected_head_commit:
        raise ChecksumContractError(
            "HEAD changed before checksum metadata refresh could begin"
        )
    _ensure_refresh_inputs_clean(repository_root, reference=head_commit)

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

    framework_sources = _git_module_candidate(
        repository_root,
        source_directory="mss-boot",
        label="Framework",
        source_commit=head_commit,
        verify_clean=False,
    )
    admin_sources = _git_module_candidate(
        repository_root,
        source_directory="admin",
        label="Admin",
        source_commit=head_commit,
        verify_clean=False,
    )
    framework_sum, framework_go_mod_sum, framework_file_count = (
        _calculate_module_candidate_sums(
            repository_root,
            module=FRAMEWORK_MODULE,
            source_directory="mss-boot",
            label="Framework",
            version=version,
            go_command=go_command,
            sources=framework_sources,
        )
    )
    committed_admin_go_sum = next(
        source.content
        for source in admin_sources
        if source.relative == PurePosixPath("go.sum")
    )
    updated_admin_go_sum = _updated_go_sum(
        committed_admin_go_sum,
        module=FRAMEWORK_MODULE,
        version=version,
        module_sum=framework_sum,
        go_mod_sum=framework_go_mod_sum,
        label="admin/go.sum",
    )
    updated_admin_sources = [
        GitCandidateFile(source.relative, updated_admin_go_sum, source.mode)
        if source.relative == PurePosixPath("go.sum")
        else source
        for source in admin_sources
    ]
    admin_sum, admin_go_mod_sum, admin_file_count = _calculate_module_candidate_sums(
        repository_root,
        module=ADMIN_MODULE,
        source_directory="admin",
        label="Admin",
        version=version,
        go_command=go_command,
        sources=updated_admin_sources,
    )

    template_relative = "templates/application/go.sum"
    updated_template_go_sum = _updated_go_sum(
        _head_file_bytes(
            repository_root,
            template_relative,
            source_commit=head_commit,
        ),
        module=FRAMEWORK_MODULE,
        version=version,
        module_sum=framework_sum,
        go_mod_sum=framework_go_mod_sum,
        label=template_relative,
    )
    updated_template_go_sum = _updated_go_sum(
        updated_template_go_sum,
        module=ADMIN_MODULE,
        version=version,
        module_sum=admin_sum,
        go_mod_sum=admin_go_mod_sum,
        label=template_relative,
    )
    test_relative = "tools/release/test_verify_framework_admin_checksum.py"
    updated_test = _updated_test_constants(
        _head_file_bytes(
            repository_root,
            test_relative,
            source_commit=head_commit,
        ),
        framework_sum=framework_sum,
        framework_go_mod_sum=framework_go_mod_sum,
        admin_sum=admin_sum,
        admin_go_mod_sum=admin_go_mod_sum,
    )

    outputs = {
        "admin/go.sum": updated_admin_go_sum,
        template_relative: updated_template_go_sum,
        test_relative: updated_test,
    }
    for relative, content in outputs.items():
        _verify_update_target_state(
            repository_root,
            relative=relative,
            expected=content,
            source_commit=head_commit,
        )
    _ensure_refresh_inputs_clean(repository_root, reference=head_commit)
    if _head_commit(repository_root) != head_commit:
        raise ChecksumContractError(
            "HEAD changed before refreshed checksum metadata could be written"
        )
    updated_files: list[str] = []
    for relative, content in outputs.items():
        if _head_commit(repository_root) != head_commit:
            raise ChecksumContractError(
                "HEAD changed while refreshed checksum metadata was being written"
            )
        if _write_if_changed(repository_root / relative, content):
            updated_files.append(relative)
    _ensure_refresh_inputs_clean(repository_root, reference=head_commit)
    for relative, expected in outputs.items():
        current = (repository_root / relative).read_bytes()
        if current.replace(b"\r\n", b"\n") != expected.replace(b"\r\n", b"\n"):
            raise ChecksumContractError(
                f"refreshed checksum metadata changed before success: {relative}"
            )
    if _head_commit(repository_root) != head_commit:
        raise ChecksumContractError(
            "HEAD changed while refreshed checksum metadata was being written"
        )
    return {
        "success": True,
        "write": True,
        "version": version,
        "sum": framework_sum,
        "goModSum": framework_go_mod_sum,
        "candidateFiles": framework_file_count,
        "adminSum": admin_sum,
        "adminGoModSum": admin_go_mod_sum,
        "adminCandidateFiles": admin_file_count,
        "updatedFiles": updated_files,
        "sourceMode": "candidate",
        "sourceCommit": head_commit,
        "headCommit": head_commit,
    }


def _head_commit(repository_root: Path) -> str:
    result = _run(
        ["git", "rev-parse", "--verify", "HEAD^{commit}"],
        cwd=repository_root,
    )
    commit = result.stdout.strip()
    if result.returncode != 0 or not re.fullmatch(r"[0-9a-f]{40}", commit):
        raise ChecksumContractError("cannot resolve the exact candidate HEAD commit")
    return commit


def _verify_repository_tree(
    repository_root: Path,
    *,
    version: str,
    go_command: str = "go",
    source_commit: str | None = None,
    expected_head_commit: str | None = None,
) -> dict[str, object]:
    if not VERSION_RE.fullmatch(version):
        raise ChecksumContractError("version must be an exact stable vX.Y.Z value")
    repository_root = repository_root.resolve()
    head_commit = _head_commit(repository_root)
    if expected_head_commit is not None and head_commit != expected_head_commit:
        raise ChecksumContractError(
            "HEAD changed before Framework/Admin checksum evidence could be built"
        )
    component_source_commit = source_commit or head_commit
    _ensure_clean_paths(
        repository_root,
        ["templates/application"],
        label="Thin Host metadata",
        reference=head_commit,
    )
    _ensure_clean_paths(
        repository_root,
        ["go.work"],
        label="workspace",
        reference=head_commit,
    )
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
    if source_commit is None:
        candidate_sum, candidate_go_mod_sum, file_count = calculate_candidate_sums(
            repository_root,
            version=version,
            go_command=go_command,
            source_commit=component_source_commit,
        )
        source_label = "final candidate tree"
    else:
        framework_sources = _git_module_candidate(
            repository_root,
            source_directory="mss-boot",
            label="Framework",
            source_commit=source_commit,
            verify_clean=False,
        )
        candidate_sum, candidate_go_mod_sum, file_count = (
            _calculate_module_candidate_sums(
                repository_root,
                module=FRAMEWORK_MODULE,
                source_directory="mss-boot",
                label="Framework",
                version=version,
                go_command=go_command,
                sources=framework_sources,
            )
        )
        source_label = f"published stable commit {source_commit}"
    if recorded_sum != candidate_sum:
        raise ChecksumContractError(
            f"Admin Framework Module checksum does not match the {source_label}: "
            f"recorded {recorded_sum}, calculated {candidate_sum}"
        )
    if recorded_go_mod_sum != candidate_go_mod_sum:
        raise ChecksumContractError(
            f"Admin Framework go.mod checksum does not match the {source_label}: "
            f"recorded {recorded_go_mod_sum}, calculated {candidate_go_mod_sum}"
        )
    template_go_sum = repository_root / "templates" / "application" / "go.sum"
    template_framework_sum, template_framework_go_mod_sum = _recorded_module_sums(
        template_go_sum,
        module=FRAMEWORK_MODULE,
        version=version,
        label="templates/application/go.sum",
    )
    if template_framework_sum != candidate_sum:
        raise ChecksumContractError(
            f"Thin Host Framework Module checksum does not match the {source_label}: "
            f"recorded {template_framework_sum}, calculated {candidate_sum}"
        )
    if template_framework_go_mod_sum != candidate_go_mod_sum:
        raise ChecksumContractError(
            f"Thin Host Framework go.mod checksum does not match the {source_label}: "
            f"recorded {template_framework_go_mod_sum}, calculated {candidate_go_mod_sum}"
        )
    template_admin_sum, template_admin_go_mod_sum = _recorded_module_sums(
        template_go_sum,
        module=ADMIN_MODULE,
        version=version,
        label="templates/application/go.sum",
    )
    if source_commit is None:
        candidate_admin_sum, candidate_admin_go_mod_sum, admin_file_count = (
            calculate_admin_candidate_sums(
                repository_root,
                version=version,
                go_command=go_command,
                source_commit=component_source_commit,
            )
        )
    else:
        admin_sources = _git_module_candidate(
            repository_root,
            source_directory="admin",
            label="Admin",
            source_commit=source_commit,
            verify_clean=False,
        )
        candidate_admin_sum, candidate_admin_go_mod_sum, admin_file_count = (
            _calculate_module_candidate_sums(
                repository_root,
                module=ADMIN_MODULE,
                source_directory="admin",
                label="Admin",
                version=version,
                go_command=go_command,
                sources=admin_sources,
            )
        )
    if template_admin_sum != candidate_admin_sum:
        raise ChecksumContractError(
            f"Thin Host Admin Module checksum does not match the {source_label}: "
            f"recorded {template_admin_sum}, calculated {candidate_admin_sum}"
        )
    if template_admin_go_mod_sum != candidate_admin_go_mod_sum:
        raise ChecksumContractError(
            f"Thin Host Admin go.mod checksum does not match the {source_label}: "
            f"recorded {template_admin_go_mod_sum}, calculated {candidate_admin_go_mod_sum}"
        )
    end_paths = [
        ".mss/release-policy.yaml",
        "templates/application",
        "go.work",
        "admin/go.mod",
        "admin/go.sum",
    ]
    if source_commit is None:
        end_paths.extend(["mss-boot", "admin", "LICENSE"])
    _ensure_clean_paths(
        repository_root,
        end_paths,
        label="checksum evidence",
        reference=head_commit,
    )
    if _head_commit(repository_root) != head_commit:
        raise ChecksumContractError(
            "HEAD changed while Framework/Admin checksum evidence was being built"
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
        "sourceMode": "candidate" if source_commit is None else "current-stable",
        "sourceCommit": component_source_commit,
        "headCommit": head_commit,
    }


def verify_repository(
    repository_root: Path,
    *,
    version: str,
    go_command: str = "go",
) -> dict[str, object]:
    """Verify an unpublished candidate against the clean exact HEAD tree."""

    return _verify_repository_tree(
        repository_root,
        version=version,
        go_command=go_command,
    )


def _resolve_published_source_commit(
    repository_root: Path,
    *,
    source_commit: str,
    head_commit: str | None = None,
) -> str:
    if not re.fullmatch(r"[0-9a-f]{40}", source_commit):
        raise ChecksumContractError(
            "published stable source commit must be a full lowercase commit SHA"
        )
    resolved = _run(
        ["git", "rev-parse", "--verify", "--end-of-options", f"{source_commit}^{{commit}}"],
        cwd=repository_root,
    )
    if resolved.returncode != 0 or resolved.stdout.strip() != source_commit:
        raise ChecksumContractError(
            f"published stable source commit does not resolve exactly: {source_commit}"
        )
    ancestry = _run(
        [
            "git",
            "merge-base",
            "--is-ancestor",
            source_commit,
            head_commit or "HEAD",
        ],
        cwd=repository_root,
    )
    if ancestry.returncode == 1:
        raise ChecksumContractError(
            "published stable source commit must be an ancestor of HEAD"
        )
    if ancestry.returncode != 0:
        raise ChecksumContractError(
            "cannot verify published stable source ancestry: "
            + (ancestry.stderr.strip() or ancestry.stdout.strip())
        )
    return source_commit


def _verify_published_repository(
    repository_root: Path,
    *,
    version: str,
    source_commit: str,
    expected_head_commit: str,
    go_command: str = "go",
) -> dict[str, object]:
    """Verify current adopter metadata against a prevalidated published tree."""

    repository_root = repository_root.resolve()
    exact_commit = _resolve_published_source_commit(
        repository_root,
        source_commit=source_commit,
        head_commit=expected_head_commit,
    )
    _ensure_clean_paths(
        repository_root,
        ["admin/go.mod", "admin/go.sum"],
        label="published Admin metadata",
        reference=expected_head_commit,
    )
    return _verify_repository_tree(
        repository_root,
        version=version,
        go_command=go_command,
        source_commit=exact_commit,
        expected_head_commit=expected_head_commit,
    )


def _load_release_policy(
    repository_root: Path,
    *,
    reference: str,
) -> dict[str, object]:
    policy_relative = ".mss/release-policy.yaml"
    _ensure_clean_paths(
        repository_root,
        [policy_relative],
        label="release policy",
        reference=reference,
    )
    module_path = Path(__file__).with_name("check_release_policy.py")
    spec = importlib.util.spec_from_file_location(
        "mss_checksum_release_policy",
        module_path,
    )
    if spec is None or spec.loader is None:
        raise ChecksumContractError("cannot load the canonical release policy parser")
    module = importlib.util.module_from_spec(spec)
    try:
        spec.loader.exec_module(module)
        return module.load_policy(repository_root / policy_relative)
    except (OSError, module.PolicyError) as exc:
        raise ChecksumContractError(f"invalid canonical release policy: {exc}") from exc


def _verify_current_stable_tags(
    repository_root: Path,
    *,
    version: str,
    source_commit: str,
) -> dict[str, str]:
    # Docs is deliberately absent: its website-only tag is asynchronous and
    # must never gate the Framework/Admin distribution identity.
    tags = {
        "root": version,
        "framework": f"mss-boot/{version}",
        "admin": f"admin/{version}",
    }
    for component, tag in tags.items():
        resolved = _run(
            [
                "git",
                "rev-parse",
                "--verify",
                "--end-of-options",
                f"refs/tags/{tag}^{{commit}}",
            ],
            cwd=repository_root,
        )
        if resolved.returncode != 0:
            raise ChecksumContractError(
                f"published stable {component} tag is missing: {tag}"
            )
        if resolved.stdout.strip() != source_commit:
            raise ChecksumContractError(
                f"published stable {component} tag {tag} does not resolve to "
                f"policy commit {source_commit}"
            )
    return tags


def verify_current_stable_repository(
    repository_root: Path,
    *,
    version: str,
    go_command: str = "go",
) -> dict[str, object]:
    """Verify the current stable release using only its policy-bound identity."""

    repository_root = repository_root.resolve()
    head_commit = _head_commit(repository_root)
    policy = _load_release_policy(repository_root, reference=head_commit)
    stable_version = policy.get("currentStableVersion")
    if version != stable_version:
        raise ChecksumContractError(
            f"current-stable mode requires policy version {stable_version}, not {version}"
        )
    source_commit = policy.get("currentStableCommit")
    if not isinstance(source_commit, str):
        raise ChecksumContractError(
            "release policy currentStableCommit must be a full commit SHA"
        )
    exact_commit = _resolve_published_source_commit(
        repository_root,
        source_commit=source_commit,
        head_commit=head_commit,
    )
    tags = _verify_current_stable_tags(
        repository_root,
        version=version,
        source_commit=exact_commit,
    )
    result = _verify_published_repository(
        repository_root,
        version=version,
        source_commit=exact_commit,
        expected_head_commit=head_commit,
        go_command=go_command,
    )
    result["sourceTags"] = tags
    return result


def _require_active_future_candidate(
    repository_root: Path,
    *,
    version: str,
) -> str:
    repository_root = repository_root.resolve()
    head_commit = _head_commit(repository_root)
    policy = _load_release_policy(repository_root, reference=head_commit)
    stable_version = policy.get("currentStableVersion")
    next_version = policy.get("nextPublicVersion")
    target_state = policy.get("releaseTargetState")
    if (
        version != next_version
        or version == stable_version
        or target_state != "active"
    ):
        raise ChecksumContractError(
            "checksum metadata may be refreshed only for the active future "
            "candidate declared by release policy"
        )
    if _head_commit(repository_root) != head_commit:
        raise ChecksumContractError(
            "HEAD changed while future-candidate policy was being verified"
        )
    return head_commit


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path("."))
    parser.add_argument("--version", required=True)
    parser.add_argument("--go", default="go", dest="go_command")
    parser.add_argument(
        "--source-mode",
        choices=("candidate", "current-stable"),
        default="candidate",
        help=(
            "verify exact HEAD as a candidate, or derive the immutable current "
            "stable commit from the canonical release policy"
        ),
    )
    parser.add_argument(
        "--write",
        action="store_true",
        help="refresh only the approved checksum metadata and test constants",
    )
    return parser


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    try:
        if args.write:
            if args.source_mode != "candidate":
                raise ChecksumContractError(
                    "current-stable mode is read-only and cannot refresh metadata"
                )
            policy_head_commit = _require_active_future_candidate(
                args.root,
                version=args.version,
            )
            result = refresh_repository_metadata(
                args.root,
                version=args.version,
                go_command=args.go_command,
                expected_head_commit=policy_head_commit,
            )
        elif args.source_mode == "current-stable":
            result = verify_current_stable_repository(
                args.root,
                version=args.version,
                go_command=args.go_command,
            )
        else:
            policy_head_commit = _require_active_future_candidate(
                args.root,
                version=args.version,
            )
            result = _verify_repository_tree(
                args.root,
                version=args.version,
                go_command=args.go_command,
                expected_head_commit=policy_head_commit,
            )
    except (ChecksumContractError, OSError) as exc:
        print(f"Framework/Admin checksum contract failed: {exc}", file=sys.stderr)
        return 1
    json.dump(result, sys.stdout, sort_keys=True)
    sys.stdout.write("\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
