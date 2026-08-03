# Changelog

All notable changes to the mss-boot framework will be documented in this file.

## [v0.7.3] - 2026-06-07

### Added

- Structured GitHub issue forms and refreshed open-source entry points.
- Local developer Makefile targets for `test`, `coverage`, and `tidy`.

### Changed

- Refreshed README quick-start guidance to use the latest `v0.7.3` module version.
- Corrected Makefile build metadata import path to `github.com/mss-boot-io/mss-boot`.

### Fixed

- Kept GORM query-cache tag invalidation complete for create, update, and delete paths.
- Hardened Mongo delete handling by validating ObjectID input before deletion.

### Validation

- GitHub CI, lint, CodeQL, docs-drift, PR guard, and govulncheck passed before release.
- Local `go test ./pkg/response/actions/mgm ./...` passed on the release commit.

## [v0.7.1] - 2026-04-06

### Added

#### Core Framework Enhancements
- **Advanced Error Handling**: Standardized error codes with comprehensive error categorization
- **Action Scope Management**: Context-aware operation scoping for better resource management
- **Enhanced Testing Infrastructure**: Comprehensive test suite with 80%+ coverage requirements
- **Integration Testing Support**: Robust integration tests for database and API interactions

#### Documentation
- **Comprehensive API Documentation**: Complete Swagger documentation for all core components
- **Migration Guides**: Clear upgrade paths from previous versions
- **Usage Examples**: Detailed code examples for common use cases

### Changed

#### Dependency Updates
- Updated all Go module dependencies to latest stable versions
- Enhanced compatibility with Go 1.21+
- Improved security by addressing known vulnerabilities in dependencies
- Optimized dependency tree for reduced binary size

#### Error Handling
- Unified error response format across all components
- Enhanced error propagation with proper context preservation
- Improved error logging with structured fields
- Better error recovery mechanisms

#### Action Scope Management
- Refined scope boundaries for clearer operation isolation
- Enhanced context passing between components
- Improved resource cleanup and lifecycle management
- Better integration with existing middleware stack

### Fixed

#### Stability Improvements
- Resolved nil pointer dereference risks in core components
- Fixed race conditions in concurrent operations
- Addressed memory leaks in long-running services
- Improved error handling in edge cases

#### Performance Optimizations
- Reduced memory allocation in hot paths
- Optimized database query performance
- Enhanced caching strategies for frequently accessed data
- Improved response times for high-concurrency scenarios

### Security

#### Security Enhancements
- Strengthened input validation across all components
- Enhanced authentication and authorization checks
- Improved secret handling and storage
- Added security headers to HTTP responses

## [v0.7.0] - Previous Version

### Added
- Initial support for DynamoDB
- Configuration provider framework
- Istio tracing integration
- Out-of-the-box service templates

### Changed
- Core architecture refactoring
- Improved modularity and extensibility
- Enhanced documentation structure

### Removed
- Deprecated legacy components
- Insecure default configurations

## Migration Guide

### From v0.7.0 to v0.7.1

#### Error Handling Changes
```go
// Old (v0.7.0)
if err != nil {
    return err
}

// New (v0.7.1) - Use standardized error codes
if err != nil {
    return errors.Wrap(err, "operation_failed", errors.WithCode(errors.CodeInternal))
}
```

#### Action Scope Usage
```go
// New in v0.7.1 - Context-aware operations
ctx := action.NewContext(context.Background(), "user_operation")
result, err := service.Process(ctx, request)
```

#### Dependency Updates
Ensure your `go.mod` is updated:
```go
require github.com/mss-boot-io/mss-boot v0.7.1
```

Run dependency update:
```bash
go mod tidy
```

## Upgrade Instructions

### Basic Upgrade
```bash
# Update go.mod
go get github.com/mss-boot-io/mss-boot@v0.7.1

# Tidy dependencies
go mod tidy

# Run tests to verify compatibility
go test ./...
```

### Full Migration
```bash
# 1. Backup your current code
cp -r my-service my-service-backup

# 2. Update dependency
go get github.com/mss-boot-io/mss-boot@v0.7.1

# 3. Update error handling patterns
#   - Replace custom error codes with standardized ones
#   - Add action scopes to critical operations

# 4. Run comprehensive tests
go test ./... -coverprofile=coverage.out
go test -tags=integration ./...

# 5. Verify functionality
#   - Test all critical user flows
#   - Validate error scenarios
#   - Check performance metrics
```

## Breaking Changes

### Minimal Breaking Changes
- **Error Code Structure**: Custom error codes should be migrated to standardized format
- **Context Usage**: Consider adding action scopes for better operation tracking
- **Dependency Updates**: Some third-party APIs may have minor breaking changes

### Compatibility Notes
- All existing APIs remain functional with warnings for deprecated patterns
- Database schemas are backward compatible
- Configuration formats remain unchanged

## Known Issues

### Current Limitations
1. **Multi-tenant Support**: Still experimental, not recommended for production
2. **Advanced Tracing**: Some tracing features require manual configuration
3. **Edge Case Error Handling**: Rare edge cases may still produce generic errors

### Workarounds
- Use standardized error codes for all new development
- Implement proper action scoping for complex operations
- Monitor logs for any unexpected error patterns

## Future Roadmap

### Planned Features
- **v0.8.0**: Enhanced multi-tenant support
- **v0.8.0**: Advanced observability with OpenTelemetry
- **v0.9.0**: Service mesh integration improvements
- **v1.0.0**: Production-ready stability guarantee

### Long-term Goals
- Comprehensive testing suite with 90%+ coverage
- Advanced security features including RBAC
- Enhanced developer experience with better tooling
- Expanded ecosystem with additional service templates

## Contributors

Thanks to all contributors who made v0.7.1 possible.

## License

MIT License - see [LICENSE](./LICENSE) for details.
