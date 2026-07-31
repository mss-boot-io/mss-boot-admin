package model

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/sanity-io/litter"
	"gorm.io/gorm"

	"gopkg.in/yaml.v3"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/9/10 15:32:51
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/9/10 15:32:51
 */

var id = strings.ReplaceAll(uuid.New().String(), "-", "")

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("DB() error = %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})
	return db
}

// PaginationTest pagination params
type PaginationTest struct {
	Page     int64 `form:"page" query:"page"`
	PageSize int64 `form:"pageSize" query:"pageSize"`
}

// GetPage get page
func (e *PaginationTest) GetPage() int64 {
	if e.Page <= 0 {
		return 1
	}
	return e.Page
}

// GetPageSize get page size
func (e *PaginationTest) GetPageSize() int64 {
	if e.PageSize <= 0 {
		return 10
	}
	return e.PageSize
}

func TestModel_TableName(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "test",
			path: "../../testdata/test.yml",
			want: "test",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rb, err := os.ReadFile(tt.path)
			if err != nil {
				t.Fatalf("ReadFile() error = %m", err)
			}
			m := &Model{}
			err = yaml.Unmarshal(rb, m)
			if err != nil {
				t.Fatalf("Unmarshal() error = %m", err)
			}
			if got := m.TableName(); got != tt.want {
				t.Errorf("TableName() = %s, want %v", got, tt.want)
			}
		})
	}
}

func TestModel_Migrate(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		dsn       string
		wantError bool
	}{
		{
			name:      "test0",
			path:      "../../testdata/test.yml",
			dsn:       "root:123456@tcp(127.0.0.1:3306)/test?charset=utf8mb4&parseTime=True&loc=Local",
			wantError: false,
		},
	}
	for _, tt := range tests {
		rb, err := os.ReadFile(tt.path)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		m := &Model{}
		err = yaml.Unmarshal(rb, m)
		if err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}
		db := openTestDB(t)
		if err = m.Migrate(db); (err != nil) != tt.wantError {
			t.Errorf("Migrate() error = %v, wantError %v", err, tt.wantError)
		}
	}
}

func TestModel_Create(t *testing.T) {
	tests := []struct {
		name string
		path string
		body string
		dsn  string
		want bool
	}{
		{
			name: "test0",
			path: "../../testdata/test.yml",
			body: fmt.Sprintf(`{"id": "%s","name": "test0"}`, id),
			dsn:  "root:123456@tcp(127.0.0.1:3306)/test?charset=utf8mb4&parseTime=True&loc=Local",
			want: false,
		},
		{
			name: "test1",
			path: "../../testdata/test.yml",
			body: `{"name": "test1"}`,
			dsn:  "root:123456@tcp(127.0.0.1:3306)/test?charset=utf8mb4&parseTime=True&loc=Local",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rb, err := os.ReadFile(tt.path)
			if err != nil {
				t.Fatalf("ReadFile() error = %v", err)
			}
			m := &Model{}
			err = yaml.Unmarshal(rb, m)
			if err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			m.Init()
			db := openTestDB(t)
			if err = m.Migrate(db); err != nil {
				t.Fatalf("Migrate() error = %v", err)
			}
			// 创建一个虚拟的 HTTP 请求和响应
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodPost, "/test", bytes.NewBuffer([]byte(tt.body)))

			// 创建一个 Gin 引擎并绑定路由
			r := gin.Default()
			r.POST("/test", func(ctx *gin.Context) {
				item := m.MakeModel()
				m.Default(item)
				if err = ctx.ShouldBindJSON(item); err != nil {
					ctx.Status(http.StatusInternalServerError)
					t.Fatalf("ShouldBindJSON() error = %v", err)
				}
				if err = db.Scopes(m.TableScope).Create(item).Error; err != nil {
					ctx.Status(http.StatusInternalServerError)
					t.Fatalf("Create() error = %v", err)
				}
				ctx.Status(http.StatusOK)
			})
			// 使用虚拟请求进行请求处理
			r.ServeHTTP(w, req)
			// 检查响应
			if w.Code != http.StatusOK {
				t.Errorf("Expected status code %d, but got %d", http.StatusOK, w.Code)
			}
		})
	}
}

