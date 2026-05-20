<script setup>
/**
 * Reader.vue - PDF 阅读器主组件
 * 
 * 该组件负责管理整个 PDF 阅读体验，包括：
 * - PDF 文档加载和解析
 * - 页面渲染和管理
 * - 缩放控制
 * - 滚动和页面导航
 * - 截图功能
 */
import { ref, nextTick, onMounted, onBeforeUnmount, watch, markRaw } from 'vue';

// 导入各个功能模块的 composables
import { 
  usePdfDocument,
  usePdfPages,
  useScaleAdjust,
  useScrollPage,
  useScreenshot,
  useReadingProgress
} from './composables';

// 导入页面渲染组件
import PdfPageShell from './components/PdfPageShell.vue';

// 定义组件属性
const props = defineProps({
  pdfSource: String,   // PDF 文件路径或 URL
  bookPath: String,    // 书籍文件路径（用于保存阅读进度）
});

// 定义组件事件
const emit = defineEmits(['loaded', 'pagechange']);

// DOM 元素引用
const readerContainer = ref(null); // 阅读器容器元素
const pdfCanvas = ref(null); // PDF 画布容器元素(所有画布)

// PDF 文档管理 - 负责加载和解析 PDF 文件
const { pdfDoc, totalPages, LoadPdfDocument } = usePdfDocument();

// 页面管理与渲染 - 负责页面的创建、渲染和生命周期管理
const { 
  RegisterPageRefs,    // 注册页面 DOM 引用(所有页面)
  ClearPages,          // 清除所有页面
  InitPageHeights,     // 初始化页面高度
  SetupIntersectionObserver, // 设置交叉观察器用于懒加载
  RerenderVisiblePages,// 重新渲染可见页面
  pageCanvasRefs,      // 页面 Canvas 引用集合
  pageTextLayerRefs,   // 页面文本层引用集合
  pageContainerRefs,   // 页面容器引用集合
  renderingPages,      // 正在渲染的页面集合
  currentPages,        // 当前可见页面集合
  pageWidth,           // 页面宽度（用于缩放）
 } = usePdfPages(pdfDoc, totalPages, readerContainer, pdfCanvas);

 // 缩放管理 - 处理页面缩放相关功能
 const { 
  FitWidth,    // 适应宽度
  HandleWheel, // 处理滚轮事件
  ZoomIn,      // 放大
  ZoomOut,     // 缩小
  HandleResize,// 处理窗口大小变化
} = useScaleAdjust(RerenderVisiblePages, readerContainer, pageWidth, emit);

// 滚动管理 - 处理页面滚动和导航
const {
  currentPage,       // 当前页码
  GoToPage,          // 跳转到指定页
  GoToNextPage,      // 下一页
  GoToPrevPage,      // 上一页
  ResetLastScrollTop,// 重置滚动位置记录
} = useScrollPage({ readerContainer, totalPages, pdfCanvas, pageContainerRefs, emit });

// 截图管理 - 提供页面截图功能
const { captureCurrentPage, startAreaSelection } = useScreenshot(currentPage, pageCanvasRefs);

// ========================================
// 阅读进度管理
// ========================================
// 
// 实现思路：
// 1. 使用 PDF 内容哈希（SHA1）作为唯一标识，而非文件路径
//    - 优势：即使文件被移动或重命名，阅读进度依然有效
//    - 相同内容的文件共享同一个进度
// 
// 2. 等待 PDF 加载完成后再翻页
//    - 避免在 DOM 未就绪时翻页导致的问题
//    - 通过 isPdfLoaded 标记和 watch 机制实现
// 
// 3. 自动保存（带防抖）
//    - 用户翻页后延迟 2 秒保存，避免频繁写入
//    - 如果在此期间再次翻页，重置定时器
// 
// 4. 组件卸载时清理定时器，防止内存泄漏

/**
 * bookPathRef - 书籍路径的响应式引用
 * 
 * 用途：
 * - 跟踪当前打开的书籍文件路径
 * - 用于计算 SHA1 哈希并关联阅读进度
 * 
 * 注意：
 * - 从 props.bookPath 初始化
 * - 通过 watch 监听 props 变化并同步更新
 */
const bookPathRef = ref(props.bookPath || '');

/**
 * 解构阅读进度管理的方法
 * 
 * restoreProgress: 恢复上次阅读位置（在 PDF 加载前调用）
 * markPdfLoaded: 标记 PDF 已加载完成（在 PDF 加载后调用）
 * setupAutoSave: 设置自动保存监听器（在 PDF 加载后调用）
 * 
 * 参数说明：
 * - bookPathRef: 书籍路径的响应式引用
 * - currentPage: 当前页码的响应式引用
 * - { value: null }: readerRef 占位符（本组件不需要直接调用 GoToPage）
 */
