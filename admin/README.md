# Importable Admin application

## v1.3.6 candidate status

v1.3.6 is the selected complete package-first candidate, but
`github.com/mss-boot-io/mss-boot-admin/admin@v1.3.6` is not public yet and must
not be installed or combined with unpublished candidate components. It can
publish only after one exact merged-main preview qualifies the complete train.

v1.3.5 remains an immutable-partial train. It published the Admin identity
`github.com/mss-boot-io/mss-boot-admin/admin@v1.3.5`, but it did not publish
the Root tools, official npmjs package, Docs, or complete external-consumer
path. The component remains public and immutable; it is not, by itself, a
complete Thin Host distribution.

The release policy still identifies **v1.3.2** as the current stable
distribution. Do not combine the v1.3.5 Admin module with another patch,
Foundation source, a local replacement, or an unpublished frontend package to
manufacture a mixed distribution.

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

The v1.3.6 candidate generates this composition as part of a version-bound Thin
Host. Until its Root tools, official npmjs package, images, and
external-consumer evidence are public, this file describes the composition
boundary only and is not an application-creation or setup guide. v1.3.6 is not adoptable
before publication and public external-consumer reconciliation.

The v1.3.6 candidate bootstrap contract reads the initial administrator password
through hidden input or a one-use `MSS_ADMIN_INITIAL_PASSWORD` secret, stores
only a one-way verifier, and exposes no default password. The generated local
application identity remains user `admin` at `http://127.0.0.1:8001`, but
those facts do not authorize prepublication v1.3.6 adoption or make the
incomplete v1.3.5 train adoptable.

See the [component status](../docs/docs/getting-started/packages.md) and
[Admin architecture](../docs/docs/architecture/complete-admin-distribution-and-thin-business-host.zh-CN.md).
Foundation source validation remains contributor-only in
[`AGENTS.md`](./AGENTS.md).
