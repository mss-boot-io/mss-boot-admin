package system

import (
	"errors"
	"runtime"

	"github.com/mss-boot-io/mss-boot-admin/admin/models"
	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/enum"

	"github.com/mss-boot-io/mss-boot-admin/mss-boot/pkg/migration"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	Username string
	Password string
	Domain   string
)

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
  # Configure GitHub OAuth credentials through a protected environment-specific override.
  clientID: ''
  clientSecret: ''
  scopes:
    - read:user
    - user:email
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

		adminRole := &models.Role{Default: true}
		err = tx.Model(&models.Role{}).
			Where(clause.Eq{Column: clause.Column{Name: "default"}, Value: true}).
			First(adminRole).Error
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			adminRole = &models.Role{
				Name:   "admin",
				Status: enum.Enabled,
				Remark: "admin",
			}
			err = tx.Create(adminRole).Error
			if err != nil {
				return err
			}
			err = tx.Table(adminRole.TableName()).Where("id = ?", adminRole.ID).Updates(map[string]any{
				"default": true,
				"root":    true,
			}).Error
			if err != nil {
				return err
			}
		}
		if adminRole.ID == "" {
			return migration.Migrate.CreateVersion(tx, version)
		}
		adminUser := &models.User{
			UserLogin: models.UserLogin{
				RoleID:   adminRole.ID,
				Username: Username,
				Password: Password,
				Status:   enum.Enabled,
			},
			Name: "admin",
		}
		err = tx.Where("username = ?", Username).FirstOrCreate(adminUser).Error
		if err != nil {
			return err
		}

		return migration.Migrate.CreateVersion(tx, version)
	})
}
