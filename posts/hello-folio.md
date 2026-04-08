---
title: "欢迎使用 Folio"
author: "Folio Team"
date: "2026-03-03T08:00:00Z"
tags: ["博客", "Go", "Folio"]
draft: false
---

# 欢迎使用 Folio

这是一篇示例文章，用来演示当前博客的基础写作格式。

## 你可以从这里开始

复制这篇文章后，按需修改 Front Matter：

- `title`：文章标题
- `author`：作者（可选；不填会回退到 `config.json` 的 `author_name`）
- `date`：发布时间（建议 RFC3339）
- `tags`：标签数组
- `draft`：是否草稿（`true` 不会在前台显示）

## Front Matter 示例

```yaml
---
title: "我的第一篇文章"
author: "Wofiporia"
date: "2026-03-03T10:00:00Z"
tags: ["博客", "随笔"]
draft: false
---
```

## 正文示例

Folio 当前支持基础 Markdown 渲染，包括标题、段落、列表、引用、代码块和链接等。

更多语法可以看：[grammar语法展示](../../post/grammar-folio/)

> 这是一个引用块示例。

```go
package main

import "fmt"

func main() {
    fmt.Println("Hello, Folio")
}
```

## 下一步建议

1. 继续在 `posts/` 新增你的文章
2. 提交并 push 到 `blog` 分支触发自动发布
3. 在 GitHub Pages 检查线上效果
