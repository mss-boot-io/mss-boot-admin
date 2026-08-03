package mgos

/*
 * @Author: lwnmengjing
 * @Date: 2022/3/11 16:43
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2022/3/11 16:43
 */

import (
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
)

// MakeCondition builds a typed MongoDB condition from static struct tags and bound values.
func MakeCondition(q any) (bson.M, bson.D) {
	condition := &Public{}
	ResolveSearchQuery(q, condition)
	var filter bson.M
	var andFilter bson.M
	var orFilter bson.M
	if len(condition.And) > 0 {
		if len(condition.And) > 1 {
			andFilter = bson.M{"$and": condition.And}
		} else {
			andFilter = condition.And[0]
		}
	}
	if len(condition.Or) > 0 {
		if len(condition.Or) > 1 {
			orFilter = bson.M{"$or": condition.Or}
		} else {
			orFilter = condition.Or[0]
		}
	}
	if len(condition.And) > 0 && len(condition.Or) > 0 {
		filter = bson.M{"$and": []bson.M{andFilter, orFilter}}
	} else if len(condition.And) > 0 {
		filter = andFilter
	} else if len(condition.Or) > 0 {
		filter = orFilter
	}
	return filter, condition.Order
}

// CompileCondition validates the complete filter and sort as BSON and returns
// detached typed documents suitable for the MongoDB driver. Query keys come
// exclusively from validated static struct tags; request values are represented
// as BSON values rather than query source text.
func CompileCondition(q any) (bson.M, bson.D, error) {
	filter, order := MakeCondition(q)

	compiledFilter := bson.M{}
	if len(filter) > 0 {
		data, err := bson.Marshal(filter)
		if err != nil {
			return nil, nil, fmt.Errorf("marshal Mongo search filter: %w", err)
		}
		if err := bson.Unmarshal(data, &compiledFilter); err != nil {
			return nil, nil, fmt.Errorf("unmarshal Mongo search filter: %w", err)
		}
	}

	compiledOrder := bson.D{}
	if len(order) > 0 {
		data, err := bson.Marshal(order)
		if err != nil {
			return nil, nil, fmt.Errorf("marshal Mongo search sort: %w", err)
		}
		if err := bson.Unmarshal(data, &compiledOrder); err != nil {
			return nil, nil, fmt.Errorf("unmarshal Mongo search sort: %w", err)
		}
	}
	return compiledFilter, compiledOrder, nil
}
