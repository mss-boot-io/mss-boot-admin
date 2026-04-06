# mss-boot-admin AGENTS

## Scope
- This file applies to the Go backend in `mss-boot-admin/`.
- Inherit workspace rules from the root `AGENTS.md`, then apply the backend-specific rules below.

## Default interaction model
- Even when backend work is needed, the default user-facing entry remains leader.
- Treat backend implementation here as leader-routed work unless the user explicitly asks to work directly at the backend-specialist level.
- Use `aigc/prompts/roles/leader-role.zh-CN.md` for orchestration and `aigc/prompts/roles/backend-developer-role.zh-CN.md` for backend-specialist execution context.

## Project shape
- Stack: Gin + GORM + Casbin + JWT/OAuth2 + Swagger.
- This project is the backend service for the admin system.
- Primary directories and responsibilities:
  - `apis/`: REST controllers and route-facing handlers.
  - `cmd/`: CLI entrypoints such as `migrate` and `server`.
  - `config/`: configuration loading and definitions.
  - `dto/`: request, response, and search DTOs.
  - `models/`: GORM entities and shared model bases.
  - `service/`: business logic.
  - `middleware/`: auth, permission, tenant, logging, recovery.
  - `router/`: controller registration and route wiring.
  - `center/`: central services such as tenant, cache, queue, and statistics.
  - `pkg/`: shared helpers and extension points.
  - `compose/`: integration and dependency environments.

## Data and tenancy conventions
- Use the project base model types instead of inventing parallel model foundations.
- Tenant-aware models should build on `ModelGormTenant`.
- Tenant isolation relies on tenant-aware scoping and the existing tenant model infrastructure.
- Keep `TenantID`, creator-related fields, and remark-style metadata aligned with existing model patterns.
- Respect existing table, foreign-key, and index naming conventions.

## Controller and API conventions
- Prefer the existing `response.Controller` and `controller.NewSimple(...)` patterns for CRUD-style APIs.
- Register controllers through the established controller/response registration flow.
- Keep REST routes aligned with existing generated CRUD conventions.
- Add Swagger annotations for public API handlers and keep them in the project’s established annotation style.

## DTO conventions
- Search DTOs should reuse the project search base types where applicable.
- Request/response DTOs should use the existing tag conventions:
  - `json` for payload fields
  - `binding` for validation rules
  - `query` together with `form` for query/form parameters
  - `uri` for path parameters
- Match current DTO naming and validation style instead of introducing a new schema pattern.

## Auth, middleware, and task patterns
- Reuse existing auth and login provider patterns rather than creating parallel authentication flows.
- Register middleware through the existing middleware registry/pipeline.
- For scheduled work, follow the current task model and handler registration approach.
- For permission work, stay aligned with existing Casbin role/menu/API modeling.

## Configuration and startup
- Configuration priority is environment variables first, then CLI/config providers down to local/embed config.
- Treat `DB_DSN` as a key environment variable for local startup and migration flows.
- Standard backend workflow:
  1. Set database connection variables.
  2. Run `go run main.go migrate`.
  3. Run `go run main.go server -a` when API docs/routes need regeneration.
  4. Run `go run main.go server`.

## Testing and verification
- Unit tests use `*_test.go` naming.
- Shared test fixtures belong in `testdata/`.
- Integration-style validation can depend on `compose/*/docker-compose.yml` environments.
- Before considering backend work complete, prefer the smallest relevant verification: focused tests first, then broader checks if your change affects startup, generated API docs, migrations, or integration boundaries.

## Security and performance guardrails
- Avoid raw SQL string concatenation when GORM parameterization fits.
- Preserve password, token, and revocation patterns already used by the project.
- Validate input and avoid output patterns that would weaken XSS protections.
- Watch for N+1 queries, missing indexes, and cache misuse when changing data-heavy code.
- Use existing locking, queue, and context-based timeout patterns for concurrency-sensitive work.

## Common troubleshooting anchors
- Tenant issues: check `Referer` handling and tenant-domain configuration.
- Permission issues: inspect Casbin policy loading and role/menu/API relationships.
- Migration issues: check `DB_DSN`, database compatibility, and migration files under `cmd/migrate/migration/`.

## Contribution hygiene
- Follow normal Go formatting/lint expectations already used in the project.
- Keep docs and annotations in sync with behavior changes.
- Do not bypass the framework’s existing extension points when an established pattern already exists.

## Backend startup commands
**IMPORTANT**: Backend startup commands must be run asynchronously to avoid blocking the terminal.

### Correct async startup pattern
```bash
# From mss-boot-admin directory, use setsid to run in background:
cd /home/lwx/go/src/github.com/mss-boot-io/mss-boot-admin
setsid /tmp/mss-boot-admin server > /tmp/backend.log 2>&1 &
sleep 5
lsof -i :8080  # Verify port is listening

# Or with go run (build first for reliability):
go build -o /tmp/mss-boot-admin .
setsid /tmp/mss-boot-admin server > /tmp/backend.log 2>&1 &
```

### WRONG - will block terminal
```bash
# These will cause the terminal to hang:
go run . server      # Blocks until killed
go run . server -a   # Blocks until killed
/tmp/mss-boot-admin server  # Blocks until killed
```

### API endpoint format
- All API endpoints are under `/admin/api/` path
- Example: `/admin/api/options`, `/admin/api/users`
- Requires authentication (JWT token in cookie or header)
- Frontend proxy: `/admin/` -> `http://localhost:8080`

### Common startup issues
- `-a` flag requires database data and will exit after saving API routes
- If `-a` causes startup failure, run without it first
- Redis must be running on `127.0.0.1:6379` (check with `docker ps`)
