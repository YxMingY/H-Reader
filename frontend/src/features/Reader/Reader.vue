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
  useScreenshot
} from './composables';

// 导入页面渲染组件
import PdfPageShell from './components/PdfPageShell.vue';

// 定义组件属性
const props = defineProps({
  pdfSource: String, // PDF 文件路径或 URL
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

/**
 * 加载 PDF 文档
 * @param {string} source - PDF 文件路径或 URL
 */
const loadPdf = async (source) => {
  try {
    // 清除之前的页面数据
    await ClearPages();
    // 加载新的 PDF 文档
    await LoadPdfDocument(source);
    await nextTick();
    // 重置滚动位置
    ResetLastScrollTop();
    // 自适应宽度
    FitWidth();
    // 使用第一页高度预先设置所有页面容器高度，确保滚动条位置正确
    await InitPageHeights();
    // 等页面尺寸就位后再挂观察器，避免它过早读取到不完整的布局
    SetupIntersectionObserver();
    // 触发加载完成事件
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