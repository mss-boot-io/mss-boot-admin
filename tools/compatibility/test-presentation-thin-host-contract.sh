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

if backend_entries != [entry]:
    raise SystemExit(f"backend presentation inventory/hash drift: {backend_entries!r} != {[entry]!r}")
if frontend_entries != [entry]:
    raise SystemExit(f"frontend presentation registry/hash drift: {frontend_entries!r} != {[entry]!r}")
if frontend_inventory != [entry[0]]:
    raise SystemExit(f"frontend presentation inventory drift: {frontend_inventory!r} != {[entry[0]]!r}")
if f'generatedPresentationRegistry["{entry[0]}"]' not in generated_page:
    raise SystemExit("generated page registry key does not match the generated capability inventory")

if core_snapshot.get("sources") != [".mss/core-pages/user-list.yaml"]:
    raise SystemExit(f"Foundation core source inventory drift: {core_snapshot.get('sources')!r}")
core_manifests = core_snapshot.get("manifests")
if not isinstance(core_manifests, list) or len(core_manifests) != 1:
    raise SystemExit(f"Foundation core manifest inventory must contain exactly user.list: {core_manifests!r}")
core_manifest = core_manifests[0]
core_entry = (core_manifest.get("pageKey"), core_manifest.get("definitionHash"))
if core_entry[0] != "user.list" or not re.fullmatch(hash_pattern, str(core_entry[1])):
    raise SystemExit(f"invalid Foundation user.list identity: {core_entry!r}")
if core_manifest.get("definitionHash") not in core_definitions or "user.list" not in core_definitions:
    raise SystemExit("generated Go core definitions do not match the Foundation core manifest identity")

core_frontend_entries, core_frontend_inventory = parse_frontend_registry(
    core_frontend_registry,
    "corePresentationInventory",
)
if core_frontend_entries != [core_entry]:
    raise SystemExit(
        f"Foundation Go-manifest/TypeScript-registry hash drift: {core_frontend_entries!r} != {[core_entry]!r}"
    )
if core_frontend_inventory != [core_entry[0]]:
    raise SystemExit(
        f"Foundation core frontend inventory drift: {core_frontend_inventory!r} != {[core_entry[0]]!r}"
    )
if core_entry[1] not in core_frontend_registry:
    raise SystemExit("Foundation TypeScript core definition omits the generated manifest hash")

core_keys = {page_key for page_key, _ in core_frontend_entries}
business_keys = {page_key for page_key, _ in frontend_entries}
if core_keys & business_keys:
    raise SystemExit(f"Thin Host generated business registry copied a Foundation core page: {core_keys & business_keys}")
combined_inventory = core_frontend_inventory + frontend_inventory
if combined_inventory != ["user.list", "supplier.list"]:
    raise SystemExit(
        f"composed presentation inventory must be core union host business: {combined_inventory!r}"
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
	"testing"

	corepresentation "github.com/mss-boot-io/mss-boot-admin/admin/presentation/core"
)

func TestFoundationCorePackageMatchesGeneratedManifest(t *testing.T) {
	raw, err := os.ReadFile(os.Getenv("MSS_CORE_PRESENTATION_MANIFEST"))
	if err != nil {
		t.Fatalf("read generated core manifest: %v", err)
	}
	var snapshot struct {
		Sources   []string `json:"sources"`
		Manifests []struct {
			PageKey        string `json:"pageKey"`
			DefinitionHash string `json:"definitionHash"`
		} `json:"manifests"`
	}
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		t.Fatalf("decode generated core manifest: %v", err)
	}
	if len(snapshot.Sources) != 1 || snapshot.Sources[0] != ".mss/core-pages/user-list.yaml" {
		t.Fatalf("unexpected core sources: %#v", snapshot.Sources)
	}
	if len(snapshot.Manifests) != 1 {
		t.Fatalf("unexpected core manifests: %#v", snapshot.Manifests)
	}
	definitions := corepresentation.Definitions()
	if len(definitions) != 1 {
		t.Fatalf("unexpected packaged core definitions: %#v", definitions)
	}
	if definitions[0].PageKey != "user.list" ||
		definitions[0].PageKey != snapshot.Manifests[0].PageKey ||
		definitions[0].DefinitionHash != snapshot.Manifests[0].DefinitionHash {
		t.Fatalf("packaged core identity drift: %#v != %#v", definitions[0], snapshot.Manifests[0])
	}
}
''',
    encoding="utf-8",
)
PY
(
  cd -- "${go_consumer_root}"
  GOWORK=off MSS_CORE_PRESENTATION_MANIFEST="${foundation_core_manifest}" \
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
  "${admin_web_root}/package.json" \
  "${foundation_core_manifest}" <<'PY'
import json
import sys
from pathlib import Path

web = Path(sys.argv[1])
tarball = Path(sys.argv[2]).resolve()
admin_manifest = json.loads(Path(sys.argv[3]).read_text(encoding="utf-8"))
core_snapshot = json.loads(Path(sys.argv[4]).read_text(encoding="utf-8"))
core_hash = core_snapshot["manifests"][0]["definitionHash"]
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
  userPresentationListComponents,
  userPresentationRegistryEntry,
  userPresentationSearchComponents,
} from './node_modules/@mss-boot-io/admin-web/src/modules/administration/userPresentation';

describe('packaged Foundation core presentation', () => {
  it('resolves the generated user.list registry and compiled UserManagement adapter', () => {
    expect(corePresentationInventory).toEqual(['user.list']);
    expect(userPresentationRegistryEntry).toBe(corePresentationRegistry['user.list']);
    expect(userPresentationRegistryEntry.definitionHash).toBe('__CORE_HASH__');
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
""".replace("__CORE_HASH__", core_hash),
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
python3 - "${consumer_admin_web}" "${foundation_core_manifest}" <<'PY'
import json
import sys
from pathlib import Path

package = Path(sys.argv[1])
core_snapshot = json.loads(Path(sys.argv[2]).read_text(encoding="utf-8"))
core_manifest = core_snapshot["manifests"][0]
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

core_registry_path = package / "src/generated/core-presentation-registry.generated.ts"
user_adapter_path = package / "src/modules/administration/userPresentation.ts"
user_management_path = package / "src/modules/administration/UserManagement.tsx"
for required in (core_registry_path, user_adapter_path, user_management_path):
    if not required.is_file():
        raise SystemExit(f"installed Admin Web package omits {required.relative_to(package)}")
core_registry = core_registry_path.read_text(encoding="utf-8")
user_adapter = user_adapter_path.read_text(encoding="utf-8")
user_management = user_management_path.read_text(encoding="utf-8")
if core_manifest["pageKey"] not in core_registry or core_manifest["definitionHash"] not in core_registry:
    raise SystemExit("installed core registry does not match the Foundation generated manifest")
if "corePresentationRegistry['user.list']" not in user_adapter:
    raise SystemExit("installed UserManagement adapter does not bind the generated user.list core entry")
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
