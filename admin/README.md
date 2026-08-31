# Importable Admin application

## v1.3.7 stable status

v1.3.7 is the current stable, adoptable Complete Admin Distribution. The public
module `github.com/mss-boot-io/mss-boot-admin/admin@v1.3.7` was qualified with
the matching Framework, Admin Web, Root tools, and images from exact merged-main
commit `77b53d41092741eac62fa6418c0bdbf87413c7cd`. Consumers must pin the complete
v1.3.7 train and must not combine this module with another patch or a local
replacement.

v1.3.5 and v1.3.6 remain immutable partial trains. v1.3.6 published the Admin
identity `github.com/mss-boot-io/mss-boot-admin/admin@v1.3.6` from commit
`b1fe47a3a83209574e09d53526b122dd2cbc5277`, but that train never completed.
Each public component remains immutable audit evidence and is not, by itself, a
complete Thin Host distribution. The Docs website publishes independently; a
Docs failure or later versioned Docs revision does not change this Admin
module's stable identity.

## Composition contract

`admin/app` is the complete backend composition entrypoint. It owns
authentication, authorization, configuration, migrations, HTTP delivery, and
the protected compile-time business extension boundary.

Owned business modules register explicitly:

```go
package main

import (
    "context"
    "log"

    "github.com/mss-boot-io/mss-boot-admin/admin/app"
    "github.com/acme/orders-admin/internal/modules/orders"
)

func main() {
    if err := app.ExecuteContext(
        context.Background(),
        app.WithBusinessModules(orders.Module()),
    ); err != nil {
        log.Fatal(err)
    }
}
```

Do not copy Admin startup, security middleware, migrations, or core routes into
a business repository. A business module may extend the protected group only
through `admin/business`; UI visibility never replaces backend authorization.

The v1.3.7 tool generates this composition as part of a version-bound Thin Host.
Use the public Root tool and package onboarding guide instead of copying this
repository's Admin source into a business application.

The v1.3.7 bootstrap contract reads the initial administrator password
through hidden input or a one-use `MSS_ADMIN_INITIAL_PASSWORD` secret, stores
only a one-way verifier, and exposes no default password. The generated local
application identity is user `admin` at `http://127.0.0.1:8001`.

See the [component status](../docs/docs/getting-started/packages.md) and
[Admin architecture](../docs/docs/architecture/complete-admin-distribution-and-thin-business-host.zh-CN.md).
Foundation source validation remains contributor-only in
[`AGENTS.md`](./AGENTS.md).