const {
  restoreProgress,
  markPdfLoaded,
  setupAutoSave,
} = useReadingProgress(bookPathRef, currentPage, { value: null });

/**
 * loadPdf - 加载 PDF 文档并恢复阅读进度
 * 
 * 执行流程：
 * 1. 清除之前的页面数据（ClearPages）
 * 2. 加载新的 PDF 文档（LoadPdfDocument）
 * 3. 等待 Vue 更新 DOM（nextTick）
 * 4. 重置滚动位置记录（ResetLastScrollTop）
 * 5. 自适应宽度（FitWidth）
 * 6. 初始化所有页面容器高度（InitPageHeights）
 *    - 使用第一页高度预先设置，确保滚动条位置正确
 * 7. 设置交叉观察器（SetupIntersectionObserver）
 *    - 用于懒加载，只渲染可见区域的页面
 * 8. 标记 PDF 已加载完成（markPdfLoaded）
 *    - 允许后续的翻页操作
 * 9. 恢复阅读进度（restoreProgress + GoToPage）
 *    - 如果有保存的进度且页码 > 1，跳转到该页
 *    - 等待 nextTick 确保 DOM 完全渲染后再翻页
 * 10. 设置自动保存（setupAutoSave）
 *     - 监听页码变化，自动保存阅读进度
 * 11. 触发加载完成事件（emit 'loaded'）
 * 
 * @param {string} source - PDF 文件路径或 URL
 */
const loadPdf = async (source) => {
  try {
    // 步骤 1-2: 清除旧数据并加载新 PDF
    await ClearPages();
    await LoadPdfDocument(source);
    await nextTick();
    
    // 步骤 3-7: 初始化和布局
    ResetLastScrollTop();
    FitWidth();
    await InitPageHeights();
    SetupIntersectionObserver();
    
    // 步骤 8: 标记 PDF 已加载完成
    markPdfLoaded();
    
    // 步骤 9: 恢复阅读进度（如果有保存的进度）
    if (bookPathRef.value) {
      const savedPage = await restoreProgress();
      if (savedPage > 1) {
        // 等待下一帧再翻页，确保 DOM 完全渲染
        await nextTick();
        GoToPage(savedPage);
      }
    }
    
    // 步骤 10: 设置自动保存
    setupAutoSave();
    
    // 步骤 11: 触发加载完成事件
    emit('loaded', pdfDoc.value);
  } catch (err) {
    console.error('PDF 加载失败:', err);
  }
};

// 监听 pdfSource 属性变化，自动重新加载 PDF
watch(() => props.pdfSource, async (newSource) => {
  if (newSource) {
    await loadPdf(newSource);
  }
});

// ========================================
// 监听 bookPath 变化
// ========================================
// 
// 用途：
// - 当父组件（App.vue）切换书籍时，更新 bookPathRef
// - 确保阅读进度管理与当前书籍保持同步
// 
// 场景：
// - 用户在书架点击另一本书
// - App.vue 更新 currentBookPath
// - Reader 组件接收到新的 bookPath prop
// - 触发此 watch，更新 bookPathRef
watch(() => props.bookPath, (newBookPath) => {
  bookPathRef.value = newBookPath || '';
});

// 组件挂载时加载 PDF
onMounted(() => {
  if (props.pdfSource) {
    loadPdf(props.pdfSource);
  }
});

// 暴露给父组件的方法和属性
defineExpose({
  pdfDoc,           // PDF 文档对象
  totalPages,       // 总页数
  currentPage,      // 当前页码
  ZoomIn,           // 放大方法
  ZoomOut,          // 缩小方法
  FitWidth,         // 适应宽度方法
  GoToPage,         // 跳转页面方法
  GoToNextPage,     // 下一页方法
  GoToPrevPage,     // 上一页方法
  captureCurrentPage,   // 截取当前页方法
  startAreaSelection,   // 开始区域选择方法
});

</script>
<template>
  <div class="reader-view" ref="readerContainer">
    <div class="pdf-canvas-container" ref="pdfCanvas">
      <PdfPageShell
      v-for="pageNum in totalPages"
      :key="pageNum"
      :pageNum="pageNum"
      :register="RegisterPageRefs"
      />
    </div>
  </div>
</template>


<style>
.reader-view {
  height: 100%;
  width: 100%;
  box-sizing: border-box;
  overflow-y: auto;
  overflow-x: auto; /* 允许左右滚动缩放后的内容 */
  background: #525659;
  display: flex;
  justify-content: center; /* 居中显示 */
  padding: 20px;
}

.pdf-canvas-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  min-width: 0;
}
</style>