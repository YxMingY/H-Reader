# H-Reader

**在 PDF 里查词、记笔记，读到哪里就问到哪里。**

H-Reader 是一款面向论文、教材和技术文档的桌面 PDF 阅读器，提供 macOS、Windows 和 Linux 构建任务。

## 下载 v1.0.0-beta.20261126.1

预发行版下载：[Gitee Release](https://gitee.com/xming-kaede/hreader/releases/tag/v1.0.0-beta.20261126.1) · [GitHub Release](https://github.com/YxMingY/H-Reader/releases/tag/v1.0.0-beta.20261126.1)

| 平台 | 下载附件 |
| --- | --- |
| macOS Apple Silicon | `HReader-v1.0.0-beta.20261126.1-macos-arm64.zip`，解压后打开应用 |
| Windows x64 | `HReader-v1.0.0-beta.20261126.1-windows-amd64.exe`，便携版，下载后直接运行 |

两个发布页均提供 `SHA256SUMS.txt` 供校验。

## 阅读时能做什么

- **遇到生词，选中就能查。** 内置 ECDICT 英汉词典，离线显示释义，也能识别部分跨行断开的英文单词。
- **卡在某一段，用「原地问」。** 选中文字后直接在页面上提问和追问。保存后的对话可以从原位置的标记重新打开。
- **想留个想法，写在页上。** 从选文菜单或页面右键菜单打开「笔记」，下次读到这里时点击标记即可找回。
- **想深入讨论，打开右侧栏。** 把选文、整页截图或框选截图加入对话。回复支持流式显示、Markdown 和 LaTeX。
- **用大头针把侧栏消息钉在 PDF 上。** 在阅读 PDF 时，点击右侧栏消息旁的 📌「标记位置」，再点选 PDF 上的位置。之后双击页面上的大头针，即可打开对应会话并跳到那条消息。

- **跳到公式，再回到阅读位置。** 选中可识别的公式编号，在菜单中点「跳转公式」。看完后点「返回原位」，回到跳转前的位置。

书架会记录阅读进度，下次打开可接着读。

## 开始阅读

1. 安装适用于你系统的构建产物，或按下方步骤从源码构建。
2. 首次启动时，选择存放 PDF 的文件夹。
3. 点击「进入书架」，双击一本书开始阅读。阅读、离线查词和笔记都可以直接使用。

### 可选配置 AI

在右上角打开「设置」，添加 OpenAI 兼容供应商，填写 API 地址、API Key 和模型。可以分别为「原地问」和「右侧栏对话」选择供应商与模型。要发送截图，请为侧栏选择支持图片输入的模型。macOS 还支持将截图复制到系统剪贴板。

阅读进度、标注、笔记和聊天记录保存在本地。使用 AI 对话时，请求内容会发送到所选供应商的 API。

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
