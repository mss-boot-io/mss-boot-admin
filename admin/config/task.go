package config

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/12/5 22:28:26
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/12/5 22:28:26
 */

type Task struct {
	Spec   string `yaml:"spec" json:"spec"`
	Enable bool   `yaml:"enable" json:"enable"`
}
