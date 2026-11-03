# 项目结构详细说明

本文档详细描述 H-Reader 项目的目录结构和各文件功能。

## 目录树

```
hreader/
├── main.go                      # 应用入口
├── greetservice.go              # 示例服务
│
├── services/                    # 后端服务层
│   ├── book/                    # 书籍管理模块
│   │   ├── bookservice.go       # PDF 文件管理服务
│   │   ├── annotation_service.go # 大头针标注服务
│   │   └── reading_progress.go  # 阅读进度记录服务
│   ├── chat/                    # AI 聊天模块
│   │   ├── service.go           # 聊天服务主逻辑
│   │   ├── session_manager.go   # 会话管理
│   │   ├── global_session.go    # 全局会话状态
│   │   ├── store.go             # 消息持久化存储
│   │   ├── message_handler.go   # 消息处理逻辑
│   │   ├── config_manager.go    # 配置管理
│   │   └── types.go             # 数据类型定义
│   └── config/                  # 配置模块
│       └── config.go            # 应用配置管理
│
├── llmkit/                      # LLM 工具包（独立模块）
│   ├── client.go                # API 客户端封装
│   ├── conversation.go          # 对话管理核心
│   ├── traced_conversation.go   # 带追踪的对话
│   ├── message_builder.go       # 消息构建器
│   ├── trace_state.go           # 追踪状态管理
│   └── debug.go                 # 调试工具
│
├── frontend/                    # 前端应用
│   ├── src/
│   │   ├── App.vue              # 根组件
│   │   ├── main.js              # 入口文件
│   │   │
│   │   ├── features/            # 功能模块
│   │   │   ├── Reader/          # PDF 阅读器模块
│   │   │   │   ├── components/
│   │   │   │   │   ├── PdfPageShell.vue      # PDF 页面容器
│   │   │   │   │   ├── AnnotationPin.vue     # 大头针标注组件
│   │   │   │   │   └── FormulaPreview.vue    # 公式预览组件
│   │   │   │   ├── composables/
│   │   │   │   │   ├── usePdfDocument.js         # PDF 文档管理
│   │   │   │   │   ├── usePdfPages.js            # 页面渲染管理
│   │   │   │   │   ├── useScaleAdjust.js         # 缩放控制
│   │   │   │   │   ├── useScrollPage.js          # 滚动与导航
│   │   │   │   │   ├── useScreenshot.ts          # 截图功能
│   │   │   │   │   ├── useReadingProgress.js     # 阅读进度管理
│   │   │   │   │   ├── useAnnotations.js         # 大头针标注管理
│   │   │   │   │   ├── usePageDetection.ts       # 页面检测工具
│   │   │   │   │   ├── useFormulaNavigation.js   # 公式跳转导航
│   │   │   │   │   └── index.js                  # 统一导出
│   │   │   │   └── Reader.vue                    # 阅读器主组件
│   │   │   │
│   │   │   ├── Bookshelf/         # 书架模块
│   │   │   │   ├── components/
│   │   │   │   │   ├── BookGrid.vue          # 书籍网格展示
│   │   │   │   │   └── index.js
│   │   │   │   ├── composables/
│   │   │   │   │   ├── useLibrary.js         # 图书馆管理
│   │   │   │   │   └── index.js
│   │   │   │   ├── Bookshelf.vue             # 书架主组件
│   │   │   │   └── OnboardingGuide.vue       # 首次启动引导
│   │   │   │
│   │   │   ├── ChatPanel/         # AI 聊天面板模块
│   │   │   │   ├── components/
│   │   │   │   │   ├── SessionList.vue       # 会话列表
│   │   │   │   │   ├── MessageDisplay.vue    # 消息展示
│   │   │   │   │   ├── InputArea.vue         # 输入区域
│   │   │   │   │   └── index.js
│   │   │   │   ├── composables/
│   │   │   │   │   ├── useSession.js         # 会话管理
│   │   │   │   │   ├── useChatInput.js       # 输入管理
│   │   │   │   │   ├── useChatStream.js      # 流式响应
│   │   │   │   │   ├── useMessageActions.js  # 消息操作
│   │   │   │   │   ├── useScrollBottom.js    # 滚动控制
│   │   │   │   │   ├── useTools.js           # 工具函数
│   │   │   │   │   └── index.js
│   │   │   │   └── ChatPanel.vue             # 聊天面板主组件
│   │   │   │
│   │   │   └── SettingsMenu.vue   # 设置菜单
│   │   │
│   │   ├── components/            # 公共组件
│   │   │   ├── ImagePreview.vue   # 图片预览组件
│   │   │   └── ToastConfirm.vue   # Toast 确认框组件
│   │   │
│   │   └── composables/           # 公共组合式函数
│   │       ├── useAnnotationNavigation.js  # 标注导航
│   │       ├── useAppSettings.js           # 应用设置
│   │       ├── useChatPanel.js             # 聊天面板
│   │       ├── useImagePreview.js          # 图片预览
│   │       ├── useScreenshot.js            # 截图功能
│   │       ├── useToast.js                 # Toast 提示
│   │       └── index.js
│   │
│   ├── bindings/                # Wails 自动生成的绑定
│   └── public/
│       └── pdf.worker.min.js    # PDF.js Worker
│
├── build/                       # 构建配置
├── bin/                         # 编译输出
├── config.json                  # 运行时配置文件
├── Taskfile.yml                 # 任务自动化配置
├── go.mod                       # Go 模块依赖
├── go.sum                       # Go 依赖校验和
└── LICENSE                      # Apache 2.0 许可证
```

## 模块说明

### 后端服务层 (services/)

采用分层架构，按功能模块划分：

| 模块 | 职责 |
|------|------|
| `book/` | PDF 文件管理、大头针标注、阅读进度记录 |
| `chat/` | AI 对话会话管理、消息处理、持久化存储 |
| `config/` | 应用配置管理（API Key、模型设置等） |

### LLM 工具包 (llmkit/)

独立的 LLM 交互模块，提供：

- API 客户端封装（支持 OpenAI 兼容接口）
- 对话上下文管理
- 消息构建与格式化
- 请求追踪与调试

### 前端功能模块 (frontend/src/features/)

按功能域划分：

| 模块 | 职责 |
|------|------|
| `Reader/` | PDF 渲染、缩放、截图、标注、公式导航 |
| `Bookshelf/` | 书籍管理、书架展示、首次启动引导 |
| `ChatPanel/` | AI 对话、会话管理、消息展示与操作 |
| `SettingsMenu.vue` | 应用设置界面 |

### 公共组件 (frontend/src/components/)

跨模块复用的 UI 组件：

- `ImagePreview.vue` - 图片预览
- `ToastConfirm.vue` - 操作确认提示框

### 公共组合式函数 (frontend/src/composables/)

跨模块复用的业务逻辑：

- `useAnnotationNavigation.js` - 标注导航逻辑
- `useAppSettings.js` - 应用设置逻辑
- `useChatPanel.js` - 聊天面板逻辑
- `useImagePreview.js` - 图片预览逻辑
- `useScreenshot.js` - 截图逻辑
- `useToast.js` - Toast 提示逻辑
