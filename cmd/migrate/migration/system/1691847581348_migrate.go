package system

import (
	"runtime"

	"github.com/mss-boot-io/mss-boot-admin-api/cmd/migrate/migration"
	"github.com/mss-boot-io/mss-boot-admin-api/models"
	"github.com/mss-boot-io/mss-boot/pkg/enum"
	common "github.com/mss-boot-io/mss-boot/pkg/migration/models"
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

		// TODO: here to write the content to be changed

		// TODO: e.g. modify table field, please delete this code during use
		//err := tx.Migrator().RenameColumn(&models.SysConfig{}, "config_id", "id")
		//if err != nil {
		// 	return err
		//}
		role := models.Role{
			Name:   "admin",
			Root:   true,
			Status: enum.Enabled,
			Remark: "admin",
		}
		err := tx.Create(&role).Error
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
			},
			{
				Name: "generator",
				Path: "/generator",
				Icon: "form",
				Sort: 19,
			},
			{
				Name: "task",
				Path: "/task",
				Icon: "wallet",
				Sort: 18,
			},
			{
				Name: "role",
				Path: "/role",
				Icon: "team",
				Sort: 17,
				Children: []*models.Menu{
					{
						Name:       "control",
						Path:       "/role/:id",
						HideInMenu: true,
					},
				},
			},
			{
				Name: "user",
				Path: "/users",
				Icon: "user",
				Sort: 16,
				Children: []*models.Menu{
					{
						Name:       "control",
						Path:       "/users/:id",
						HideInMenu: true,
					},
				},
			},
			{
				Name: "menu",
				Path: "/menu",
				Icon: "menu",
				Sort: 15,
				Children: []*models.Menu{
					{
						Name:       "control",
						Path:       "/menu/:id",
						HideInMenu: true,
					},
				},
			},
			{
				Name: "language",
				Path: "/language",
				Icon: "translation",
				Sort: 14,
				Children: []*models.Menu{
					{
						Name:       "control",
						Path:       "/language/:id",
						HideInMenu: true,
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
				Status: enum.Enabled,
			},
			{
				Name:   "en-US",
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
