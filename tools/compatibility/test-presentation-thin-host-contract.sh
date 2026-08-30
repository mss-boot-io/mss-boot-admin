#!/usr/bin/env bash
set -euo pipefail

foundation_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd -P)"
admin_web_root="${foundation_root}/web/antd-v6"
foundation_core_manifest="${foundation_root}/admin/presentation/core/manifest.generated.json"
foundation_core_definitions="${foundation_root}/admin/presentation/core/definitions_generated.go"
foundation_core_frontend_registry="${admin_web_root}/src/generated/core-presentation-registry.generated.ts"
temporary_parent="${TMPDIR:-/tmp}"
temporary_root="$(mktemp -d "${temporary_parent%/}/mss-presentation-thin-host.XXXXXX")"

case "${temporary_root}" in
  "${temporary_parent%/}"/mss-presentation-thin-host.*) ;;
  *)
    echo "refusing to use unexpected temporary directory: ${temporary_root}" >&2
    exit 1
    ;;
esac

cleanup() {
  rm -rf -- "${temporary_root}"
}
trap cleanup EXIT

for command in go node python3 corepack tar; do
  command -v "${command}" >/dev/null 2>&1 || {
    echo "${command} is required for the presentation Thin Host contract test" >&2
    exit 1
  }
done

for core_output in \
  "${foundation_core_manifest}" \
  "${foundation_core_definitions}" \
  "${foundation_core_frontend_registry}"; do
  [[ -f "${core_output}" ]] || {
    echo "Foundation core presentation output is missing: ${core_output#"${foundation_root}/"}" >&2
    exit 1
  }
done

host_root="${temporary_root}/host"
pack_root="${temporary_root}/package"
mkdir -p -- "${host_root}/.mss/modules" "${pack_root}"

python3 - "${foundation_root}" "${host_root}" <<'PY'
import sys
from pathlib import Path

foundation = Path(sys.argv[1])
host = Path(sys.argv[2])
replacements = {
    "__MSS_APP_NAME__": "presentation-thin-host",
    "__MSS_APP_DISPLAY_NAME_YAML__": "Presentation Thin Host",
    "__MSS_APP_REPOSITORY__": "example/presentation-thin-host",
    "__MSS_APP_MODULE__": "example.com/presentation-thin-host",
    "__MSS_DISTRIBUTION_NAME__": "mss-boot-admin",
    "__MSS_DISTRIBUTION_VERSION__": "v1.3.7",
    "__MSS_DISTRIBUTION_BACKEND_MODULE__": "github.com/mss-boot-io/mss-boot-admin/admin",
    "__MSS_DISTRIBUTION_BACKEND_VERSION__": "v1.3.7",
    "__MSS_DISTRIBUTION_FRONTEND_PACKAGE__": "@mss-boot-io/admin-web",
    "__MSS_DISTRIBUTION_FRONTEND_VERSION__": "1.3.7",
}
for relative in (".mss/project.yaml", ".mss/capabilities.yaml", ".mss/commands.yaml"):
    content = (foundation / "templates/application" / relative).read_text(encoding="utf-8")
    for source, target in replacements.items():
        content = content.replace(source, target)
    if "__MSS_" in content:
        raise SystemExit(f"unresolved Thin Host placeholder in {relative}")
    output = host / relative
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(content, encoding="utf-8")
(host / ".mss/modules/example-supplier.yaml").write_bytes(
    (foundation / ".mss/modules/example-supplier.yaml").read_bytes()
)
PY

(
  cd -- "${foundation_root}"
  go run ./cmd/mss --root "${host_root}" module generate \
    .mss/modules/example-supplier.yaml \
    --write \
    --frontend-target antd-v6 \
    --format json >/dev/null
)

if [[ -e "${host_root}/.mss/core-pages" ]]; then
  echo "Thin Host generation copied Foundation .mss/core-pages into the host" >&2
  exit 1
fi
mapfile -t leaked_core_outputs < <(
  find "${host_root}" -type f \( \
    -name 'core-presentation-registry.generated.ts' -o \
    -path '*/presentation/core/definitions_generated.go' -o \
    -path '*/presentation/core/manifest.generated.json' \
  \) -print
)
if [[ "${#leaked_core_outputs[@]}" -ne 0 ]]; then
  echo "Thin Host generation copied Foundation core output: ${leaked_core_outputs[*]}" >&2
  exit 1
fi

generated_test="${host_root}/web/src/generated/modules/supplier/presentation.generated.test.ts"
generated_page="${host_root}/web/src/generated/modules/supplier/SupplierPage.tsx"
generated_frontend_registry="${host_root}/web/src/generated/presentation-registry.generated.ts"
generated_backend_registry="${host_root}/internal/modules/all/presentation_generated.go"
generated_backend_manifest="${host_root}/internal/modules/supplier/presentation_manifest.generated.json"
for generated_output in \
  "${generated_test}" \
  "${generated_page}" \
  "${generated_frontend_registry}" \
  "${generated_backend_registry}" \
  "${generated_backend_manifest}"; do
  [[ -f "${generated_output}" ]] || {
    echo "Thin Host generation omitted ${generated_output#"${host_root}/"}" >&2
    exit 1
  }
done
grep -Fq "from '@mss-boot-io/admin-web/runtime/presentation'" "${generated_test}" || {
  echo "generated Thin Host test does not use the public presentation runtime subpath" >&2
  exit 1
}
if grep -Fq "../../../shared/" "${generated_test}"; then
  echo "generated Thin Host test depends on Foundation-only shared sources" >&2
  exit 1
