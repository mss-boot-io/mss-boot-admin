package models

import (
	"context"
	"github.com/mss-boot-io/mss-boot/pkg/config/gormdb"
	"github.com/mss-boot-io/mss-boot/pkg/response/actions"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/11/30 16:09:53
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/11/30 16:09:53
 */

type Github struct {
	actions.ModelGorm
	Email    string `bson:"email" json:"email"`
	Username string `bson:"username" json:"username"`
	Password string `bson:"password" json:"password"`
}

func (*Github) TableName() string {
	return "mss_boot_github"
}

// GetMyGithubConfig 获取当前用户的github配置
func GetMyGithubConfig(c context.Context, email string) (*Github, error) {
	g := &Github{}
	err := gormdb.DB.WithContext(c).Where("email = ?", email).First(g).Error
	if err != nil {
		return nil, err
	}
	return g, nil
}
