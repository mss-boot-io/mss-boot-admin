package pkg

import (
	"embed"
	"strings"
)

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

// Commit is the source revision used to build the binary. Release workflows
// override both Version and Commit with -ldflags so archives and containers
// remain traceable to an immutable source revision.
var Commit = "unknown"

// fixme 这里后面可能会改为读取CHNAGELOG.md文件中的版本号
func init() {
	Version = strings.TrimSpace(Version)
	if Version == "" {
		rb, err := versionFS.ReadFile("version")
		if err != nil {
			Version = "devel"
		} else {
			Version = strings.TrimSpace(string(rb))
		}
	}
	Commit = strings.TrimSpace(Commit)
	if Commit == "" {
		Commit = "unknown"
	}
}

// BuildVersion returns the human-readable version reported by the Admin CLI.
func BuildVersion() string {
	return FormatBuildVersion(Version, Commit)
}

// FormatBuildVersion combines a release version and source revision while
// keeping development builds concise.
func FormatBuildVersion(version, commit string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		version = "devel"
	}
	commit = strings.TrimSpace(commit)
	if commit == "" || commit == "unknown" {
		return version
	}
	return version + " (commit " + commit + ")"
}
