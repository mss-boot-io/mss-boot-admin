# Business module specifications

Create `AdminModule` specifications in this directory and use the Foundation
`mss` CLI with this repository as `--root` to generate the backend, frontend,
migration, authorization, menu, test, and documentation delivery unit.

From the exact Foundation checkout pinned by `.mss/lock.yaml`:

```shell
go run ./cmd/mss --root /path/to/thin-host spec init supplier \
  --kind module --output .mss/modules/supplier.yaml --write
go run ./cmd/mss --root /path/to/thin-host spec validate \
  .mss/modules/supplier.yaml --format json
go run ./cmd/mss --root /path/to/thin-host module generate \
  .mss/modules/supplier.yaml --frontend-target antd-v6 --write --format json
```

Review and commit the specification before generated output. The initializer
assigns deterministic module-specific migration identities; generation still
fails closed if either identity conflicts with an existing module or migration.
