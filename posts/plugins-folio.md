---
title: "plugins插件展示"
author: "Folio Team"
date: "2026-04-07T22:40:00+08:00"
tags: ["插件", "展示"]
draft: false
---

# Folio 插件说明

这篇文章详细介绍当前内置的两个插件：`music_player` 与 `private_posts`。

## 插件机制概览

- 插件目录：`plugins/<name>/`
- 最小结构：

```text
plugins/
└── <name>/
    ├── main.go
    └── plugin.json
```

- 若 `plugin.json` 未显式指定 `command/args`，默认执行：

```bash
go run ./plugins/<name>
```

- 启用方式（推荐）：

```jsonc
{
  "plugins": ["music_player", "private_posts"]
}
```

## 插件一：music_player

### 功能

- 在页面右侧注入悬浮音乐播放器。
- 支持 Turbo 页面切换时保持播放器状态。
- 支持拖拽位置与最小化。

### 触发 Hook

- `before_page_render`

### 静态资源

- 插件资源目录：`plugins/music_player/static/`
- 构建后映射到：`/static/plugins/music_player/`

### 默认配置（`plugins/music_player/plugin.json`）

```jsonc
{
  "name": "music_player",
  "hooks": ["before_page_render"],
  "timeout_ms": 5000,
  "fail_fast": true,
  "config": {
    "src": "/static/plugins/music_player/music.mp3",
    "title": "Now Playing",
    "volume": 0.8
  }
}
```

### 参数说明

- `src`：音频文件 URL。
- `title`：播放器标题文本。
- `volume`：初始音量（0 到 1）。

## 插件二：private_posts

### 功能

- 对 `front matter` 中 `private: true` 的文章进行加密处理（仅静态构建阶段）。
- 构建后会：
  - 生成 `post/<slug>/payload.json`（密文载荷）。
  - 用密码页覆盖 `post/<slug>/index.html`。
- 列表页可保留文章入口；访问详情页需输入密码。

### 触发 Hook

- `after_static_build`

### 文章 Front Matter 示例

```yaml
---
title: "Private Folio"
date: "2026-04-07T12:00:00Z"
private: true
---
```

### 默认配置（`plugins/private_posts/plugin.json`）

```jsonc
{
  "name": "private_posts",
  "hooks": ["after_static_build"],
  "timeout_ms": 30000,
  "fail_fast": true,
  "config": {
    "posts_dir": "posts",
    "password": "Your-Strong-Password",
    "kdf_iterations": 210000
  }
}
```

### 参数说明

- `posts_dir`：Markdown 文章目录。
- `password`：解密密码（当前固定从插件配置读取）。
- `kdf_iterations`：PBKDF2 迭代次数，越高越抗暴力破解，但解锁更慢。

### 演示

- 可直接点击首页文章 `private-folio` 进入密码页。
- 输入"Folio-Private-2026-Access-Key"(默认密码)即可解锁内容。

### 安全边界

- 这是“静态前端解密”方案，不等同后端鉴权。
- 明文不会直接出现在私密文章页面源码中，但攻击者可离线尝试猜密码。
- 建议使用高熵长随机密码，并保持较高 KDF 参数。
