package source

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/5/29 07:42:12
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/5/29 07:42:12
 */

// Scheme scheme
type Scheme string

const (
	// SchemeYaml yaml
	SchemeYaml Scheme = "yaml"
	// SchemeYml yml
	SchemeYml Scheme = "yml"
	// SchemeJSOM json
	SchemeJSOM Scheme = "json"
)

// String string
func (s Scheme) String() string {
	return string(s)
}

type Driver interface {
	GenerateBytes() ([]byte, error)
	GetExtend() Scheme
}
