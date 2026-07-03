import { defineConfig } from 'vitepress'

// https://vitepress.dev/reference/site-config
export default defineConfig({
  lang: 'zh-CN',
  title: 'Whois Hacker',
  titleTemplate: false,
  description: '一站式 WHOIS 域名情报查询工具 - 域名/IP/ASN/RDAP/反向查询、批量处理、关联分析、监控告警',
  head: [
    ['link', { rel: 'icon', type: 'image/svg+xml', href: '/whois-skills/favicon.svg' }],
    ['meta', { name: 'keywords', content: 'WHOIS,域名查询,ASN,RDAP,IP查询,反向WHOIS,网络安全,情报收集,Go' }],
    ['meta', { name: 'author', content: 'CyberSpaceSec' }],
    ['meta', { property: 'og:title', content: 'Whois Hacker - 一站式 WHOIS 域名情报查询工具' }],
    ['meta', { property: 'og:description', content: '域名/IP/ASN/RDAP/反向查询、批量处理、关联分析、监控告警' }],
    ['meta', { property: 'og:type', content: 'website' }]
  ],

  // GitHub Pages 部署：仓库名为 whois-skills，base 为 /whois-skills/
  base: '/whois-skills/',
  cleanUrls: true,
  lastUpdated: true,

  markdown: {
    lineNumbers: true,
    theme: { light: 'github-light', dark: 'one-dark-pro' },
    config: (md) => {
      // 支持 badge 图标语法
      md.use(badgePlugin)
    }
  },

  themeConfig: {
    // 文档仓库地址
    repo: 'cyberspacesec/whois-skills',
    repoLabel: 'GitHub',

    docsDir: 'website',
    docsBranch: 'main',
    editLink: {
      pattern: 'https://github.com/cyberspacesec/whois-skills/edit/main/website/:path',
      text: '在 GitHub 上编辑此页'
    },

    lastUpdated: {
      text: '最后更新于'
    },

    search: {
      provider: 'local',
      options: {
        translations: {
          button: { buttonText: '搜索文档', buttonAriaLabel: '搜索文档' },
          modal: {
            displayDetails: '显示详情',
            resetButtonTitle: '清除查询',
            backButtonTitle: '返回',
            noResultsText: '没有找到结果',
            footer: {
              selectText: '选择',
              navigateText: '切换',
              closeText: '关闭'
            }
          }
        }
      }
    },

    // 社交链接
    socialLinks: [
      { icon: 'github', link: 'https://github.com/cyberspacesec/whois-skills' }
    ],

    // 页脚
    footer: {
      message: '基于 MIT 协议发布',
      copyright: 'Copyright © 2024-present CyberSpaceSec'
    },

    // 导航栏
    nav: nav(),

    // 侧边栏
    sidebar: sidebar()
  }
})

function nav() {
  return [
    { text: '🏠 首页', link: '/' },
    { text: '📖 指南', link: '/guide/getting-started' },
    {
      text: '📚 API 文档',
      items: [
        { text: 'WHOIS 核心', link: '/api/whois/query' },
        { text: 'HTTP API', link: '/api/http/overview' },
        { text: 'MCP 协议', link: '/api/mcp/overview' }
      ]
    },
    { text: '🔧 模块', link: '/modules/overview' },
    { text: '🚀 部署', link: '/deploy/docker' },
    {
      text: '🌐',
      items: [
        { text: 'GitHub', link: 'https://github.com/cyberspacesec/whois-skills' }
      ]
    }
  ]
}

