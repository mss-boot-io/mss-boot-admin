package mgm

/*
 * @Author: lwnmengjing
 * @Date: 2023/1/25 17:14:19
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2023/1/25 17:14:19
 */

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kamva/mgm/v3"
	"github.com/kamva/mgm/v3/builder"
	"github.com/kamva/mgm/v3/field"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/mss-boot-io/mss-boot/pkg"
	"github.com/mss-boot-io/mss-boot/pkg/response"
	"github.com/mss-boot-io/mss-boot/pkg/search/mgos"
)

// Search action
type Search struct {
	Base
	TreeField string
	Depth     int
	Search    response.Searcher
}

// NewSearch new search action
func NewSearch(b Base, search response.Searcher) *Search {
	return &Search{
		Base:   b,
		Search: search,
	}
}

// String action name
func (*Search) String() string {
	return "search"
}

func (e *Search) Handler() gin.HandlersChain {
	h := func(c *gin.Context) {
		if e.Model == nil {
			response.Make(c).Err(http.StatusNotImplemented, "not implemented")
			return
		}
		e.searchMgm(c)
	}
	chain := gin.HandlersChain{h}
	return chain
}

func (e *Search) searchMgm(c *gin.Context) {
	req := pkg.DeepCopy(e.Search).(response.Searcher)
	api := response.Make(c).Bind(req)
	if api.Error != nil {
		api.Err(http.StatusUnprocessableEntity)
		return
	}
	filter, sort := mgos.MakeCondition(req)

	count, err := mgm.Coll(e.Model).CountDocuments(c, filter)
	if err != nil {
		api.AddError(err).Log.ErrorContext(c, "count items error", "error", err)
		api.Err(http.StatusInternalServerError)
		return
	}
	linkConfigs := getLinkTag(e.Model)
	if len(linkConfigs) == 0 {
		ops := options.Find()
		ops.SetLimit(req.GetPageSize())
		if len(sort) > 0 {
			ops.SetSort(sort)
		}
		ops.SetSkip(req.GetPageSize() * (req.GetPage() - 1))

		result, err := mgm.Coll(e.Model).Find(c, filter, ops)
		if err != nil {
			api.AddError(err).Log.ErrorContext(c, "find items error", "error", err)
			api.Err(http.StatusInternalServerError)
			return
		}
		defer result.Close(c)
		items := make([]any, 0, req.GetPageSize())
		for result.Next(c) {
			// var data any
			m := pkg.ModelDeepCopy(e.Model)
			err = result.Decode(m)
			if err != nil {
				api.AddError(err).Log.ErrorContext(c, "decode items error", "error", err)
				api.Err(http.StatusInternalServerError)
				return
			}
			items = append(items, m)
		}
		api.PageOK(items, count, req.GetPage(), req.GetPageSize())
		return
	}
	// use Aggregate
	//https://docs.mongodb.com/manual/reference/operator/aggregation/lookup/
	pipeline := bson.A{
		// builder.S(builder.Lookup(authorColl.Name(), "author_id", field.ID, "author")),
	}
	for i := range linkConfigs {
		pipeline = append(pipeline,
			builder.S(
				builder.Lookup(
					linkConfigs[i].CollectionName,
					linkConfigs[i].LocalField,
					field.ID, linkConfigs[i].ForeignField)))
	}
	// limit skip sort
	pipeline = append(pipeline, bson.D{
		{Key: "$limit", Value: req.GetPageSize()},
	}, bson.D{
		{Key: "$skip", Value: req.GetPageSize() * (req.GetPage() - 1)},
	})
	if sort != nil {
		pipeline = append(pipeline, bson.D{
			{Key: "$sort", Value: sort},
		})
	}
	result, err := mgm.Coll(e.Model).Aggregate(c, pipeline)
	if err != nil {
		api.AddError(err).Log.ErrorContext(c, "find items error", "error", err)
		api.Err(http.StatusInternalServerError)
		return
	}
	defer result.Close(c)
	items := make([]any, 0, req.GetPageSize())
	for result.Next(c) {
		m := pkg.ModelDeepCopy(e.Model)
		var bm bson.M
		err = result.Decode(bm)
		if err != nil {
			api.AddError(err)
			api.Err(http.StatusInternalServerError)
			return
		}
		if bm == nil {
			continue
		}
		// todo bson.M to model
		err = BsonMTransferModel(bm, m)
		if err != nil {
			api.AddError(err).Log.ErrorContext(c, "transfer bson.M to model error", "error", err)
			return
		}
		items = append(items, m)
	}
	api.PageOK(items, count, req.GetPage(), req.GetPageSize())
}

