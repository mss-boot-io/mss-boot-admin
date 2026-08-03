package server

/*
 * @Author: lwnmengjing
 * @Date: 2021/6/7 5:39 下午
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2021/6/7 5:39 下午
 */

import (
	"context"
	"fmt"
)

// Manager server manage
type Manager interface {
	Add(...Runnable)
	Start(context.Context) error
}

// Runnable runnable
type Runnable interface {
	fmt.Stringer
	// Start 启动
	Start(ctx context.Context) error
}
