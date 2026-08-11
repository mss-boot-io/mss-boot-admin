package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/mss-boot-io/mss-boot-admin/admin/middleware"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/admin/modules/supplier"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration"
	migrationmodels "github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration/models"
)

type supplierRouteDependencies struct {
	authentication gin.HandlerFunc
	principal      supplier.AdminPrincipalResolver
	events         supplier.EventCollector
}

func mountSupplierRoutesAfterMigrationReadiness(
	ctx context.Context,
	db *gorm.DB,
	group *gin.RouterGroup,
) error {
	if middleware.Auth == nil {
		return errors.New("supplier authentication middleware is not initialized")
	}
	return mountSupplierRoutesWithDependencies(ctx, db, group, supplierRouteDependencies{
		authentication: middleware.Auth.MiddlewareFunc(),
		principal:      middleware.GetVerify,
		events:         supplierDomainEventLogger{},
	})
}

func mountSupplierRoutesWithDependencies(
	ctx context.Context,
	db *gorm.DB,
	group *gin.RouterGroup,
	dependencies supplierRouteDependencies,
) error {
	if ctx == nil {
		return errors.New("supplier route composition context is required")
	}
	if db == nil || group == nil {
		return errors.New("supplier route composition is not initialized")
	}
	if dependencies.authentication == nil {
		return errors.New("supplier authentication middleware is required")
	}
	if err := verifySupplierRuntimeReadiness(ctx, db); err != nil {
		return err
	}
	application, err := supplier.NewService(db, dependencies.events)
	if err != nil {
		return fmt.Errorf("compose supplier application: %w", err)
	}
	authorizer, err := supplier.NewAdminAuthorizer(db, dependencies.principal)
	if err != nil {
		return fmt.Errorf("compose supplier authorization: %w", err)
	}
	api := group.Group("/api", dependencies.authentication)
	if err := supplier.RegisterRoutes(api, application, authorizer); err != nil {
		return fmt.Errorf("register supplier routes: %w", err)
	}
	return nil
}

func verifySupplierRuntimeReadiness(ctx context.Context, db *gorm.DB) error {
	if ctx == nil {
		return errors.New("supplier schema readiness context is required")
	}
	if db == nil {
		return errors.New("supplier schema readiness database is required")
	}
	if err := migration.Migrate.ValidateRegistrations(); err != nil {
		return fmt.Errorf("supplier migration registration readiness failed: %w", err)
	}

	wanted := []string{
		supplier.SupplierMigrationID.String(),
		supplier.SupplierAuthorizationMigrationID.String(),
	}
	var applied []migrationmodels.Migration
	if err := db.WithContext(ctx).
		Where("version IN ?", wanted).
		Find(&applied).Error; err != nil {
		return fmt.Errorf("supplier migration readiness failed: %w", err)
	}
	found := make(map[string]struct{}, len(applied))
	for _, version := range applied {
		found[version.Version] = struct{}{}
	}
	missing := make([]string, 0, len(wanted))
	for _, version := range wanted {
		if _, ok := found[version]; !ok {
			missing = append(missing, version)
		}
	}
	if len(missing) != 0 {
		return fmt.Errorf(
			"supplier migration readiness failed: unapplied versions %s",
			strings.Join(missing, ", "),
		)
	}
	readyDB := db.WithContext(ctx)
	if !readyDB.Migrator().HasTable(new(supplier.Supplier)) {
		return errors.New("supplier migration readiness failed: supplier table is unavailable")
	}
	if !readyDB.Migrator().HasTable(new(models.CasbinRule)) {
		return errors.New("supplier authorization readiness failed: Admin policy table is unavailable")
	}
	return nil
}

type supplierDomainEventLogger struct{}

func (supplierDomainEventLogger) Collect(ctx context.Context, event supplier.DomainEvent) {
	if ctx == nil {
		ctx = context.Background()
	}
	if event == nil {
		slog.WarnContext(ctx, "supplier domain event committed without a typed event")
		return
	}
	slog.InfoContext(ctx, "supplier domain event committed", "event", event.EventName())
}
