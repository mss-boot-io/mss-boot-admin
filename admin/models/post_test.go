package models

import (
	"reflect"
	"sort"
	"testing"

	"github.com/mss-boot-io/mss-boot-admin/admin/pkg"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPostHooksAndTreeHelpers(t *testing.T) {
	post := &Post{
		ParentID:   "parent",
		DeptIDSArr: []string{"dept-a", "dept-b"},
	}
	post.ID = "post-1"
	if err := post.BeforeSave(nil); err != nil {
		t.Fatalf("BeforeSave: %v", err)
	}
	if post.DeptIDS != "dept-a,dept-b" {
		t.Fatalf("DeptIDS = %q", post.DeptIDS)
	}
	post.DeptIDSArr = nil
	if err := post.AfterFind(nil); err != nil {
		t.Fatalf("AfterFind: %v", err)
	}
	if !reflect.DeepEqual(post.DeptIDSArr, []string{"dept-a", "dept-b"}) {
		t.Fatalf("DeptIDSArr = %#v", post.DeptIDSArr)
	}
	if post.TableName() != "mss_boot_posts" || post.GetIndex() != "post-1" || post.GetParentID() != "parent" {
		t.Fatalf("unexpected post identity: %#v", post)
	}
}

func TestPostSortAndAddChildren(t *testing.T) {
	root := &Post{}
	low := &Post{Sort: 1}
	low.ID = "low"
	high := &Post{Sort: 10, Children: []*Post{{Sort: 2}, {Sort: 8}}}
	high.ID = "high"
	root.AddChildren([]pkg.TreeImp{low, high, nil})
	root.SortChildren()

	if len(root.Children) != 2 || root.Children[0].ID != "high" || root.Children[1].ID != "low" {
		t.Fatalf("sorted children = %#v", root.Children)
	}
	if root.Children[0].Children[0].Sort != 8 || root.Children[0].Children[1].Sort != 2 {
		t.Fatalf("nested sort order = %#v", root.Children[0].Children)
	}

	list := PostList{low, high}
	if list.Len() != 2 || !list.Less(1, 0) {
		t.Fatalf("post list comparison unexpected: %#v", list)
	}
	list.Swap(0, 1)
	if list[0] != high {
		t.Fatal("post list swap failed")
	}
}

func TestPostDescendantQueriesReturnDirectAndRecursiveChildren(t *testing.T) {
	database, err := gorm.Open(sqlite.Open("file:post-descendants?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open SQLite: %v", err)
	}
	if err := database.AutoMigrate(&Post{}); err != nil {
		t.Fatalf("migrate posts: %v", err)
	}

	posts := []*Post{
		newPost("root", "", 1),
		newPost("child-a", "root", 2),
		newPost("child-b", "root", 3),
		newPost("grandchild", "child-a", 4),
	}
	for _, post := range posts {
		if err := database.Omit("Children").Create(post).Error; err != nil {
			t.Fatalf("create post %s: %v", post.ID, err)
		}
	}

	root := newPost("root", "", 1)
	direct := root.GetChildrenID(database)
	sort.Strings(direct)
	if !reflect.DeepEqual(direct, []string{"child-a", "child-b"}) {
		t.Fatalf("direct children = %#v", direct)
	}

	root.Children = nil
	all := root.GetAllChildrenID(database)
	sort.Strings(all)
	if !reflect.DeepEqual(all, []string{"child-a", "child-b", "grandchild"}) {
		t.Fatalf("all descendants = %#v", all)
	}

	preloaded := newPost("preloaded", "", 1)
	preloaded.Children = []*Post{newPost("known", "preloaded", 1)}
	if got := preloaded.GetAllChildrenID(nil); !reflect.DeepEqual(got, []string{"known"}) {
		t.Fatalf("preloaded descendants = %#v", got)
	}
}

func newPost(id, parentID string, order int) *Post {
	post := &Post{
		ParentID: parentID,
		Name:     id,
		Code:     id,
		Sort:     order,
	}
	post.ID = id
	return post
}
