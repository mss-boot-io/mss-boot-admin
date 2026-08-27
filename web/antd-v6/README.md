# @mss-boot-io/admin-web

## v1.3.6 candidate status

v1.3.6 is the selected complete package-first candidate, but
`web/antd-v6/v1.3.6` and `@mss-boot-io/admin-web@1.3.6` are not public yet.
The exact candidate tarball must pass the shared Root preview and later publish
from the same merged-main commit before it is an adopter dependency.

v1.3.5 remains an immutable-partial train. `web/antd-v6/v1.3.5` and the corresponding GitHub Release and GitHub
Packages identity are public and immutable. The official npmjs identity
`@mss-boot-io/admin-web@1.3.5` was **not** published, so v1.3.5 has no
supported public npm installation path and cannot complete a Thin Host.

The release policy still identifies **v1.3.2** as the current stable
distribution. A GitHub Packages artifact, Release tarball, local package, or
source checkout must not be substituted for the missing official npmjs
publication.

## Package contract

Admin Web is the Distribution's single complete browser application: React 19,
Ant Design 6, Umi Max, React Query, generated API contracts, authentication
shell, page states, locales, and the narrow business-route extension.

The v1.3.6 candidate npm package keeps these declared exports:

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

These are candidate complete-distribution contracts, not evidence that v1.3.6
or the missing v1.3.5 npmjs package can be installed. See
[package status](../../docs/docs/getting-started/packages.md) and
[mss-shop status](../../docs/docs/getting-started/mss-shop.md).
Repository-source build and publication commands remain contributor-only in
[`AGENTS.md`](./AGENTS.md) and [`CHANGELOG.md`](./CHANGELOG.md).
