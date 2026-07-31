package source

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/3/14 23:47:41
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/3/14 23:47:41
 */

// Entity 配置实体
type Entity interface {
	OnChange()
}

type PrefixHook interface {
	Init()
}

type PostHook interface {
	Entity
}