// LinkConfig link config
type LinkConfig struct {
	// FieldName field name
	FieldName string
	// CollectionName collection name
	CollectionName string
	// LocalField local field
	LocalField string
	// ForeignField foreign field
	ForeignField string
}

// BsonMTransferModel bson.M to model
func BsonMTransferModel(bm bson.M, model any) error {
	typeOf := reflect.TypeOf(model).Elem()
	valueOf := reflect.ValueOf(model).Elem()
	for i := 0; i < typeOf.NumField(); i++ {
		f := typeOf.Field(i)
		tagBson := f.Tag.Get("bson")
		if tagBson == "-" {
			continue
		}
		if tagBson == "" {
			tagBson = f.Name
		}
		if f.Type.Kind() == reflect.Struct {
			if f.Type.Name() == "mgm.DefaultModel" {
				dm := valueOf.Field(i).Interface().(mgm.DefaultModel)
				dm.SetID(bm["_id"].(primitive.ObjectID))
				dm.UpdatedAt = bm["updated_at"].(time.Time)
				dm.CreatedAt = bm["created_at"].(time.Time)
				continue
			}
			if strings.Contains(tagBson, "inline") {
				err := BsonMTransferModel(bm, valueOf.Field(i).Interface())
				if err != nil {
					return err
				}
				continue
			}

			continue
		}
		if f.Type.Kind() == reflect.Array {
			// transfer bson.M to array model
			switch ms := bm[tagBson].(type) {
			case []bson.M:
				bsonBytes, _ := bson.Marshal(ms)
				err := bson.Unmarshal(bsonBytes, valueOf.Field(i).Interface())
				if err != nil {
					return err
				}
			default:
				return fmt.Errorf("type %s not is array", reflect.TypeOf(ms).String())
			}
			continue
		}
		valueOf.Field(i).Set(reflect.ValueOf(bm[tagBson]))
	}
	return nil
}

// getLinkTag get link tag from object
func getLinkTag(model any) []LinkConfig {
	configs := make([]LinkConfig, 0)
	typeOf := reflect.TypeOf(model).Elem()
	valueOf := reflect.ValueOf(model).Elem()
	for i := 0; i < typeOf.NumField(); i++ {
		f := typeOf.Field(i)
		if f.Type.Kind() == reflect.Struct {
			if f.Type.String() == "mgm.DefaultModel" {
				continue
			}
			vm, ok := valueOf.Field(i).Addr().Interface().(mgm.Model)
			if !ok {
				continue
			}
			m := pkg.ModelDeepCopy(vm)
			configs = append(configs, LinkConfig{
				FieldName:      f.Tag.Get("bson"),
				CollectionName: mgm.CollName(m),
				LocalField:     getLinkLocalField(typeOf, f.Name),
				ForeignField:   field.ID,
			})
		}
	}
	return configs
}

func getLinkLocalField(typeOf reflect.Type, relationName string) string {
	for _, name := range []string{relationName + "ID", relationName + field.ID} {
		for i := 0; i < typeOf.NumField(); i++ {
			f := typeOf.Field(i)
			if f.Name != name {
				continue
			}
			tag := f.Tag.Get("bson")
			if tag == "" || tag == "-" {
				return name
			}
			return strings.Split(tag, ",")[0]
		}
	}
	return relationName + field.ID
}
