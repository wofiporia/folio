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
- 已启用 Turbo 导航（同源页面切换默认无整页刷新）

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

## 开发检查

安装并确认 Go 版本：

```bash
go version
```

安装 `golangci-lint`：

```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

运行开发检查：

```bash
make test
```

## 插件系统（实验性）

Folio 支持通过外部进程插件扩展构建/内容流程，协议为 `stdin/stdout` 的 JSON。

当前可用 Hook：

- `after_posts_loaded`：文章加载完成后，可返回新文章列表。
- `after_static_build`：静态构建完成后，可生成额外产物。
- `before_page_render`：页面渲染前，可注入额外 head 片段（如脚本/样式）。

插件目录约定（即插即用）：

```text
plugins/
└── <name>/
    ├── main.go
    └── plugin.json
```

当 `plugin.json` 未显式配置 `command/args` 时，默认自动执行：

```bash
go run ./plugins/<name>
```

最简启用方式（只写插件名）：

```jsonc
{
  "plugins": ["music_player"]
}
```

支持两种写法：

- 字符串数组：`["music_player"]`（推荐，最简）。
- 对象数组：用于覆盖插件参数（`timeout_ms`、`fail_fast`、`config` 等）。

内置/示例插件：

- `music_player`：`before_page_render` 阶段注入页面右下角悬浮播放器（默认播放 `{{.BasePath}}/static/plugins/music_player/music.mp3`）。
  - 插件静态资源目录：`plugins/music_player/static/`（会映射到 `/static/plugins/music_player/`）。
  - 支持拖拽移动位置。
  - 支持最小化与隐藏。

`music_player` 高级配置示例：

```jsonc
{
  "plugins": [
    {
      "name": "music_player",
      "config": {
        "src": "/static/plugins/music_player/music.mp3",
        "title": "Now Playing",
        "volume": 0.8
      }
    }
  ]
}
```

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
