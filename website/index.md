---
layout: home

hero:
  name: Whois Hacker
  text: 一站式 WHOIS 域名情报查询工具
  tagline: 🚀 域名 / IP / ASN / RDAP / 反向查询 · 批量处理 · 关联分析 · 监控告警 · MCP 协议集成
  image:
    src: /favicon.svg
    alt: Whois Hacker
  actions:
    - theme: brand
      text: 🚀 快速开始
      link: /guide/getting-started
    - theme: alt
      text: 📖 项目介绍
      link: /guide/introduction
    - theme: alt
      text: 📚 API 文档
      link: /api/whois/overview

features:
  - icon: 🔍
    title: 域名 WHOIS 查询
    details: 支持优先级队列、并发聚合、结果校验、引导跟随（referral），自动重试与错误分类，覆盖 130+ TLD。
    link: /api/whois/query
  - icon: 🌐
    title: IP WHOIS 查询
    details: 通过 IANA 引导自动定位 RIR 服务器，结构化解析 ARIN/RIPE/APNIC/LACNIC/AFRINIC 五大格式。
    link: /api/whois/ipwhois
  - icon: 🔢
    title: ASN 查询
    details: 整合 RADB 与 RDAP 双数据源，支持上游/下游/对等 BGP 关系、前缀列表与批量查询。
    link: /api/whois/asn-enhanced
  - icon: 📡
    title: RDAP 查询
    details: 实现 RFC 9083，内置 bootstrap 映射，支持域名 / IP / ASN / Entity 四类查询，40+ TLD。
    link: /api/whois/rdap
  - icon: 🔄
    title: 反向 WHOIS
    details: 按注册人邮箱、组织、注册人姓名反向检索关联域名，Provider 接口可对接第三方服务。
    link: /api/whois/reverse
  - icon: 📋
    title: 批量查询
    details: 流式批量处理器，支持断点续查、并发限速、进度回调、剩余时间预估，原子化断点写入。
    link: /api/whois/batch
  - icon: 🔗
    title: 关联分析
    details: 按邮箱/注册人/组织/NS/注册商聚类多域名，构建关联图与资产画像，识别同主体资产。
    link: /api/whois/correlation
  - icon: 👁️
    title: 域名监控
    details: 周期性检查域名到期、状态/注册人/NS 变更，分级告警（info/warning/critical）。
    link: /api/whois/monitor
  - icon: ⭐
    title: 质量评估
    details: 完整性/时效性/可信度三维评分，检测隐私保护服务（Domains By Proxy 等 13 种规则）。
    link: /api/whois/quality
  - icon: 💾
    title: 多层缓存
    details: 本地内存与 Redis 双实现，支持缓存预热、TTL 过期、命中率统计。
    link: /api/whois/cache
  - icon: 🔒
    title: 代理池
    details: SOCKS5/HTTP 代理池，轮询调度、健康检查、故障熔断，支持代理认证。
    link: /api/whois/proxy
  - icon: ⏱️
    title: 智能调度
    details: 按服务器响应时间与限速反馈自适应调整查询间隔、退避与并发，含自适应令牌桶。
    link: /api/whois/scheduler
  - icon: 🚨
    title: 监控告警
    details: CPU/内存/错误率/失败率四类内置告警规则，支持 Email/Slack/Webhook 通知。
    link: /modules/metrics
  - icon: 🤖
    title: MCP 协议
    details: 任务规划/执行/审批状态机，将 WHOIS 能力封装为可控的 Mission Control 流程。
    link: /api/mcp/overview
  - icon: 🌍
    title: IDN 支持
    details: 国际化域名 Punycode 转换与规范化，原生支持多语言域名查询。
    link: /api/whois/idn
  - icon: 📤
    title: 多格式导出
    details: 查询结果导出为 JSON、CSV、Markdown 三种格式，便于集成与分享。
    link: /api/whois/export
---

<div class="vp-doc" style="margin-top: 48px;">

## 🎯 Whois Hacker 解决什么问题

传统 WHOIS 查询面临诸多痛点：**单点查询效率低**、**国际域名编码复杂**、**各注册局格式不统一**、**高频查询易被封禁**、**结果难以结构化与关联**。

Whois Hacker 用一套统一的 Go 库与 HTTP 服务，将域名、IP、ASN、RDAP、反向 WHOIS 等分散的能力整合为**一致的可编程接口**，并在其上叠加缓存、代理、限流、调度、监控、告警、关联分析等工程化能力，让情报收集从"手工拼凑"升级为"工程化流水线"。

</div>

<div class="feature-grid" style="margin-top: 32px;">

<div class="feature-card">
<span class="feature-icon">⚡</span>
<div class="feature-title">高性能</div>
<div class="feature-desc">优先级队列 + 并发聚合 + 信号量限流，单机可支撑大规模批量查询。</div>
</div>

<div class="feature-card">
<span class="feature-icon">🛡️</span>
<div class="feature-title">抗封禁</div>
<div class="feature-desc">代理池轮询、令牌桶限速、自适应调度退避，规避高频查询限制。</div>
</div>

<div class="feature-card">
<span class="feature-icon">🔌</span>
<div class="feature-title">易集成</div>
<div class="feature-desc">Go 库 + HTTP API + MCP 协议三套接口，适配 CLI、Web、Agent 场景。</div>
</div>

<div class="feature-card">
<span class="feature-icon">📊</span>
<div class="feature-title">可观测</div>
<div class="feature-desc">Prometheus / OpenTelemetry 指标 + 内置统计 + 性能百分位监控。</div>
</div>

</div>

<style>
.feature-grid .feature-card .feature-icon { font-size: 28px; }
</style>
