# H-Reader

H-Reader 是一款 AI 辅助桌面 PDF 阅读器。你可以在选中文字后查词、记笔记、就地提问，也可以用右侧栏继续讨论选文和截图。

## 主要功能

| 功能 | 用法 |
| --- | --- |
| 书架和阅读 | 选择书籍目录后从书架打开 PDF。支持文本选择、缩放、适应宽度、页码跳转和阅读进度恢复。选中可识别的公式编号时，可跳转到公式并返回原位。 |
| 离线查词 | 选中英文单词，在选文菜单中查看内置 ECDICT 词典的释义。支持识别部分跨行断开的单词。 |
| 页内笔记 | 通过选文菜单或页面上的右键菜单打开「笔记」，用页面标记找回已保存的内容。 |
| 原地问 | 通过选文菜单或页面上的右键菜单打开「原地问」，在 PDF 页内提问和追问。保存后的对话可通过页面标记再次打开。 |
| 右侧栏对话 | 支持书架和单本书各自的会话、流式回复、Markdown、LaTeX 和图片附件。选文菜单提供「插入聊天」「提问」和「解释」。可将侧栏消息标记到 PDF 页面，并从标记跳回对应消息。 |
| 截图 | 支持整页和框选截图。右侧栏打开时，截图会加入聊天附件。macOS 支持通过原生接口将截图复制到剪贴板。 |

书籍阅读进度、标注、笔记和聊天记录保存在本地。使用 AI 功能时，请求内容会发往所选供应商的 API。

## 开始使用

1. 安装与你的操作系统匹配的构建产物，或按下文从源码构建。
2. 打开应用，选择存放 PDF 的目录，然后从书架打开一本书。
3. 使用 AI 功能前，在设置中添加 OpenAI 兼容供应商，填写 API 地址、API Key 和可用模型，再分别为「右侧栏对话」和「原地问」选择供应商与模型。使用截图提问时，侧栏模型需要支持图片输入。

### 快捷键

| 快捷键 | 操作 |
| --- | --- |
| `Ctrl+Q` / `Cmd+Q` | 在阅读视图中启动框选截图 |
| `Ctrl` / `Cmd` + 滚轮 | 缩放 PDF |
| `Esc` | 取消截图框选或标记放置 |
| `Shift+Enter` / `Ctrl+Enter` / `Cmd+Enter` / `Alt+Enter` | 发送右侧栏消息 |
| `Enter` | 在「原地问」的输入框中发送消息，`Shift+Enter` 换行 |

## 从源码构建

需要 Go 1.25 或更新版本、Node.js 22.13 或更新版本、npm，以及目标平台所需的 Wails 构建依赖。仓库提供 macOS、Windows 和 Linux 的桌面构建任务。

在仓库根目录执行以下命令。Wails CLI 版本与 `go.mod` 中的依赖一致。

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-alpha.74
wails3 build
```

构建产物位于 `bin/`。开发模式会启动 Vite 开发服务器并运行桌面应用。

```bash
wails3 task dev
```

## 技术栈

后端使用 Go 和 Wails v3，前端使用 Vue 3 和 Vite。PDF 渲染由 PDF.js 提供，聊天消息使用 markdown-it、highlight.js 和 KaTeX 渲染。目录、模块职责和本地数据路径见 [PROJECT_STRUCTURE.md](PROJECT_STRUCTURE.md)。

## 许可证

项目使用 [Apache License 2.0](LICENSE)。内置词典的授权信息见 [ECDICT 许可证](frontend/public/licenses/ECDICT.txt)。