fi
grep -Fq "from '@mss-boot-io/admin-web/runtime/presentation/client'" "${generated_page}" || {
  echo "generated Supplier page does not use the public effective presentation client" >&2
  exit 1
}
grep -Fq "from '../../presentation-registry.generated'" "${generated_page}" || {
  echo "generated Supplier page does not import the Thin Host application registry" >&2
  exit 1
}
grep -Fq 'generatedPresentationRegistry["supplier.list"]' "${generated_page}" || {
  echo "generated Supplier page does not inject its application-local presentation entry" >&2
  exit 1
}
grep -Fq 'const presentation = presentationRuntime.model;' "${generated_page}" || {
  echo "generated Supplier page does not consume the effective presentation model" >&2
  exit 1
}
if grep -Fq '/src/shared/' "${generated_page}" || grep -Fq '../../../shared/' "${generated_page}"; then
  echo "generated Supplier page bypasses the public Admin Web runtime exports" >&2
  exit 1
fi

python3 - \
  "${generated_backend_registry}" \
  "${generated_backend_manifest}" \
  "${generated_frontend_registry}" \
  "${generated_page}" \
  "${foundation_core_manifest}" \
  "${foundation_core_definitions}" \
  "${foundation_core_frontend_registry}" <<'PY'
import json
import re
import sys
from pathlib import Path

backend_registry = Path(sys.argv[1]).read_text(encoding="utf-8")
backend_manifest = json.loads(Path(sys.argv[2]).read_text(encoding="utf-8"))["manifest"]
frontend_registry = Path(sys.argv[3]).read_text(encoding="utf-8")
generated_page = Path(sys.argv[4]).read_text(encoding="utf-8")
core_snapshot = json.loads(Path(sys.argv[5]).read_text(encoding="utf-8"))
core_definitions = Path(sys.argv[6]).read_text(encoding="utf-8")
core_frontend_registry = Path(sys.argv[7]).read_text(encoding="utf-8")

expected_core_sources = [
    ".mss/core-pages/department-list.yaml",
    ".mss/core-pages/language-list.yaml",
    ".mss/core-pages/log-audit.yaml",
    ".mss/core-pages/log-login.yaml",
    ".mss/core-pages/log-runtime.yaml",
    ".mss/core-pages/menu-list.yaml",
    ".mss/core-pages/notice-list.yaml",
    ".mss/core-pages/online-session-list.yaml",
    ".mss/core-pages/option-list.yaml",
    ".mss/core-pages/post-list.yaml",
    ".mss/core-pages/role-list.yaml",
    ".mss/core-pages/system-config-list.yaml",
    ".mss/core-pages/task-list.yaml",
    ".mss/core-pages/user-list.yaml",
]
expected_core_entries = [
    ("department.list", "sha256:01d4677adb1b5aeacc0e81ef51e7f4f46bdaad17e73a8c38075704c2e304be5d"),
    ("language.list", "sha256:ff7b315dad5e7dfac8d49dce8c861bfb7bedf53917bc4e349f30d40c3fee7e15"),
    ("log.audit", "sha256:59fd4abe332bf09a1b0bf6e51db0f1eda68ae4f1f13c788ba833ecd23ae53672"),
    ("log.login", "sha256:4a95df60fcb4c04ece595a0eebaf9248adaffe8c3bb3f165b7a95c110bf2dbb3"),
    ("log.runtime", "sha256:cfea5bb3cb1b40609961c58e2ab6a5ea233328d9102f36e65bc8859099aa1525"),
    ("menu.list", "sha256:fccb8f0274fca51f53f2a9275b37c437f34bd5a488405584163b1cd5d1047273"),
    ("notice.list", "sha256:9c6008eee24d6723210dbe1a673fc89639484db1a200475e046f91de2ee88f32"),
    ("online-session.list", "sha256:9d63e964add45526f33909cc1967f3c3000e98dc2c8482ab317923ece0c5f599"),
    ("option.list", "sha256:4a7f22ec4f1d67f77ca47fe43ad01b733a5ccab355a5aaa26e70c1008836ea1b"),
    ("post.list", "sha256:538e929e7939343e0a367385ee504069d991e0ecb9de46721dea7f1549cb28a2"),
    ("role.list", "sha256:2c45710008671e2553efaf940df1964486366f8dc0e769958558c592f8a053b3"),
    ("system-config.list", "sha256:9cec42c9129b5f0e509d46ee286cfdf76805c2fec4929b9927cf0a800a73e24d"),
    ("task.list", "sha256:26d1b40ec2fb58041f3f64e361ffb069b128e16a216a6ee82e8333cd5dd4a3a4"),
    ("user.list", "sha256:2a114a21ae575eabbb8b91f7ee3b0c288549985e6eeee9473eecdda91267e33f"),
]
expected_business_entry = (
    "supplier.list",
    "sha256:20979dbd25719ae69c07e383627849910d8a7fb8beb95b023362d56a4c4d72c7",
)

entry = (backend_manifest["pageKey"], backend_manifest["definitionHash"])
hash_pattern = r"sha256:[0-9a-f]{64}"
backend_entries = re.findall(rf"// ([a-z0-9][a-z0-9.-]*) ({hash_pattern})$", backend_registry, re.MULTILINE)


