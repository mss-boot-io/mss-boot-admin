# 2026-06-05 README 可信信号修复记忆

## 背景

`mss-boot-admin-antd` README 顶部 badge 指向 `mss-boot-admin` 后端仓库，外部贡献者看到的 CI、Release、License 信号不准确。

## 处理

- 将英文和中文 README badge 改为当前前端仓库的 CI、CodeQL、OpenSSF Scorecard、Release、License。
- 增加 Contributing、Security、Support、Good first issue、AI memory 入口。

## 验证

- `git diff --check`

## 后续

其他核心仓库 README 也应保持同样的社区入口和 badge 口径。
