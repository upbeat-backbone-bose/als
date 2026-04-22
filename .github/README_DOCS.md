# GitHub Pages 文档部署指南

## 快速开始

### 1. 启用 GitHub Pages

1. 访问：https://github.com/upbeat-backbone-bose/als/settings/pages
2. **Source** 选择：**GitHub Actions**
3. 保存设置

### 2. 推送文档

```bash
cd /workspace

# 添加文档
git add docs/ .github/workflows/docs.yml

# 提交
git commit -m "docs: 添加项目文档"

# 推送（自动部署）
git push origin master
```

### 3. 查看部署

1. Actions → **Deploy Docs to GitHub Pages**
2. 等待完成（1-2 分钟）
3. 访问：`https://upbeat-backbone-bose.github.io/als/`

## 目录结构

```
als/
├── docs/                          # 文档目录（提交到 git）
│   ├── README.md                  # 文档首页
│   ├── ARCHITECTURE.md            # 系统架构
│   ├── INTERFACES.md              # 接口文档
│   ├── DEVELOPER_GUIDE.md         # 开发者指南
│   ├── mkdocs.yml                 # MkDocs 配置（可选）
│   ├── 专有概念/
│   └── 模块/
└── .github/workflows/
    └── docs.yml                   # 自动部署工作流
```

## 更新文档

```bash
# 编辑文档
vim docs/ARCHITECTURE.md

# 提交并推送
git add docs/
git commit -m "docs: 更新文档"
git push origin master
```

## 说明

- **自动部署**: 推送到 master 且修改 `docs/` 时自动触发
- **部署地址**: `https://upbeat-backbone-bose.github.io/als/`
- **文档格式**: Markdown 或 MkDocs（推荐）

## 本地预览（可选）

```bash
cd docs
pip install mkdocs mkdocs-material
mkdocs serve
```

访问：http://127.0.0.1:8000

---

详细指南：[docs/DEPLOYMENT_GUIDE.md](docs/DEPLOYMENT_GUIDE.md)
