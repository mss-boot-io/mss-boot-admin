# Importable Admin application

The v1.3.5 candidate publishes
`github.com/mss-boot-io/mss-boot-admin/admin@v1.3.5` as the complete Admin
backend for a Thin Host. It owns authentication, authorization, configuration,
migrations, HTTP delivery, and the protected compile-time business extension
boundary. Use the command below only after the public module reconciles with
the coordinated release commit.

Add the exact public module:

```sh
go get github.com/mss-boot-io/mss-boot-admin/admin@v1.3.5
```

Compose owned business modules explicitly:

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

Create the complete host with `mss new app` instead of assembling these files
by hand. See [packages](../docs/docs/getting-started/packages.md) and the
[Admin architecture](../docs/docs/architecture/complete-admin-distribution-and-thin-business-host.zh-CN.md).

For a fresh Thin Host, interactive `mss setup` reads the initial administrator
password through its built-in hidden prompt. Non-interactive automation passes
the same one-use value through `MSS_ADMIN_INITIAL_PASSWORD` for the setup
process only. It must be 8-128 characters and contain a letter and a number.
The migration stores a one-way verifier; the value is not a CLI argument or
generated file, and completed migrations do not require it again.
The initial username is `admin`; sign in at the generated Admin Web address
with the password supplied to the first setup. There is no default password.

Foundation source validation remains documented for contributors in
[`AGENTS.md`](./AGENTS.md); it is not an adopter installation path.
