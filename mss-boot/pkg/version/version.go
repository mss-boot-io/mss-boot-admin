package version

/*
 * @Author: lwnmengjing
 * @Date: 2021/5/18 1:05 下午
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2021/5/18 1:05 下午
 */

import (
	"fmt"
	"runtime"
)

// Get 获取version info
func Get() Info {
	return Info{
		Major:        gitMajor,
		Minor:        gitMinor,
		GitVersion:   gitVersion,
		GitCommit:    gitCommit,
		GitTreeState: gitTreeState,
		BuildDate:    buildDate,
		GoVersion:    runtime.Version(),
		Compiler:     runtime.Compiler,
		Platform:     fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}
