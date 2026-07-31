package handler

/*
 * @Author: lwnmengjing
 * @Date: 2021/5/18 3:13 下午
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2021/5/18 3:13 下午
 */

import (
	"context"
	"log/slog"

	"github.com/mss-boot-io/mss-boot/core/tools/utils"
)

// Handler 基类
type Handler struct {
	ID        string
	RequestID string
	Log       *slog.Logger
}

// Make 构建
func (e *Handler) Make(c context.Context) {
	e.RequestID = utils.GetRequestID(c)
	e.Log = slog.Default().With("traceID", e.RequestID)
}

// Make 构建
func Make(c context.Context) *Handler {
	h := &Handler{}
	h.Make(c)
	return h
}
