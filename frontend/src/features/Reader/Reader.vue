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
  useReadingProgress,
  useAnnotations,
  useFormulaNavigation,
  findPageByPoint
} from './composables';

// 导入页面渲染组件
import PdfPageShell from './components/PdfPageShell.vue';
import AnnotationPin from './components/AnnotationPin.vue';
import FormulaPreview from './components/FormulaPreview.vue';

// 定义组件属性
const props = defineProps({
  pdfSource: String,   // PDF 文件路径或 URL
  bookPath: String,    // 书籍文件路径（用于保存阅读进度）
});

// 定义组件事件
const emit = defineEmits(['loaded', 'pagechange', 'navigate-to-message']);

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
// 书籍路径管理
// ========================================

/**
 * bookPathRef - 书籍路径的响应式引用
 * 
 * 用途：
 * - 跟踪当前打开的书籍文件路径
 * - 用于计算 SHA1 哈希并关联阅读进度和大头针
 * 
 * 注意：
 * - 从 props.bookPath 初始化
 * - 通过 watch 监听 props 变化并同步更新
 */
const bookPathRef = ref(props.bookPath || '');

// ========================================
// 公式导航管理
// ========================================

const {
  selectedFormula,
  showFormulaPopup,
  popupPosition,
  hasJumped,
  extractFormulas,
  setupSelectionListener,
  jumpToFormula,
  returnToPreviousPosition,
  cleanup: cleanupFormulaNav,
} = useFormulaNavigation({
  pdfDoc,
  totalPages,
  readerContainer,
  pageContainerRefs,
  pageCanvasRefs,
  GoToPage,
});

// ========================================
// 大头针标注管理
// ========================================
// 
// 实现思路：
// 1. 使用 PDF 内容哈希（SHA1）作为唯一标识，与阅读进度一致
// 2. 坐标归一化存储（0-1 范围），适配不同缩放比例
// 3. 支持双击导航到对应的 AI 对话
// 4. 选择模式：用户点击“标记位置”后，再点击 PDF 创建大头针

/**
 * 解构大头针管理的方法
 * 
 * loadAnnotations: 加载当前 PDF 的所有大头针
 * addAnnotation: 添加大头针
 * removeAnnotation: 删除大头针
 * getCurrentPageAnnotations: 获取当前页的大头针列表（computed）
 * startSelectionMode: 开始选择模式（等待用户点击 PDF）
 * cancelSelectionMode: 取消选择模式
 * handlePageClick: 处理 PDF 页面点击（在选择模式下创建大头针）
 * handlePinDoubleClick: 处理大头针双击事件
 */
const {
  annotations,
  isSelectingMode,
  getCurrentPageAnnotations,
  loadAnnotations,
  addAnnotation,
  removeAnnotation,
  startSelectionMode,
  cancelSelectionMode,
  handlePageClick,
  handlePinDoubleClick,
} = useAnnotations(bookPathRef, currentPage, (data) => {
  // 当用户双击大头针时，通知父组件进行导航
  emit('navigate-to-message', data);
});

// ========================================
// 阅读进度管理
// ========================================

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
    
    // 步骤 8.5: 加载大头针数据
    if (bookPathRef.value) {
      await loadAnnotations();
    }
    
    // 步骤 8.6: 提取公式编号并设置选择监听
    await extractFormulas();
    setupSelectionListener();
    
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

// ⭐ 关键：组件卸载时清理 PDF.js 创建的 hidden canvas
onBeforeUnmount(() => {
  // 清理公式导航监听
  cleanupFormulaNav();
  
  // 清理 PDF.js 创建的隐藏 canvas 元素
  const hiddenCanvases = document.querySelectorAll('.hiddenCanvasElement');
  hiddenCanvases.forEach(canvas => {
    canvas.remove();
  });
  console.log(`清理了 ${hiddenCanvases.length} 个隐藏的 canvas 元素`);
});

// ========================================
// 大头针辅助函数
// ========================================

/**
 * 处理阅读器点击（在选择模式下）
 */
const handleReaderClick = async (event) => {
  if (!isSelectingMode.value) return;
  
  // 如果点击的是大头针或其他 UI 元素，不处理
  if (event.target.closest('.annotation-pin') || event.target.closest('.selection-mode-hint')) {
    return;
  }
  
  // 使用工具函数查找点击位置所在的页面
  const pageInfo = findPageByPoint(
    event.clientX,
    event.clientY,
    pageCanvasRefs,
    totalPages.value
  );
  
  if (!pageInfo) {
    console.log('点击不在任何页面上，取消选择');
    cancelSelectionMode();
    return;
  }
  
  const { pageNum, canvas, rect } = pageInfo;
  
  // 计算点击位置相对于 canvas 的坐标
  const clickX = event.clientX - rect.left;
  const clickY = event.clientY - rect.top;
  
  console.log('创建大头针:', {
    pageNum,
    clickX,
    clickY,
    canvasWidth: rect.width,
    canvasHeight: rect.height
  });
  
  // 调用 handlePageClick 创建大头针
  await handlePageClick(pageNum, clickX, clickY, rect.width, rect.height);
};

