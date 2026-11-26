# H-Reader 项目结构

## 目录概览

```text
hreader/
├── main.go                     # 注册 Wails 服务、创建桌面窗口并嵌入前端构建产物
├── greetservice.go             # Wails 示例服务
├── services/
│   ├── book/                   # 书架、阅读进度、大头针、笔记、截图剪贴板
│   ├── chat/                   # 侧栏会话、页内对话、流式请求、会话存储
│   ├── config/                 # 应用设置、API 供应商及模型配置
│   └── dictionary/             # 内置 ECDICT 词库和离线查询
├── llmkit/                     # Go 模块内的 LLM 客户端和会话工具包
├── frontend/
│   ├── src/
│   │   ├── main.js             # Vue 应用入口
│   │   ├── App.vue             # 书架、阅读器、聊天侧栏和设置的组装
│   │   ├── features/
│   │   │   ├── Bookshelf/      # 书架、书籍网格和首次启动引导
│   │   │   ├── Reader/         # PDF 阅读器、选文菜单和页内交互
│   │   │   ├── ChatPanel/      # 侧栏会话列表、消息展示和输入
│   │   │   └── SettingsMenu.vue # 应用设置及 AI 供应商和模型选择
│   │   ├── components/        # 图片预览和操作确认组件
│   │   └── composables/       # 应用级设置、阅读器控制、截图交付和标注导航
│   ├── bindings/              # Wails 生成的前端绑定
│   ├── public/                # PDF.js Worker、全局样式、字体和第三方许可证
│   └── vite.config.js         # Vite、Vue 和 Wails 插件配置
├── build/                      # Wails 配置、各平台构建任务和打包资源
├── Taskfile.yml                 # build、run、dev、package 等任务入口
├── go.mod
└── go.sum
```

构建生成 `frontend/dist/` 和 `bin/`。开发模式读取 `build/config.yml`，桌面应用通过 Vite 加载前端页面。

## 后端模块

| 模块 | 主要入口和职责 |
| --- | --- |
| `services/book/` | `bookservice.go` 管理书籍目录与 PDF 文件访问，`reading_progress.go` 保存阅读位置，`annotation_service.go` 保存关联侧栏消息的大头针，`note_service.go` 管理页内笔记。截图相关文件提供 Go 端剪贴板接口。 |
| `services/chat/` | `service.go` 提供 Wails 聊天接口。侧栏会话由 `session_manager.go`、`global_session.go`、`message_handler.go` 和 `store.go` 协作处理。`inline_conversation.go` 管理页内对话。`config_manager.go` 负责聊天服务的模型配置。 |
| `services/config/` | `config.go` 负责应用配置的读取与写入，`api_profiles.go` 定义 API 供应商、模型、侧栏与页内模型选择及旧配置迁移。 |
| `services/dictionary/` | `dictionary.go` 读取嵌入的 `ecdict.csv`，为 PDF 选文提供离线英文查词。 |
| `llmkit/` | `client.go` 封装 OpenAI 兼容客户端，`conversation.go` 和 `traced_conversation.go` 管理对话。上下文窗口、预算压缩和流式超时分别由 `context_window.go`、`context_budget.go` 和 `stream_timeout.go` 实现。 |

## 前端入口

| 功能 | 主要文件 |
| --- | --- |
| PDF 加载和渲染 | `features/Reader/Reader.vue` 组装阅读器。`usePdfDocument.js` 加载 PDF 并配置 Worker，`usePdfPages.js` 和 `PdfPageShell.vue` 负责页面渲染。 |
| 阅读和选文交互 | 阅读器的 `composables/` 包含缩放、滚动、阅读进度、公式导航及截图逻辑。`SelectionActionMenu.vue` 展示选文操作和查词结果。 |
| 页内标记 | `AnnotationPin.vue` 显示关联侧栏消息的大头针。`NoteMarker.vue` 显示页内笔记和对话标记。`useNotes.js` 管理笔记，`useInlineAsk.js` 与 `InlineAskPopup.vue` 管理页内对话。 |
| 右侧栏对话 | `features/ChatPanel/ChatPanel.vue` 组装会话列表、输入区和消息展示，邻近的 `composables/` 负责会话切换、输入和流式响应。 |
| 截图交付 | 阅读器下的 `useScreenshot.ts` 生成截图，公共 `composables/useScreenshot.js` 负责复制到剪贴板及向已打开的侧栏添加图片附件。 |

前端通过 `frontend/bindings/` 调用 `main.go` 中注册的 Wails 服务。修改 Go 服务接口后，构建任务会重新生成绑定。

## 本地数据

| 默认位置 | 用途 |
| --- | --- |
| `~/.hreader/` | 阅读进度 `reading_progress.json`、按书籍存储的 `annotations_<book_hash>.json` 和 `notes_<book_hash>.json`。 |
| `<用户配置目录>/hreader/config.json` | 应用设置、书架路径、API 供应商凭据及模型选择。 |
| `<用户配置目录>/hreader/chat_store/scopes/` | `library/` 存放书架侧栏会话，`books/<book_hash>/` 存放单本书的侧栏会话和 `inline_sessions/` 页内对话。 |

用户配置目录由 Go 的 `os.UserConfigDir()` 决定。PDF 保存在用户选定的书籍目录中。
