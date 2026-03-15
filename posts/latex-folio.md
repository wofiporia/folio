---
title: "LaTeX 展示"
author: "Folio Team" 
date: "2026-03-14T09:00:00Z"
tags: ["LaTeX", "Math", "展示"]
draft: false
---

# LaTeX 展示

这篇文章用于展示在 Folio 中常见的 LaTeX 公式写法，方便直接复制使用。

## 1. 行内与块级公式

行内公式示例：\(\sum_{i=1}^{n} a_i\)

块级公式（`$$...$$`）：

$$
\sum_{i=1}^{n} i = \frac{n(n+1)}{2}
$$

块级公式（`\[...\]`）：

\[
\int_{0}^{1} x^2\,dx = \frac{1}{3}
\]

## 2. 上下标、分数、根号

\[
x^2,\quad x^{n+1},\quad a_i,\quad a_{i,j}
\]

\[
\frac{a+b}{c+d},\quad \dfrac{1}{1+x},\quad \sqrt{x},\quad \sqrt[n]{x}
\]

## 3. 求和、积分、极限

\[
\sum_{k=1}^{n} k,\quad \prod_{k=1}^{n} k
\]

\[
\int_a^b f(x)\,dx,\quad \iint_D f(x,y)\,dA
\]

\[
\lim_{x\to 0}\frac{\sin x}{x}=1
\]

## 4. 矩阵与向量

\[
A=
\begin{bmatrix}
1 & 2 \\
3 & 4
\end{bmatrix}
\]

\[
\vec{v}=\begin{bmatrix}x\\y\\z\end{bmatrix},\quad
\|\vec{v}\|=\sqrt{x^2+y^2+z^2}
\]

## 5. 分段函数

\[
f(x)=
\begin{cases}
x^2, & x\ge 0 \\
-x, & x<0
\end{cases}
\]

## 6. 对齐公式

\[
\begin{aligned}
(a+b)^2 &= a^2 + 2ab + b^2 \\
(a-b)^2 &= a^2 - 2ab + b^2
\end{aligned}
\]

## 7. 编号公式

\begin{equation}
E = mc^2
\label{eq:emc2}
\end{equation}

引用示例：公式 \(\ref{eq:emc2}\)。

## 8. 概率与统计符号

\[
P(A),\ P(A\mid B),\ \mathbb{E}[X],\ \mathrm{Var}(X)
\]

\[
X\sim \mathcal{N}(\mu,\sigma^2)
\]

## 9. 希腊字母

\[
\alpha,\beta,\gamma,\delta,\epsilon,\theta,\lambda,\mu,\pi,\sigma,\phi,\psi,\omega
\]

\[
\Gamma,\Delta,\Theta,\Lambda,\Pi,\Sigma,\Phi,\Psi,\Omega
\]

## 10. 常见问题

1. 公式显示成普通文本：确认文章页已加载 MathJax 脚本。
2. 公式不完整：检查 `{}`、`[]`、`()` 是否成对。
3. 复杂公式建议使用块级写法，避免被 Markdown 语法干扰。
