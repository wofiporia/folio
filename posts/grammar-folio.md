---
title: "grammar语法展示"
author: "Folio Team"
date: "2026-04-08T14:00:00+08:00"
tags: ["语法", "展示"]
draft: false
---

[TOC]

# Grammar Folio

这篇文章展示了常用 Markdown 语法的效果。

---

## 1. 标题与段落

### 1.1 小节标题

普通段落文本示例。

---

## 2. 强调与行内语法

- **粗体**
- *斜体*
- ~~删除线~~
- `行内代码`

---

## 3. 链接

- 内部链接：[返回首页](../../)
- 外部链接（带 title）：[GitHub](https://github.com "Open GitHub")

---

## 4. 图片

推荐放置目录：

```text
themes/default/static/images/
```

当前仓库示例图片：

```text
themes/default/static/images/wofiporia.jpg
```

文章内推荐引用：

```md
![wofiporia](../../static/images/wofiporia.jpg "头像示例")
```

渲染效果：

![wofiporia](../../static/images/wofiporia.jpg "头像示例")

---

## 5. 引用

> 这是一个引用块示例。

---

## 6. 列表

### 6.1 无序列表（含嵌套）

- 一级 A
  - 二级 A-1
    - 三级 A-1-a
- 一级 B

### 6.2 有序列表（含嵌套）

1. 第一步
  1. 子步骤 1
  2. 子步骤 2
2. 第二步

### 6.3 任务列表

- [x] 已完成事项
- [ ] 待办事项

---

## 7. 表格

| 语法 | 示例 | 说明 |
| --- | --- | --- |
| 粗体 | `**text**` | 强调文本 |
| 删除线 | `~~text~~` | 标记废弃内容 |
| 图片 | `![alt](url)` | 渲染图片 |

---

## 8. 代码块

```go
package main

import "fmt"

func main() {
    fmt.Println("hello grammar-folio")
}
```

---

## 9. 脚注

这是一个脚注示例[^a]，这里再引用一次同一个脚注[^a]。

[^a]: 这是脚注内容，位于文章末尾自动汇总。

