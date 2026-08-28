# @mss-boot-io/admin-web

## v1.3.7 candidate status

v1.3.7 is the selected complete package-first candidate, but
`web/antd-v6/v1.3.7` and `@mss-boot-io/admin-web@1.3.7` are not public yet.
The exact candidate tarball must pass the shared Root preview and later publish
from the same merged-main commit before it is an adopter dependency.

v1.3.5 and v1.3.6 remain immutable-partial trains. `web/antd-v6/v1.3.6`, its
GitHub Release, and GitHub Packages assets are public and immutable, but the
official npmjs identity `@mss-boot-io/admin-web@1.3.6` was **not** published.
Neither partial train has a supported complete installation path.

The release policy still identifies **v1.3.2** as the current stable
distribution. A GitHub Packages artifact, Release tarball, local package, or
source checkout must not be substituted for the missing official npmjs
publication.

## Package contract

Admin Web is the Distribution's single complete browser application: React 19,
Ant Design 6, Umi Max, React Query, generated API contracts, authentication
shell, page states, locales, and the narrow business-route extension.

The v1.3.7 candidate npm package keeps these declared exports:

- `@mss-boot-io/admin-web` or `/runtime` for the packaged runtime;
- `/business` for generated business registration;
- `/preset` for the supported Umi preset;
- `/styles` for the public style entry;
- `/testing` for package-supported test helpers.

The package also defines the `mss-admin-web` command used by a generated Thin
Host. Consumers must not import `src/*`, copy core pages, redirect aliases to
Foundation source, or create a second SPA.

Business routes register together with their menu projection before the final
403/404 fallbacks. Client access checks improve user experience; backend
authorization remains authoritative. Retained pages represent loading, empty,
retryable error, denied, and responsive states with synchronized Chinese and
English locale keys.

The candidate also contains the statically compiled presentation-configuration
console. Its profiles remain strictly data-only, its frontend permission state
is advisory to backend RBAC, and production business capability registration is
empty so existing pages retain compiled defaults.

These are candidate complete-distribution contracts, not evidence that v1.3.7
or either partial train can be installed. Official npm publication remains
credentialless through the exact `npm-release.yml` plus `npm-auto` Trusted
Publisher identity; no npm token is restored. See
[package status](../../docs/docs/getting-started/packages.md) and
[mss-shop status](../../docs/docs/getting-started/mss-shop.md).
Repository-source build and publication commands remain contributor-only in
[`AGENTS.md`](./AGENTS.md) and [`CHANGELOG.md`](./CHANGELOG.md).
