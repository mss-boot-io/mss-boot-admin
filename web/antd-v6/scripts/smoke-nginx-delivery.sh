#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
web_dir="$(cd "${script_dir}/.." && pwd)"
image_name="mss-boot-admin-antd-v6-smoke:local-$$"
container_id=""

cleanup() {
  if [[ -n "${container_id}" ]]; then
    docker rm --force "${container_id}" >/dev/null 2>&1 || true
  fi
  docker image rm --force "${image_name}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

cd "${web_dir}"
test -s dist/index.html
docker build --tag "${image_name}" . >/dev/null
container_id="$(docker run --detach --rm --publish 127.0.0.1::80 "${image_name}")"

published_port=""
for _attempt in $(seq 1 30); do
  published_port="$(docker port "${container_id}" 80/tcp 2>/dev/null | sed -n 's/.*://p' | head -n 1)"
  if [[ -n "${published_port}" ]] && curl --silent --fail "http://127.0.0.1:${published_port}/healthz" >/dev/null; then
    break
  fi
  sleep 1
done
test -n "${published_port}"

entry_asset="$(find dist -maxdepth 1 -type f -name 'umi.*.js' -printf '%f\n' | sort | head -n 1)"
test -n "${entry_asset}"
base_url="http://127.0.0.1:${published_port}"
spa_headers="$(curl --silent --show-error --dump-header - --output /dev/null "${base_url}/workplace/")"
dynamic_spa_headers="$(curl --silent --show-error --dump-header - --output /dev/null "${base_url}/departments/42")"
entry_headers="$(curl --silent --show-error --dump-header - --output /dev/null --header 'Accept-Encoding: gzip' --header 'Via: 1.1 ingress' "${base_url}/${entry_asset}")"
missing_headers="$(curl --silent --show-error --dump-header - --output /dev/null "${base_url}/missing.deadbeef.async.js")"
worker_headers="$(curl --silent --show-error --dump-header - --output /dev/null "${base_url}/service-worker.js")"

grep --quiet --ignore-case '^HTTP/.* 200' <<<"${spa_headers}"
grep --quiet --ignore-case '^Cache-Control: .*no-store' <<<"${spa_headers}"
grep --quiet --ignore-case '^HTTP/.* 200' <<<"${dynamic_spa_headers}"
grep --quiet --ignore-case '^Cache-Control: .*no-store' <<<"${dynamic_spa_headers}"
grep --quiet --ignore-case '^HTTP/.* 200' <<<"${entry_headers}"
grep --quiet --ignore-case '^Content-Encoding: gzip' <<<"${entry_headers}"
grep --quiet --ignore-case '^Cache-Control: .*immutable' <<<"${entry_headers}"
grep --quiet --ignore-case '^HTTP/.* 404' <<<"${missing_headers}"
grep --quiet --ignore-case '^Cache-Control: .*no-store' <<<"${missing_headers}"
grep --quiet --ignore-case '^HTTP/.* 404' <<<"${worker_headers}"
grep --quiet --ignore-case '^Cache-Control: .*no-store' <<<"${worker_headers}"

echo 'Nginx v6 delivery smoke test passed'
