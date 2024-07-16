package system

import (
	"runtime"
	"time"

	adminPKG "github.com/mss-boot-io/mss-boot-admin/pkg"

	"github.com/mss-boot-io/mss-boot-admin/models"
	"github.com/mss-boot-io/mss-boot/pkg/enum"

	"github.com/mss-boot-io/mss-boot/pkg/migration"
	"gorm.io/gorm"
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
  # 日志存放路径，关闭控制台日志后，日志文件存放位置
  # path: temp/logs
  # 日志输出，file：文件，default：命令行，其他：命令行
  stdout: default #控制台日志，启用后，不输出到文件
  # 日志等级, trace, debug, info, warn, error, fatal
  level: info
  addSource: true
database:
  driver: mysql
  source: '{{ .Env.DB_DSN }}'
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
  origin: http://localhost:8080
  staticPath:
    /public: public
task:
  enable: false
  spec: '0/30 * * * * ?'
oauth2:
  #mss-boot-io组织用于测试的github oauth2配置
  clientID: 6f4b8f6b0eb0941896ee
  clientSecret: 1542df33bbfa7dca64760f9469c7276bebdf23e4
  scopes:
    - user
    - repo
  redirectURL: "http://localhost:8000/user/github-callback"
  endpoint:
    authURL: "https://github.com/login/oauth/authorize"
    tokenURL: "https://github.com/login/oauth/access_token"
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

		m := &models.Model{
			Name:        "demo",
			Description: "demo",
			Table:       "mss_boot_demo",
			Path:        "demo",
			Auth:        true,
			MultiTenant: true,
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
				Type:       "string",
				Size:       64,
				Sort:       100,
				PrimaryKey: "true",
				FieldFrontend: &models.FieldFrontend{
					HideInForm: true,
				},
			},
			{
				ModelID:     m.ID,
				Name:        "name",
				Label:       "名称",
				Type:        "string",
				Size:        255,
				Sort:        99,
				UniqueIndex: "name",
				FieldFrontend: &models.FieldFrontend{
					Rules: []adminPKG.BaseRule{
						{
							Required: true,
						},
					},
				},
			},
			//{
			//	ModelID:       m.ID,
			//	Name:          "status",
			//	Label:         "状态",
			//	Type:          "string",
			//	Size:          10,
			//	Sort:          98,
			//	ValueEnumName: "<optionID>",
			//},
		}
		err = tx.Create(&cs).Error
		if err != nil {
			return err
		}

		expire := time.Now().Add(100 * 365 * 24 * time.Hour)

		tenant := &models.Tenant{
			Name:   "mss-boot-io",
			Remark: "mss-boot-io",
			Status: enum.Enabled,
			Expire: &expire,
			Domains: []*models.TenantDomain{
				{
					Name:   "local",
					Domain: "localhost:8000",
				},
			},
			Default: true,
			AdminUser: models.AdminUser{
				Username: Username,
				Password: Password,
			},
		}
		err = tx.Create(tenant).Error
		if err != nil {
			return err
		}
		err = tx.Table(tenant.TableName()).
			Where("id = ?", tenant.ID).
			Update("default", true).Error
		if err != nil {
			return err
		}

		err = tenant.Migrate(tenant, tx)
		if err != nil {
			return err
		}

		return migration.Migrate.CreateVersion(tx, version)
	})
}
