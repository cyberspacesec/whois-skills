# 📜 更新日志

> 📖 whois-skills 更新日志，基于 Git 提交历史整理。当前为初始版本。

---

## 📋 概览

| 项目 | 内容 |
|------|------|
| 当前版本 | `0.1.0`（初始版本） |
| 首次提交 | `8f02394` init |
| 最新提交 | `5e0f8ee` Improve .gitignore and sync submodules |
| 协议 | MIT |

---

## 🗓️ 版本历史

### v0.1.0 — 初始版本

**提交记录**（由旧到新）：

| Commit | 说明 | 日期 |
|--------|------|------|
| `8f02394` | init — 项目初始化，包含全部核心模块与功能 | 2026/07 |
| `17c1ed0` | docs: 添加 Go 客户端库使用文档，包括基本用法和高级特性示例 | 2026/07 |
| `5e0f8ee` | Improve .gitignore and sync submodules — 改进 .gitignore 并同步子模块 | 2026/07 |

---

## ✅ 已实现功能清单

### 🔎 whois 模块（`pkg/whois`，24 源文件）

**查询能力**：

- 域名 WHOIS 查询（重试、错误分类、结果校验、优先级队列聚合）
- IP WHOIS 查询 + 5 大 RIR 响应解析（ARIN/RIPE/APNIC/LACNIC/AFRINIC）
- ASN 查询（RADB + RDAP + BGP 增强）
- RDAP 查询（域名/IP/ASN，bootstrap）
- 域名可用性检查
- 反向 WHOIS（Provider 接口）
- WHOIS 服务器管理与健康检查

**解析处理**：

- 错误分类与可重试判断
- IDN/Punycode 转换与域名规范化
- WHOIS 信息对比（diff）
- 数据质量评估与隐私检测
- 原始响应格式检测与格式化
- 多域名关联分析与图谱
- 导出 JSON/CSV/Markdown

**工程化**：

- 流式批量查询与断点续传
- 本地/Redis 缓存与预热
- 代理池与自定义拨号
- 令牌桶限速器
- 智能调度与自适应限速

**情报分析**：

- 域名监控与到期告警

**配置与可观测**：

- 库级配置 + YAML 加载
- 指标提供者（Prometheus/OpenTelemetry 复合）

### 🌐 api 模块（`pkg/api`，3 源文件）

- 16+ HTTP 端点（WHOIS/IP/ASN/RDAP/批量/格式化/导出/IDN/系统）
- 中间件链（Recovery/Logging/CORS/Auth）
- 批量查询会话管理
- MCP 路由集成

### 🤖 mcp 模块（`pkg/mcp`，3 源文件）

- Request/Task 状态机（pending → done → approved）
- 10 个 MCP HTTP 端点
- Controller 双能力（任务管理 + WHOIS 查询封装）

### 📊 metrics 模块（`pkg/metrics`，4 源文件）

- 四类指标采集（API/WHOIS/缓存/系统）
- 4 条默认告警规则（CPU/内存/API 错误率/WHOIS 失败率）
- Email/Slack/Webhook 三种通知器
- 基于 gopsutil 的系统指标

### ⏱️ monitor 模块（`pkg/monitor`，1 源文件）

- WHOIS 查询性能监控
- P90/P95/P99 百分位延迟
- 装饰器模式 `WithPerformanceMonitoring`

### 🔐 security 模块（`pkg/security`，3 源文件 + config）

- API Key 生成/验证/权限/速率限制
- `AuthMiddleware` 工厂
- 请求日志记录器

### 🚀 cmd 模块（`cmd/whois-hacker`，2 源文件）

- 17 个命令行 flag
- YAML 配置加载（命令行 > YAML > 默认）
- 缓存/代理/监控/告警初始化
- 信号优雅关闭 + 指标导出

### 🐳 部署

- 多阶段 Dockerfile（静态链接 + UPX 压缩 + 非 root）
- Makefile（build/build-all/docker/docker-multi）
- docker-compose.yml
- GitHub Actions CI（test）

---

## 🗺️ 未来计划（占位）

::: details 🚧 待办事项（规划中）
- [ ] 接入 security 模块到主服务（替换 api 占位 AuthMiddleware）
- [ ] 接入 monitor 模块到查询层
- [ ] 实现 `serve` 子命令分发
- [ ] 修复 docker-compose.yml 路径不一致问题
- [ ] 支持 main.go 读取环境变量
- [ ] GitHub Actions 发布 workflow（多平台二进制 + Release + Docker）
- [ ] GitHub Pages 文档站自动部署
- [ ] RequestStore 持久化（脱离内存）
- [ ] 更多反向 WHOIS Provider 实现
:::

---

## 🔗 相关链接

- [模块总览](../modules/overview.md)
- [GitHub 仓库](https://github.com/cyberspacesec/whois-skills)
- [协议](./license.md)
