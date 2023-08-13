package pkg

import (
	"strings"

	"github.com/google/uuid"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/12 21:43:02
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/12 21:43:02
 */

// SimpleID simple id
func SimpleID() string {
	return strings.ReplaceAll(uuid.New().String(), "-", "")
}
