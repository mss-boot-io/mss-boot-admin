package main

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/8/6 08:33:26
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/8/6 08:33:26
 */

import (
	"context"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/mss-boot-io/mss-boot-admin-api/config"
	"github.com/mss-boot-io/mss-boot-admin-api/router"
	"github.com/mss-boot-io/mss-boot/core/server"
)

// @title admin API
// @version 0.0.1
// @description admin接口文档
// @securityDefinitions.apikey Bearer
// @in header
// @name Authorization
// @host localhost:8080
// @BasePath
func main() {
	ctx := context.Background()

	r := gin.Default()
	router.Init(r.Group("/admin"))

	config.Cfg.Init(r)

	log.Println("starting admin manage")

	err := server.Manage.Start(ctx)
	if err != nil {
		log.Fatal(err)
	}
}
