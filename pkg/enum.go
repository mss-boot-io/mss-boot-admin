package pkg

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/12/21 10:35:52
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/12/21 10:35:52
 */

type AccessType string

const (
	// DirectoryAccessType 目录类型
	DirectoryAccessType AccessType = "DIRECTORY"
	// MenuAccessType 菜单类型
	MenuAccessType AccessType = "MENU"
	// APIAccessType API类型
	APIAccessType AccessType = "API"
	// ComponentAccessType 组件类型
	ComponentAccessType AccessType = "COMPONENT"
)

func (a AccessType) String() string {
	return string(a)
}

type LoginProvider string

const (
	// GithubLoginProvider github oauth provider
	GithubLoginProvider LoginProvider = "github"
	// LarkLoginProvider lark oauth provider
	LarkLoginProvider LoginProvider = "lark"
	// EmailLoginProvider email login provider
	EmailLoginProvider LoginProvider = "email"
)

func (o LoginProvider) String() string {
	return string(o)
}
