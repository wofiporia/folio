# Folio

一个基于 Go + 文件系统的轻量博客，支持自动发布到 GitHub Pages。  
演示地址：https://wofiporia.github.io/folio/

## 分支策略

- `main`：公开模板分支（不自动发布）
- `blog`：个人内容分支（自动发布到 GitHub Pages）

当前工作流仅监听 `blog`，避免 `main` 覆盖线上页面。

## 使用方式

推荐优先使用 **Template**，不建议直接 Fork。

### 方案 A：Use this template（推荐）

1. 点击仓库右上角 `Use this template` 创建你自己的仓库。
2. 在新仓库中创建并切换到 `blog` 分支（首次可从 `main` 创建）。
3. 在仓库 `Settings -> Pages` 中将 `Source` 设为 `GitHub Actions`。
4. 进入 `Settings -> Environments -> github-pages`，在 `Deployment branches` 中添加并允许 `blog` 分支（若界面有 `All branches` 也可直接选它）。
5. 确保 `Actions` 已启用，向 `blog` push 一次（或手动触发 workflow）。
6. 等待 `Deploy Pages` 成功后访问你的站点。

### 方案 B：Fork（可用）

1. 点击 `Fork` 到你的账号。
2. 在 fork 仓库创建 `blog` 分支。
3. 在 fork 仓库进入 `Settings -> Pages`，`Source` 选择 `GitHub Actions`。
4. 进入 `Settings -> Environments -> github-pages`，在 `Deployment branches` 中添加并允许 `blog` 分支（若界面有 `All branches` 也可直接选它）。
5. 确保 `Actions` 已启用，向 `blog` push 一次（或手动触发 workflow）。
6. 等待 `Deploy Pages` 成功后访问你的站点。

## 配置项：`PAGES_BASE_PATH`

在仓库 `Settings -> Secrets and variables -> Actions -> Variables` 中新增变量：

- 变量名：`PAGES_BASE_PATH`
- 示例值：
  - 项目页（默认）：`/your-repo-name`
  - 用户主页根路径：`/`
  - 自定义子路径：`/blog`

优先级：

1. `PAGES_BASE_PATH`（若设置）
2. 自动回退 `/<repo-name>`

## 内容发布方式

所有内容来自 `posts/*.md`，通过 Git 提交发布：

1. 切到 `blog` 分支
2. 在 `posts/` 新建或编辑 Markdown 文件
3. 提交并 push 到 `blog`
4. GitHub Actions 自动构建并发布

Front Matter 示例：

```markdown
---
title: "我的第一篇文章"
date: "2026-03-03T10:00:00Z"
tags: ["博客", "Go"]
draft: false
---

# 欢迎使用 Folio

这是正文内容。
```

## 技术栈

| 组件 | 选型 | 说明 |
| --- | --- | --- |
| 后端 | Go | 单文件部署，性能稳定 |
| HTTP | `net/http` | 标准库，无额外依赖 |
| 模板 | `html/template` | 服务端模板渲染，默认安全转义 |
| 内容 | Markdown + YAML Front Matter | 易写作、易版本管理 |
| 存储 | 文件系统（`posts/*.md`） | 透明、易备份、易迁移 |
| 前端 | 原生 HTML/CSS/JavaScript | 无构建链路 |

## 功能

- 首页：`/`
- 文章页：`/post/{slug}`
- 标签页：`/tags`
- 归档页：`/archives`
- 搜索页：`/search`（前端读取 `search-index.json`）
- 草稿过滤：`draft: true` 不在前台展示
- Markdown 渲染增强：标题、段落、代码块、列表、引用、链接、粗体、斜体
- SEO 元信息：`description`、Open Graph、`canonical`、文章发布时间标签

## 本地开发（可选）

```bash
go run .
```

本地访问：`http://localhost:8080`

静态导出（与 Pages 构建一致）：

```bash
go run ./cmd/build -out dist -base-path /your-repo-name
```


## 项目结构

```text
folio/
├── main.go
├── cmd/build/main.go
├── posts/
├── static/
├── templates/
├── .github/workflows/pages.yml
└── README.md
```
