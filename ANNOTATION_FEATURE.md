# PDF 大头针标注功能使用说明

## 功能概述

PDF 大头针功能允许你在阅读 PDF 时，将特定的 AI 对话与 PDF 的某个位置关联起来。双击大头针可以快速跳转到对应的 AI 对话。

## 核心特性

1. **位置标记**：在 PDF 任意位置放置大头针
2. **会话关联**：每个大头针对应一个 AI 会话和消息
3. **双向导航**：双击大头针 → 跳转到对应对话
4. **持久化存储**：基于 PDF 内容哈希保存，移动文件后依然有效
5. **坐标归一化**：自动适配不同缩放比例

## 使用方法

### 创建大头针（待实现）

目前大头针的基础设施已完成，但创建交互还需要在 ChatPanel 中添加按钮。计划中的使用流程：

1. 在 ChatPanel 的消息旁边点击"📌 标记位置"按钮
2. PDF 阅读器进入选择模式（鼠标变成十字准星）
3. 点击 PDF 页面的某个位置
4. 大头针自动创建并保存

### 查看大头针

- 大头针会显示在 PDF 页面上对应的位置
- Hover 时显示提示信息
- 只显示当前页的大头针

### 删除大头针

- 右键点击大头针
- 确认删除

### 导航到对话

- 双击大头针
- 自动打开聊天面板
- 切换到对应的会话
- 滚动到对应的消息

## 技术实现

### 后端（Go）

**文件**：`services/book/annotation_service.go`

**数据结构**：
```go
type Annotation struct {
    ID           string  // 唯一标识（UUID）
    SessionID    string  // 会话 ID
    MessageIndex int     // 消息索引
    PageNum      int     // PDF 页码
    X            float64 // X 坐标（归一化 0-1）
    Y            float64 // Y 坐标（归一化 0-1）
    CreatedAt    int64   // 创建时间戳
    Note         string  // 备注（可选）
}
```

**API 方法**：
- `AddAnnotation(bookPath, annotation)` - 添加/更新大头针
- `GetAnnotations(bookPath)` - 获取所有大头针
- `RemoveAnnotation(bookPath, id)` - 删除大头针
- `UpdateAnnotation(bookPath, annotation)` - 更新大头针

**存储方式**：
- 文件路径：`~/.hreader/annotations_{sha1_hash}.json`
- 基于 PDF 内容 SHA1 哈希，确保同一本书的不同副本共享大头针

### 前端（Vue）

**Composable**：`frontend/src/features/Reader/composables/useAnnotations.js`

提供的方法：
- `loadAnnotations()` - 加载大头针
- `addAnnotation(pageNum, x, y, sessionId, messageIndex, pageWidth, pageHeight)` - 添加大头针
- `removeAnnotation(id)` - 删除大头针
- `startSelectionMode(sessionId, messageIndex)` - 开始选择模式
- `handlePageClick(...)` - 处理页面点击创建大头针
- `getCurrentPageAnnotations` - 当前页的大头针列表（computed）

**组件**：
- `AnnotationPin.vue` - 大头针显示组件
- `PdfPageShell.vue` - 集成大头针渲染
- `Reader.vue` - 管理大头针状态和事件

### 坐标系统

**归一化存储**：
- 存储时使用 0-1 范围的归一化坐标
- `normalizedX = pixelX / pageWidth`
- `normalizedY = pixelY / pageHeight`

**渲染时还原**：
- `actualX = normalizedX * currentPageWidth`
- `actualY = normalizedY * currentPageHeight`

这样无论用户如何缩放 PDF，大头针都能准确显示在正确的位置。

## 待完成的工作

### Phase 3: ChatPanel 集成

需要在 ChatPanel 的消息组件中添加"标记位置"按钮：

1. 在 `MessageDisplay.vue` 中为每条消息添加按钮
2. 点击按钮调用 `readerRef.value.startSelectionMode(sessionId, messageIndex)`
3. 显示选择模式的视觉反馈

### 优化项

1. **消息定位**：目前使用消息索引，建议改为使用时间戳或其他唯一标识
2. **视觉优化**：改进大头针样式，支持自定义颜色和图标
3. **批量管理**：添加大头针管理界面，支持批量删除、编辑备注
4. **搜索过滤**：支持按会话、关键词搜索大头针
5. **导出导入**：支持导出/导入大头针数据

## 注意事项

1. **重新编译**：修改 Go 后端后需要运行 `wails build` 重新生成 bindings
2. **依赖安装**：已安装 `uuid` 包用于生成唯一 ID
3. **并发安全**：后端使用 `sync.RWMutex` 保证并发安全
4. **错误处理**：渲染任务取消时会静默处理 `RenderingCancelledException`

## 测试建议

1. 打开一个 PDF 文件
2. 手动创建一些测试大头针（可以通过控制台调用 API）
3. 验证大头针是否正确显示
4. 测试双击导航功能
5. 测试缩放后大头针位置是否正确
6. 测试删除功能

## 示例代码（手动测试）

```javascript
// 在浏览器控制台中测试
const reader = document.querySelector('[ref="readerRef"]').__vueParentComponent.ctx.exposed;

// 模拟创建大头针
await reader.addAnnotation(
  1,              // 页码
  100,            // X 坐标
  200,            // Y 坐标
  'session-123',  // 会话 ID
  5,              // 消息索引
  600,            // 页面宽度
  840             // 页面高度
);
```

---

**创建时间**：2026-07-08  
**版本**：v1.0 (基础架构完成)
