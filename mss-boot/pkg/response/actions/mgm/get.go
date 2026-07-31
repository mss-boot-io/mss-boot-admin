package mgm

/*
 * @Author: lwnmengjing
 * @Date: 2023/1/26 00:42:12
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2023/1/26 00:42:12
 */

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kamva/mgm/v3"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/mss-boot-io/mss-boot/pkg"
	"github.com/mss-boot-io/mss-boot/pkg/response"
)

// Get action
type Get struct {
	Base
	Key string
}

// NewGet new get action
func NewGet(b Base, key string) *Get {
	return &Get{
		Base: b,
		Key:  key,
	}
}

// String action name
func (*Get) String() string {
	return "get"
}

func (e *Get) Handler() gin.HandlersChain {
	h := func(c *gin.Context) {
		if e.Model == nil {
			response.Make(c).Err(http.StatusNotImplemented, "not implemented")
		}
		e.get(c, e.Key)
	}
	chain := gin.HandlersChain{h}
	return chain
}

func (e *Get) get(c *gin.Context, key string) {
	api := response.Make(c)
	m := pkg.ModelDeepCopy(e.Model)
	id, err := primitive.ObjectIDFromHex(c.Param(key))
	if err != nil {
		api.AddError(err)
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	err = mgm.Coll(e.Model).
		FindOne(c, bson.M{"_id": id}).
		Decode(m)
	if err != nil {
		api.AddError(err)
		if errors.Is(err, mongo.ErrNoDocuments) {
			api.Err(http.StatusNotFound)
			return
		}
		api.Err(http.StatusInternalServerError)
		return
	}
	api.OK(m)
}
