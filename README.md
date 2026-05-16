# H-Reader: AI辅助 阅读器

<div align="center">

**哪里不会点哪里的 PDF 阅读器**

[![Go](https://img.shields.io/badge/Go-1.25+-blue.svg)](https://golang.org/)
[![Vue](https://img.shields.io/badge/Vue-3.x-brightgreen.svg)](https://vuejs.org/)
[![Wails](https://img.shields.io/badge/Wails-v3-orange.svg)](https://wails.io/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

</div>

## 📖 项目简介

H-Reader 是一款基于 Wails v3 构建的跨平台桌面 PDF 阅读器，集成了 AI 对话功能。它采用 Go + Vue.js 技术栈，提供流畅的 PDF 阅读体验和智能化的学习辅助功能。

### ✨ 核心特性

- **📄 PDF 管理** - 便捷的 PDF 文件管理和浏览
- **📖 流畅阅读** - 高性能 PDF 渲染引擎，支持缩放、翻页、文本选择
- **📸 便捷截图** - 支持整页截图和区域框选截图
- **🤖 AI 对话** - 集成大语言模型，支持针对 PDF 内容的智能问答
- **💾 本地存储** - 聊天记录本地保存，随时回顾
- **🎨 现代 UI** - 简洁美观的用户界面，支持深色主题
- **⚡ 跨平台** - 基于 Wails，支持 Windows、macOS、Linux

## 🏗️ 技术架构

### 后端 (Go)
- **框架**: Wails v3 (Alpha) - 现代化的 Go 桌面应用框架
- **PDF 处理**: 通过前端 pdfjs-dist 库处理
- **AI 集成**: OpenAI API 兼容接口
- **数据存储**: 本地 JSON 文件存储聊天记录

### 前端 (Vue.js)
- **框架**: Vue 3 Composition API
- **构建工具**: Vite 5
- **PDF 渲染**: pdfjs-dist 5.x
- **Markdown 渲染**: markdown-it + highlight.js
- **状态管理**: Vue 响应式系统

### 项目结构

```
hreader/
├── main.go                 # 应用入口
├── bookservice.go          # PDF 书籍管理服务
├── chatservice.go          # AI 聊天服务
├── chatstore.go            # 聊天记录存储
├── config.go               # 配置管理
├── greetservice.go         # 示例服务
├── llmkit/                 # LLM 工具包
│   ├── client.go           # API 客户端
│   ├── conversation.go     # 对话管理
│   ├── message_builder.go  # 消息构建器
│   └── traced_conversation.go
├── frontend/               # 前端代码
│   ├── src/
│   │   ├── features/
│   │   │   ├── Reader/     # PDF 阅读器模块
│   │   │   │   ├── components/
│   │   │   │   │   └── PdfPageShell.vue
│   │   │   │   ├── composables/
│   │   │   │   │   ├── usePdfDocument.js    # PDF 文档管理
│   │   │   │   │   ├── usePdfPages.js       # 页面渲染管理
│   │   │   │   │   ├── useScaleAdjust.js    # 缩放控制
│   │   │   │   │   ├── useScrollPage.js     # 滚动与导航
│   │   │   │   │   └── useScreenshot.ts     # 截图功能
│   │   │   │   └── Reader.vue               # 阅读器主组件
│   │   │   ├── Bookshelf.vue      # 书架界面
│   │   │   ├── ChatPanel.vue      # AI 聊天面板
│   │   │   └── SettingsMenu.vue   # 设置菜单
│   │   └── App.vue                # 应用根组件
│   ├── public/
│   │   └── pdf.worker.min.js      # PDF.js Worker
│   └── package.json
├── build/                  # 构建配置
├── bin/                    # 编译输出目录
└── Taskfile.yml            # 任务自动化配置
```

## 🚀 快速开始

### 方式一：直接使用

1. 下载最新版本的 `hreader.exe`
2. 双击运行即可
3. 在应用内设置页面配置你的 API Key


---

### 方式二：从源码编译

#### 环境要求

- **Go**: 1.25 或更高版本
- **Node.js**: 18.x 或更高版本（用于前端构建）
- **Wails CLI**: `go install github.com/wailsapp/wails/v3/cmd/wails3@latest`

#### 编译步骤

```bash
# 1. 克隆仓库
git clone <repository-url>
cd hreader

# 2. 一键编译
wails3 build

# 编译完成后，可执行文件位于 bin/ 目录
```


### 方式三：开发调试

```bash
# 启动开发模式（支持热重载）
wails3 dev

# Wails 会自动：
# - 安装前端依赖
# - 启动 Vite 开发服务器
# - 编译并运行 Go 后端
# - 实现前后端热重载
```

---


## 📚 功能详解

### PDF 阅读器

#### 核心功能
- **懒加载渲染**: 使用 Intersection Observer API 实现按需渲染，提升性能
- **高分辨率支持**: 自动适配 Retina/高 DPI 屏幕
- **文本层**: 支持文本选择和复制
- **平滑滚动**: 优化的滚动体验和页码跟踪

#### 交互操作
- **缩放**: 
  - Ctrl/Cmd + 滚轮缩放
  - 放大/缩小按钮
  - 适应宽度
- **导航**:
  - 滚动自动切换页码
  - 跳转到指定页
  - 上一页/下一页
- **截图**:
  - 整页截图
  - 区域框选截图（拖动鼠标选择区域）

### AI 聊天

- **流式响应**: 实时显示 AI 回复
- **上下文管理**: 自动维护对话历史
- **Markdown 支持**: 格式化显示代码和文本
- **本地存储**: 聊天记录持久化保存


## 📄 许可证

本项目采用 Apache3 许可证 - 详见 [LICENSE](LICENSE) 文件

## 🙏 致谢

- [Wails](https://wails.io/) - 优秀的 Go 桌面应用框架
- [Vue.js](https://vuejs.org/) - 渐进式 JavaScript 框架
- [PDF.js](https://mozilla.github.io/pdf.js/) - Mozilla 的 PDF 渲染库
- [pdfjs-dist](https://www.npmjs.com/package/pdfjs-dist) - PDF.js 的 NPM 包

---

<div align="center">

**Made with ❤️ using Wails + Vue.js**

如有问题或建议，欢迎提 Issue！

</div>
