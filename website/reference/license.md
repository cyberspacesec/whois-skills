# 📄 协议（MIT License）

> 📖 whois-skills 采用 MIT 开源协议，鼓励自由使用、修改与分发。

---

## 📋 概览

| 项目 | 内容 |
|------|------|
| 协议 | MIT License |
| 版权持有 | CyberSpaceSec |
| 仓库 | [cyberspacesec/whois-skills](https://github.com/cyberspacesec/whois-skills) |

---

## 📜 MIT License

```
MIT License

Copyright (c) CyberSpaceSec

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

::: tip 💡 MIT 协议要点
MIT 是最宽松的开源协议之一：允许商业使用、修改、分发、再授权甚至闭源使用，唯一要求是保留版权声明与协议文本。软件按「现状」提供，不承担任何担保责任。
:::

---

## 🤝 开源贡献指引

欢迎为 whois-skills 贡献代码！请遵循以下流程：

### 1. Fork 仓库

在 GitHub 页面点击 **Fork**，将 `cyberspacesec/whois-skills` fork 到你自己的账号下。

### 2. 克隆并创建分支

```bash
git clone https://github.com/<your-username>/whois-skills.git
cd whois-skills
git remote add upstream https://github.com/cyberspacesec/whois-skills.git
git checkout -b feature/your-feature
```

::: tip 分支命名建议
- 新功能：`feature/xxx`
- 修复：`fix/xxx`
- 文档：`docs/xxx`
:::

### 3. 开发与测试

```bash
# 确保编译通过
go build ./...

# 运行全部测试
make test
# 或
go test -v ./...
```

- 遵循 Go 标准格式：`gofmt -w .`、`go vet ./...`
- 新增功能请补充对应 `_test.go` 测试
- 公共 API 需有文档注释

### 4. 提交并推送

```bash
git add .
git commit -m "feat: 添加 xxx 功能"
git push origin feature/your-feature
```

::: details 提交信息规范（建议）
- `feat:` 新功能
- `fix:` 修复
- `docs:` 文档
- `refactor:` 重构
- `test:` 测试
- `chore:` 杂项
:::

### 5. 发起 Pull Request

在 GitHub 页面向 `cyberspacesec/whois-skills` 的 `main` 分支发起 PR，描述：

- 变更内容与动机
- 是否影响现有行为
- 测试情况

### 6. 代码评审

维护者会进行 review，请根据反馈在原分支上追加 commit（不要新开 PR），推送后会自动更新 PR。

---

## 📬 联系

- Issue：[提交问题](https://github.com/cyberspacesec/whois-skills/issues)
- PR：[发起拉取请求](https://github.com/cyberspacesec/whois-skills/pulls)

---

## 🔗 相关链接

- [更新日志](./changelog.md)
- [模块总览](../modules/overview.md)
- [快速开始](../guide/getting-started.md)
