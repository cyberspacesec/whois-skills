/**
 * Whois Hacker - 网站交互脚本
 */

document.addEventListener('DOMContentLoaded', function() {
    // 代码块高亮效果
    const codeBlocks = document.querySelectorAll('pre code');
    if (codeBlocks.length) {
        highlightCodeBlocks();
    }

    // 导航栏滚动效果
    const navbar = document.querySelector('.navbar');
    if (navbar) {
        window.addEventListener('scroll', function() {
            if (window.scrollY > 50) {
                navbar.classList.add('shadow');
            } else {
                navbar.classList.remove('shadow');
            }
        });
    }

    // 复制按钮
    addCopyButtons();

    // 平滑滚动
    enableSmoothScroll();

    // 特别处理docs.html页面的代码块
    if (window.location.pathname.includes('docs.html')) {
        enhanceDocsCodeBlocks();
    }

    // 检查API服务状态
    checkApiStatus();

    // 初始化Bootstrap工具提示
    initializeBootstrapComponents();
});

/**
 * 初始化Bootstrap组件
 */
function initializeBootstrapComponents() {
    // 初始化提示框
    var tooltipTriggerList = [].slice.call(document.querySelectorAll('[data-bs-toggle="tooltip"]'));
    tooltipTriggerList.map(function (tooltipTriggerEl) {
        return new bootstrap.Tooltip(tooltipTriggerEl);
    });

    // 初始化手风琴
    var accordionElements = document.querySelectorAll('.accordion');
    if (accordionElements.length > 0) {
        accordionElements.forEach(function(accordion) {
            // Bootstrap 5自动处理手风琴
        });
    }
}

/**
 * 为代码块添加语法高亮效果
 */
function highlightCodeBlocks() {
    // 为了不引入额外的依赖，这里使用简单的CSS高亮
    // 如果需要更复杂的高亮，可以引入Prism.js或Highlight.js
    const codeBlocks = document.querySelectorAll('pre code');
    codeBlocks.forEach(block => {
        // 确保代码块内容是字符串而非HTML
        let content = block.innerHTML;
        
        // 转义已存在的HTML实体以避免重复处理
        content = content
            .replace(/&amp;/g, '&amp;amp;')
            .replace(/&lt;/g, '&amp;lt;')
            .replace(/&gt;/g, '&amp;gt;');
            
        // 对不同类型的代码应用不同的高亮规则
        let highlightedContent;

        // 检测是否包含常见的shell命令标记
        if (content.includes('$') || content.includes('#') || content.includes('whois-hacker') || 
            content.includes('curl') || content.includes('docker')) {
            highlightedContent = highlightShellCode(content);
        }
        // 检测是否包含JSON格式
        else if (content.includes('{') && content.includes('}')) {
            highlightedContent = highlightJsonCode(content);
        }
        // 检测是否为配置文件格式
        else if (content.includes(':') && !content.includes(';')) {
            highlightedContent = highlightConfigCode(content);
        }
        // 默认高亮处理
        else {
            highlightedContent = highlightGenericCode(content);
        }
        
        // 应用高亮
        block.innerHTML = highlightedContent;
        
        // 添加类
        block.parentElement.classList.add('code-block');
    });
}

/**
 * Shell命令高亮
 */
