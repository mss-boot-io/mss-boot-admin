package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/mss-boot-io/mss-boot-admin/admin/center"
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
	useDatabase databaseAccess,
	group *gin.RouterGroup,
) error {
	if middleware.Auth == nil {
		return errors.New("supplier authentication middleware is not initialized")
	}
	return mountSupplierRoutesWithDatabaseDependencies(ctx, useDatabase, group, supplierRouteDependencies{
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
	if db == nil {
		return errors.New("supplier route composition is not initialized")
	}
	return mountSupplierRoutesWithDatabaseDependencies(
		ctx,
		func(operation func(*gorm.DB) error) error { return operation(db) },
		group,
		dependencies,
	)
}

func mountSupplierRoutesWithDatabaseDependencies(
	ctx context.Context,
	useDatabase databaseAccess,
	group *gin.RouterGroup,
	dependencies supplierRouteDependencies,
) error {
	if ctx == nil {
		return errors.New("supplier route composition context is required")
	}
	if useDatabase == nil || group == nil {
		return errors.New("supplier route composition is not initialized")
	}
	if dependencies.authentication == nil {
		return errors.New("supplier authentication middleware is required")
	}
	if err := useDatabase(func(db *gorm.DB) error {
		if err := verifySupplierRuntimeReadiness(ctx, db); err != nil {
			return err
		}
		if _, err := supplier.NewService(db, dependencies.events); err != nil {
			return fmt.Errorf("compose supplier application: %w", err)
		}
		if _, err := supplier.NewAdminAuthorizer(db, dependencies.principal); err != nil {
			return fmt.Errorf("compose supplier authorization: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}
	application := &requestSupplierApplication{events: dependencies.events}
	authorizer := &requestSupplierAuthorizer{principal: dependencies.principal}
	api := group.Group(
		"/api",
		leaseSupplierRequestDatabase(useDatabase),
		dependencies.authentication,
	)
	if err := supplier.RegisterRoutes(api, application, authorizer); err != nil {
		return fmt.Errorf("register supplier routes: %w", err)
	}
	return nil
}

func leaseSupplierRequestDatabase(useDatabase databaseAccess) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		err := useDatabase(func(db *gorm.DB) error {
			if err := center.BindRequestDatabase(ctx, db); err != nil {
				return err
			}
			ctx.Next()
			return nil
		})
		if err == nil {
			return
		}
		ctx.Abort()
		if !ctx.Writer.Written() {
			ctx.JSON(http.StatusServiceUnavailable, gin.H{"error": "database unavailable"})
		}
	}
}

type requestSupplierApplication struct {
	events supplier.EventCollector
}

func (application *requestSupplierApplication) service(
	ctx context.Context,
) (*supplier.Service, error) {
	db, ok := center.RequestDatabase(ctx)
	if !ok {
		return nil, supplier.ErrSupplierPersistence
	}
	service, err := supplier.NewService(db, application.events)
	if err != nil {
		return nil, fmt.Errorf("%w: compose request service", supplier.ErrSupplierPersistence)
	}
	return service, nil
}

func (application *requestSupplierApplication) List(
	ctx context.Context,
	query supplier.ListSupplierQuery,
) (supplier.SupplierPage, error) {
	service, err := application.service(ctx)
	if err != nil {
		return supplier.SupplierPage{}, err
	}
	return service.List(ctx, query)
}

func (application *requestSupplierApplication) Get(
	ctx context.Context,
	id string,
) (*supplier.Supplier, error) {
	service, err := application.service(ctx)
	if err != nil {
		return nil, err
	}
	return service.Get(ctx, id)
}

func (application *requestSupplierApplication) Create(
	ctx context.Context,
	input *supplier.CreateSupplierInput,
) (*supplier.Supplier, error) {
	service, err := application.service(ctx)
	if err != nil {
		return nil, err
	}
	return service.Create(ctx, input)
}

func (application *requestSupplierApplication) Update(
	ctx context.Context,
	id string,
	input *supplier.UpdateSupplierInput,
) (*supplier.Supplier, error) {
	service, err := application.service(ctx)
	if err != nil {
		return nil, err
	}
	return service.Update(ctx, id, input)
}

func (application *requestSupplierApplication) Delete(ctx context.Context, id string) error {
	service, err := application.service(ctx)
	if err != nil {
		return err
	}
	return service.Delete(ctx, id)
}

func (application *requestSupplierApplication) Export(
	ctx context.Context,
	query supplier.ListSupplierQuery,
	destination io.Writer,
) error {
	service, err := application.service(ctx)
	if err != nil {
		return err
	}
	return service.Export(ctx, query, destination)
}

type requestSupplierAuthorizer struct {
	principal supplier.AdminPrincipalResolver
}

func (authorizer *requestSupplierAuthorizer) Authorize(
	ctx *gin.Context,
	permission string,
) error {
	if ctx == nil || ctx.Request == nil {
		return supplier.ErrSupplierAuthorizationDenied
	}
	db, ok := center.RequestDatabase(ctx.Request.Context())
	if !ok {
		return supplier.ErrSupplierAuthorizationUnavailable
	}
	requestAuthorizer, err := supplier.NewAdminAuthorizer(db, authorizer.principal)
	if err != nil {
		return fmt.Errorf("%w: compose request authorizer", supplier.ErrSupplierAuthorizationUnavailable)
	}
	return requestAuthorizer.Authorize(ctx, permission)
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