def parse_frontend_registry(source: str, inventory_symbol: str):
    entries = re.findall(
        rf'^\s+"([a-z0-9][a-z0-9.-]*)": \{{\n\s+definitionHash: "({hash_pattern})",$',
        source,
        re.MULTILINE,
    )
    inventory_match = re.search(
        rf"{re.escape(inventory_symbol)} = \[(.*?)\] as const;",
        source,
        re.DOTALL,
    )
    if inventory_match is None:
        raise SystemExit(f"frontend presentation inventory is missing: {inventory_symbol}")
    inventory = re.findall(r'"([a-z0-9][a-z0-9.-]*)"', inventory_match.group(1))
    return entries, inventory


frontend_entries, frontend_inventory = parse_frontend_registry(
    frontend_registry,
    "generatedPresentationInventory",
)

if entry != expected_business_entry:
    raise SystemExit(f"Supplier manifest identity drift: {entry!r} != {expected_business_entry!r}")
if backend_entries != [expected_business_entry]:
    raise SystemExit(
        f"backend presentation inventory/hash drift: {backend_entries!r} != {[expected_business_entry]!r}"
    )
if frontend_entries != [expected_business_entry]:
    raise SystemExit(
        f"frontend presentation registry/hash drift: {frontend_entries!r} != {[expected_business_entry]!r}"
    )
if frontend_inventory != [expected_business_entry[0]]:
    raise SystemExit(
        f"frontend presentation inventory drift: {frontend_inventory!r} != {[expected_business_entry[0]]!r}"
    )
if f'generatedPresentationRegistry["{expected_business_entry[0]}"]' not in generated_page:
    raise SystemExit("generated page registry key does not match the generated capability inventory")

if core_snapshot.get("sources") != expected_core_sources:
    raise SystemExit(
        f"Foundation core source inventory drift: {core_snapshot.get('sources')!r} != {expected_core_sources!r}"
    )
core_manifests = core_snapshot.get("manifests")
if not isinstance(core_manifests, list):
    raise SystemExit(f"Foundation core manifest inventory is not a list: {core_manifests!r}")
core_manifest_entries = [
    (manifest.get("pageKey"), manifest.get("definitionHash"))
    for manifest in core_manifests
]
if core_manifest_entries != expected_core_entries:
    raise SystemExit(
        f"Foundation core manifest inventory/hash drift: {core_manifest_entries!r} != {expected_core_entries!r}"
    )

definitions_match = re.search(r'^const definitionsJSON = (".*")$', core_definitions, re.MULTILINE)
if definitions_match is None:
    raise SystemExit("generated Go core definitionsJSON constant is missing")
try:
    decoded_definitions = json.loads(json.loads(definitions_match.group(1)))
except json.JSONDecodeError as error:
    raise SystemExit(f"generated Go core definitionsJSON is invalid: {error}") from error
go_core_entries = [
    (definition.get("pageKey"), definition.get("definitionHash"))
    for definition in decoded_definitions
]
if go_core_entries != expected_core_entries:
    raise SystemExit(
        f"Foundation generated Go inventory/hash drift: {go_core_entries!r} != {expected_core_entries!r}"
    )

core_frontend_entries, core_frontend_inventory = parse_frontend_registry(
    core_frontend_registry,
    "corePresentationInventory",
)
if core_frontend_entries != expected_core_entries:
    raise SystemExit(
        f"Foundation TypeScript registry inventory/hash drift: {core_frontend_entries!r} != {expected_core_entries!r}"
    )
expected_core_inventory = [page_key for page_key, _ in expected_core_entries]
if core_frontend_inventory != expected_core_inventory:
    raise SystemExit(
        f"Foundation core frontend inventory drift: {core_frontend_inventory!r} != {expected_core_inventory!r}"
    )

core_keys = {page_key for page_key, _ in core_frontend_entries}
business_keys = {page_key for page_key, _ in frontend_entries}
if core_keys & business_keys:
    raise SystemExit(f"Thin Host generated business registry copied a Foundation core page: {core_keys & business_keys}")
combined_inventory = core_frontend_inventory + frontend_inventory
expected_combined_inventory = expected_core_inventory + [expected_business_entry[0]]
if combined_inventory != expected_combined_inventory:
    raise SystemExit(
        "composed presentation inventory must be the fixed Foundation core union host Supplier: "
        f"{combined_inventory!r} != {expected_combined_inventory!r}"
    )
PY

go_consumer_root="${temporary_root}/go-consumer"
mkdir -p -- "${go_consumer_root}"
python3 - "${foundation_root}" "${go_consumer_root}" <<'PY'
import sys
from pathlib import Path

