package pkg

import "os"

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/1/25 17:31:06
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/1/25 17:31:06
 */

// PathCreate create path
func PathCreate(dir string) error {
	return os.MkdirAll(dir, os.ModePerm)
}

// PathExist path exist
func PathExist(addr string) bool {
	s, err := os.Stat(addr)
	if err != nil {
		return false
	}
	return s.IsDir()
}