function sidebar() {
  return {
    '/guide/': [
      {
        text: '🎮 开始',
        collapsed: false,
        items: [
          { text: '✨ 项目介绍', link: '/guide/introduction' },
          { text: '🚀 快速开始', link: '/guide/getting-started' },
          { text: '📥 安装指南', link: '/guide/installation' }
        ]
      },
      {
        text: '📖 核心概念',
        collapsed: false,
        items: [
          { text: '🏗️ 架构总览', link: '/guide/architecture' },
          { text: '🧩 模块全景', link: '/guide/modules-overview' },
          { text: '🔄 查询流程', link: '/guide/query-flow' },
          { text: '⚙️ 配置系统', link: '/guide/configuration' }
        ]
      },
      {
        text: '🎓 教程',
        collapsed: false,
        items: [
          { text: '🎯 域名查询', link: '/guide/tutorial-domain' },
          { text: '🌐 IP 查询', link: '/guide/tutorial-ip' },
          { text: '🔢 ASN 查询', link: '/guide/tutorial-asn' },
          { text: '📋 批量查询', link: '/guide/tutorial-batch' },
          { text: '🔍 关联分析', link: '/guide/tutorial-correlation' }
        ]
      }
    ],
    '/api/whois/': sidebarWhois(),
    '/api/http/': sidebarHttp(),
    '/api/mcp/': sidebarMcp(),
    '/modules/': sidebarModules(),
    '/deploy/': sidebarDeploy(),
    '/reference/': sidebarReference()
  }
}

function sidebarWhois() {
  return [
    {
      text: '🔍 WHOIS 核心包',
      collapsed: false,
      items: [
        { text: '📖 概览', link: '/api/whois/overview' },
        { text: '🔎 查询引擎 query.go', link: '/api/whois/query' },
        { text: '🔢 ASN 查询', link: '/api/whois/asn' },
        { text: '🚀 增强版 ASN', link: '/api/whois/asn-enhanced' },
        { text: '✅ 可用性检测', link: '/api/whois/availability' },
        { text: '📋 批量查询 batch', link: '/api/whois/batch' },
        { text: '💾 缓存 cache', link: '/api/whois/cache' },
        { text: '⚙️ 配置 config', link: '/api/whois/config' },
        { text: '🔗 关联分析 correlation', link: '/api/whois/correlation' },
        { text: '📊 差异对比 diff', link: '/api/whois/diff' },
        { text: '❌ 错误体系 errors', link: '/api/whois/errors' },
        { text: '📤 导出 export', link: '/api/whois/export' },
        { text: '📝 格式化 format', link: '/api/whois/format' },
        { text: '🌍 IDN 国际化域名', link: '/api/whois/idn' },
        { text: '🔬 IP 解析 ipparser', link: '/api/whois/ipparser' },
        { text: '🌐 IP WHOIS ipwhois', link: '/api/whois/ipwhois' },
        { text: '👁️ 域名监控 monitor', link: '/api/whois/monitor' },
        { text: '📈 可观测性 observability', link: '/api/whois/observability' },
        { text: '🔒 代理 proxy', link: '/api/whois/proxy' },
        { text: '⭐ 质量评估 quality', link: '/api/whois/quality' },
        { text: '⏱️ 速率限制 ratelimit', link: '/api/whois/ratelimit' },
        { text: '📡 RDAP 查询', link: '/api/whois/rdap' },
        { text: '🔄 反向查询 reverse', link: '/api/whois/reverse' },
        { text: '🎛️ 智能调度 scheduler', link: '/api/whois/scheduler' },
        { text: '🖥️ 服务器管理 servers', link: '/api/whois/servers' }
      ]
    }
  ]
}

function sidebarHttp() {
  return [
    {
      text: '🌐 HTTP API',
      collapsed: false,
      items: [
        { text: '📖 概览', link: '/api/http/overview' },
        { text: '🖥️ 服务器 server', link: '/api/http/server' },
        { text: '🛡️ 中间件 middleware', link: '/api/http/middleware' },
        { text: '📨 响应 response', link: '/api/http/response' },
        { text: '🔌 端点总览', link: '/api/http/endpoints' }
      ]
    },
    {
      text: '📡 端点详解',
      collapsed: false,
      items: [
        { text: '🔎 WHOIS 端点', link: '/api/http/endpoint-whois' },
        { text: '🌐 IP 端点', link: '/api/http/endpoint-ip' },
        { text: '🔢 ASN 端点', link: '/api/http/endpoint-asn' },
        { text: '📡 RDAP 端点', link: '/api/http/endpoint-rdap' },
        { text: '✅ 可用性端点', link: '/api/http/endpoint-availability' },
        { text: '📊 对比端点', link: '/api/http/endpoint-diff' },
        { text: '⭐ 质量端点', link: '/api/http/endpoint-quality' },
        { text: '🔗 关联端点', link: '/api/http/endpoint-correlation' },
        { text: '📋 批量端点', link: '/api/http/endpoint-batch' },
        { text: '📝 格式化端点', link: '/api/http/endpoint-format' },
        { text: '📤 导出端点', link: '/api/http/endpoint-export' },
        { text: '🌍 IDN 端点', link: '/api/http/endpoint-idn' },
        { text: '🖥️ 服务器端点', link: '/api/http/endpoint-servers' },
        { text: '📈 监控端点', link: '/api/http/endpoint-metrics' },
        { text: '🚨 告警端点', link: '/api/http/endpoint-alerts' },
        { text: '❤️ 健康检查', link: '/api/http/endpoint-health' }
      ]
    }
  ]
}

