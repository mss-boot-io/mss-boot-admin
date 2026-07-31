---
title: grpc服务
nav:
  order: 2
  title: "coding"
date: 2023-10-31T15:40:00+08:00
description: grpc服务文档
keywords: [grpc]
---


## 目录结构
```shell
├── cmd                         //命令行如果用到数据库可以将migrate模块引入
│   ├── cobra.go
│   ├── server
│   │   └── server.go
├── config
│   ├── config.go               //配置项
│   ├── config_test.go          //测试
│   ├── application.yml         //全局配置
│   ├── application-local.yml   //本地配置
│   ├── application-dev.yml     //dev环境配置
│   ├── application-prod.yml    //prod环境配置
│   └── application-test.yml    //test环境配置
├── proto                       //proto文件目录
│   ├── xxx.proto
│   ├── xxx.pb.go
│   ├── xxx_grpc.pb.go
│   ├── go.mod
│   └── go.sum
├── handlers                    //grpc服务handler代码
│   ├── xxx.go
│   └── xxx_test.go
├── Dockerfile                        
├── generate.go                 //生成命令集成                        
├── go.mod
├── go.sum
├── main.go
└── README.md
```

## 快速开始
1. 根据指南获取[service-grpc](https://github.com/mss-boot-io/service-grpc)代码
2. 根据自己的需求，修改项目对应名称
3. 根据自己的需求，修改项目对应的proto文件
4. 创建config/application-local.yml修改自己的配置
5. 运行`make migrate`，生成数据库表
6. 运行`make run`，启动项目
