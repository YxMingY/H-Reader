# 阅读进度记忆功能 - 实现说明

## 📋 功能概述

当用户从一本书离开时，自动记住离开时的页码，下次进入这本书就回到这个页码。

---

## 🎯 核心设计思路

### 1. 使用 PDF 内容哈希而非文件路径

**为什么？**
- ✅ 即使用户移动或重命名 PDF 文件，阅读进度依然有效
- ✅ 同一本书的不同副本（相同内容）共享同一个进度
- ✅ 避免路径变化导致的进度丢失问题

**如何实现？**
```go
// 后端计算 SHA1 哈希
hash := sha1.Sum(fileData)
bookHash := hex.EncodeToString(hash[:])  // 40字符十六进制字符串
```

---

### 2. 等待 PDF 加载完成后再翻页

**为什么？**
- ❌ 如果在 DOM 未就绪时翻页，可能导致页面渲染错误
- ❌ PDF.js 需要时间解析和渲染页面
- ✅ 确保用户体验流畅，无闪烁或错位

**如何实现？**
```javascript
// 前端使用标记 + watch 机制
const isPdfLoaded = ref(false);

// 1. PDF 加载完成后设置标记
markPdfLoaded();  // isPdfLoaded.value = true

// 2. 翻页前检查标记
if (!isPdfLoaded.value) {
  await new Promise((resolve) => {
    watch(isPdfLoaded, (loaded) => {
      if (loaded) resolve();
    });
  });
}
```

---

### 3. 自动保存（带防抖）

**为什么？**
- ❌ 每次翻页都保存会导致频繁的文件 I/O
- ❌ 影响性能和用户体验
- ✅ 延迟 2 秒保存，如果期间再次翻页则重置定时器

**如何实现？**
```javascript
let saveProgressTimer = null;

const debouncedSaveProgress = (page) => {
  // 清除之前的定时器
  if (saveProgressTimer) {
    clearTimeout(saveProgressTimer);
  }
  
  // 设置新的定时器（2秒后保存）
  saveProgressTimer = setTimeout(() => {
    BookService.SaveReadingProgress(bookPath, page);
  }, 2000);
};

// 监听页码变化
watch(currentPage, (newPage) => {
  if (isPdfLoaded.value && newPage > 1) {
    debouncedSaveProgress(newPage);
  }
});
```

---

### 4. 组件卸载时立即保存并清理资源

**为什么？**
- ❌ 定时器未清理会导致内存泄漏
- ❌ 可能触发已卸载组件的状态更新
- ❌ **如果用户快速离开，防抖定时器可能被清除但未保存**
- ✅ **卸载时立即保存当前页码，确保数据不丢失**

**如何实现？**
```javascript
onBeforeUnmount(() => {
  // 步骤 1: 清除防抖定时器（避免重复保存）
  if (saveProgressTimer) {
    clearTimeout(saveProgressTimer);
    saveProgressTimer = null;
  }
  
  // 步骤 2: 立即保存当前页码
  if (isPdfLoaded.value && currentPage.value > 1 && bookPath.value) {
    console.log(`[阅读进度] 离开书籍，立即保存`);
    BookService.SaveReadingProgress(bookPath.value, currentPage.value);
  }
});
```

---

## 🏗️ 架构设计

### 数据流

```
用户操作 → Reader.vue → useReadingProgress → BookService → Backend
   ↓                                              ↓
翻页事件 → debouncedSaveProgress → SaveReadingProgress → CalculateFileHash
                                                        ↓
                                                   progressStore.SaveProgress
                                                        ↓
                                                   异步写入 JSON 文件
```

### 恢复流程

```
打开书籍 → App.vue.openBook() → Reader.vue.loadPdf()
                                    ↓
                              LoadPdfDocument()
                                    ↓
                              markPdfLoaded()
                                    ↓
                              restoreProgress() → BookService.GetReadingProgress
                                                      ↓
                                                 CalculateFileHash
                                                      ↓
                                                 progressStore.GetProgress
                                                      ↓
                                                 GoToPage(savedPage)
```

---

## 📁 文件结构

### 后端（Go）

```
f:\hreader\
├── bookservice.go          # 阅读进度 API
│   ├── ReadingProgress     # 数据结构
│   ├── readingProgressStore # 存储管理器
│   ├── CalculateFileHash()  # 计算 SHA1 哈希
│   ├── SaveReadingProgress() # 保存进度（对外 API）
│   └── GetReadingProgress()  # 获取进度（对外 API）
└── main.go
    └── initProgressStore()  # 初始化存储
```

### 前端（Vue 3）

```
f:\hreader\frontend\src\features\Reader\
├── Reader.vue              # 阅读器主组件
│   ├── loadPdf()           # 加载 PDF 并恢复进度
│   └── watch bookPath      # 监听书籍切换
└── composables\
    ├── useReadingProgress.js  # 阅读进度管理
    │   ├── restoreProgress()  # 恢复进度
    │   ├── markPdfLoaded()    # 标记加载完成
    │   ├── setupAutoSave()    # 设置自动保存
    │   └── debouncedSaveProgress() # 防抖保存
    └── index.js            # 统一导出
```

---

## 🔑 关键代码片段

### 后端：数据结构

