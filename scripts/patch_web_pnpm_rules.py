#!/usr/bin/env python3
"""Write the explicit pnpm workspace peer-compatibility policy.

The listed missing peers are optional adapters exposed by Umi, minifiers,
stylelint, WebSocket, and Sass packages. The project does not use those optional
capabilities. The allowed versions document tested compatibility for legacy
editor/DVA packages that predate React 18 and for Utoo's less-loader range.
"""

from pathlib import Path


ROOT = Path(__file__).resolve().parents[1] / "web" / "antd"

OPTIONAL_PEERS = (
    "@minify-html/node",
    "@rspack/core",
    "@swc/core",
    "@swc/css",
    "@swc/html",
    "@swc/wasm",
    "@types/webpack",
    "@volar/vue-language-plugin-pug",
    "@volar/vue-typescript",
    "bufferutil",
    "canvas",
    "clean-css",
    "cssnano",
    "csso",
    "debug",
    "encoding",
    "esbuild",
    "fibers",
    "html-minifier-terser",
    "luxon",
    "node-notifier",
    "node-sass",
    "postcss",
    "postcss-html",
    "postcss-jsx",
    "postcss-less",
    "postcss-markdown",
    "postcss-scss",
    "react-native",
    "sass-embedded",
    "sockjs-client",
    "stylus",
    "supports-color",
    "uglify-js",
    "utf-8-validate",
    "webpack-cli",
    "webpack-dev-server",
    "webpack-hot-middleware",
    "webpack-plugin-serve",
)


def main() -> None:
    lines = [
        "packages:",
        "  - workers-site",
        "",
        "peerDependencyRules:",
        "  allowedVersions:",
        "    dva-core: 2.0.4",
        "    less-loader: 11.1.0",
        "    react: 18.2.0",
        "    react-dom: 18.2.0",
        "  ignoreMissing:",
    ]
    lines.extend(f"    - {dependency}" for dependency in OPTIONAL_PEERS)
    (ROOT / "pnpm-workspace.yaml").write_text(
        "\n".join(lines) + "\n",
        encoding="utf-8",
    )


if __name__ == "__main__":
    main()
