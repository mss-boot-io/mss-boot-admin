package version

/*
 * @Author: lwnmengjing
 * @Date: 2021/5/18 1:06 下午
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2021/5/18 1:06 下午
 */

// Info contains versioning information.
// TODO: Add []string of api versions supported? It's still unclear
// how we'll want to distribute that information.
type Info struct {
	Major        string `json:"major"`
	Minor        string `json:"minor"`
	GitVersion   string `json:"gitVersion"`
	GitCommit    string `json:"gitCommit"`
	GitTreeState string `json:"gitTreeState"`
	BuildDate    string `json:"buildDate"`
	GoVersion    string `json:"goVersion"`
	Compiler     string `json:"compiler"`
	Platform     string `json:"platform"`
}

// String returns info as a human-friendly version string.
func (info Info) String() string {
	return info.GitVersion
}