function sidebarMcp() {
  return [
    {
      text: '🤖 MCP 协议',
      collapsed: false,
      items: [
        { text: '📖 概览', link: '/api/mcp/overview' },
        { text: '🎮 控制器 controller', link: '/api/mcp/controller' },
        { text: '🗄️ 数据模型 models', link: '/api/mcp/models' },
        { text: '🖥️ 服务端 server', link: '/api/mcp/server' }
      ]
    },
    {
      text: '🔧 任务管理端点',
      collapsed: false,
      items: [
        { text: '📋 请求规划', link: '/api/mcp/endpoint-request-planning' },
        { text: '➡️ 获取下一任务', link: '/api/mcp/endpoint-get-next-task' },
        { text: '✅ 标记完成', link: '/api/mcp/endpoint-mark-task-done' },
        { text: '👍 审批任务', link: '/api/mcp/endpoint-approve-task' },
        { text: '🎯 审批请求', link: '/api/mcp/endpoint-approve-request' },
        { text: '📂 任务详情', link: '/api/mcp/endpoint-task-details' },
        { text: '📜 列出请求', link: '/api/mcp/endpoint-list-requests' },
        { text: '➕ 添加任务', link: '/api/mcp/endpoint-add-tasks' },
        { text: '✏️ 更新任务', link: '/api/mcp/endpoint-update-task' },
        { text: '🗑️ 删除任务', link: '/api/mcp/endpoint-delete-task' }
      ]
    }
  ]
}

function sidebarModules() {
  return [
    {
      text: '🧩 模块',
      collapsed: false,
      items: [
        { text: '📖 模块总览', link: '/modules/overview' },
        { text: '🔍 whois', link: '/modules/whois' },
        { text: '🌐 api', link: '/modules/api' },
        { text: '🤖 mcp', link: '/modules/mcp' },
        { text: '📈 metrics', link: '/modules/metrics' },
        { text: '👁️ monitor', link: '/modules/monitor' },
        { text: '🔒 security', link: '/modules/security' },
        { text: '⚙️ cmd', link: '/modules/cmd' }
      ]
    }
  ]
}

function sidebarDeploy() {
  return [
    {
      text: '🚀 部署',
      collapsed: false,
      items: [
        { text: '🐳 Docker 部署', link: '/deploy/docker' },
        { text: '📦 二进制部署', link: '/deploy/binary' },
        { text: '🐙 GitHub Actions', link: '/deploy/github-actions' },
        { text: '🌐 GitHub Pages', link: '/deploy/github-pages' },
        { text: '🎼 Docker Compose', link: '/deploy/compose' }
      ]
    }
  ]
}

function sidebarReference() {
  return [
    {
      text: '📚 参考资料',
      collapsed: false,
      items: [
        { text: '📖 FAQ', link: '/reference/faq' },
        { text: '🐛 故障排查', link: '/reference/troubleshooting' },
        { text: '📝 更新日志', link: '/reference/changelog' },
        { text: '📜 协议', link: '/reference/license' }
      ]
    }
  ]
}

// Badge 插件：支持 ::badge 语法和图标 emoji 内联
function badgePlugin(md) {
  // 简单的内联图标容器，渲染为带样式的 span
  const originalRender = md.renderInline.bind(md)
  md.renderInline = (src, env) => {
    return originalRender(src, env)
  }
}
