package mgm

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/2/15 03:54:31
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/2/15 03:54:31
 */

import (
	"testing"

	"github.com/kamva/mgm/v3"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type testModel struct {
	mgm.DefaultModel `bson:",inline"`
	Name             string             `json:"name" bson:"name"`
	AuthorID         primitive.ObjectID `json:"authorID" bson:"author_id"`
	Author           author             `json:"author" bson:"author"`
}

type author struct {
	mgm.DefaultModel `bson:",inline"`
	Name             string `json:"name" bson:"name"`
}

func Test_getLinkTag(t *testing.T) {
	type args struct {
		model any
	}

	tests := []struct {
		name               string
		args               args
		wantCollectionName string
		wantLocalField     string
		wantForeignField   string
	}{
		{
			name: "test",
			args: args{
				model: &testModel{},
			},
			wantCollectionName: "authors",
			wantLocalField:     "author_id",
			wantForeignField:   "_id",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			list := getLinkTag(tt.args.model)
			if len(list) != 1 {
				t.Fatalf("getLinkTag() got %d configs, want 1", len(list))
			}
			if list[0].CollectionName != tt.wantCollectionName {
				t.Errorf("getLinkTag() gotCollectionName = %v, want %v", list[0].CollectionName, tt.wantCollectionName)
			}
			if list[0].LocalField != tt.wantLocalField {
				t.Errorf("getLinkTag() gotLocalField = %v, want %v", list[0].LocalField, tt.wantLocalField)
			}
			if list[0].ForeignField != tt.wantForeignField {
				t.Errorf("getLinkTag() gotForeignField = %v, want %v", list[0].ForeignField, tt.wantForeignField)
			}
		})
	}
}
