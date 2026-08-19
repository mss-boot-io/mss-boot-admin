package server

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/business"
	"github.com/mss-boot-io/mss-boot-admin/admin/center"
	"gorm.io/gorm"
)

// leaseBusinessRequestDatabase keeps one authoritative database lease alive
// until every protected business handler has returned.
func leaseBusinessRequestDatabase(useDatabase databaseAccess) gin.HandlerFunc {
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

type businessEventLogger struct{}

func (businessEventLogger) Collect(ctx context.Context, event business.Event) {
	if ctx == nil {
		ctx = context.Background()
	}
	if event == nil {
		slog.WarnContext(ctx, "business domain event committed without a typed event")
		return
	}
	slog.InfoContext(ctx, "business domain event committed", "event", event.EventName())
}