foundation = Path(sys.argv[1]).resolve()
consumer = Path(sys.argv[2])
(consumer / "go.mod").write_text(
    "\n".join(
        [
            "module example.com/mss-presentation-core-consumer",
            "",
            "go 1.26.6",
            "",
            "require github.com/mss-boot-io/mss-boot-admin/admin v0.0.0",
            "",
            f"replace github.com/mss-boot-io/mss-boot-admin/admin => {(foundation / 'admin').as_posix()}",
            "",
        ]
    ),
    encoding="utf-8",
)
(consumer / "core_contract_test.go").write_text(
    r'''package corecontract

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	corepresentation "github.com/mss-boot-io/mss-boot-admin/admin/presentation/core"
)

type coreIdentity struct {
	PageKey        string `json:"pageKey"`
	DefinitionHash string `json:"definitionHash"`
}

var expectedCoreSources = []string{
	".mss/core-pages/department-list.yaml",
	".mss/core-pages/language-list.yaml",
	".mss/core-pages/log-audit.yaml",
	".mss/core-pages/log-login.yaml",
	".mss/core-pages/log-runtime.yaml",
	".mss/core-pages/menu-list.yaml",
	".mss/core-pages/notice-list.yaml",
	".mss/core-pages/online-session-list.yaml",
	".mss/core-pages/option-list.yaml",
	".mss/core-pages/post-list.yaml",
	".mss/core-pages/role-list.yaml",
	".mss/core-pages/system-config-list.yaml",
	".mss/core-pages/task-list.yaml",
	".mss/core-pages/user-list.yaml",
}

var expectedCoreIdentities = []coreIdentity{
	{PageKey: "department.list", DefinitionHash: "sha256:01d4677adb1b5aeacc0e81ef51e7f4f46bdaad17e73a8c38075704c2e304be5d"},
	{PageKey: "language.list", DefinitionHash: "sha256:ff7b315dad5e7dfac8d49dce8c861bfb7bedf53917bc4e349f30d40c3fee7e15"},
	{PageKey: "log.audit", DefinitionHash: "sha256:59fd4abe332bf09a1b0bf6e51db0f1eda68ae4f1f13c788ba833ecd23ae53672"},
	{PageKey: "log.login", DefinitionHash: "sha256:4a95df60fcb4c04ece595a0eebaf9248adaffe8c3bb3f165b7a95c110bf2dbb3"},
	{PageKey: "log.runtime", DefinitionHash: "sha256:cfea5bb3cb1b40609961c58e2ab6a5ea233328d9102f36e65bc8859099aa1525"},
	{PageKey: "menu.list", DefinitionHash: "sha256:fccb8f0274fca51f53f2a9275b37c437f34bd5a488405584163b1cd5d1047273"},
	{PageKey: "notice.list", DefinitionHash: "sha256:9c6008eee24d6723210dbe1a673fc89639484db1a200475e046f91de2ee88f32"},
	{PageKey: "online-session.list", DefinitionHash: "sha256:9d63e964add45526f33909cc1967f3c3000e98dc2c8482ab317923ece0c5f599"},
	{PageKey: "option.list", DefinitionHash: "sha256:4a7f22ec4f1d67f77ca47fe43ad01b733a5ccab355a5aaa26e70c1008836ea1b"},
	{PageKey: "post.list", DefinitionHash: "sha256:538e929e7939343e0a367385ee504069d991e0ecb9de46721dea7f1549cb28a2"},
	{PageKey: "role.list", DefinitionHash: "sha256:2c45710008671e2553efaf940df1964486366f8dc0e769958558c592f8a053b3"},
	{PageKey: "system-config.list", DefinitionHash: "sha256:9cec42c9129b5f0e509d46ee286cfdf76805c2fec4929b9927cf0a800a73e24d"},
	{PageKey: "task.list", DefinitionHash: "sha256:26d1b40ec2fb58041f3f64e361ffb069b128e16a216a6ee82e8333cd5dd4a3a4"},
	{PageKey: "user.list", DefinitionHash: "sha256:2a114a21ae575eabbb8b91f7ee3b0c288549985e6eeee9473eecdda91267e33f"},
}

var expectedBusinessIdentity = coreIdentity{
	PageKey:        "supplier.list",
	DefinitionHash: "sha256:20979dbd25719ae69c07e383627849910d8a7fb8beb95b023362d56a4c4d72c7",
}

func TestFoundationCorePackageMatchesFixedDistributionContract(t *testing.T) {
	raw, err := os.ReadFile(os.Getenv("MSS_CORE_PRESENTATION_MANIFEST"))
	if err != nil {
		t.Fatalf("read generated core manifest: %v", err)
	}
	var snapshot struct {
		Sources   []string       `json:"sources"`
		Manifests []coreIdentity `json:"manifests"`
	}
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		t.Fatalf("decode generated core manifest: %v", err)
	}
	if !reflect.DeepEqual(snapshot.Sources, expectedCoreSources) {
		t.Fatalf("core source contract drift: %#v != %#v", snapshot.Sources, expectedCoreSources)
	}
	if !reflect.DeepEqual(snapshot.Manifests, expectedCoreIdentities) {
		t.Fatalf("core manifest identity drift: %#v != %#v", snapshot.Manifests, expectedCoreIdentities)
	}

	definitions := corepresentation.Definitions()
	packagedIdentities := make([]coreIdentity, 0, len(definitions))
	for _, definition := range definitions {
		packagedIdentities = append(packagedIdentities, coreIdentity{
			PageKey:        definition.PageKey,
			DefinitionHash: definition.DefinitionHash,
		})
	}
	if !reflect.DeepEqual(packagedIdentities, expectedCoreIdentities) {
		t.Fatalf("packaged Go core identity drift: %#v != %#v", packagedIdentities, expectedCoreIdentities)
	}

	businessRaw, err := os.ReadFile(os.Getenv("MSS_BUSINESS_PRESENTATION_MANIFEST"))
	if err != nil {
		t.Fatalf("read generated business manifest: %v", err)
	}
	var businessSnapshot struct {
		Manifest coreIdentity `json:"manifest"`
	}
	if err := json.Unmarshal(businessRaw, &businessSnapshot); err != nil {
		t.Fatalf("decode generated business manifest: %v", err)
	}
	if businessSnapshot.Manifest != expectedBusinessIdentity {
		t.Fatalf("generated business identity drift: %#v != %#v", businessSnapshot.Manifest, expectedBusinessIdentity)
	}
	combinedIdentities := append(append([]coreIdentity{}, packagedIdentities...), businessSnapshot.Manifest)
	expectedCombinedIdentities := append(append([]coreIdentity{}, expectedCoreIdentities...), expectedBusinessIdentity)
	if !reflect.DeepEqual(combinedIdentities, expectedCombinedIdentities) {
		t.Fatalf("external Go core-plus-business composition drift: %#v != %#v", combinedIdentities, expectedCombinedIdentities)
	}
}
''',
    encoding="utf-8",
)
PY
(
  cd -- "${go_consumer_root}"
  GOWORK=off \
    MSS_CORE_PRESENTATION_MANIFEST="${foundation_core_manifest}" \
    MSS_BUSINESS_PRESENTATION_MANIFEST="${generated_backend_manifest}" \
    go test -mod=mod ./...
)