func TestModel_Update(t *testing.T) {
	tests := []struct {
		name string
		path string
		body string
		dsn  string
		want bool
	}{
		{
			name: "test",
			path: "../../testdata/test.yml",
			body: `{"name":"testn"}`,
			dsn:  "root:123456@tcp(127.0.0.1:3306)/test?charset=utf8mb4&parseTime=True&loc=Local",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rb, err := os.ReadFile(tt.path)
			if err != nil {
				t.Fatalf("ReadFile() error = %v", err)
			}
			m := &Model{}
			err = yaml.Unmarshal(rb, m)
			if err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			db := openTestDB(t)
			if err = m.Migrate(db); err != nil {
				t.Fatalf("Migrate() error = %v", err)
			}
			if err = db.Exec("INSERT INTO test (id, name) VALUES (?, ?)", id, "test1").Error; err != nil {
				t.Fatalf("seed test model error = %v", err)
			}
			// 创建一个虚拟的 HTTP 请求和响应
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodPut, "/test/"+id, bytes.NewBuffer([]byte(tt.body)))

			// 创建一个 Gin 引擎并绑定路由
			r := gin.Default()
			r.PUT("/test/:id", func(ctx *gin.Context) {
				item := m.MakeModel()
				if err = ctx.ShouldBindJSON(item); err != nil {
					ctx.Status(http.StatusInternalServerError)
					t.Fatalf("ShouldBindJSON() error = %v", err)
				}
				if err = db.Scopes(m.TableScope, m.URI(ctx)).Updates(item).Error; err != nil {
					ctx.Status(http.StatusInternalServerError)
					t.Fatalf("Update() error = %v", err)
				}
				ctx.Status(http.StatusOK)
			})
			// 使用虚拟请求进行请求处理
			r.ServeHTTP(w, req)
			// 检查响应
			if w.Code != http.StatusOK {
				t.Errorf("Expected status code %d, but got %d", http.StatusOK, w.Code)
			}
		})
	}
}

func TestModel_Delete(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		dsn       string
		wantError bool
	}{
		{
			name:      "test",
			path:      "../../testdata/test.yml",
			dsn:       "root:123456@tcp(127.0.0.1:3306)/test?charset=utf8mb4&parseTime=True&loc=Local",
			wantError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rb, err := os.ReadFile(tt.path)
			if err != nil {
				t.Fatalf("ReadFile() error = %v", err)
			}
			m := &Model{}
			err = yaml.Unmarshal(rb, m)
			if err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			db := openTestDB(t)
			if err = m.Migrate(db); err != nil {
				t.Fatalf("Migrate() error = %v", err)
			}
			if err = db.Exec("INSERT INTO test (id, name) VALUES (?, ?)", id, "test1").Error; err != nil {
				t.Fatalf("seed test model error = %v", err)
			}
			// 创建一个虚拟的 HTTP 请求和响应
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodDelete, "/test/"+id, nil)
			r := gin.Default()
			r.DELETE("/test/:id", func(ctx *gin.Context) {
				if err = db.Scopes(m.URI(ctx)).Delete(m.MakeModel()).Error; err != nil {
					ctx.Status(http.StatusInternalServerError)
					t.Fatalf("Delete() error = %v", err)
				}
				ctx.Status(http.StatusOK)
			})
			r.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Errorf("Expected status code %d, but got %d", http.StatusOK, w.Code)
			}
			if tt.wantError != (err != nil) {
				t.Errorf("Delete() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestModel_List(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		dsn       string
		wantError bool
	}{
		{
			name:      "test",
			path:      "../../testdata/test.yml",
			dsn:       "root:123456@tcp(127.0.0.1:3306)/test?charset=utf8mb4&parseTime=True&loc=Local",
			wantError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rb, err := os.ReadFile(tt.path)
			if err != nil {
				t.Fatalf("ReadFile() error = %v", err)
			}
			m := &Model{}
			err = yaml.Unmarshal(rb, m)
			if err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			db := openTestDB(t)
			if err = m.Migrate(db); err != nil {
				t.Fatalf("Migrate() error = %v", err)
			}
			if err = db.Exec("INSERT INTO test (id, name) VALUES (?, ?)", id, "test1").Error; err != nil {
				t.Fatalf("seed test model error = %v", err)
			}
			// 创建一个虚拟的 HTTP 请求和响应
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodGet, "/test?pageSize=10&current=1&name=test1", nil)
			r := gin.Default()
			r.GET("/test", func(ctx *gin.Context) {
				items := m.MakeList()
				litter.Dump(items)
				page := &PaginationTest{}
				var count int64
				if err = db.Scopes(m.TableScope, m.Search(ctx), m.Pagination(ctx, page)).Find(items).Limit(-1).Count(&count).Error; err != nil {
					ctx.Status(http.StatusInternalServerError)
					t.Fatalf("Find() error = %v", err)
				}
				fmt.Println(count)
				litter.Dump(items)
				litter.Dump(page)
				ctx.Status(http.StatusOK)
			})
			r.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Errorf("Expected status code %d, but got %d", http.StatusOK, w.Code)
			}
			if tt.wantError != (err != nil) {
				t.Errorf("Data() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}
