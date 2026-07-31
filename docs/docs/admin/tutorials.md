---
title: 配置操作
order: 12
nav:
  order: 1
  title: admin
description: 用户配置与操作教程
keywords: [admin config tutorial]
---

`mss-boot-admin`的系统配置已经集成在数据库中，可以通过gorm支持的数据库进行配置，
虽然mss-boot-admin的配置支持本地文件、embed、s3协议的对象存储，但是这些配置源在后台管理系统中都不是首选，
后台管理系统的配置源首选就是数据库，因为数据库的配置可以通过后台管理系统进行配置，而且可以通过配置的方式进行迁移。

### 配置逻辑
作为软件工程项目，一般都会经历开发、测试、生产等多个环境，每个环境的配置都是不一样的，所以`mss-boot-admin`的配置也是分环境的，
我们将这些不同阶段的环境成为`stage`，默认`stage`为`local`， 即为本地开发环境。
一般提交代码并合并到主分支，这时候应该将代码发布到`beta`环境。
当`beta`环境的代码测试通过后，就可以发布到`prod`环境。

所以`mss-boot-admin`的配置分为两个部分：
`application.yml`和`application-{stage}.yml`, 其中`application.yml`为公共配置，`application-{stage}.yml`为各个环境的配置，
:::warning
优先级顺序为：application-{stage}.yml > application.yml

所以 application-{stage}.yml中的值会覆盖application.yml中的值
:::

### 定制配置

   将项目启动后，打开`http://localhost:8000`, 点击`系统管理` ->`系统配置`，即可进入系统配置页面，点击`新建`按钮，即可新建配置，如下图所示：
 
![系统配置](/images/config01.jpg)

    下面给出一个oauth2配置的示例，配置如下：

![系统配置](/images/config02.jpg)

### 模板支持
:::info
通过模板支持(go template所有语法)，可以将配置文件中的一些敏感信息通过环境变量传入，这样就可以避免将敏感信息提交到代码仓库或者数据库中。
:::

> `mss-boot-admin`的配置支持模板，模板使用`{{` `}}`包裹， 目前支持环境变量作为模板变量传入，且支持go template所有语法, 如下所示：
```yaml
oauth2:
  clientID: {{ .Env.CLIENTID }}
  clientSecret: {{ .Env.CLIENTSECRET }}
```
> 在这个示例中，`{{ .Env.CLIENTID }}`和`{{ .Env.CLIENTSECRET }}`就是模板变量，这两个变量的值是从环境变量中获取的，所以在启动服务时，需要将github的clientId和clientSecret作为环境变量传入，如下所示：
```shell
export CLIENTID=xxxx
export CLIENTSECRET=xxxx
```

### 缓存配置
:::info
利用缓存模块(memory/redis)可以提高系统性能，同时系统的应用配置模块也支持缓存配置，可以通过缓存配置来提高系统性能，自己在开发时，可以合理利用缓存来提高系统性能。
:::
> `mss-boot-admin`的缓存配置支持`redis`和`memory`两种缓存方式，`memory`为内存缓存一般用于本地开发，`redis`为redis缓存，一般用于生产环境，如下所示：
> 
#### 将redis密码作为环境变量传入
```shell
export REDIS_PASSWORD=xxxx
```
#### 配置memory缓存
```yaml
cache:
  memory: ''
```
#### 配置redis缓存
```yaml
cache:
  redis:
    addr: 'localhost:6379'
    password: {{ .Env.REDIS_PASSWORD }}
    db: 9
```

### 队列配置
:::info
利用队列模块(memory/kafka/redis/nsq)可以将一些耗时的任务放入队列中，然后异步执行，这样可以提高系统性能，同时系统的应用配置模块也支持队列配置，可以通过队列配置来提高系统性能和可靠性，自己在开发时，可以合理利用队列。
:::
> `mss-boot-admin`的队列配置支持`memory`、`kafka`、`nsq`和`redis`四种队列方式，`memory`为内存队列一般用于本地开发，`kafka`为kafka队列，一般用于生产环境，`nsq`为nsq队列，一般用于生产环境且对消息没有顺序要求的场景，`redis`为redis队列，一般用于负载极小的场景。
> 
:::warning
`kafka`适用于对消息有顺序要求的场景，`nsq`适用于对消息没有顺序要求的场景，`redis`适用于负载极小的场景，请根据自己的业务场景选择合适的队列。
:::
#### 配置memory队列
```yaml
queue:
  memory:
    poolNum: 10 # 队列池大小
```
#### 配置kafka队列
```yaml
queue:
  kafka:
    brokers:
      - 'localhost:9092'
    version: '3.6.0' # kafka版本(默认1.0.0)
```
#### 配置nsq队列
```yaml
queue:
  nsq:
    addresses:
      - 'localhost:4150'
    lookupdAddr: 'localhost:4161' # nsqlookupd地址, 用于监听和topic管理nsqd节点, 供消费者使用
    adminAddr: 'localhost:4171'  # nsqadmin地址, 用于获取所有节点信息供生产者使用
```
### 分布式锁配置
:::info
利用分布式锁可以解决分布式环境下的并发问题，同时系统的应用配置模块也支持分布式锁配置，可以通过分布式锁配置来解决分布式环境下的并发问题，自己在开发时，可以合理利用分布式锁。
:::
> `mss-boot-admin`的分布式锁配置只支持`redis`，一般用于生产环境。
> 
#### 配置redis分布式锁
```yaml
### 如果cache和queue都没有使用redis，需要完整配置redis
locker:
  redis:
    addr: 'localhost:6379'
    password: {{ .Env.REDIS_PASSWORD }}
    db: 9
```
```yaml
### 如果cache或queue使用redis，只需要配置redis这个参数即可
locker:
  redis: {}
```
