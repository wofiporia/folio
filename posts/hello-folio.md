---
title: "欢迎使用 Folio"
date: "2026-03-03T08:00:00Z"
tags: ["博客", "Go", "Folio"]
draft: false
---

# 欢迎使用 Folio

这是一篇示例文章，用来演示 Folio 的默认写作格式。

## 你可以从这里开始

你可以直接复制这篇文章，然后修改：

- `title`：文章标题
- `date`：发布时间（建议 RFC3339）
- `tags`：标签数组
- `draft`：是否草稿（`true` 不会在前台显示）

## Front Matter 示例

```yaml
---
title: "我的第一篇文章"
date: "2026-03-03T10:00:00Z"
tags: ["博客", "随笔"]
draft: false
---
```

## 正文示例

Folio 当前支持基础 Markdown 渲染（标题、段落、代码块）。

```go
package main

import "fmt"

func main() {
    fmt.Println("Hello, Folio")
}
```

## 下一步建议

1. 在 `posts/` 继续添加你的文章
2. 提交到 `blog` 分支触发自动发布
3. 访问 GitHub Pages 查看效果
