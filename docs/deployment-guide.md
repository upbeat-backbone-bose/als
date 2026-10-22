# 文档发布指南

**最后更新**: 2026-10-22

本文档说明如何将项目文档发布到 GitHub Pages。

## 目录结构

```
als/
├── docs/                      # GitHub Pages 文档目录（14 份 .md）
│   ├── index.md               # 文档索引（站点首页入口）
│   ├── architecture.md        # 系统架构
│   ├── interfaces.md          # 接口文档
│   ├── developer-guide.md     # 开发者指南
│   ├── deployment-guide.md    # 文档发布指南（本文）
│   ├── backend.md / ui.md / config.md / console.md / fakeshell.md   # 模块文档
│   ├── session.md / feature-flags.md                            # 概念文档
│   ├── backend-perf.md        # 后端性能基线
│   └── actions-retention.md   # Actions 执行记录保留策略
├── mkdocs.yml                 # MkDocs 配置（位于仓库根目录）
├── requirements-dev.txt       # 文档构建依赖
├── scripts/                   # 脚本
└── .github/workflows/
    └── docs.yml               # GitHub Actions 工作流
```

> `docs/` 是文档的**唯一事实来源**，仓库中不存在其它「源文档目录」（也没有 `.monkeycode/docs/`）。

## 工作流程

### 自动发布

当满足以下条件时，文档会自动发布：

1. **推送到 master 分支**
   ```bash
   git push origin master
   ```

2. **修改了会触发构建的文件**（`.github/workflows/docs.yml` 的 `on.push.paths`）
   - `docs/**`
   - `mkdocs.yml`
   - `requirements-dev.txt`
   - `.github/workflows/docs.yml`

3. **GitHub Actions 自动执行**
   - 检出代码
   - 安装 Python 3.11 与 `requirements-dev.txt` 中的 MkDocs 依赖
   - `mkdocs build` 生成静态站点
   - 部署到 GitHub Pages

### 手动触发

1. 进入 GitHub 仓库页面
2. 点击 **Actions** 标签
3. 选择 **Deploy Docs to GitHub Pages** 工作流
4. 点击 **Run workflow** 按钮
5. 选择分支（通常是 `master`）
6. 点击 **Run workflow**

## GitHub Pages 配置

### 1. 启用 GitHub Pages

1. 进入仓库 **Settings**
2. 点击左侧 **Pages**
3. 在 **Source** 下选择：
   - **Deploy from a branch**
   - Branch: `gh-pages` / `(root)`
4. 点击 **Save**

### 2. 访问文档

部署成功后，文档将在以下地址可用：

```
https://<username>.github.io/<repository>/
```

例如：
```
https://upbeat-backbone-bose.github.io/als/
```

## 更新文档

### 方式一：直接修改 docs 目录

```bash
# 1. 编辑文档
vim docs/architecture.md

# 2. 提交更改
git add docs/
git commit -m "docs: 更新架构文档"

# 3. 推送触发自动部署
git push origin master
```

### 方式二：使用 AI 辅助生成文档

```bash
# 使用项目 Wiki skill 生成/更新文档
/project-wiki

# 把生成结果落到 docs 目录
cp -r <生成目录>/* docs/
git add docs/
git commit -m "docs: 同步 AI 生成的文档"
git push origin master
```

> 仓库中**不存在** `.monkeycode/docs/` 之类的独立源文档目录，文档直接维护在 `docs/` 下。

## MkDocs 主题（可选）

### 安装 MkDocs

```bash
# 本地开发时安装
pip install mkdocs mkdocs-material pymdown-extensions
```

### 本地预览

```bash
cd docs
mkdocs serve
```

访问：http://127.0.0.1:8000

### 构建

```bash
cd docs
mkdocs build
```

输出目录：`docs/site/`

## 自定义配置

### 修改 mkdocs.yml

```yaml
site_name: 您的站点名称
site_description: 站点描述
repo_url: https://github.com/your-username/your-repo

theme:
  name: material
  palette:
    - scheme: default  # 浅色模式
    - scheme: slate    # 深色模式

nav:
  - 首页：README.md
  - 文档名称：文件路径.md
```

### 添加自定义域名

1. 在 `docs/` 目录下创建 `CNAME` 文件
2. 内容为您的域名：
   ```
   docs.example.com
   ```
3. 提交并推送：
   ```bash
   echo "docs.example.com" > docs/CNAME
   git add docs/CNAME
   git commit -m "docs: 添加自定义域名"
   git push origin master
   ```
4. 在 DNS 提供商处配置 CNAME 记录

## 故障排查

### 问题 1: 部署失败

**检查 GitHub Actions 日志**：
1. 进入 **Actions** 标签
2. 点击失败的工作流
3. 查看错误信息

**常见问题**：
- Python 版本不匹配
- MkDocs 配置错误
- 文件路径错误

### 问题 2: 页面显示 404

**解决方法**：
1. 确认 GitHub Pages 已正确配置
2. 等待几分钟（部署需要时间）
3. 检查 `_config.yml` 或 `mkdocs.yml` 配置

### 问题 3: 文档未更新

**检查点**：
- 文件路径是否正确
- 是否在 master 分支
- GitHub Actions 是否成功运行
- 浏览器缓存（强制刷新 Ctrl+F5）

## 最佳实践

### 1. 文档组织

```
docs/
├── README.md           # 首页和导航
├── 核心文档/
│   ├── 架构.md
│   └── API.md
├── 指南/
│   ├── 快速开始.md
│   └── 教程.md
└── 参考/
    ├── 配置.md
    └── 命令.md
```

### 2. 提交信息规范

```bash
# 文档新增
git commit -m "docs: 添加 API 文档"

# 文档更新
git commit -m "docs: 更新架构说明"

# 文档修复
git commit -m "docs: 修复拼写错误"

# 文档重构
git commit -m "docs: 重组文档结构"
```

### 3. 版本控制

对于重要版本文档：

```bash
# 创建版本文档目录
mkdir -p docs/v1.0
cp -r docs/* docs/v1.0/

# 或在文档顶部添加版本信息
---
version: 1.0
date: 2026-04-22
---
```

> （以上为**示例**，非本文档的实际版本信息；本文档的实际标记是文件顶部的「最后更新」。）

## 相关资源

- [GitHub Pages 官方文档](https://docs.github.com/en/pages)
- [MkDocs 文档](https://www.mkdocs.org/)
- [Material for MkDocs](https://squidfunk.github.io/mkdocs-material/)
- [GitHub Actions 文档](https://docs.github.com/en/actions)

## 联系支持

如有问题，请：
1. 查看 [GitHub Issues](https://github.com/upbeat-backbone-bose/als/issues)
2. 提交新的 Issue 描述您的问题
