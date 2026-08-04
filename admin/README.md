# mss-boot Admin application

This directory contains the deployable reference Admin application as an independent Go module.

```bash
cd admin
go run . server
go test ./...
```

The repository workspace resolves the sibling `mss-boot/` framework. The committed relative replace keeps independent module validation deterministic inside the monorepo until a stable nested framework release is published.
