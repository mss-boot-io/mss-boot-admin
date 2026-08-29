#!/usr/bin/env bash
set -euo pipefail

foundation_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd -P)"
admin_web_root="${foundation_root}/web/antd-v6"
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

for command in go node python3 corepack tar realpath; do
  command -v "${command}" >/dev/null 2>&1 || {
    echo "${command} is required for the presentation Thin Host contract test" >&2
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
    "__MSS_DISTRIBUTION_VERSION__": "v1.3.6",
    "__MSS_DISTRIBUTION_BACKEND_MODULE__": "github.com/mss-boot-io/mss-boot-admin/admin",
    "__MSS_DISTRIBUTION_BACKEND_VERSION__": "v1.3.6",
    "__MSS_DISTRIBUTION_FRONTEND_PACKAGE__": "@mss-boot-io/admin-web",
    "__MSS_DISTRIBUTION_FRONTEND_VERSION__": "1.3.6",
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

generated_test="${host_root}/web/src/generated/modules/supplier/presentation.generated.test.ts"
[[ -f "${generated_test}" ]] || {
  echo "Thin Host generation omitted the Supplier presentation contract test" >&2
  exit 1
}
grep -Fq "from '@mss-boot-io/admin-web/runtime/presentation'" "${generated_test}" || {
  echo "generated Thin Host test does not use the public presentation runtime subpath" >&2
  exit 1
}
if grep -Fq "../../../shared/" "${generated_test}"; then
  echo "generated Thin Host test depends on Foundation-only shared sources" >&2
  exit 1
fi

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

python3 - "${host_root}/web" "${tarball}" <<'PY'
import json
import sys
from pathlib import Path

web = Path(sys.argv[1])
tarball = Path(sys.argv[2]).resolve()
web.mkdir(parents=True, exist_ok=True)
manifest = {
    "name": "presentation-thin-host-web",
    "version": "0.0.0",
    "private": True,
    "type": "module",
    "packageManager": "pnpm@10.34.5",
    "dependencies": {"@mss-boot-io/admin-web": f"file:{tarball}"},
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
                "src/generated/modules/supplier/presentation.generated.ts",
                "src/generated/modules/supplier/presentation.adapter.generated.tsx",
                "src/generated/modules/supplier/presentation.generated.test.ts",
                "runtime.presentation.test.ts",
            ],
        },
        indent=2,
    )
    + "\n",
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
PY

consumer_node_modules="${host_root}/web/node_modules"
consumer_admin_web="${consumer_node_modules}/@mss-boot-io/admin-web"
mkdir -p -- "${consumer_admin_web}" "${consumer_node_modules}/@types"
tar -xzf "${tarball}" -C "${consumer_admin_web}" --strip-components=1
ln -s -- "$(realpath "${admin_web_root}/node_modules/vitest")" "${consumer_node_modules}/vitest"
ln -s -- "$(realpath "${admin_web_root}/node_modules/typescript")" "${consumer_node_modules}/typescript"
ln -s -- "$(realpath "${admin_web_root}/node_modules/@types/node")" "${consumer_node_modules}/@types/node"

(
  cd -- "${host_root}/web"
  "${admin_web_root}/node_modules/.bin/tsc" \
  --project tsconfig.presentation.json \
  --noEmit
  "${admin_web_root}/node_modules/.bin/vitest" run \
    --config vitest.presentation.config.mts \
    src/generated/modules/supplier/presentation.generated.test.ts \
    runtime.presentation.test.ts
)

echo "presentation Thin Host contract passed: external package subpath, TypeScript, and targeted Vitest"
