package system

import (
	"net/http"
	"runtime"

	"github.com/mss-boot-io/mss-boot/pkg/enum"
	common "github.com/mss-boot-io/mss-boot/pkg/migration/models"
	"gorm.io/gorm"

	"github.com/mss-boot-io/mss-boot-admin-api/cmd/migrate/migration"
	"github.com/mss-boot-io/mss-boot-admin-api/models"
	adminPKG "github.com/mss-boot-io/mss-boot-admin-api/pkg"
)

var Username string
var Password string

func init() {
	_, fileName, _, _ := runtime.Caller(0)
	migration.Migrate.SetVersion(migration.GetFilename(fileName), _1691847581348Migrate)
}

func _1691847581348Migrate(db *gorm.DB, version string) error {
	return db.Transaction(func(tx *gorm.DB) error {

		systemConfig := &models.SystemConfig{
			Name:   "application",
			Ext:    "yaml",
			Remark: "系统配置",
			Content: `
server:
  addr: 0.0.0.0:8080
logger:
  # 日志类型 default: go-admin-core构建的默认日志插件, zap: zap插件
  type: default
  # 日志存放路径，关闭控制台日志后，日志文件存放位置
  # path: temp/logs
  # 日志输出，file：文件，default：命令行，其他：命令行
  stdout: default #控制台日志，启用后，不输出到文件
  # 日志等级, trace, debug, info, warn, error, fatal
  level: info
  # 日志格式 json json格式
  formatter: default
  addSource: true
database:
  driver: mysql
  source: 'root:123456@tcp(127.0.0.1:3306)/mss-boot-admin-local?charset=utf8&parseTime=True&loc=Local&timeout=1000ms'
  name: mss-boot-admin-local
  config:
    disableForeignKeyConstraintWhenMigrating: true
  casbinModel: |
    [request_definition]
    r = sub, tp, obj, act

    [policy_definition]
    p = sub, tp, obj, act

    [policy_effect]
    e = some(where (p.eft == allow))

    [matchers]
    m = r.sub == p.sub && r.tp == p.tp && keyMatch(r.obj, p.obj) && regexMatch(r.act, p.act)
  timeout: 10s
auth:
  realm: 'mss-boot-admin zone'
  key: 'mss-boot-admin-secret'
  timeout: '12h'
  maxRefresh: '1h'
  identityKey: 'mss-boot-admin-identity-key'
application:
  mode: dev
  origin: http://127.0.0.1:8080
  staticPath:
    /public: public
task:
  enable: false
  spec: '0/30 * * * * ?'
oauth2:
  clientID: 6f4b8f6b0eb0941896ee
  clientSecret: 1542df33bbfa7dca64760f9469c7276bebdf23e4
  scopes:
    - user
    - repo
  redirectURL: "http://127.0.0.1:8000/user/github-callback"
  endpoint:
    authURL: "https://github.com/login/oauth/authorize"
    tokenURL: "https://github.com/login/oauth/access_token"
  allowGroup:
    - mss-boot-io
`,
		}
		err := tx.Create(systemConfig).Error
		if err != nil {
			return err
		}
		err = tx.Table(systemConfig.TableName()).Where("id = ?", systemConfig.ID).Updates(map[string]interface{}{
			"built_in": true,
		}).Error
		if err != nil {
			return err
		}

		role := models.Role{
			Name:   "admin",
			Status: enum.Enabled,
			Remark: "admin",
		}
		err = tx.Create(&role).Error
		if err != nil {
			return err
		}
		err = tx.Table(role.TableName()).Where("id = ?", role.ID).Updates(map[string]interface{}{
			"default": true,
			"root":    true,
		}).Error
		if err != nil {
			return err
		}
		user := &models.User{
			UserLogin: models.UserLogin{
				RoleID:   role.ID,
				Username: Username,
				Password: Password,
				Email:    "lwnmengjing@gmail.com",
				Status:   enum.Enabled,
			},
			Name:     "林文祥",
			Avatar:   "https://avatars.githubusercontent.com/u/12806223?v=4",
			Country:  "China",
			Province: "320000",
			City:     "320800",
			Address:  "生态新城枚槹路大桥",
			Profile:  "https://docs.mss-boot-io.top",
			Title:    "后端开发工程师",
			Tags: models.ArrayString{
				"有想法",
			},
			Phone: "18012345678",
		}
		err = tx.Create(user).Error
		if err != nil {
			return err
		}

		// init menu
		menus := []models.Menu{
			{
				Name: "welcome",
				Path: "/welcome",
				Icon: "smile",
				Sort: 20,
				Type: adminPKG.MenuAccessType,
			},
			{
				Name: "system",
				Path: "/",
				Icon: "setting",
				Sort: 19,
				Type: adminPKG.DirectoryAccessType,
				Children: []*models.Menu{
					{
						Name: "task",
						Path: "/task",
						Icon: "wallet",
						Type: adminPKG.MenuAccessType,
						Sort: 20,
						Children: []*models.Menu{
							{
								Name:   "/admin/api/tasks",
								Path:   "/admin/api/tasks",
								Method: http.MethodGet,
								Type:   adminPKG.APIAccessType,
							},
							{
								Name:   "/admin/api/tasks/*",
								Path:   "/admin/api/tasks/:id",
								Method: http.MethodGet,
								Type:   adminPKG.APIAccessType,
							},
							{
								Name:       "control",
								Path:       "/task/:id",
								HideInMenu: true,
								Type:       adminPKG.MenuAccessType,
							},
							{
								Name:       "create",
								Path:       "/task/create",
								HideInMenu: true,
								Type:       adminPKG.ComponentAccessType,
								Children: []*models.Menu{
									{
										Name:   "/admin/api/tasks",
										Path:   "/admin/api/tasks",
										Method: http.MethodPost,
										Type:   adminPKG.APIAccessType,
									},
								},
							},
							{
								Name:       "edit",
								Path:       "/task/edit",
								HideInMenu: true,
								Type:       adminPKG.ComponentAccessType,
								Children: []*models.Menu{
									{
										Name:   "/admin/api/tasks/*",
										Path:   "/admin/api/tasks/:id",
										Method: http.MethodPut,
										Type:   adminPKG.APIAccessType,
									},
								},
							},
							{
								Name:       "delete",
								Path:       "/task/delete",
								HideInMenu: true,
								Type:       adminPKG.ComponentAccessType,
								Children: []*models.Menu{
									{
										Name:   "/admin/api/tasks/*",
										Path:   "/admin/api/tasks/:id",
										Method: http.MethodDelete,
										Type:   adminPKG.APIAccessType,
									},
								},
							},
							{
								Name:       "operate",
								Path:       "/task/operate",
								HideInMenu: true,
								Type:       adminPKG.ComponentAccessType,
								Children: []*models.Menu{
									{
										Name:   "/admin/api/tasks/*/*",
										Path:   "/admin/api/tasks/:operate/:id",
										Method: http.MethodGet,
										Type:   adminPKG.APIAccessType,
									},
								},
							},
						},
					},
					{
						Name: "role",
						Path: "/role",
						Icon: "team",
						Sort: 19,
						Type: adminPKG.MenuAccessType,
						Children: []*models.Menu{
							{
								Name:   "/admin/api/roles",
								Path:   "/admin/api/roles",
								Method: http.MethodGet,
								Type:   adminPKG.APIAccessType,
							},
							{
								Name:   "/admin/api/roles/*",
								Path:   "/admin/api/roles/:id",
								Method: http.MethodGet,
								Type:   adminPKG.APIAccessType,
							},
							{
								Name:       "control",
								Path:       "/role/:id",
								HideInMenu: true,
								Type:       adminPKG.MenuAccessType,
							},
							{
								Name:       "auth",
								Path:       "/role/auth",
								HideInMenu: true,
								Type:       adminPKG.ComponentAccessType,
								Children: []*models.Menu{
									{
										Name:   "/admin/api/menu/tree",
										Path:   "/admin/api/menu/tree",
										Method: http.MethodGet,
										Type:   adminPKG.APIAccessType,
									},
									{
										Name:   "/admin/api/role/authorize/*",
										Path:   "/admin/api/role/authorize/:id",
										Method: http.MethodGet,
										Type:   adminPKG.APIAccessType,
									},
									{
										Name:   "/admin/api/role/authorize/*",
										Path:   "/admin/api/role/authorize/:id",
										Method: http.MethodPost,
										Type:   adminPKG.APIAccessType,
									},
								},
							},
							{
								Name:       "create",
								Path:       "/role/create",
								HideInMenu: true,
								Type:       adminPKG.ComponentAccessType,
								Children: []*models.Menu{
									{
										Name:   "/admin/api/roles",
										Path:   "/admin/api/roles",
										Method: http.MethodPost,
										Type:   adminPKG.APIAccessType,
									},
								},
							},
							{
								Name:       "edit",
								Path:       "/role/edit",
								HideInMenu: true,
								Type:       adminPKG.ComponentAccessType,
								Children: []*models.Menu{
									{
										Name:   "/admin/api/roles/*",
										Path:   "/admin/api/roles/:id",
										Method: http.MethodPut,
										Type:   adminPKG.APIAccessType,
									},
								},
							},
							{
								Name:       "delete",
								Path:       "/role/delete",
								HideInMenu: true,
								Type:       adminPKG.ComponentAccessType,
								Children: []*models.Menu{
									{
										Name:   "/admin/api/roles/*",
										Path:   "/admin/api/roles/:id",
										Method: http.MethodDelete,
										Type:   adminPKG.APIAccessType,
									},
								},
							},
						},
					},
					{
						Name: "user",
						Path: "/users",
						Icon: "user",
						Sort: 18,
						Type: adminPKG.MenuAccessType,
						Children: []*models.Menu{
							{
								Name:   "/admin/api/users",
								Path:   "/admin/api/users",
								Method: http.MethodGet,
								Type:   adminPKG.APIAccessType,
							},
							{
								Name:   "/admin/api/users/*",
								Path:   "/admin/api/users/:id",
								Method: http.MethodGet,
								Type:   adminPKG.APIAccessType,
							},
							{
								Name:       "control",
								Path:       "/users/:id",
								HideInMenu: true,
								Type:       adminPKG.MenuAccessType,
							},
							{
								Name:       "create",
								Path:       "/users/create",
								HideInMenu: true,
								Type:       adminPKG.ComponentAccessType,
								Children: []*models.Menu{
									{
										Name:   "/admin/api/users",
										Path:   "/admin/api/users",
										Method: http.MethodPost,
										Type:   adminPKG.APIAccessType,
									},
								},
							},
							{
								Name:       "edit",
								Path:       "/users/edit",
								HideInMenu: true,
								Type:       adminPKG.ComponentAccessType,
								Children: []*models.Menu{
									{
										Name:   "/admin/api/users/*",
										Path:   "/admin/api/users/:id",
										Method: http.MethodPut,
										Type:   adminPKG.APIAccessType,
									},
								},
							},
							{
								Name:       "delete",
								Path:       "/users/delete",
								HideInMenu: true,
								Type:       adminPKG.ComponentAccessType,
								Children: []*models.Menu{
									{
										Name:   "/admin/api/users/*",
										Path:   "/admin/api/users/:id",
										Method: http.MethodDelete,
										Type:   adminPKG.APIAccessType,
									},
								},
							},
							{
								Name:       "password-reset",
								Path:       "/users/password-reset",
								HideInMenu: true,
								Type:       adminPKG.ComponentAccessType,
								Children: []*models.Menu{
									{
										Name:   "/admin/api/user/*/password-reset",
										Path:   "/admin/api/user/:id/password-reset",
										Method: http.MethodPut,
										Type:   adminPKG.APIAccessType,
									},
								},
							},
						},
					},
					{
						Name: "menu",
						Path: "/menu",
						Icon: "menu",
						Sort: 17,
						Type: adminPKG.MenuAccessType,
						Children: []*models.Menu{
							{
								Name:   "/admin/api/menus",
								Path:   "/admin/api/menus",
								Method: http.MethodGet,
								Type:   adminPKG.APIAccessType,
							},
							{
								Name:   "/admin/api/menus/*",
								Path:   "/admin/api/menus/:id",
								Method: http.MethodGet,
								Type:   adminPKG.APIAccessType,
							},
							{
								Name:       "control",
								Path:       "/menu/:id",
								HideInMenu: true,
								Type:       adminPKG.MenuAccessType,
							},
							{
								Name:       "create",
								Path:       "/menu/create",
								HideInMenu: true,
								Type:       adminPKG.ComponentAccessType,
								Children: []*models.Menu{
									{
										Name:   "/admin/api/menus",
										Path:   "/admin/api/menus",
										Method: http.MethodPost,
										Type:   adminPKG.APIAccessType,
									},
								},
							},
							{
								Name:       "edit",
								Path:       "/menu/edit",
								HideInMenu: true,
								Type:       adminPKG.ComponentAccessType,
								Children: []*models.Menu{
									{
										Name:   "/admin/api/menus/*",
										Path:   "/admin/api/menus/:id",
										Method: http.MethodPut,
										Type:   adminPKG.APIAccessType,
									},
								},
							},
							{
								Name:       "delete",
								Path:       "/menu/delete",
								HideInMenu: true,
								Type:       adminPKG.ComponentAccessType,
								Children: []*models.Menu{
									{
										Name:   "/admin/api/menus/*",
										Path:   "/admin/api/menus/:id",
										Method: http.MethodDelete,
										Type:   adminPKG.APIAccessType,
									},
								},
							},
							{
								Name:       "bind-api",
								Path:       "/menu/bind-api",
								HideInMenu: true,
								Type:       adminPKG.ComponentAccessType,
								Children: []*models.Menu{
									{
										Name:   "/admin/api/menu/bind-api",
										Path:   "/admin/api/menu/bind-api",
										Method: http.MethodPost,
										Type:   adminPKG.APIAccessType,
									},
								},
							},
						},
					},
					{
						Name: "language",
						Path: "/language",
						Icon: "translation",
						Sort: 16,
						Type: adminPKG.MenuAccessType,
						Children: []*models.Menu{
							{
								Name:   "/admin/api/languages",
								Path:   "/admin/api/languages",
								Method: http.MethodGet,
								Type:   adminPKG.APIAccessType,
							},
							{
								Name:   "/admin/api/languages/*",
								Path:   "/admin/api/languages/:id",
								Method: http.MethodGet,
								Type:   adminPKG.APIAccessType,
							},
							{
								Name:       "control",
								Path:       "/language/:id",
								HideInMenu: true,
								Type:       adminPKG.MenuAccessType,
							},
							{
								Name:       "create",
								Path:       "/language/create",
								HideInMenu: true,
								Type:       adminPKG.ComponentAccessType,
								Children: []*models.Menu{
									{
										Name:   "/admin/api/languages",
										Path:   "/admin/api/languages",
										Method: http.MethodPost,
										Type:   adminPKG.APIAccessType,
									},
								},
							},
							{
								Name:       "edit",
								Path:       "/language/edit",
								HideInMenu: true,
								Type:       adminPKG.ComponentAccessType,
								Children: []*models.Menu{
									{
										Name:   "/admin/api/languages/*",
										Path:   "/admin/api/languages/:id",
										Method: http.MethodPut,
										Type:   adminPKG.APIAccessType,
									},
								},
							},
							{
								Name:       "delete",
								Path:       "/language/delete",
								HideInMenu: true,
								Type:       adminPKG.ComponentAccessType,
								Children: []*models.Menu{
									{
										Name:   "/admin/api/languages/*",
										Path:   "/admin/api/languages/:id",
										Method: http.MethodDelete,
										Type:   adminPKG.APIAccessType,
									},
								},
							},
						},
					},
					{
						Name: "system-config",
						Path: "/system-config",
						Icon: "inbox",
						Sort: 15,
						Type: adminPKG.MenuAccessType,
						Children: []*models.Menu{
							{
								Name:   "/admin/api/system-configs",
								Path:   "/admin/api/system-configs",
								Method: http.MethodGet,
								Type:   adminPKG.APIAccessType,
							},
							{
								Name:   "/admin/api/system-configs/*",
								Path:   "/admin/api/system-configs/:id",
								Method: http.MethodGet,
								Type:   adminPKG.APIAccessType,
							},
							{
								Name:       "control",
								Path:       "/system-config/:id",
								HideInMenu: true,
								Type:       adminPKG.MenuAccessType,
							},
							{
								Name:       "create",
								Path:       "/system-config/create",
								HideInMenu: true,
								Type:       adminPKG.ComponentAccessType,
								Children: []*models.Menu{
									{
										Name:   "/admin/api/system-configs",
										Path:   "/admin/api/system-configs",
										Method: http.MethodPost,
										Type:   adminPKG.APIAccessType,
									},
								},
							},
							{
								Name:       "edit",
								Path:       "/system-config/edit",
								HideInMenu: true,
								Type:       adminPKG.ComponentAccessType,
								Children: []*models.Menu{
									{
										Name:   "/admin/api/system-configs/*",
										Path:   "/admin/api/system-configs/:id",
										Method: http.MethodPut,
										Type:   adminPKG.APIAccessType,
									},
								},
							},
							{
								Name:       "delete",
								Path:       "/system-config/delete",
								HideInMenu: true,
								Type:       adminPKG.ComponentAccessType,
								Children: []*models.Menu{
									{
										Name:   "/admin/api/system-configs/*",
										Path:   "/admin/api/system-configs/:id",
										Method: http.MethodDelete,
										Type:   adminPKG.APIAccessType,
									},
								},
							},
						},
					},
				},
			},
			{
				Name: "generator",
				Path: "/generator",
				Icon: "form",
				Sort: 18,
				Type: adminPKG.MenuAccessType,
				Children: []*models.Menu{
					{
						Name:   "/admin/api/template/get-branches",
						Path:   "/admin/api/template/get-branches",
						Method: http.MethodGet,
						Type:   adminPKG.APIAccessType,
					},
					{
						Name:   "/admin/api/template/get-path",
						Path:   "/admin/api/template/get-path",
						Method: http.MethodGet,
						Type:   adminPKG.APIAccessType,
					},
					{
						Name:   "/admin/api/template/get-params",
						Path:   "/admin/api/template/get-params",
						Method: http.MethodGet,
						Type:   adminPKG.APIAccessType,
					},
					{
						Name:   "/admin/api/template/generate",
						Path:   "/admin/api/template/generate",
						Method: http.MethodPost,
						Type:   adminPKG.APIAccessType,
					},
				},
			},
		}

		err = tx.Create(&menus).Error
		if err != nil {
			return err
		}

		messages := []models.Message{
			{
				UserID:   user.ID,
				Type:     "message",
				Title:    "郑曦月",
				SubTitle: "的私信",
				Avatar:   "//p1-arco.byteimg.com/tos-cn-i-uwbnlip3yd/8361eeb82904210b4f55fab888fe8416.png~tplv-uwbnlip3yd-webp.webp",
				Content:  "审批请求已发送，请查收",
				Time:     "今天 12:30:01",
			},
			{
				UserID:   user.ID,
				Type:     "message",
				Title:    "宁波",
				SubTitle: "的回复",
				Avatar:   "//p1-arco.byteimg.com/tos-cn-i-uwbnlip3yd/3ee5f13fb09879ecb5185e440cef6eb9.png~tplv-uwbnlip3yd-webp.webp",
				Content:  "此处 bug 已经修复，如有问题请查阅文档或者继续 github 提 issue～",
				Time:     "今天 12:30:01",
			},
			{
				UserID:   user.ID,
				Type:     "message",
				Title:    "宁波",
				SubTitle: "的回复",
				Avatar:   "//p1-arco.byteimg.com/tos-cn-i-uwbnlip3yd/3ee5f13fb09879ecb5185e440cef6eb9.png~tplv-uwbnlip3yd-webp.webp",
				Content:  "此处 bug 已经修复",
				Time:     "今天 12:30:01",
			},
			{
				UserID:  user.ID,
				Type:    "todo",
				Title:   "域名服务",
				Content: "内容质检队列于 2021-12-01 19:50:23 进行变更，请重新",
				Tag:     []string{"未开始", "gray"},
			},
			{
				UserID:  user.ID,
				Type:    "todo",
				Title:   "内容审批通知",
				Content: "宁静提交于 2021-11-05，需要您在 2011-11-07之前审批",
				Tag:     []string{"进行中", "arcoblue"},
			},
			{
				UserID:  user.ID,
				Type:    "notice",
				Title:   "质检队列变更",
				Content: "您的产品使用期限即将截止，如需继续使用产品请前往购…",
				Tag:     []string{"即将到期", "red"},
			},
			{
				UserID:  user.ID,
				Type:    "notice",
				Title:   "规则开通成功",
				Content: "内容屏蔽规则于 2021-12-01 开通成功并生效。",
				Tag:     []string{"已开通", "green"},
			},
		}
		err = tx.Create(&messages).Error
		if err != nil {
			return err
		}

		m := &models.Model{
			Name:        "demo",
			Description: "demo",
			Table:       "demo",
			Path:        "demo",
		}
		err = tx.Create(m).Error
		if err != nil {
			return err
		}

		cs := []models.Field{
			{
				ModelID:    m.ID,
				Name:       "id",
				Label:      "ID",
				Show:       []byte(`{"show":true,"width":100,"align":"center","sortable":true,"ellipsis":true}`),
				Type:       "string",
				Size:       64,
				PrimaryKey: "true",
			},
			{
				ModelID:     m.ID,
				Name:        "name",
				Label:       "名称",
				Show:        []byte(`{"show":true,"width":100,"align":"center","sortable":true,"ellipsis":true}`),
				Type:        "string",
				Size:        255,
				UniqueIndex: "name",
			},
		}
		err = tx.Create(&cs).Error
		if err != nil {
			return err
		}

		languages := []models.Language{
			{
				Name:   "zh-CN",
				Remark: "简体中文",
				Status: enum.Enabled,
			},
			{
				Name:   "en-US",
				Remark: "English",
				Status: enum.Enabled,
			},
		}
		err = tx.Create(&languages).Error
		if err != nil {
			return err
		}

		return tx.Create(&common.Migration{
			Version: version,
		}).Error
	})
}