(
  cd -- "${admin_web_root}"
  corepack pnpm@10.34.5 pack --pack-destination "${pack_root}" >/dev/null
)
mapfile -t tarballs < <(find "${pack_root}" -maxdepth 1 -type f -name '*.tgz' -print)
if [[ "${#tarballs[@]}" -ne 1 ]]; then
  echo "Admin Web pack must produce exactly one tarball" >&2
  exit 1
fi
tarball="${tarballs[0]}"

declare -A npm_tar_inventory=()
while IFS= read -r package_entry; do
  npm_tar_inventory["${package_entry}"]=1
done < <(tar -tzf "${tarball}")
for required_package_entry in \
  package/src/generated/core-presentation-registry.generated.ts \
  package/src/modules/administration/userPresentation.ts \
  package/src/modules/administration/tablePresentation.ts \
  package/src/modules/operations/tablePresentation.ts \
  package/src/modules/language/tablePresentation.ts \
  package/src/modules/option/tablePresentation.ts \
  package/src/modules/session/tablePresentation.ts \
  package/src/modules/administration/UserManagement.tsx; do
  if [[ -z "${npm_tar_inventory["${required_package_entry}"]+present}" ]]; then
    echo "Admin Web tarball omits packaged Foundation core runtime: ${required_package_entry}" >&2
    exit 1
  fi
