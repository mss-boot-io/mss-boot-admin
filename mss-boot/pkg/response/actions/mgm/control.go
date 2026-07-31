package mgm

/*
 * @Author: lwnmengjing
 * @Date: 2023/1/25 17:13:38
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2023/1/25 17:13:38
 */

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kamva/mgm/v3"
	"github.com/mss-boot-io/mss-boot/pkg"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// Control action
type Control struct {
	Base
	Key string
}

// NewControl new control action
func NewControl(b Base, key string) *Control {
	return &Control{
		Base: b,
		Key:  key,
	}
}

func (e *Control) Handler() gin.HandlersChain {
	h := func(c *gin.Context) {
		if e.Model == nil {
			response.Make(c).Err(http.StatusNotImplemented, "not implemented")
			return
		}
		switch c.Request.Method {
		case http.MethodPost:
			e.create(c)
		case http.MethodPut:
			e.update(c)
		default:
			response.Make(c).Err(http.StatusNotImplemented, "not implemented")
		}
	}
	chain := gin.HandlersChain{h}
	return chain
}

// String action name
func (*Control) String() string {
	return "control"
}

func (e *Control) create(c *gin.Context) {
	m := pkg.ModelDeepCopy(e.Model)
	api := response.Make(c).Bind(m)
	if api.Error != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	err := mgm.Coll(e.Model).CreateWithCtx(c, m)
	if err != nil {
		api.AddError(err).Log.ErrorContext(c, "Create error", "error", err)
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(nil)
}

func (e *Control) update(c *gin.Context) {
	m := pkg.ModelDeepCopy(e.Model)
	api := response.Make(c).Bind(m)
	if api.Error != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	id, err := primitive.ObjectIDFromHex(c.Param(e.Key))
	if err != nil {
		api.AddError(err)
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	m.SetID(id)
	err = mgm.Coll(e.Model).UpdateWithCtx(c, m)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			api.Err(http.StatusNotFound)
			return
		}
		api.AddError(err).Log.ErrorContext(c, "Update error", "error", err)
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(m)
}
