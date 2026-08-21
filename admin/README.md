# mss-boot Admin application

This directory contains the complete reusable and deployable Admin application
as an independent Go module:

```text
github.com/mss-boot-io/mss-boot-admin/admin
```

The coordinated stable target is `admin/v1.3.0`. After publication, an external
consumer pins it exactly:

```bash
go get github.com/mss-boot-io/mss-boot-admin/admin@v1.3.0
```

`app/` is the public composition root and `business/` is the compile-time
extension boundary. A Thin Host imports the complete Admin, registers owned
business modules explicitly, and does not copy core startup, authentication,
session, migration, route, or middleware source.

For repository-local development:

```bash
cd admin
STAGE=local go run . migrate
STAGE=local go run . server -a
STAGE=local go run . server
go test ./...
```

The one-shot `server -a` command synchronizes the API registry from the exact
mounted route tree. Run it with the same Stage and database as the resident
server; otherwise menu management can show an empty “Bind API” selection.

The repository `go.work` resolves the sibling `mss-boot/` framework for local
development. The published Admin module pins the exact public Framework version
without a committed `replace`; protected release qualification resolves that
version with `GOWORK=off` after the matching Framework release is public.

Before a release, the repository also creates a clean external Go module,
resolves both public versions with `GOWORK=off`, verifies their source hashes
and checksums, and builds a representative Thin Host. Business routes remain
behind the complete Admin middleware and migration-readiness boundary.
