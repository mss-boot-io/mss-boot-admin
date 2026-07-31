# mss-boot-io 社区运营巡检记忆 - 2026-06-30

## 背景

- 自动化任务：`mss-boot-community-ops-triage`
- 时间：2026-06-30
- 范围：只看 `mss-boot-io` 四个核心开源仓库：`mss-boot`、`mss-boot-admin`、`mss-boot-admin-antd`、`mss-boot-docs`
- 约束：外部非 GitHub 平台只准备草稿，不在没有维护者当次确认的情况下发布；持久记忆必须写入 `mss-boot-docs/aigc`

## 巡检结论

- GitHub Discussions 无新互动；`mss-boot-docs` 的最新 Discussions 仍是 2026-06-14 的三篇社区更新与 reviewer call。
- 四个仓库新增 W26 Weekly Digest issue，均由 GitHub Actions 创建，无人工回复需要处理。
- 主线质量任务整体健康；本轮合并后的 main workflow 均已确认成功。
- `mss-boot-docs#45` 仍处于 `CHANGES_REQUESTED`，作者 `12ain` 暂无新回复；继续等待作者补齐 PR Guard 内容和工具链告警更新。
- `mss-boot-docs#40` 是我此前提交的 2026-06-14 社区更新草稿，内容已过期，已关闭并删除分支，避免合入错误的安全治理表述。

## 本轮已完成

### 合并 actions/checkout Dependabot PR

这四个 PR 都是纯 GitHub Actions workflow 变更，仅将 `actions/checkout@v6` 升级到 `actions/checkout@v7`。处理方式：检查 diff、approve、squash merge、确认 main workflow。

- `mss-boot#393`
  - 合并提交：`9d84520947e0a3189257c3c7a5209bdb6b5c38dd`
  - main 验证：`ci`、`CodeQL Advanced`、`govulncheck`、`OpenSSF Scorecard`、`Copilot Setup Steps`、`GitHub Actions Mirror` 全部成功。
- `mss-boot-admin#404`
  - 合并提交：`9ae3ba18658da412389a47a7767d341ad1ad61a6`
  - main 验证：`CodeQL`、`govulncheck`、`Swagger`、`OpenSSF Scorecard`、`Copilot Setup Steps`、`GitHub Actions Mirror` 全部成功。
- `mss-boot-admin-antd#118`
  - 合并提交：`43e6b28c7535faf466eaca77ab90a2747efa186f`
  - 先通过 REST API 更新落后的 Dependabot 分支，再确认 PR 检查全绿后合并。
  - main 验证：`CI`、`CodeQL`、`OpenSSF Scorecard`、`Copilot Setup Steps`、`GitHub Actions Mirror` 全部成功。
- `mss-boot-docs#46`
  - 合并提交：`dd53ab00d48fe560aa85b0b93f3e9fdbe6616718`
  - 先通过 REST API 更新落后的 Dependabot 分支，再确认 PR 检查全绿后合并。
  - main 验证：`CI`、`CodeQL`、`OpenSSF Scorecard`、`Copilot Setup Steps`、`(prod) Deploy docs` 全部成功。

### 关闭过期 PR

- `mss-boot-docs#40` 已关闭。
- 关闭原因：草稿中仍写着 `mss-boot-admin-antd` Dependabot alert 清零，但后续安全治理状态已变更为旧前端栈仍有迁移项，继续合并会误导社区。
- 当前权威记录改以 `mss-boot-docs#50`、`mss-boot-admin-antd#101` 和本记忆文件为准。

## 剩余队列

### 可继续治理的 Dependabot PR

`mss-boot`：

- `#391`：`k8s.io/api` `0.36.1 -> 0.36.2`
- `#390`：`github.com/go-playground/validator/v10` `10.30.2 -> 10.30.3`
- `#389`：`golang.org/x/crypto` `0.52.0 -> 0.53.0`
- `#388`：`github.com/aws/aws-sdk-go-v2/feature/rds/auth` `1.6.28 -> 1.6.29`
- `#387`：`github.com/go-sql-driver/mysql` `1.9.3 -> 1.10.0`

`mss-boot-admin`：

- `#402`：`k8s.io/client-go` `0.36.1 -> 0.36.2`
- `#401`：`golang.org/x/net` `0.55.0 -> 0.56.0`
- `#400`：`k8s.io/api` `0.36.1 -> 0.36.2`
- `#399`：`k8s.io/apimachinery` `0.36.1 -> 0.36.2`
- `#398`：`github.com/aws/aws-sdk-go-v2/service/s3` `1.103.2 -> 1.104.0`

建议下一轮按“同生态依赖先合并、每轮合并后等 main workflow 全绿”的顺序处理：

1. 先处理 `mss-boot-admin` 的 Kubernetes 三件套：`#399`、`#400`、`#402`。
2. 再处理 `mss-boot` 的 Kubernetes `#391`。
3. 然后处理低风险 patch：validator、AWS SDK、MySQL driver。
4. 每个仓库一轮合并后确认 `CI`/`CodeQL`/`govulncheck`/`Scorecard`/Mirror 成功，再继续下一组。

### 社区互动

- 当前没有新的外部贡献者 issue/PR/discussion 回复需要互动。
- `mss-boot-docs#45` 不应催促式追问；若作者再次更新，应优先检查是否修复了 PR Guard 失败和“Final”表述问题。
- Weekly Digest issue 暂不关闭；它们可作为社区透明记录，后续可以考虑自动加 `digest` 标签并按月归档。

## Good First Issue 候选

- `docs: add Dependabot PR merge playbook`
  - 内容：说明 Dependabot PR 的评估顺序、何时需要本地验证、何时可以直接依赖 GitHub checks、如何记录 main workflow 结果。
  - 适合标签：`documentation`、`good first issue`、`area/docs`、`area/devops`
- `docs: add weekly digest triage policy`
  - 内容：定义 Weekly Digest issue 是否保留、何时关闭、如何按月汇总。
  - 适合标签：`documentation`、`good first issue`、`area/devops`

## 外部社区草稿

### 中文短帖

mss-boot 本轮继续推进开源治理和供应链维护：四个核心仓库已统一将 GitHub Actions 的 `actions/checkout` 升级到 v7，涉及 `mss-boot`、`mss-boot-admin`、`mss-boot-admin-antd`、`mss-boot-docs`。所有变更均为 workflow-only，PR 检查和合并后的 main workflow 已全部通过。

接下来会继续处理 Go module 依赖升级，并推进 admin-antd 旧前端栈的安全迁移 RFC。欢迎对 Go/Kubernetes 依赖升级、CI 治理、前端工具链迁移有经验的朋友参与 review。

GitHub: https://github.com/mss-boot-io

### English Short Post

mss-boot continued its open-source maintenance work this week. All four core repositories have moved GitHub Actions workflows from `actions/checkout@v6` to `actions/checkout@v7`: `mss-boot`, `mss-boot-admin`, `mss-boot-admin-antd`, and `mss-boot-docs`.

These were workflow-only Dependabot updates. PR checks and post-merge main workflows are green across CI, CodeQL, Scorecard, docs deployment, and related governance jobs.

Next up: Go module dependency reviews and the admin-antd legacy frontend security migration RFC. Reviews around Go/Kubernetes dependencies, CI governance, and frontend toolchain migration are welcome.

GitHub: https://github.com/mss-boot-io
