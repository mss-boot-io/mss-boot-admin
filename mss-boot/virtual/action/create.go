package action

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-openapi/spec"

	"github.com/mss-boot-io/mss-boot/pkg"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/virtual/model"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/9/17 08:06:51
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/9/17 08:06:51
 */

// Create action
type Create struct {
	*Base
}

// NewCreate new create action 真实的
func NewCreate(b *Base) *Create {
	return &Create{
		Base: b,
	}
}

// String print action name
func (*Create) String() string {
	return "create"
}

// Handler create action handler
func (e *Create) Handler() gin.HandlersChain {
	h := func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodPost:
			api := response.Make(c)
			// create
			m := e.GetModel(c)
			if m == nil {
				// no set model
				api.Err(http.StatusNotFound)
				return
			}
			if m.Auth {
				response.AuthHandler(c)
			}
			req := m.MakeModel()
			m.Default(req)
			if api.Bind(req).Error != nil {
				api.Err(http.StatusUnprocessableEntity)
				return
			}
			if m.MultiTenant && e.TenantIDFunc != nil {
				tenantID, err := e.TenantIDFunc(c)
				if err != nil {
					api.AddError(err).Log.ErrorContext(c, "get tenant id error", PathKey, c.Param(PathKey))
					api.Err(http.StatusInternalServerError)
					return
				}
				pkg.SetValue(req, "TenantID", tenantID)
			}
			if err := gormdb.DB.Scopes(m.TableScope).Create(req).Error; err != nil {
				api.AddError(err).Log.ErrorContext(c, "create error", PathKey, c.Param(PathKey))
				api.Err(http.StatusInternalServerError)
				return
			}
			api.OK(nil)
			return
		default:
			c.AbortWithStatus(http.StatusMethodNotAllowed)
		}
	}
	return gin.HandlersChain{h}
}

// GenOpenAPI gen open api method: post, Parameters: nil
func (e *Create) GenOpenAPI(m *model.Model) *spec.PathItemProps {
	return &spec.PathItemProps{
		Post: &spec.Operation{
			OperationProps: spec.OperationProps{
				Tags:        []string{m.Name},
				Summary:     "create " + m.Name,
				Description: "create " + m.Name,
				Consumes:    []string{"application/json"},
				Produces:    []string{"application/json"},
				Parameters: []spec.Parameter{
					{
						ParamProps: spec.ParamProps{
							Name:        "data",
							Description: m.Name + " create input body",
							Required:    true,
							In:          "body",
							Schema: &spec.Schema{
								SchemaProps: spec.SchemaProps{
									Ref: spec.MustCreateRef("#/definitions/" + m.Name),
								},
							},
						},
					},
				},
				Security: []map[string][]string{
					{
						"Authorization": {},
					},
				},
				Responses: &spec.Responses{
					ResponsesProps: spec.ResponsesProps{
						StatusCodeResponses: map[int]spec.Response{
							http.StatusCreated: {
								ResponseProps: spec.ResponseProps{
									Description: "OK",
								},
							},
						},
					},
				},
			},
		},
	}
}