```go
type ReadingProgress struct {
    BookHash  string `json:"book_hash"`   // SHA1 哈希（主键）
    BookPath  string `json:"book_path"`   // 文件路径（辅助信息）
    Page      int    `json:"page"`        // 页码
    UpdatedAt string `json:"updated_at"`  // 更新时间
}
```

### 后端：保存进度

```go
func (a *BookService) SaveReadingProgress(bookPath string, page int) error {
    // 1. 计算文件哈希
    bookHash, err := CalculateFileHash(bookPath)
    if err != nil {
        return err
    }
    
    // 2. 保存到内存并异步持久化
    return progressStore.SaveProgress(bookHash, bookPath, page)
}
```

### 前端：恢复进度

```javascript
const restoreProgress = async () => {
  if (!bookPath.value) {
    return 1;
  }

  try {
    const savedPage = await BookService.GetReadingProgress(bookPath.value);
    console.log(`[阅读进度] 恢复: ${bookPath.value} - 第 ${savedPage} 页`);
    return savedPage;
  } catch (err) {
    console.error('[阅读进度] 获取失败:', err);
    return 1;
  }
};
```

### 前端：加载流程

```javascript
const loadPdf = async (source) => {
  // 1. 加载 PDF
  await LoadPdfDocument(source);
  
  // 2. 初始化和布局
  await InitPageHeights();
  SetupIntersectionObserver();
  
  // 3. 标记加载完成
  markPdfLoaded();
  
  // 4. 恢复阅读进度
  if (bookPathRef.value) {
    const savedPage = await restoreProgress();
    if (savedPage > 1) {
      await nextTick();
      GoToPage(savedPage);
    }
  }
  
  // 5. 设置自动保存
  setupAutoSave();
};
```

---

## 📊 存储格式

### JSON 文件位置

```
~/.hreader/reading_progress.json
```

### 文件内容示例

```json
{
  "a1b2c3d4e5f6...": {
    "book_hash": "a1b2c3d4e5f6...",
    "book_path": "C:\\books\\test.pdf",
    "page": 42,
    "updated_at": "2026-05-16T10:30:00+08:00"
  },
  "f7e8d9c0b1a2...": {
    "book_hash": "f7e8d9c0b1a2...",
    "book_path": "D:\\documents\\paper.pdf",
    "page": 15,
    "updated_at": "2026-05-16T09:15:00+08:00"
  }
}
```

---

## 🧪 测试场景

### 场景 1：正常阅读

1. 打开书籍 A，翻到第 10 页
2. 返回书架
3. 再次打开书籍 A
4. ✅ 应该自动跳转到第 10 页

### 场景 2：文件移动

1. 打开书籍 B，翻到第 20 页
2. 在文件管理器中移动书籍 B 到其他文件夹
3. 重新扫描书架
4. 打开书籍 B
5. ✅ 应该自动跳转到第 20 页（因为哈希值不变）

### 场景 3：快速翻页

1. 打开书籍 C
2. 快速连续翻页：1 → 5 → 10 → 15
3. 等待 2 秒
4. 检查 JSON 文件
5. ✅ 应该只保存最后一次翻页（第 15 页）

### 场景 3.5：快速离开

1. 打开书籍 D，翻到第 25 页
2. **立即点击返回按钮**（不等防抖定时器触发）
3. 再次打开书籍 D
4. ✅ 应该跳转到第 25 页（卸载时立即保存）

### 场景 4：多本书切换

1. 打开书籍 A，翻到第 5 页
2. 返回书架，打开书籍 B，翻到第 8 页
3. 返回书架，再次打开书籍 A
4. ✅ 应该跳转到第 5 页（书籍 A 的进度）
5. 返回书架，再次打开书籍 B
6. ✅ 应该跳转到第 8 页（书籍 B 的进度）

---

## ⚠️ 注意事项

### 1. 性能优化

- ✅ 使用异步保存，不阻塞 UI
- ✅ 防抖机制减少文件 I/O
- ✅ 内存缓存，读取速度快

### 2. 容错处理

- ✅ 文件不存在时返回默认值 1
- ✅ 保存失败只打印日志，不影响用户体验
- ✅ JSON 解析失败时使用空进度

### 3. 并发安全

- ✅ 使用 `sync.RWMutex` 保护并发访问
- ✅ 读锁允许多个 goroutine 同时读取
- ✅ 写锁保证数据一致性

### 4. 内存管理

- ✅ 组件卸载时清理定时器
- ✅ **组件卸载时立即保存进度，防止数据丢失**
- ✅ 避免内存泄漏

---

## 🚀 未来优化方向

### 1. 云端同步

- 将阅读进度同步到云端
- 实现多设备间的进度同步

### 2. 阅读统计

- 记录每本书的阅读时长
- 生成阅读报告

### 3. 书签功能

- 支持多个书签点
- 书签备注和分类

### 4. 智能推荐

- 根据阅读进度推荐相关内容
- 提醒继续阅读

---

## 📝 总结

阅读进度记忆功能通过以下核心技术实现：

1. **SHA1 哈希标识**：确保文件移动后进度依然有效
2. **加载状态管理**：等待 PDF 就绪后再翻页
3. **防抖自动保存**：平衡性能和用户体验
4. **卸载时立即保存**：**防止快速离开时数据丢失**
5. **资源清理机制**：防止内存泄漏

整个实现遵循了 Vue 3 组合式 API 的最佳实践，代码清晰、可维护性强，并且具有良好的扩展性。
