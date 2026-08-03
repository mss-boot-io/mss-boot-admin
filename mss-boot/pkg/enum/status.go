package enum

/*
 * @Author: lwnmengjing
 * @Date: 2022/3/14 9:10
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2022/3/14 9:10
 */

// Status type for enum
type Status string

const (
	// Unknown status for unknown
	Unknown Status = ""
	// Enabled enabled
	Enabled Status = "enabled"
	// Disabled disabled
	Disabled Status = "disabled"
	// Locked lock status
	Locked Status = "locked"
)

// String string
func (s Status) String() string {
	return string(s)
}
