package pkg

import "embed"

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/10 00:26:51
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/10 00:26:51
 */

//go:embed version
var versionFS embed.FS

// Version is the version of the binary
var Version string

// fixme 这里后面可能会改为读取CHNAGELOG.md文件中的版本号
func init() {
	if Version == "" {
		rb, err := versionFS.ReadFile("version")
		if err != nil {
			Version = "unknown"
		} else {
			Version = string(rb)
		}
	}
}
