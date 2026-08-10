---
title: Challenge 内部安全 checkpoint
order: 7
description: 累积进 v1.1.0 的一次性邮件验证码原子状态、SecretRef 配置与开发检查边界
keywords: [v1.1.0 checkpoint challenge otp redis email security]
---

# Challenge 内部安全 checkpoint

本文记录 `D0-safety` 的第一个内部实现检查点。项目不再计划发布 `v1.0.1`；本变更将随
`v1.1.0` 一起交付，也不代表
Storage Runtime v2 的公共 `ChallengeStore` API 已经冻结。当前跨 Framework/Admin 的 Go
接口是明确标注为 provisional 的过渡桥，不是兼容性承诺。公共资源 API、named Redis、
数据库邮箱唯一约束与 server schema-readiness 的冻结 SHA 重跑，以及真实 Redis Cluster conformance
仍属于后续门禁。

## 已落地的安全边界

- 六位验证码来自 `crypto/rand`，保留前导零。
- Redis key 使用独立 subject locator secret 对规范化邮箱与 purpose 做 HMAC；邮箱和
  purpose 不以明文进入 key。
- Redis 只保存带版本 pepper 的 HMAC verifier，不保存验证码明文。
- 邮件发送严格经过 `Begin -> deliver -> Commit`；请求上下文仍可用时，发送失败执行带版本的
  `Abort`。请求取消、进程退出或发送方挂起时由短期 pending lease 原子回收，不能依赖脱离请求的
  后台补偿。
- 旧 active challenge 在新邮件发送失败时继续有效；失败发送仍计入滚动 quota，但不启动
  resend cooldown。
- pending reservation 有独立短 lease；进程崩溃或发送挂起后可以回收，迟到的
  Commit/Abort 不能覆盖较新状态。
- 错误尝试只原子增加 attempts，达到上限才锁定；正确验证码并发验证最多成功一次。
- 登录、注册、密码重置使用三个固定 purpose，不能跨用途复用。
- subject、调用方和全局三层发码限制都在 Redis 中原子执行；调用方标识经过 HMAC，Redis
  不保存原始 IP。只有 `application.trustedProxies` 明确允许的代理才可提供转发地址，默认
  不信任 `X-Forwarded-For`。
- SMTP 发送继承请求取消，具有明确 deadline 和进程内并发上限；原始 SMTP 错误、收件人
  和验证码不会进入运行日志。
- Redis、pepper 或状态损坏时 fail closed；Admin 返回固定的 `503`，不回退到内存或旧
  email-only store。
- 合法邮箱的发码请求不先查询账户，存在与不存在账户返回相同的 `202` 响应并走相同发信
  路径；cooldown/quota 返回 `429`，非法邮箱或 purpose 返回 `422`。
- 邮箱身份临时按不超过 100 字节的 ASCII 地址整体 case-fold；歧义查询固定失败，不任意
  选择账户。邮件注册使用独立的 18 字节 opaque username，不再把最长 100 字节的邮箱写入
  legacy `varchar(20)` username。D2 已加入三 dialect active/non-empty 唯一迁移和安全写入边界，
  server schema-readiness 开发 checkpoint 也会在业务路由挂载前 fail closed；但冻结 SHA 上尚未重跑
  该正负套件和 MySQL/PostgreSQL 双 DSN zero-skip evidence；
  因此自助邮箱修改继续禁用，邮件注册与首次 OAuth 建号仍不应在生产启用。
- 公开 Profile 的 `emailChallengeReady` 每次请求重新检查 Redis 和 SMTP 配置，不进入
  静态缓存；前端还会同时检查 `emailEnabled`，注册额外检查 `registerEnabled`。这个字段
  只表示投递/状态机就绪，不表示数据库 identity 已通过唯一性门禁。

## 配置

Challenge 默认关闭。开启前必须准备两个相互独立、至少 32 字节、标准 Base64 编码的
环境变量；YAML 只保存 `env://` 引用，不保存原始秘密：

```yaml
challenge:
  enabled: true
  keySecretRef: env://MSS_CHALLENGE_KEY_SECRET
  currentPepper:
    version: v1
    secretRef: env://MSS_CHALLENGE_PEPPER_V1
  activeTTL: 5m
  pendingLease: 30s
  resendCooldown: 1m
  issueWindow: 1h
  issueLimit: 5
  maxAttempts: 5
  idempotencyLease: 2m
  callerWindow: 10m
  callerLimit: 10
  globalWindow: 10m
  globalLimit: 1000

application:
  # 默认空列表：不信任任何转发客户端地址。仅填写受控反向代理 IP/CIDR。
  trustedProxies: []
```

可用以下方式生成测试或部署秘密；值只写入目标秘密管理系统，不写入仓库或日志：

```shell
openssl rand -base64 32
```

