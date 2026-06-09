# GitHub Actions 执行记录保留策略

## 目标

定期清理 GitHub Actions 历史执行记录，降低 Actions 页面噪音和长期存储压力，同时保留近期排错和发布审计所需的记录。

## 采用方案

新增独立工作流 `.github/workflows/cleanup-workflow-runs.yml`：

- 触发方式：每周一 03:17 UTC 自动执行，并支持 `workflow_dispatch` 手动执行。
- 权限范围：仅授予 `actions: write` 和 `contents: read`。
- 清理对象：只处理 `completed` 状态的 workflow runs，不处理 queued、in_progress、requested、waiting 等运行中或等待中的记录。
- 默认保留策略：删除 90 天以前的 completed 记录，但每个 workflow 至少保留最近 20 次 completed 记录。
- 保护范围：默认不清理 `Build with release`、`docker image build`、`Deploy Docs to GitHub Pages` 这些发布、镜像和部署类 workflow。
- 手动验证：手动触发时默认 `dry_run=true`，用于预览将删除的记录；定时触发默认实际删除。

## 策略理由

- 90 天窗口覆盖常见的问题追踪和回归定位周期。
- 每个 workflow 至少保留 20 次，避免低频发布、文档部署等工作流因超过天数而失去全部历史。
- 发布、镜像和 Pages 部署记录具有更高审计价值，默认排除在自动删除范围之外。
- 周期选择避开整点，降低 GitHub Actions 定时任务在高峰期延迟或丢弃的概率。
- 使用 GitHub REST API 直接删除 workflow run，避免引入第三方清理 Action 作为供应链依赖。

## 调整方式

在 Actions 页面手动运行 `Cleanup workflow runs` 时可以调整：

- `retention_days`：按天数保留，默认 `90`。
- `min_runs_per_workflow`：每个 workflow 至少保留的最近执行次数，默认 `20`。
- `dry_run`：是否只预览不删除，默认 `true`。
- `protected_workflows`：逗号分隔的 workflow 名称列表，匹配的 workflow 不会被自动删除。

如需改变定时清理策略，修改 `.github/workflows/cleanup-workflow-runs.yml` 中的 `RETENTION_DAYS`、`MIN_RUNS_PER_WORKFLOW` 默认值或 cron 表达式。

## 已知限制

GitHub REST API 在按条件列出 workflow runs 时可能存在结果上限。当前脚本按 `completed` 状态分页扫描，适合本仓库当前规模；如果历史执行记录累计超过 API 查询上限，最旧的一部分记录可能需要改成按时间窗口分片清理。该限制会导致漏删，不会扩大删除范围。

## 参考依据

- GitHub REST API 支持列出和删除 workflow runs。
- 删除 workflow run 需要 Actions 写权限。
- `schedule` 事件只在默认分支生效，且 GitHub 建议避开整点高峰。
