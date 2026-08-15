package apis

import (
	"errors"
	"net/http"
	"strings"

	"github.com/mss-boot-io/mss-boot-admin/admin/center"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin/admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/admin/middleware"
	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/actions"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/response/controller"
	"gorm.io/gorm"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/12/18 23:55:10
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/12/18 23:55:10
 */

func init() {
	response.AppendController(newNoticeController())
}

func newNoticeController() *Notice {
	return &Notice{
		Simple: controller.NewSimple(
			controller.WithAuth(true),
			controller.WithModel(new(models.Notice)),
			controller.WithSearch(new(dto.NoticeSearch)),
			controller.WithModelProvider(actions.ModelProviderGorm),
			controller.WithHandlers(gin.HandlersChain{protectOperationalResponse}),
			controller.WithScope(noticeOwnerScope),
			controller.WithBeforeCreate(prepareNoticeCreate),
			controller.WithBeforeUpdate(prepareNoticeUpdate),
			controller.WithWriteErrorMapper(operationalWriteErrorMapper("NOTICE", "notice")),
		),
	}
}

type Notice struct {
	*controller.Simple
}

// unreadNoticeLimit bounds the global-header payload while leaving the full
// paginated notice management endpoint available for older notices.
const unreadNoticeLimit = 100

//func (e *Notice) GetAction(key string) response.Action {
//	return nil
//}

func (e *Notice) Other(r *gin.RouterGroup) {
	r.GET("/notice/unread", middleware.OptionalAuth(), protectOperationalResponse, e.Unread)
	r.PUT("/notice/read/:id", response.AuthHandler, protectOperationalResponse, e.MarkRead)
	r.GET("/notice/read/:id", response.AuthHandler, protectOperationalResponse, e.Read)
}

// Read 获取通知
// @Summary 获取通知
// @Description 获取通知
// @Tags notice
// @Accept  application/json
// @Product application/json
// @Param id path string true "id"
// @Success 200 {object} models.Notice
// @Router /admin/api/notice/read/{id} [get]
// @Security Bearer
func (e *Notice) Read(ctx *gin.Context) {
	api := response.Make(ctx)
	userID, ok := currentOperationalUserID(ctx)
	if !ok {
		api.Err(http.StatusUnauthorized)
		return
	}
	id := strings.TrimSpace(ctx.Param("id"))
	if id == "" || len(id) > 64 || strings.ContainsAny(id, ",\x00") {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	var notice models.Notice
	err := center.Default.GetDB(ctx, &models.Notice{}).Model(&models.Notice{}).
		Where("id = ?", id).
		Where("user_id = ?", userID).
		First(&notice).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			api.Err(http.StatusNotFound)
			return
		}
		api.AddError(err).Log.Error("get notice error")
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(notice)
}

// MarkRead 标记已读
// @Summary 标记已读
// @Description 标记已读
// @Tags notice
// @Accept  application/json
// @Product application/json
// @Param id path string true "id"
// @Success 200
// @Router /admin/api/notice/read/{id} [put]
// @Security Bearer
func (e *Notice) MarkRead(ctx *gin.Context) {
	api := response.Make(ctx)
	userID, ok := currentOperationalUserID(ctx)
	if !ok {
		api.Err(http.StatusUnauthorized)
		return
	}
	id := strings.TrimSpace(ctx.Param("id"))
	if id == "" || len(id) > 64 || strings.ContainsAny(id, ",\x00") {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	query := center.Default.GetDB(ctx, &models.Notice{}).Model(&models.Notice{}).
		Where("user_id = ?", userID)
	switch id {
	case "all":
	case models.NoticeTypeMessage.String(),
		models.NoticeTypeEvent.String(),
		models.NoticeTypeNotification.String(),
		models.NoticeTypeMail.String():
		query = query.Where("type = ?", id)
	default:
		query = query.Where("id = ?", id)
	}
	result := query.Update("`read`", true)
	if result.Error != nil {
		api.AddError(result.Error).Log.Error("update notice error")
		api.Err(http.StatusInternalServerError)
		return
	}
	if id != "all" && id != models.NoticeTypeMessage.String() &&
		id != models.NoticeTypeEvent.String() && id != models.NoticeTypeNotification.String() &&
		id != models.NoticeTypeMail.String() && result.RowsAffected == 0 {
		api.Err(http.StatusNotFound)
		return
	}
	api.OK(struct{}{})
}

// Unread 获取未读通知列表
// @Summary 获取未读通知列表
// @Description 获取未读通知列表
// @Tags notice
// @Accept  application/json
// @Product application/json
// @Success 200 {object} []models.Notice
// @Router /admin/api/notice/unread [get]
// @Security Bearer
func (e *Notice) Unread(ctx *gin.Context) {
	api := response.Make(ctx)
	verify := response.VerifyHandler(ctx)
	if verify == nil {
		api.OK(nil)
		return
	}
	list, err := listUnreadNotices(center.Default.GetDB(ctx, &models.Notice{}), verify.GetUserID())
	if err != nil {
		api.AddError(err).Log.Error("get notice list error")
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(list)
}

func listUnreadNotices(db *gorm.DB, userID string) ([]*models.Notice, error) {
	list := make([]*models.Notice, 0)
	err := db.Model(&models.Notice{}).
		Where(map[string]any{"user_id": userID, "read": false}).
		Order("created_at DESC").
		Order("id DESC").
		Limit(unreadNoticeLimit).
		Find(&list).Error
	return list, err
}

// Get 获取通知
// @Summary 获取通知
// @Description 获取通知
// @Tags notice
// @Accept  application/json
// @Product application/json
// @Param id path string true "id"
// @Success 200 {object} models.Notice
// @Router /admin/api/notices/{id} [get]
// @Security Bearer
func (e *Notice) Get(*gin.Context) {}

// Create 创建通知
// @Summary 创建通知
// @Description 创建通知
// @Tags notice
// @Accept  application/json
// @Product application/json
// @Param data body models.Notice true "data"
// @Success 201
// @Router /admin/api/notices [post]
// @Security Bearer
func (e *Notice) Create(*gin.Context) {}

// Update 更新通知
// @Summary 更新通知
// @Description 更新通知
// @Tags notice
// @Accept application/json
// @Product application/json
// @Param id path string true "id"
// @Param data body models.Notice true "data"
// @Success 200
// @Router /admin/api/notices/{id} [put]
// @Security Bearer
func (e *Notice) Update(*gin.Context) {}

// Delete 删除通知
// @Summary 删除通知
// @Description 删除通知
// @Tags notice
// @Param id path string true "id"
// @Success 204
// @Router /admin/api/notices/{id} [delete]
// @Security Bearer
func (e *Notice) Delete(*gin.Context) {}

// List 通知列表数据
// @Summary 通知列表数据
// @Description 通知列表数据
// @Tags notice
// @Accept  application/json
// @Product application/json
// @Param title query string false "title"
// @Param status query string false "status"
// @Param userID query string false "userID"
// @Param current query int false "current"
// @Param pageSize query int false "pageSize"
// @Success 200 {object} response.Page{data=[]models.Notice}
// @Router /admin/api/notices [get]
// @Security Bearer
func (e *Notice) List(*gin.Context) {}
