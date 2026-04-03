# Contributing to mss-boot-admin

感谢您有兴趣为 mss-boot-admin 做贡献！

## 📋 目录

- [行为准则](#行为准则)
- [如何贡献](#如何贡献)
- [开发流程](#开发流程)
- [代码规范](#代码规范)
- [提交规范](#提交规范)
- [Pull Request 流程](#pull-request-流程)

## 行为准则

本项目采用贡献者公约作为行为准则。参与此项目即表示您同意遵守其条款。请阅读 [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) 了解详情。

## 如何贡献

### 报告 Bug

如果您发现了 bug，请通过 [GitHub Issues](https://github.com/mss-boot-io/mss-boot-admin/issues) 提交。

提交 Bug 报告时，请包含：

1. **清晰的标题和描述**
2. **复现步骤**
3. **预期行为**
4. **实际行为**
5. **环境信息**（操作系统、Go 版本、Node 版本等）
6. **日志或截图**（如果适用）

### 建议新功能

我们欢迎新功能建议！请在 Issue 中详细描述：

1. **功能描述**
2. **使用场景**
3. **预期效果**
4. **可能的实现方式**（可选）

### 改进文档

文档改进是最容易上手的方式：

- 修正错别字
- 改进说明
- 添加示例
- 翻译文档

## 开发流程

### 1. Fork 和 Clone

```bash
# Fork 后 clone 您的仓库
git clone https://github.com/YOUR_USERNAME/mss-boot-admin.git
cd mss-boot-admin

# 添加上游仓库
git remote add upstream https://github.com/mss-boot-io/mss-boot-admin.git
```

### 2. 创建分支

```bash
# 从 main 创建功能分支
git checkout -b feature/your-feature-name

# 或修复分支
git checkout -b fix/your-bug-fix
```

### 3. 开发环境设置

#### 后端

```bash
# 安装 Go 1.21+
go version

# 安装依赖
go mod download

# 运行测试
go test ./...

# 本地运行
go run . migrate
go run . server
```

#### 前端

```bash
cd mss-boot-admin-antd

# 安装 Node 18+
node --version

# 安装 pnpm
npm install -g pnpm

# 安装依赖
pnpm install

# 本地运行
pnpm dev
```

### 4. 进行开发

- 遵循 [代码规范](#代码规范)
- 编写测试
- 更新文档

### 5. 提交变更

```bash
# 添加文件
git add .

# 提交（遵循提交规范）
git commit -m "feat: add new feature"

# 推送到您的 fork
git push origin feature/your-feature-name
```

### 6. 创建 Pull Request

在 GitHub 上创建 Pull Request，填写 PR 模板。

## 代码规范

### Go 代码规范

1. **格式化**

```bash
# 使用 gofmt 格式化代码
gofmt -w .

# 或使用 goimports
goimports -w .
```

2. **命名规范**

- 包名：小写单词，不使用下划线
- 导出函数/变量：大写开头
- 私有函数/变量：小写开头
- 常量：大写或驼峰

3. **注释规范**

```go
// User 用户模型
// 用于存储用户信息
type User struct {
    ID       string `json:"id"`       // 用户ID
    Username string `json:"username"` // 用户名
}

// CreateUser 创建用户
// 参数：
//   - ctx: 上下文
//   - user: 用户信息
// 返回：
//   - error: 错误信息
func CreateUser(ctx context.Context, user *User) error {
    // 实现
}
```

4. **错误处理**

```go
// 推荐：提供上下文信息
if err != nil {
    return fmt.Errorf("failed to create user: %w", err)
}

// 不推荐：忽略错误
if err != nil {
    log.Println(err)
}
```

5. **测试**

```go
func TestCreateUser(t *testing.T) {
    tests := []struct {
        name    string
        user    *User
        wantErr bool
    }{
        {
            name:    "valid user",
            user:    &User{Username: "test"},
            wantErr: false,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := CreateUser(context.Background(), tt.user)
            if (err != nil) != tt.wantErr {
                t.Errorf("CreateUser() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### TypeScript/React 代码规范

1. **格式化**

```bash
# 使用 Prettier 格式化
pnpm format

# 检查格式
pnpm lint
```

2. **命名规范**

- 组件：大驼峰（PascalCase）
- 函数/变量：小驼峰（camelCase）
- 常量：大写下划线（UPPER_SNAKE_CASE）
- 文件名：小写连字符（kebab-case）

3. **组件规范**

```typescript
// 推荐：函数组件 + TypeScript
interface UserCardProps {
  user: API.User;
  onEdit?: (id: string) => void;
}

const UserCard: React.FC<UserCardProps> = ({ user, onEdit }) => {
  return (
    <div>
      <h3>{user.name}</h3>
      {onEdit && <button onClick={() => onEdit(user.id)}>编辑</button>}
    </div>
  );
};

export default UserCard;
```

4. **Hooks 规范**

```typescript
// 推荐：自定义 Hook
const useUserData = (userId: string) => {
  const [user, setUser] = useState<API.User>();
  const [loading, setLoading] = useState(false);
  
  useEffect(() => {
    setLoading(true);
    getUserUserId({ userID: userId })
      .then(setUser)
      .finally(() => setLoading(false));
  }, [userId]);
  
  return { user, loading };
};
```

5. **国际化**

```typescript
// 推荐：使用 i18n
import { useIntl } from '@umijs/max';

const MyComponent = () => {
  const intl = useIntl();
  
  return (
    <div>
      {intl.formatMessage({ id: 'welcome', defaultMessage: '欢迎' })}
    </div>
  );
};
```

## 提交规范

我们使用 [Conventional Commits](https://www.conventionalcommits.org/) 规范。

### 提交消息格式

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Type 类型

- `feat`: 新功能
- `fix`: 修复 bug
- `docs`: 文档变更
- `style`: 代码格式（不影响功能）
- `refactor`: 重构（不增加功能，不修复 bug）
- `perf`: 性能优化
- `test`: 测试相关
- `chore`: 构建/工具链变更
- `revert`: 回退

### Scope 范围

- `api`: API 相关
- `ui`: UI 组件
- `auth`: 认证授权
- `config`: 配置相关
- `db`: 数据库相关
- `docs`: 文档

### 示例

```bash
# 新功能
feat(auth): add OAuth2 login support

# 修复 bug
fix(api): correct user pagination offset

# 文档
docs(readme): update installation guide

# 重构
refactor(service): extract common validation logic

# 性能优化
perf(query): optimize user list query performance
```

## Pull Request 流程

### 1. PR 标题

使用与提交消息相同的格式：

```
feat(auth): add OAuth2 login support
```

### 2. PR 描述

使用 PR 模板，包括：

- **变更说明**：描述您的变更
- **相关 Issue**：链接相关 Issue
- **测试方法**：如何测试这些变更
- **截图**：如果涉及 UI 变更

### 3. 检查清单

提交 PR 前，确保：

- [ ] 代码遵循项目代码规范
- [ ] 已添加必要的测试
- [ ] 所有测试通过
- [ ] 文档已更新
- [ ] 提交消息遵循规范
- [ ] PR 标题清晰明确

### 4. Code Review

- 维护者会审核您的 PR
- 积极回应反馈意见
- 及时更新代码

### 5. 合并

PR 审核通过后，维护者会将其合并到主分支。

## 开发提示

### 保持同步

```bash
# 定期同步上游
git fetch upstream
git checkout main
git merge upstream/main
```

### 调试技巧

#### 后端调试

```bash
# 启用 pprof
go run . server

# 访问性能分析
# http://localhost:8080/debug/pprof/
```

#### 前端调试

```bash
# 开发模式
pnpm dev

# 构建生产版本
pnpm build

# 类型检查
pnpm exec tsc --noEmit
```

### 日志查看

```bash
# 后端日志
tail -f logs/app.log

# 前端控制台
# 浏览器开发者工具
```

## 获取帮助

- **文档**: https://docs.mss-boot-io.top
- **Issues**: https://github.com/mss-boot-io/mss-boot-admin/issues
- **讨论**: https://github.com/mss-boot-io/mss-boot-admin/discussions

## 许可证

通过向本项目提交代码，您同意您的代码将在 MIT 许可证下发布。

---

再次感谢您的贡献！🎉