done
for package_entry in "${!npm_tar_inventory[@]}"; do
  case "${package_entry}" in
    package/.mss/core-pages/* | package/admin/presentation/core/*)
      echo "Admin Web tarball copied a non-frontend Foundation core source: ${package_entry}" >&2
      exit 1
      ;;
  esac
done

python3 - \
  "${host_root}/web" \
  "${tarball}" \
  "${admin_web_root}/package.json" <<'PY'
import json
import sys
from pathlib import Path

web = Path(sys.argv[1])
tarball = Path(sys.argv[2]).resolve()
admin_manifest = json.loads(Path(sys.argv[3]).read_text(encoding="utf-8"))
web.mkdir(parents=True, exist_ok=True)
dependencies = dict(admin_manifest["dependencies"])
dependencies["@mss-boot-io/admin-web"] = f"file:{tarball}"
manifest = {
    "name": "presentation-thin-host-web",
    "version": "0.0.0",
    "private": True,
    "type": "module",
    "packageManager": "pnpm@10.34.5",
    "engines": {"node": ">=24.0.0 <25", "pnpm": "10.34.5"},
    "dependencies": dict(sorted(dependencies.items())),
}
(web / "package.json").write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
(web / "tsconfig.presentation.json").write_text(
    json.dumps(
        {
            "compilerOptions": {
                "target": "ESNext",
                "module": "ESNext",
                "moduleResolution": "Bundler",
                "strict": True,
                "noEmit": True,
                "skipLibCheck": True,
                "resolveJsonModule": True,
                "jsx": "react-jsx",
                "types": ["node", "vitest/globals"],
            },
            "include": [
                "runtime.umijs-shim.d.ts",
                "src/generated/presentation-registry.generated.ts",
                "src/generated/modules/supplier/presentation.generated.ts",
                "src/generated/modules/supplier/presentation.adapter.generated.tsx",
                "src/generated/modules/supplier/presentation.generated.test.ts",
                "runtime.presentation.test.ts",
                "runtime.effective.test.ts",
                "runtime.core.test.ts",
            ],
        },
        indent=2,
    )
    + "\n",
    encoding="utf-8",
)
(web / "runtime.umijs-shim.d.ts").write_text(
    """declare module '@umijs/max' {
  export function request<T = unknown>(
    path: string,
    options?: object,
  ): Promise<T>;

  export function useIntl(): {
    locale: string;
    formatMessage(descriptor: { id: string }, values?: Record<string, unknown>): string;
  };
}
""",
    encoding="utf-8",
)
(web / "vitest.presentation.config.mts").write_text(
    """import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    environment: 'node',
    include: [
      'src/generated/modules/supplier/presentation.generated.test.ts',
      'runtime.presentation.test.ts',
      'runtime.effective.test.ts',
      'runtime.core.test.ts',
    ],
  },
});
""",
    encoding="utf-8",
)
(web / "runtime.presentation.test.ts").write_text(
    """import { describe, expect, it } from 'vitest';
import { compilePortablePresentationPattern } from '@mss-boot-io/admin-web/runtime/presentation';

describe('published presentation runtime', () => {
  it('exports Unicode-safe portable pattern compilation', () => {
    expect(compilePortablePresentationPattern('^[^A]$').test('😀')).toBe(true);
    expect(compilePortablePresentationPattern('^[A-Z0-9_-]+$').test('SUPPLIER_01\\n')).toBe(false);
    expect(() => compilePortablePresentationPattern('^.$')).toThrow(/portable/i);
    expect(() => compilePortablePresentationPattern('^\\\\s$')).toThrow(/portable/i);
  });
});
""",
    encoding="utf-8",
)
(web / "runtime.effective.test.ts").write_text(
    """import { describe, expect, it, vi } from 'vitest';

vi.mock('@umijs/max', () => ({
  request: vi.fn(),
  useIntl: () => ({ locale: 'en-US', formatMessage: ({ id }: { id: string }) => id }),
}));

import {
  createEffectivePresentationAPI,
  resolveEffectivePagePresentation,
} from '@mss-boot-io/admin-web/runtime/presentation/client';
import { generatedPresentationRegistry } from './src/generated/presentation-registry.generated';

describe('published effective presentation client', () => {
  it('resolves the public effective client against the application-local registry', async () => {
    const entry = generatedPresentationRegistry['supplier.list'];
    const client = vi.fn(async () => ({
      pageKey: entry.definition.pageKey,
      definitionHash: entry.definitionHash,
      adoption: {
        mode: 'active',
        state: 'active',
        recoveryMode: false,
        allowlisted: true,
        resolveLayers: true,
        applyLayers: true,
      },
      layers: {},
      diagnostics: [],
    }));
    const response = await createEffectivePresentationAPI(client).load(
      entry.definition.pageKey,
      new AbortController().signal,
    );
    const runtime = resolveEffectivePagePresentation({
      entry,
      locale: 'en-US',
      response,
      settled: true,
    });

    expect(client).toHaveBeenCalledWith('/presentation/effective/supplier.list', {
      method: 'GET',
      signal: expect.any(AbortSignal),
      skipErrorHandler: true,
    });
    expect(runtime.source).toBe('active');
    expect(runtime.definition.definitionHash).toBe(entry.definitionHash);
    expect(runtime.model.title).toBe('Suppliers');
  });
});
""",
    encoding="utf-8",
)
(web / "runtime.core.test.ts").write_text(
    """import { describe, expect, it } from 'vitest';
import {
  corePresentationInventory,
  corePresentationRegistry,
} from './node_modules/@mss-boot-io/admin-web/src/generated/core-presentation-registry.generated';
import {
  generatedPresentationInventory,
  generatedPresentationRegistry,
} from './src/generated/presentation-registry.generated';
import {
  departmentPresentationRegistryEntry,
  menuPresentationRegistryEntry,
  postPresentationRegistryEntry,
  rolePresentationRegistryEntry,
} from './node_modules/@mss-boot-io/admin-web/src/modules/administration/tablePresentation';
import {
  userPresentationListComponents,
  userPresentationRegistryEntry,
  userPresentationSearchComponents,
} from './node_modules/@mss-boot-io/admin-web/src/modules/administration/userPresentation';
import {
  languagePresentationRegistryEntry,
} from './node_modules/@mss-boot-io/admin-web/src/modules/language/tablePresentation';
import {
  auditLogPresentationRegistryEntry,
  loginLogPresentationRegistryEntry,
  noticePresentationRegistryEntry,
  runtimeLogPresentationRegistryEntry,
  systemConfigPresentationRegistryEntry,
  taskPresentationRegistryEntry,
} from './node_modules/@mss-boot-io/admin-web/src/modules/operations/tablePresentation';
import {
  optionPresentationRegistryEntry,
} from './node_modules/@mss-boot-io/admin-web/src/modules/option/tablePresentation';
import {
  onlineSessionPresentationRegistryEntry,
} from './node_modules/@mss-boot-io/admin-web/src/modules/session/tablePresentation';

const expectedCoreEntries = [
  ['department.list', 'sha256:01d4677adb1b5aeacc0e81ef51e7f4f46bdaad17e73a8c38075704c2e304be5d'],
  ['language.list', 'sha256:ff7b315dad5e7dfac8d49dce8c861bfb7bedf53917bc4e349f30d40c3fee7e15'],
  ['log.audit', 'sha256:59fd4abe332bf09a1b0bf6e51db0f1eda68ae4f1f13c788ba833ecd23ae53672'],
  ['log.login', 'sha256:4a95df60fcb4c04ece595a0eebaf9248adaffe8c3bb3f165b7a95c110bf2dbb3'],
  ['log.runtime', 'sha256:cfea5bb3cb1b40609961c58e2ab6a5ea233328d9102f36e65bc8859099aa1525'],
  ['menu.list', 'sha256:fccb8f0274fca51f53f2a9275b37c437f34bd5a488405584163b1cd5d1047273'],
  ['notice.list', 'sha256:9c6008eee24d6723210dbe1a673fc89639484db1a200475e046f91de2ee88f32'],
  ['online-session.list', 'sha256:9d63e964add45526f33909cc1967f3c3000e98dc2c8482ab317923ece0c5f599'],
  ['option.list', 'sha256:4a7f22ec4f1d67f77ca47fe43ad01b733a5ccab355a5aaa26e70c1008836ea1b'],
  ['post.list', 'sha256:538e929e7939343e0a367385ee504069d991e0ecb9de46721dea7f1549cb28a2'],
  ['role.list', 'sha256:2c45710008671e2553efaf940df1964486366f8dc0e769958558c592f8a053b3'],
  ['system-config.list', 'sha256:9cec42c9129b5f0e509d46ee286cfdf76805c2fec4929b9927cf0a800a73e24d'],
  ['task.list', 'sha256:26d1b40ec2fb58041f3f64e361ffb069b128e16a216a6ee82e8333cd5dd4a3a4'],
  ['user.list', 'sha256:2a114a21ae575eabbb8b91f7ee3b0c288549985e6eeee9473eecdda91267e33f'],
] as const;

const corePresentationConsumers = {
  'department.list': departmentPresentationRegistryEntry,
  'language.list': languagePresentationRegistryEntry,
  'log.audit': auditLogPresentationRegistryEntry,
  'log.login': loginLogPresentationRegistryEntry,
  'log.runtime': runtimeLogPresentationRegistryEntry,
  'menu.list': menuPresentationRegistryEntry,
  'notice.list': noticePresentationRegistryEntry,
  'online-session.list': onlineSessionPresentationRegistryEntry,
  'option.list': optionPresentationRegistryEntry,
  'post.list': postPresentationRegistryEntry,
  'role.list': rolePresentationRegistryEntry,
  'system-config.list': systemConfigPresentationRegistryEntry,
  'task.list': taskPresentationRegistryEntry,
  'user.list': userPresentationRegistryEntry,
} as const;

describe('packaged Foundation core presentation', () => {
  it('resolves the fixed 14-page inventory, hashes, and compiled page consumers', () => {
    const expectedInventory = expectedCoreEntries.map(([pageKey]) => pageKey);
    expect(corePresentationInventory).toEqual(expectedInventory);
    expect(Object.keys(corePresentationRegistry)).toEqual(expectedInventory);
    expect(Object.keys(corePresentationConsumers)).toEqual(expectedInventory);

    for (const [pageKey, definitionHash] of expectedCoreEntries) {
      const entry = corePresentationRegistry[pageKey];
      expect(entry.definition.pageKey).toBe(pageKey);
      expect(entry.definitionHash).toBe(definitionHash);
      expect(corePresentationConsumers[pageKey]).toBe(entry);
      expect(entry.definition.actions).toEqual([]);
      expect(entry.definition.dataSources).toHaveLength(1);
      expect(entry.definition.dataSources[0]?.id).toBe(pageKey);
      expect(entry.definition.dataSources[0]?.pageSizeOptions).toEqual([20, 50, 100]);
      expect(entry.definition.dataSources[0]?.maxPageSize).toBe(100);
      expect(entry.definition.dataSources[0]?.maxSortFields).toBe(0);
    }

    expect([...corePresentationInventory, ...generatedPresentationInventory]).toEqual([
      ...expectedInventory,
      'supplier.list',
    ]);
    expect(generatedPresentationRegistry['supplier.list'].definitionHash).toBe(
      'sha256:20979dbd25719ae69c07e383627849910d8a7fb8beb95b023362d56a4c4d72c7',
    );
  });

  it('keeps the handwritten UserManagement adapter bound to user.list', () => {
    expect(userPresentationRegistryEntry).toBe(corePresentationRegistry['user.list']);
    expect(userPresentationRegistryEntry.definition.dataSources).toEqual([
      {
        id: 'user.list',
        requiredPermissions: ['/users'],
        pageSizeOptions: [20, 50, 100],
        maxPageSize: 100,
        maxSortFields: 0,
      },
    ]);
    expect(userPresentationRegistryEntry.definition.actions).toEqual([]);
    expect(userPresentationListComponents.status).toBe('status-tag');
    expect(userPresentationSearchComponents).toEqual({
      name: 'input',
      status: 'status-filter',
    });
  });
});
""",
    encoding="utf-8",
)
PY

consumer_node_modules="${host_root}/web/node_modules"
consumer_admin_web="${consumer_node_modules}/@mss-boot-io/admin-web"
(
  cd -- "${host_root}/web"
  corepack pnpm@10.34.5 install \
    --ignore-scripts \
    --no-frozen-lockfile \
    --prefer-offline \
    --reporter=append-only
)
[[ -d "${consumer_admin_web}" ]] || {
  echo "external pnpm install did not resolve @mss-boot-io/admin-web" >&2
  exit 1
}
python3 - "${consumer_admin_web}" <<'PY'
import json
import re
import sys
from pathlib import Path

package = Path(sys.argv[1])
expected_core_entries = [
    ("department.list", "sha256:01d4677adb1b5aeacc0e81ef51e7f4f46bdaad17e73a8c38075704c2e304be5d"),
    ("language.list", "sha256:ff7b315dad5e7dfac8d49dce8c861bfb7bedf53917bc4e349f30d40c3fee7e15"),
    ("log.audit", "sha256:59fd4abe332bf09a1b0bf6e51db0f1eda68ae4f1f13c788ba833ecd23ae53672"),
    ("log.login", "sha256:4a95df60fcb4c04ece595a0eebaf9248adaffe8c3bb3f165b7a95c110bf2dbb3"),
    ("log.runtime", "sha256:cfea5bb3cb1b40609961c58e2ab6a5ea233328d9102f36e65bc8859099aa1525"),
    ("menu.list", "sha256:fccb8f0274fca51f53f2a9275b37c437f34bd5a488405584163b1cd5d1047273"),
    ("notice.list", "sha256:9c6008eee24d6723210dbe1a673fc89639484db1a200475e046f91de2ee88f32"),
    ("online-session.list", "sha256:9d63e964add45526f33909cc1967f3c3000e98dc2c8482ab317923ece0c5f599"),
    ("option.list", "sha256:4a7f22ec4f1d67f77ca47fe43ad01b733a5ccab355a5aaa26e70c1008836ea1b"),
    ("post.list", "sha256:538e929e7939343e0a367385ee504069d991e0ecb9de46721dea7f1549cb28a2"),
    ("role.list", "sha256:2c45710008671e2553efaf940df1964486366f8dc0e769958558c592f8a053b3"),
    ("system-config.list", "sha256:9cec42c9129b5f0e509d46ee286cfdf76805c2fec4929b9927cf0a800a73e24d"),
    ("task.list", "sha256:26d1b40ec2fb58041f3f64e361ffb069b128e16a216a6ee82e8333cd5dd4a3a4"),
    ("user.list", "sha256:2a114a21ae575eabbb8b91f7ee3b0c288549985e6eeee9473eecdda91267e33f"),
]
consumer_bindings = {
    "src/modules/administration/userPresentation.ts": ["user.list"],
    "src/modules/administration/tablePresentation.ts": [
        "role.list",
        "menu.list",
        "department.list",
        "post.list",
    ],
    "src/modules/operations/tablePresentation.ts": [
        "task.list",
        "notice.list",
        "system-config.list",
        "log.login",
        "log.audit",
        "log.runtime",
    ],
    "src/modules/language/tablePresentation.ts": ["language.list"],
    "src/modules/option/tablePresentation.ts": ["option.list"],
    "src/modules/session/tablePresentation.ts": ["online-session.list"],
}
manifest = json.loads((package / "package.json").read_text(encoding="utf-8"))
for subpath in ("./runtime/presentation", "./runtime/presentation/client"):
    export = manifest.get("exports", {}).get(subpath)
    if not isinstance(export, dict):
        raise SystemExit(f"published Admin Web package omits {subpath}")
    targets = {export.get("types"), export.get("import"), export.get("default")}
    if None in targets or len(targets) != 1:
        raise SystemExit(f"published Admin Web package has inconsistent targets for {subpath}: {export!r}")
    target = next(iter(targets))
    if not isinstance(target, str) or not target.startswith("./") or not (package / target).is_file():
        raise SystemExit(f"published Admin Web package target for {subpath} is not resolvable: {target!r}")

required_paths = [
    package / "src/generated/core-presentation-registry.generated.ts",
    package / "src/modules/administration/UserManagement.tsx",
    *(package / relative for relative in consumer_bindings),
]
for required in required_paths:
    if not required.is_file():
        raise SystemExit(f"installed Admin Web package omits {required.relative_to(package)}")
core_registry = required_paths[0].read_text(encoding="utf-8")
hash_pattern = r"sha256:[0-9a-f]{64}"
core_entries = re.findall(
    rf'^\s+"([a-z0-9][a-z0-9.-]*)": \{{\n\s+definitionHash: "({hash_pattern})",$',
    core_registry,
    re.MULTILINE,
)
inventory_match = re.search(
    r"corePresentationInventory = \[(.*?)\] as const;",
    core_registry,
    re.DOTALL,
)
if inventory_match is None:
    raise SystemExit("installed Admin Web package omits corePresentationInventory")
core_inventory = re.findall(r'"([a-z0-9][a-z0-9.-]*)"', inventory_match.group(1))
expected_inventory = [page_key for page_key, _ in expected_core_entries]
if core_entries != expected_core_entries:
    raise SystemExit(
        f"installed core registry identity drift: {core_entries!r} != {expected_core_entries!r}"
    )
if core_inventory != expected_inventory:
    raise SystemExit(
        f"installed core registry inventory drift: {core_inventory!r} != {expected_inventory!r}"
    )

bound_keys = []
for relative, page_keys in consumer_bindings.items():
    consumer = (package / relative).read_text(encoding="utf-8")
    for page_key in page_keys:
        binding = re.compile(
            rf"corePresentationRegistry\[['\"]{re.escape(page_key)}['\"]\]"
        )
        if binding.search(consumer) is None:
            raise SystemExit(f"installed {relative} does not bind the fixed {page_key} core entry")
        bound_keys.append(page_key)
if sorted(bound_keys) != sorted(expected_inventory):
    raise SystemExit(
        f"installed core consumer coverage drift: {sorted(bound_keys)!r} != {sorted(expected_inventory)!r}"
    )

user_management = (package / "src/modules/administration/UserManagement.tsx").read_text(encoding="utf-8")
if "from './userPresentation'" not in user_management:
    raise SystemExit("installed UserManagement does not consume its compiled presentation adapter")
PY

(
  cd -- "${host_root}/web"
  "${consumer_node_modules}/.bin/tsc" \
  --project tsconfig.presentation.json \
  --noEmit
  "${consumer_node_modules}/.bin/vitest" run \
    --config vitest.presentation.config.mts \
    src/generated/modules/supplier/presentation.generated.test.ts \
    runtime.presentation.test.ts \
    runtime.effective.test.ts \
    runtime.core.test.ts
)

echo "presentation Thin Host contract passed: external Go/npm consumers, core-plus-business inventory/hash parity, no core copying, runtime/effective exports, TypeScript, and targeted Vitest"