轮换时可以增加一个 `previousPepper`。current 与 previous 的 version 必须不同，最多保留
两个 verifier pepper；locator secret 不随 verifier pepper 轮换，否则已有 challenge 将
无法寻址。静态 SecretRef 语法、版本、duration 或限制值违反合同，以及解析出的 secrets
明显弱、重复或不独立时，应用初始化失败。SecretRef 暂时无法解析、Redis 不可用或配置为
本版本不支持的 Cluster/Ring 时，Admin 可以继续启动，但会发布 `nil` Challenge、readiness
为 false，相关 HTTP 流程返回 `503`；配置重载不会偷偷沿用旧 pepper/TTL 实例。

数据库 AppConfig 还必须显式启用 `security:emailEnabled`，并提供完整的
`email:smtpHost`、`email:smtpPort`、`email:username`、`email:password`。注册流还要求
`security:registerEnabled`。这些开关在发码和消费验证码时都由后端再次校验，前端隐藏不
构成授权或安全边界。

## 验收

```shell
cd mss-boot
GOWORK=off go test -race ./pkg/config/storage/cache \
  -run '^(TestChallenge.*|TestLegacyVerifyCodeStoreIsDisabled)$' -count=20

cd ../admin
GOWORK=off go test -race ./apis \
  -run '^(TestEmailChallenge.*|TestAppConfigProfile.*|TestUpdateUserInfoRejectsEmailChangeUntilCanonicalIdentityMigration)$' -count=20
GOWORK=off go test -race ./middleware \
  -run '^TestEmailChallengeLoginProviderOutageReturnsServiceUnavailable$' -count=20
GOWORK=off go test -race ./models \
  -run '^(TestEmailRegistrationRejectsExistingCanonicalIdentity|TestEmailRegistrationUsesBoundedOpaqueUsername|TestGetUserByEmailFailsClosedOnAmbiguousIdentity|TestEmailIdentityOperationsDoNotEmitSensitiveSQL|TestConcurrentEmailRegistrationFailsClosedWithoutAmbiguousIdentity)$' -count=20
GOWORK=off go test -race ./notice/email -run '^TestVerificationEmail.*$' -count=20
GOWORK=off go test -race ./pkg \
  -run '^TestCanonicalEmailNormalizesBoundedASCIIIdentity$' -count=20
GOWORK=off go test -race ./center \
  -run '^TestEmailChallengeCapabilityRequiresCanonicalBoolean$' -count=20
GOWORK=off go test -race ./config \
  -run '^TestChallengeConfigRequiresTypedHighEntropySecretRefs$' -count=20
```

这些测试覆盖发信补偿、pending lease 回收、purpose 隔离、pepper 轮换、错误尝试上限、
并发单次成功、账户防枚举、能力开关、trusted proxy、SMTP 取消、readiness、受控日志和
Redis 故障。当前测试只证明每个 subject 的状态/quota/幂等 key 共享一个显式 hash tag，
并在发出相关脚本前拒绝跨 slot 输入；调用方/全局 limiter 是另一组 key。构造器会明确
拒绝 Redis Cluster/Ring，真实多节点 Cluster/failover/NOSCRIPT fixture 尚未建立，因此
`identity.verification-challenge-redis-cluster` 仍是 Planned，不能从字符串测试推断 provider
conformance。

上述命令是开发检查；当前 `mss spec validate` 只验证合同结构，尚未执行 acceptance 或证明
测试非零命中。功能冻结后的集中验证仍须由 phase-aware evidence runner 解析 `go test -json` 并拒绝
`[no tests to run]`、skip 和零命中，不能把裸命令退出码当成发布证据。

## 升级与回滚

本切片没有数据库迁移。旧 `verify-code-*` key 不读取、不迁移，让其按原 TTL 自然过期；
legacy `NewVerifyCode` 符号暂时保留，但调用会返回明确的 disabled 错误，不能作为回滚路径。
需要停止邮件验证码时，将 `challenge.enabled` 设为 `false` 并重启；相关流程会安全地返回
`503`。不要通过恢复旧 GET/SET/DEL 行为完成回滚。

邮箱 identity 的 D2 模型、迁移和 Admin 写入 checkpoint 已落地：先对存量 active 用户做无敏感值
输出的冲突预检，再分别为 SQLite、MySQL 和 PostgreSQL 落地 active/non-empty canonical 唯一约束，
并覆盖 Admin CRUD、邮件注册和 OAuth provision；歧义记录始终 fail closed，不得自动合并或选择
“第一个”账户。生产启用还必须从 exact-freeze SHA 重跑 schema-readiness 与三数据库 evidence；详见
[D2 Canonical Email Identity 内部 checkpoint](/releases/v1-1-0-d2-canonical-email-identity)。自助邮箱修改不在
本 checkpoint 开放。
