# Folio

一个基于 Go + 文件系统的轻量静态博客，支持发布到 GitHub Pages。

演示地址：<https://wofiporia.github.io/folio/>

## 分支与发布

- `main`：模板分支（不自动发布）
- `blog`：内容分支（自动发布到 GitHub Pages）

推荐流程：使用 `Use this template` 创建仓库，然后在 `blog` 分支写作并发布。

## 快速开始

1. 使用模板创建你的仓库。
2. 新建并切换到 `blog` 分支。
3. 在仓库 `Settings -> Pages` 中把 `Source` 设为 `GitHub Actions`。
4. 在 `Settings -> Environments -> github-pages` 里允许 `blog` 分支部署。
5. push 到 `blog`，等待 Actions 完成。

## 配置文件（`config.json`）

示例：

```jsonc
{
  "site_title": "Folio",
  "site_description": "一个基于 Go 和文件系统的轻量博客。",
  "site_url": "https://your-name.github.io/your-repo",
  "author_name": "Your Name",
  "author_github": "your-github-id",
  "theme": "default",
  "default_description": "这里什么都没有写。",
  "default_og_image": "",
  "default_og_type": "website",

  "comments_provider": "utterances",
  "comments_repo": "owner/repo",
  "comments_issue_term": "pathname",
  "comments_label": "comment",
  "comments_theme": "github-light"
}
```

主要字段：

- `site_title`：站点名。
- `site_description`：站点描述（首页与默认 SEO 描述）。
- `site_url`：站点完整 URL（用于 canonical / og:url）。
- `author_name`：文章未写作者时的默认作者名。
- `author_github`：作者 GitHub 地址（支持直接写用户名，程序会自动补全为 `https://github.com/<name>`）。
- `theme`：主题名（对应 `themes/<theme>`）。
- `default_description`：缺省 SEO 描述。
- `default_og_image`：缺省 OG 图片 URL。
- `default_og_type`：缺省 OG 类型。

评论字段：

- `comments_provider`：`utterances` 或 `giscus`。
- 若配置不完整，评论区会自动关闭，不影响页面渲染。

## 主题

当前内置主题：

- `default`：简约风。
- `kinetic`：更大胆的视觉与排版。

主题目录约定：

```text
themes/
└── <theme>/
    ├── templates/
    │   ├── index.html
    │   ├── post.html
    │   ├── tags.html
    │   ├── archives.html
    │   ├── search.html
    │   ├── 404.html
    │   └── partials/
    │       ├── head-common.html
    │       └── nav.html
    └── static/
        ├── style.css
        └── favicon.png
```

说明：模板和静态资源都支持自动回退到 `themes/default`。

## 写作

文章放在 `posts/*.md`，使用 Front Matter：

```markdown
---
title: "我的第一篇文章"
author: "Your Name"
date: "2026-03-03T10:00:00Z"
tags: ["博客", "Go"]
draft: false
---
```

- `author` 可选；不填时回退到 `config.json` 的 `author_name`。
- `draft: true` 的文章不会出现在前台。

## 页面与功能

- 首页：`/`
- 文章页：`/post/{slug}`
- 标签页：`/tags`
- 归档页：`/archives`
- 搜索页：`/search`（前端读取 `search-index.json`）
- SEO：`description`、Open Graph、`canonical`、`article:published_time`

## 本地开发

启动本地服务：

```bash
go run .
```

访问：`http://localhost:8080`

导出静态站点：

```bash
go run ./cmd/build -out dist -base-path /your-repo-name
```

可选参数：

- `-config`：指定配置文件路径（默认 `config.json`）
- `-site-url`：导出时覆盖站点 URL

## GitHub Pages Base Path

可在仓库变量中设置 `PAGES_BASE_PATH`（`Settings -> Secrets and variables -> Actions -> Variables`）。

常见值：

- 项目页：`/your-repo-name`
- 用户主页根路径：`/`
- 自定义子路径：`/blog`

优先级：

1. `PAGES_BASE_PATH`
2. 自动回退 `/<repo-name>`

## 项目结构

```text
folio/
├── main.go
├── cmd/build/main.go
├── internal/folio/
│   ├── folio.go
│   ├── view.go
│   ├── routing.go
│   └── static_build.go
├── config.json
├── posts/
├── test/
├── themes/
├── .github/workflows/pages.yml
└── README.md
```
