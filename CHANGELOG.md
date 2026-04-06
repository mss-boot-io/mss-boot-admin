# Changelog

All notable changes to this project will be documented in this file.

## [v1.0.0] - 2026-04-06

### Added - Phase 4 Runtime & Operations

#### Runtime Logs
- Login logs with IP, location, user agent tracking
- Audit logs with HTTP request/response capture and operation duration
- Runtime log viewer with level filtering and keyword search
- Log cleaner service for automatic old log cleanup

#### Monitoring & Visualization
- Real-time CPU, memory, disk usage monitoring
- Trend charts with 2 decimal precision
- Network I/O statistics
- Runtime information (goroutines, heap, GC count)

#### Storage Security
- File size validation on upload
- File type whitelist validation
- MIME type verification

#### Alert System
- Configurable alert rules (CPU, memory, disk metrics)
- Multi-channel notifications (Email, DingTalk, WeChat)
- Alert history tracking
- Threshold-based triggering with duration

#### WebSocket Cluster
- Redis Pub/Sub for distributed WebSocket
- Production-ready cluster support
- Online user tracking

#### API Documentation
- Swagger annotations for all public APIs
- Interactive API documentation UI

#### i18n Improvements
- Accept-Language header handling
- ISO 639-1 language code validation
- Public language list API

### Added - Phase 5 Production Readiness

#### Deployment Standardization
- Docker deployment guide
- Nginx reverse proxy configuration
- Production configuration checklist
- Environment variable best practices

#### Testing & Verification
- Comprehensive test case documentation (70+ scenarios)
- Test execution report
- Test data initialization script
- Regression testing verification

#### Observability
- Pyroscope integration for continuous profiling
- PProf endpoints for runtime profiling
- Logging best practices guide

#### Security Baseline
- Authentication and authorization security
- Input validation requirements
- HTTPS enforcement
- Secret management guidelines

### Fixed - Product Polish Round 1

#### Backend Stability
- Menu API binding nil dereference risk
- Monitor disk partition out of bounds risk
- Alert checker duplicate close panic risk
- Sensitive endpoint authentication coverage

#### Frontend Correctness
- Login page undefined status variable
- Department/Post navigation redirect paths
- Password reset success branch logic
- Frontend polling cleanup in NoticeIcon and Generator

### Fixed - Product Polish Round 2

#### TypeScript Quality
- Removed invalid `skipErrorHandler` parameter
- Fixed NotificationContext import paths and API calls
- Removed duplicate keys in locales
- Fixed undefined index access in Log pages
- Fixed Access component property usage

#### Page Structure
- Removed double PageContainer in AppConfig
- Unified page skeleton consistency

### Fixed - Product Polish Round 3

#### Backend Consistency
- Unified authentication error response format
- Consolidated login/audit logging through service layer
- Removed package cycle in user_auth_token

#### Frontend Reusability
- Extracted AuthShell component for login/register/forget pages
- Reduced duplicate auth page layout code by 44 lines

### Changed

#### API Response Format
- Unified all mutating endpoints to return `{}` instead of `null`
- Consistent response envelope: `code`, `msg`, `data`

#### Data Precision
- Monitoring data now uses 2 decimal places
- Disk/memory units standardized to GB

#### Code Structure
- Created reusable `useMonitorData` hook
- Abstracted monitor data fetching and error handling
- Standardized error response and logging patterns

### Security

#### Authentication & Authorization
- Enhanced middleware authentication coverage
- Added AuthHandler to sensitive operational endpoints
- Improved Casbin rule management with AfterCreate hooks

#### File Upload
- File size limit enforcement
- File type whitelist validation
- MIME type verification

#### Audit Trail
- Complete HTTP request/response logging
- Operation duration tracking
- Centralized audit logging through service layer

### Documentation

#### New Documentation
- Product polish and governance plan
- Remediation checklist with priorities
- HotGo competitive analysis
- Full test case documentation
- Test execution report
- Pre-release checklist

#### Updated Documentation
- Phase 4 roadmap (all items completed)
- Phase 5 roadmap (all items completed)
- Integration test guide
- Release verification checklist

### Removed

- Multi-tenant architecture (explicitly removed from product direction)
- Code generation as primary workflow (L3 deprecated)

## Version History

| Version | Date | Description |
|---------|------|-------------|
| v1.0.0 | 2026-04-06 | Product polish, full testing, production readiness |
| - | - | Phase 4: Runtime, monitoring, alerts, i18n |
| - | - | Phase 5: Deployment, testing, security, docs |
| - | - | Three rounds of product polish |
| - | - | Full test coverage and documentation |

---

## Migration Guide

### From Previous Versions

#### Configuration Changes

1. **Auth Secret Key**
   ```yaml
   # Old (development default)
   auth:
     key: 'mss-boot-admin-secret'
   
   # New (production required)
   auth:
     key: '${AUTH_KEY}'  # Use environment variable
   ```

2. **Redis Password**
   ```yaml
   # Old (development default)
   cache:
     redis:
       password: 123456
   
   # New (production required)
   cache:
     redis:
       password: '${REDIS_PASSWORD}'
   ```

#### Database Migration

No schema changes that require special migration. Standard `go run . migrate` is sufficient.

#### API Changes

All existing APIs remain compatible. New standardized response format:

```json
// Old (some endpoints)
{
  "code": 200,
  "msg": "success",
  "data": null
}

// New (all endpoints)
{
  "code": 200,
  "msg": "success",
  "data": {}
}
```

---

## Upgrade Instructions

### Backend

```bash
# 1. Pull latest code
git pull origin main

# 2. Update dependencies
go mod tidy

# 3. Run migrations (if any)
go run . migrate

# 4. Build
go build

# 5. Deploy
# Follow deployment guide
```

### Frontend

```bash
# 1. Pull latest code
git pull origin main

# 2. Update dependencies
pnpm install

# 3. Build
pnpm build

# 4. Deploy static files
# Copy dist/ to production server
```

---

## Known Issues

### Current Limitations

1. **File Storage**
   - Only local storage supported
   - OSS/COS/MinIO support planned for Phase 6

2. **Multi-tenant**
   - Explicitly removed from product direction
   - Single-tenant architecture only

3. **Code Generation**
   - L3 features deprecated
   - Not recommended for new projects

### Workarounds

No critical workarounds needed. All known issues have been addressed in current release.

---

## Future Roadmap

### Phase 6 (Planned)

- Multi-storage backend support (OSS, COS, MinIO)
- Kubernetes deployment manifests
- AI annotation collaboration standardization
- Audit log visualization
- Extended message queue support

### Long-term

- Performance benchmarking suite
- Security penetration testing
- Automated E2E testing
- Advanced monitoring dashboards

---

## Contributors

Thanks to all contributors who made this release possible.

---

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.