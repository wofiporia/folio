# Folio

一个基于 Go + 文件系统的轻量博客，支持自动发布到 GitHub Pages。  
演示地址：https://wofiporia.github.io/folio/

## 分支策略

- `main`：公开模板分支（不自动发布）
- `blog`：个人内容分支（自动发布到 GitHub Pages）

## 使用方式

推荐优先使用 **Template**，不建议直接 Fork。

### 方案 A：Use this template（推荐）

1. 点击仓库右上角 `Use this template` 创建你自己的仓库。
2. 在新仓库中创建并切换到 `blog` 分支（首次可从 `main` 创建）。
3. 在仓库 `Settings -> Pages` 中将 `Source` 设为 `GitHub Actions`。
4. 进入 `Settings -> Environments -> github-pages`，在 `Deployment branches` 中添加并允许 `blog` 分支。
5. 向 `blog` push 一次（或手动触发 workflow）。
6. 等待 `Deploy Pages` 成功后访问你的站点。

### 方案 B：Fork（可用）

1. 点击 `Fork` 到你的账号。
2. 在 fork 仓库创建 `blog` 分支。
3. 在 fork 仓库进入 `Settings -> Pages`，`Source` 选择 `GitHub Actions`。
4. 进入 `Settings -> Environments -> github-pages`，在 `Deployment branches` 中添加并允许 `blog` 分支。
5. 向 `blog` push 一次（或手动触发 workflow）。
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

## 站点自定义配置（`config.json`）

### 示例

```jsonc
{
  "site_title": "Folio", // 站点名称（用于页面标题和 SEO site_name）
  "site_description": "一个基于 Go 和文件系统的轻量博客。", // 站点简介（默认用于 SEO 描述）
  "site_url": "https://wofiporia.github.io/folio", // 站点完整 URL（用于 canonical/og:url）
  "author_name": "Wofiporia", // 默认作者名（文章未写 author 时回退）
  "theme": "default", // 主题名（对应 themes/<theme>/）
  "default_description": "这里什么都没有写", // SEO 默认描述（页面/文章无描述时使用）
  "default_og_image": "", // Open Graph 默认图片 URL（可留空）
  "default_og_type": "website" // Open Graph 默认类型（如 website / article）
}
```

### 字段说明

- `site_title`：站点名称。
- `site_description`：站点描述，会显示在首页标题下方，同时作为站点级 SEO 描述默认值。
- `site_url`：站点完整地址（必须带协议），用于生成绝对 `canonical` 与 `og:url`。
- `author_name`：默认作者名，文章 Front Matter 未设置 `author` 时使用。
- `theme`：前端主题名。程序会优先读取 `themes/<theme>/templates/*` 与 `themes/<theme>/static/*`，并自动回退到 `themes/default`。
- `default_description`：全站 SEO 默认描述，页面没有更具体描述时使用。
- `default_og_image`：全站分享默认封面图 URL（可选）。
- `default_og_type`：Open Graph 默认类型，常见 `website`。

### 主题目录约定

```text
themes/
└── <theme-name>/
    ├── templates/
    │   ├── index.html
    │   ├── post.html
    │   ├── tags.html
    │   ├── archives.html
    │   ├── search.html
    │   └── partials/
    │       ├── head-common.html
    │       └── nav.html
    └── static/
        ├── style.css
        └── favicon.png
```

- 默认主题：`themes/default`
- 切换主题：修改 `config.json` 中 `theme` 字段并重启服务（或重新导出静态站点）
- 示例主题：`default`（简洁） / `kinetic`（更大胆的视觉与排版）

### 图标自定义

- 直接替换 `themes/<theme>/static/favicon.png` 即可。
- 若当前主题未提供，会回退到 `themes/default/static/favicon.png`。

## 内容发布方式

所有内容来自 `posts/*.md`，通过 Git 提交发布：

1. 切到 `blog` 分支
2. 在 `posts/` 新建或编辑 Markdown 文件
3. 提交并 push 到 `blog`
4. GitHub Actions 自动构建并发布

文章 Front Matter 示例：

```markdown
---
title: "我的第一篇文章"
author: "Wofiporia"
date: "2026-03-03T10:00:00Z"
tags: ["博客", "Go"]
draft: false
---
```

说明：`author` 可选；未填写时回退到 `config.json` 的 `author_name`。

## 功能

- 首页：`/`
- 文章页：`/post/{slug}`
- 标签页：`/tags`
- 归档页：`/archives`
- 搜索页：`/search`（前端读取 `search-index.json`）
- 草稿过滤：`draft: true` 不在前台展示
- Markdown 渲染增强：标题、段落、代码块、列表、引用、链接、粗体、斜体
- SEO 元信息：`description`、Open Graph、`canonical`、`article:published_time`

## 本地开发（可选）

```bash
go run .
```

本地访问：`http://localhost:8080`

静态导出（与 Pages 构建一致）：

```bash
go run ./cmd/build -out dist -base-path /your-repo-name
```

如需在导出时覆盖站点 URL（优先级高于 `config.json`）：

```bash
go run ./cmd/build -out dist -base-path /your-repo-name -site-url https://example.com
```

## 项目结构

```text
folio/
├── main.go
├── cmd/build/main.go
├── config.json
├── posts/
├── themes/
├── .github/workflows/pages.yml
└── README.md
```

