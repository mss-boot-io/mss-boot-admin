package apis

import (
	"fmt"
	"net/http"
	"sort"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/virtual/action"
	vapi "github.com/mss-boot-io/mss-boot/virtual/api"

	"github.com/mss-boot-io/mss-boot-admin/dto"
	"github.com/mss-boot-io/mss-boot-admin/models"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/1/2 16:45:08
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/1/2 16:45:08
 */

func init() {
	base := action.GetBase()
	base.TenantIDFunc = models.TenantIDScope
	//center.Default.GetTenant().GetTenant()
	e := &Virtual{
		Virtual: vapi.NewVirtual(
			base,
			//controller.WithAuth(true),
		),
	}
	response.AppendController(e)
}

type Virtual struct {
	*vapi.Virtual
}

func (e *Virtual) Other(r *gin.RouterGroup) {
	r.GET(fmt.Sprintf("/documentation/:%s", e.GetKey()), e.Documentation)
}

// Documentation 文档
// @Summary 文档
// @Description 文档
// @Tags virtual
// @Accept application/json
// @Produce application/json
// @Param key path string true "key"
// @Success 200 {object} dto.VirtualModelObject
// @Router /admin/api/documentation/{key} [get]
func (e *Virtual) Documentation(ctx *gin.Context) {
	api := response.Make(ctx)
	vm := &models.Model{}
	err := gormdb.DB.Model(vm).
		Preload("Fields").
		Where("path = ?", ctx.Param(e.GetKey())).
		First(vm).Error
	if err != nil {
		api.AddError(err).Log.Error("get model error", "key", ctx.Param(e.GetKey()))
		api.Err(http.StatusInternalServerError)
		return
	}
	object := &dto.VirtualModelObject{
		Name:    vm.Name,
		Columns: make([]*dto.ColumnType, len(vm.Fields)),
	}
	fields := models.Fields(vm.Fields)
	sort.Sort(fields)
	for i := range vm.Fields {
		object.Columns[i] = &dto.ColumnType{
			Title:     vm.Fields[i].Label,
			DataIndex: vm.Fields[i].Name,
			PK:        vm.Fields[i].PrimaryKey != "",
		}
		if vm.Fields[i].FieldFrontend != nil {
			object.Columns[i].ValueType = vm.Fields[i].FieldFrontend.FormComponent
			object.Columns[i].HideInTable = vm.Fields[i].FieldFrontend.HideInTable
			object.Columns[i].HideInDescriptions = vm.Fields[i].FieldFrontend.HideInDescriptions
			object.Columns[i].HideInForm = vm.Fields[i].FieldFrontend.HideInForm
			object.Columns[i].ValidateRules = vm.Fields[i].FieldFrontend.Rules
		}
		if vm.Fields[i].ValueEnumName != "" {
			option := &models.Option{}
			err = gormdb.DB.Model(option).
				Where("id = ?", vm.Fields[i].ValueEnumName).
				First(option).Error
			if err != nil {
				api.AddError(err).Log.Error("get option error", "name", vm.Fields[i].ValueEnumName)
				api.Err(http.StatusInternalServerError)
				return
			}
			object.Columns[i].ValueEnum = make(map[string]dto.ValueEnumType)
			if option.Items != nil {
				for j := range *option.Items {
					object.Columns[i].ValueEnum[(*option.Items)[j].Value] = dto.ValueEnumType{
						Text:   (*option.Items)[j].Label,
						Status: (*option.Items)[j].Value,
						Color:  (*option.Items)[j].Color,
					}
				}
			}
		}
	}

	api.OK(object)
}
