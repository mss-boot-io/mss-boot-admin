---
title: http服务
nav:
  order: 2
  title: "coding"
description: http服务文档
keywords: [http]
---

## 目录结构
```shell
├── apis                                //api接口
│   ├── xxx.go
│   └── xxx_test.go
├── cmd                                 //命令行
│   ├── cobra.go
│   ├── server                          //服务启动
│   │   ├── server.go
│   ├── migrate                         //数据库迁移
│   │   ├── migration
│   │   │   ├── custom                  //自定义迁移
│   │   │   │   └── xxx_migrate.go
│   │   │   ├── system                  //系统迁移
│   │   │   │   ├── xxx_migrate.go
├── config
│   ├── config.go                       //配置项
│   ├── config_test.go                  //测试
│   ├── application.yml                 //全局配置
│   ├── application-local.yml           //本地配置
│   ├── application-dev.yml             //dev环境配置
│   ├── application-prod.yml            //prod环境配置
│   └── application-test.yml            //test环境配置
├── docs                                //swagger文档
│   ├── docs.go
│   ├── swagger.json
│   └── swagger.yaml
├── dto                                 //api接口参数
│   ├── xxx.go
│   └── xxx_test.go
├── middleware                          //中间件
│   ├── xxx.go
│   └── xxx_test.go
├── models                              //数据库模型
│   ├── xxx.go
│   └── xxx_test.go
├── pkg                                 //公共包
│   ├── xxx.go
│   └── xxx_test.go
├── router                              //路由包
│   ├── router.go
│   └── router_test.go
├── Dockerfile
├── generate.go                        //生成命令集成                        
├── go.mod
├── go.sum
├── main.go
└── README.md
```

## 快速开始
1. 根据指南获取[service-http](https://github.com/mss-boot-io/service-http)代码
2. 根据自己的需求，修改项目对应名称
3. 创建config/application-local.yml修改自己的配置
4. 运行`make generate`，生成swagger文档
5. 运行`make migrate`，生成数据库表
6. 运行`make run`，启动项目