function highlightShellCode(content) {
    return content
        // 命令提示符
        .replace(/^([$#]\s.*$)/gm, '<span class="cmd-prompt">$1</span>')
        // 命令行标志
        .replace(/(--?\w+(-\w+)*)/g, '<span class="cmd-flag">$1</span>')
        // 命令名称
        .replace(/\b(whois-hacker|curl|wget|docker|git|go|make|cd|ls|mkdir)\b/g, '<span class="cmd-name">$1</span>')
        // 路径和文件名
        .replace(/\b([\w-]+\.[a-zA-Z]{2,})\b/g, '<span style="color: #f1fa8c;">$1</span>') 
        // URL
        .replace(/(https?:\/\/[^\s]+)/g, '<span style="color: #8be9fd;">$1</span>')
        // 注释
        .replace(/(#.*$)/gm, '<span style="color: #6272a4;">$1</span>')
        // 输出内容
        .replace(/^(?![$#])(.*$)/gm, '<span style="color: #f8f8f2;">$1</span>');
}

/**
 * JSON高亮
 */
function highlightJsonCode(content) {
    return content
        // JSON键
        .replace(/"([^"]+)":/g, '"<span style="color: #f8c555;">$1</span>":')
        // JSON字符串值
        .replace(/:\s*"([^"]*)"/g, ': "<span style="color: #f1fa8c;">$1</span>"')
        // JSON数字
        .replace(/:\s*(\d+)([,\n\r]|$)/g, ': <span style="color: #bd93f9;">$1</span>$2')
        // JSON布尔值和null
        .replace(/:\s*(true|false|null)([,\n\r]|$)/g, ': <span style="color: #ff79c6;">$1</span>$2');
}

/**
 * 配置文件高亮
 */
function highlightConfigCode(content) {
    return content
        // 配置键
        .replace(/^(\s*)([^:]+):/gm, '$1<span style="color: #8be9fd;">$2</span>:')
        // 配置值
        .replace(/:\s*(.+)$/gm, ': <span style="color: #f1fa8c;">$1</span>');
}

/**
 * 通用代码高亮
 */
function highlightGenericCode(content) {
    return content
        // 确保至少有基本颜色
        .replace(/(.*)/g, '<span style="color: #f8f8f2;">$1</span>');
}

/**
 * 特别处理docs.html页面中的代码块
 */
function enhanceDocsCodeBlocks() {
    // 查找文档中的所有内联代码元素
    document.querySelectorAll('.doc-content p code, .doc-content li code, .doc-content td code').forEach(code => {
        // 设置内联代码的样式
        code.style.backgroundColor = 'rgba(0,0,0,0.05)';
        code.style.color = '#2c3e50';
        code.style.padding = '2px 5px';
        code.style.borderRadius = '3px';
        code.style.border = '1px solid rgba(0,0,0,0.1)';
    });

    // 查找文档中的所有代码块
    document.querySelectorAll('.doc-content pre code').forEach(codeBlock => {
        // 确保代码块内容可见
        codeBlock.style.color = '#f8f8f2';
    });
}

/**
 * 为代码块添加复制按钮
 */
function addCopyButtons() {
    const codeBlocks = document.querySelectorAll('pre');
    
    codeBlocks.forEach(block => {
        // 如果已经有复制按钮，则不重复添加
        if (block.parentNode.querySelector('.copy-button-container')) {
            return;
        }
        
        // 创建复制按钮容器
        const buttonContainer = document.createElement('div');
        buttonContainer.className = 'copy-button-container';
        
        // 创建复制按钮
        const copyButton = document.createElement('button');
        copyButton.className = 'copy-button';
        copyButton.innerHTML = '<i class="bi bi-clipboard"></i>';
        copyButton.title = '复制到剪贴板';
        
        // 添加按钮到容器
        buttonContainer.appendChild(copyButton);
        
        // 添加容器到代码块
        if (block.parentNode) {
            block.parentNode.style.position = 'relative';
            block.parentNode.appendChild(buttonContainer);
        }
        
        // 添加复制功能
        copyButton.addEventListener('click', () => {
            // 获取纯文本内容而非HTML
            const codeElement = block.querySelector('code');
            // 创建临时元素来解析HTML实体
            const tempElement = document.createElement('div');
            tempElement.innerHTML = codeElement ? codeElement.innerHTML : block.innerHTML;
            const code = tempElement.textContent || '';

            navigator.clipboard.writeText(code).then(() => {
                // 显示成功状态
                copyButton.innerHTML = '<i class="bi bi-check"></i>';
                copyButton.classList.add('success');
                
                // 2秒后恢复
                setTimeout(() => {
                    copyButton.innerHTML = '<i class="bi bi-clipboard"></i>';
                    copyButton.classList.remove('success');
                }, 2000);
            }).catch(err => {
                console.error('无法复制文本:', err);
                copyButton.innerHTML = '<i class="bi bi-exclamation-triangle"></i>';
                copyButton.classList.add('error');
                
                // 2秒后恢复
                setTimeout(() => {
                    copyButton.innerHTML = '<i class="bi bi-clipboard"></i>';
                    copyButton.classList.remove('error');
                }, 2000);
            });
        });
    });
}

/**
 * 实现平滑滚动效果
 */
function enableSmoothScroll() {
    document.querySelectorAll('a[href^="#"]').forEach(anchor => {
        anchor.addEventListener('click', function(e) {
            e.preventDefault();
            const targetId = this.getAttribute('href');
            if (targetId === '#') return;
            
            const targetElement = document.querySelector(targetId);
            if (targetElement) {
                window.scrollTo({
                    top: targetElement.offsetTop - 80, // 导航栏高度偏移
                    behavior: 'smooth'
                });
            }
        });
    });
}

/**
 * 添加动态卡片动画效果
 */
function initializeCardAnimations() {
    const cards = document.querySelectorAll('.card');
    
    cards.forEach(card => {
        card.addEventListener('mouseenter', () => {
            card.style.transform = 'translateY(-10px)';
            card.style.boxShadow = '0 15px 30px rgba(0, 0, 0, 0.1)';
        });
        
        card.addEventListener('mouseleave', () => {
            card.style.transform = 'translateY(0)';
            card.style.boxShadow = '0 5px 15px rgba(0, 0, 0, 0.05)';
        });
    });
}

// 在窗口加载完成后初始化卡片动画
window.addEventListener('load', initializeCardAnimations);

/**
 * 创建API状态指示器
 */
function createApiStatusIndicator() {
    const apiStatusWrapper = document.createElement('div');
    apiStatusWrapper.className = 'position-fixed bottom-0 end-0 m-3';
    apiStatusWrapper.style.zIndex = '1000';
    
    const apiStatusIndicator = document.createElement('div');
    apiStatusIndicator.id = 'api-status-indicator';
    apiStatusIndicator.className = 'api-status-indicator d-flex align-items-center bg-light shadow-sm rounded px-3 py-2';
    apiStatusIndicator.innerHTML = `
        <div class="api-status-dot me-2" id="api-status-dot"></div>
        <span id="api-status-text">正在检查API服务...</span>
    `;
    
    apiStatusWrapper.appendChild(apiStatusIndicator);
    document.body.appendChild(apiStatusWrapper);
    
    // 添加样式
    const style = document.createElement('style');
    style.textContent = `
        .api-status-indicator {
            transition: all 0.3s ease;
            font-size: 0.9rem;
            cursor: pointer;
            opacity: 0.8;
        }
        .api-status-indicator:hover {
            opacity: 1;
        }
        .api-status-dot {
            width: 12px;
            height: 12px;
            border-radius: 50%;
            background-color: #aaaaaa;
            transition: background-color 0.3s ease;
        }
        .api-status-dot.online {
            background-color: #4CAF50;
        }
        .api-status-dot.offline {
            background-color: #F44336;
        }
        .api-status-dot.checking {
            background-color: #FFC107;
            animation: pulse 1.5s infinite;
        }
        @keyframes pulse {
            0% { transform: scale(0.95); opacity: 0.7; }
            50% { transform: scale(1.05); opacity: 1; }
            100% { transform: scale(0.95); opacity: 0.7; }
        }
    `;
    document.head.appendChild(style);
    
    return {
        dot: document.getElementById('api-status-dot'),
        text: document.getElementById('api-status-text')
    };
}

/**
 * 检查API服务状态
 */
function checkApiStatus() {
    const pages = ['index.html', 'api.html', 'docs.html', 'mcp.html', 'download.html', 'demo.html'];
    const currentPath = window.location.pathname;
    
    // 只在主要页面显示API状态
    if (!pages.some(page => currentPath.endsWith(page) || currentPath.endsWith('/'))) {
        return;
    }
    
    const indicator = createApiStatusIndicator();
    indicator.dot.classList.add('checking');
    
    // 构建API地址
    const apiBaseUrl = getApiBaseUrl();
    
    // 检查API健康状态
    fetch(`${apiBaseUrl}/health`, { method: 'GET' })
        .then(response => {
            if (!response.ok) {
                throw new Error(`HTTP error! Status: ${response.status}`);
            }
            return response.json();
        })
        .then(data => {
            indicator.dot.classList.remove('checking');
            
            if (data && data.success) {
                indicator.dot.classList.add('online');
                indicator.text.textContent = 'API服务运行正常';
            } else {
                indicator.dot.classList.add('offline');
                indicator.text.textContent = 'API服务异常';
            }
        })
        .catch(error => {
            indicator.dot.classList.remove('checking');
            indicator.dot.classList.add('offline');
            indicator.text.textContent = 'API服务离线';
            console.error('API状态检查失败:', error);
        })
        .finally(() => {
            // 30秒后再次检查
            setTimeout(checkApiStatus, 30000);
        });
        
    // 点击指示器显示更多信息
    document.getElementById('api-status-indicator').addEventListener('click', function() {
        checkApiVersionInfo(apiBaseUrl);
    });
}

/**
 * 获取API基础URL
 */
function getApiBaseUrl() {
    // 首先尝试从同域获取API
    const currentHost = window.location.hostname;
    const currentProto = window.location.protocol;
    
    // 检查是否在本地开发环境
    if (currentHost === 'localhost' || currentHost === '127.0.0.1') {
        // 假设API在8080端口
        return `${currentProto}//${currentHost}:8080/api`;
    }
    
    // 生产环境 - 假设API与网站在同一域下的/api路径
    return `${currentProto}//${currentHost}/api`;
}

/**
 * 检查API版本信息
 */
function checkApiVersionInfo(apiBaseUrl) {
    fetch(`${apiBaseUrl}/version`, { method: 'GET' })
        .then(response => {
            if (!response.ok) {
                throw new Error(`HTTP error! Status: ${response.status}`);
            }
            return response.json();
        })
        .then(data => {
            if (data && data.success) {
                // 创建模态框显示API信息
                showApiInfoModal({
                    version: data.version || '未知',
                    buildTime: data.build_time || '未知',
                    gitCommit: data.git_commit || '未知'
                });
            } else {
                showToast('无法获取API版本信息', 'warning');
            }
        })
        .catch(error => {
            showToast(`获取API版本失败: ${error.message}`, 'danger');
            console.error('API版本检查失败:', error);
        });
}

/**
 * 显示API信息模态框
 */
function showApiInfoModal(info) {
    // 如果已有模态框则先移除
    let apiModal = document.getElementById('apiInfoModal');
    if (apiModal) {
        apiModal.remove();
    }
    
    // 创建模态框
    apiModal = document.createElement('div');
    apiModal.className = 'modal fade';
    apiModal.id = 'apiInfoModal';
    apiModal.tabIndex = '-1';
    apiModal.setAttribute('aria-labelledby', 'apiInfoModalLabel');
    apiModal.setAttribute('aria-hidden', 'true');
    
    // 创建模态框内容
    apiModal.innerHTML = `
        <div class="modal-dialog">
            <div class="modal-content">
                <div class="modal-header">
                    <h5 class="modal-title" id="apiInfoModalLabel">API服务信息</h5>
                    <button type="button" class="btn-close" data-bs-dismiss="modal" aria-label="Close"></button>
                </div>
                <div class="modal-body">
                    <table class="table table-sm">
                        <tr>
                            <th scope="row">版本</th>
                            <td>${info.version}</td>
                        </tr>
                        <tr>
                            <th scope="row">构建时间</th>
                            <td>${info.buildTime}</td>
                        </tr>
                        <tr>
                            <th scope="row">Git提交</th>
                            <td><code>${info.gitCommit}</code></td>
                        </tr>
                        <tr>
                            <th scope="row">状态</th>
                            <td><span class="badge bg-success">在线</span></td>
                        </tr>
                    </table>
                </div>
                <div class="modal-footer">
                    <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">关闭</button>
                </div>
            </div>
        </div>
    `;
    
    // 添加到页面并显示
    document.body.appendChild(apiModal);
    const modal = new bootstrap.Modal(apiModal);
    modal.show();
}

/**
 * 显示提示消息
 */
function showToast(message, type = 'info') {
    // 创建Toast元素
    const toastId = 'toast-' + Date.now();
    const toastHtml = `
        <div id="${toastId}" class="toast align-items-center border-0 text-white bg-${type}" role="alert" aria-live="assertive" aria-atomic="true">
            <div class="d-flex">
                <div class="toast-body">
                    ${message}
                </div>
                <button type="button" class="btn-close btn-close-white me-2 m-auto" data-bs-dismiss="toast" aria-label="Close"></button>
            </div>
        </div>
    `;
    
    // 创建或获取toast容器
    let toastContainer = document.querySelector('.toast-container');
    if (!toastContainer) {
        toastContainer = document.createElement('div');
        toastContainer.className = 'toast-container position-fixed bottom-0 end-0 p-3';
        toastContainer.style.zIndex = '11';
        document.body.appendChild(toastContainer);
    }
    
    // 添加Toast到容器
    toastContainer.innerHTML += toastHtml;
    
    // 初始化并显示Toast
    const toastElement = document.getElementById(toastId);
    const toast = new bootstrap.Toast(toastElement, {
        autohide: true,
        delay: 5000
    });
    toast.show();
    
    // 移除事件
    toastElement.addEventListener('hidden.bs.toast', function() {
        toastElement.remove();
    });
} 