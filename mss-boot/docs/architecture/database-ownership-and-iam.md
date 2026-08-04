# Database ownership and RDS IAM contracts

Status: accepted for the pre-v1 database-ownership migration.

## Problem

Historically `gormdb.Database.Init()` populated process-wide `gormdb.DB` and
`gormdb.Enforcer` variables. Initialization failures called `os.Exit`, database
configuration was mutated in place, and a configuration reload replaced global
references without closing the previous primary connection pool.

RDS IAM had additional security and lifetime problems:

- MySQL selected `tls=skip-verify`, which encrypted traffic without verifying
  the server identity.
- PostgreSQL generated one IAM token while building the DSN, so later physical
  connections could reuse an expired password.
- The IAM path replaced an entry in the global dialector map, coupling one
  database instance to every later instance in the process.

These behaviors made the framework unsuitable for embedding, deterministic
failure handling, hot reload, and multiple independently owned database stacks.

## Owned handle

The primary API is now:

```go
handle, err := databaseConfig.Open(ctx)
if err != nil {
    return err
}
defer handle.Close()

use(handle.DB, handle.Enforcer)
```

`Database.Open`:

- returns `(*gormdb.Handle, error)`;
- does not mutate the configuration object;
- does not publish package globals;
- does not terminate the process;
- owns the primary `database/sql` pool;
- closes the pool if Casbin construction or another post-open step fails.

`Handle.Close` is idempotent. The caller that receives the Handle owns it and is
responsible for closing it.

## Compatibility bridge

The exported `gormdb.DB` and `gormdb.Enforcer` variables remain temporarily for
existing applications. `gormdb.InstallDefault(handle)` publishes an owned
Handle through those variables but does not take ownership or close either the
new or previous Handle.

`Database.InitContext` is a migration helper that opens a Handle, installs it as
the compatibility default, and closes the previously installed Handle.
`Database.Init` preserves the historical no-return signature but only logs an
error. New startup code must use `Open` or `InitContext` and handle errors.

The compatibility globals cannot provide race-free hot swapping to callers that
read them directly. Applications should migrate constructors, repositories,
workers, and request actions to explicit `*gormdb.Handle` or `*gorm.DB`
dependencies before v1.

## RDS IAM connection contract

RDS IAM authentication is connection-scoped, not process-startup-scoped.
Therefore a fresh authentication token is generated before every new physical
connection:

- MySQL uses a custom `database/sql/driver.Connector` whose `Connect` method
  builds a token and creates a connection-specific driver config.
- PostgreSQL uses pgx stdlib's before-connect hook to update the password on the
  copied connection config immediately before dialing.

Connection maximum lifetime is capped at 14 minutes when IAM is enabled so a
pool does not retain connections beyond the token's intended operating window.
IAM and dbresolver registrations are rejected together in this migration slice;
supporting dynamic credentials across independently addressed source and
replica pools requires an explicit resolver ownership API.

## TLS contract

IAM TLS defaults are fail-closed:

- minimum TLS version is 1.2;
- the certificate chain is verified;
- the expected server name defaults to the configured database host;
- an optional `rootCAFile` augments the system trust store;
- request parameters cannot replace IAM TLS with `skip-verify`;
- `insecureSkipVerify` is retained only as an explicit compatibility escape
  hatch and defaults to false.

Production RDS deployments should provide the current AWS RDS trust bundle
through `rootCAFile` when it is not already trusted by the host operating
system.

## Configuration

```yaml
database:
  driver: postgres
  iam:
    enable: true
    region: us-east-1
    user: application
    host: database.example.us-east-1.rds.amazonaws.com
    port: 5432
    dbName: admin
    rootCAFile: /etc/ssl/rds/global-bundle.pem
    serverName: database.example.us-east-1.rds.amazonaws.com
    insecureSkipVerify: false
```

`params` accepts URL query parameters for both drivers. PostgreSQL also accepts
simple whitespace-separated `key=value` values. Host, port, user, password,
database, and TLS mode cannot be overridden through `params`.

## Migration

1. Replace startup calls to `Database.Init()` with `Open(ctx)` or
   `InitContext(ctx)` and propagate the error.
2. Store the returned Handle in the application composition root.
3. Inject `handle.DB` and `handle.Enforcer` into repositories and services.
4. On configuration reload, open the replacement first, atomically publish or
   inject it, and only then close the previous owned Handle.
5. Call `Close()` during application shutdown.

## Rollback

The Handle API is additive. Rolling back this change restores the old process
exit and global mutation behavior, so the complete database-ownership change
set should be reverted together. Do not keep the new application ownership
wiring while reverting only the framework Handle implementation.

## Remaining work

- Move the root admin composition to an owned Handle and close it on shutdown.
- Replace direct reads of `gormdb.DB` and `gormdb.Enforcer` with injected
  dependencies.
- Add ownership for every dbresolver source and replica pool.
- Remove the compatibility globals before v1.
- Make cache, queue watcher, migration, and task construction consume explicit
  database dependencies.