/**
 * 处理右键菜单（在选择模式下取消）
 */
const handleReaderContextMenu = (event) => {
  if (!isSelectingMode.value) return;
  
  // 阻止默认右键菜单
  event.preventDefault();
  
  console.log('右键取消选择模式');
  cancelSelectionMode();
};

/**
 * 处理删除大头针
 */
const handleRemoveAnnotation = async (annotationId) => {
  try {
    await removeAnnotation(annotationId);
  } catch (err) {
    console.error('删除大头针失败:', err);
  }
};

/**
 * 获取指定页码的大头针列表
 */
const getPageAnnotations = (pageNum) => {
  const pins = annotations.value.filter(a => a.pageNum === pageNum);
  if (pins.length > 0) {
    console.log(`第 ${pageNum} 页有 ${pins.length} 个大头针:`, pins);
  }
  return pins;
};

/**
 * 获取指定页码的高度（从 canvas 中获取）
 */
const getPageHeight = (pageNum) => {
  const canvas = pageCanvasRefs[pageNum];
  if (canvas) {
    // 优先使用 canvas 的实际渲染尺寸
    // 如果 canvas 已经有 style.width 和 style.height，说明已经渲染过
    const styleWidth = canvas.style.width;
    const styleHeight = canvas.style.height;
    
    if (styleWidth && styleHeight && styleWidth.endsWith('px') && styleHeight.endsWith('px')) {
      const width = parseFloat(styleWidth);
      const height = parseFloat(styleHeight);
      
      // 根据当前 pageWidth 和已知的宽高比重新计算高度
      // 这样可以确保在缩放时高度正确更新
      if (width > 0) {
        const aspectRatio = height / width;
        return pageWidth.value * aspectRatio;
      }
    }
    
    // 降级方案：根据 pageWidth 和默认比例计算
    return pageWidth.value * 1.4;
  }
  return pageWidth.value * 1.4; // 默认 A4 比例
};

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
  // 大头针相关
  isSelectingMode,      // 是否处于选择模式
  getCurrentPageAnnotations, // 当前页的大头针列表（computed）
  startSelectionMode,     // 开始选择模式
  cancelSelectionMode,    // 取消选择模式
  handlePageClick,        // 处理页面点击（创建大头针）
  // 公式导航
  jumpToFormula,          // 跳转到公式位置
  returnToPreviousPosition, // 返回跳转前位置
});

</script>
<template>
  <div 
    class="reader-view" 
    ref="readerContainer"
    :class="{ 'selection-mode': isSelectingMode }"
    @click="handleReaderClick"
    @contextmenu="handleReaderContextMenu"
  >
    <!-- 选择模式提示 -->
    <div v-if="isSelectingMode" class="selection-mode-hint">
      👆 左键点击 PDF 页面放置大头针，右键取消
    </div>
    
    <div class="pdf-canvas-container" ref="pdfCanvas">
      <PdfPageShell
        v-for="pageNum in totalPages"
        :key="pageNum"
        :pageNum="pageNum"
        :register="RegisterPageRefs"
        :annotations="getPageAnnotations(pageNum)"
        :pageWidth="pageWidth"
        :pageHeight="getPageHeight(pageNum)"
        @pin-dblclick="handlePinDoubleClick"
        @pin-remove="handleRemoveAnnotation"
      />
    </div>
    
    <!-- 公式预览悬浮窗 -->
    <FormulaPreview
      v-if="showFormulaPopup && selectedFormula"
      :formula="selectedFormula"
      :position="popupPosition"
      :has-jumped="hasJumped"
      @jump="jumpToFormula"
      @return="returnToPreviousPosition"
      @close="showFormulaPopup = false"
    />
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
  display: block; /* 改为 block，避免 flex 布局导致的居中问题 */
  padding: 20px;
}

.pdf-canvas-container {
  display: inline-flex; /* 改为 inline-flex，让容器宽度根据内容自适应 */
  flex-direction: column;
  align-items: center; /* 保持页面水平居中 */
  min-width: 100%; /* 确保容器至少占满整个视口宽度 */
}

/* 选择模式样式 */
.reader-view.selection-mode {
  cursor: crosshair;
}

.selection-mode-hint {
  position: fixed;
  top: 20px;
  left: 50%;
  transform: translateX(-50%);
  padding: 12px 24px;
  background: rgba(0, 0, 0, 0.8);
  color: white;
  border-radius: 8px;
  font-size: 14px;
  z-index: 1000;
  pointer-events: none;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
}
</style>