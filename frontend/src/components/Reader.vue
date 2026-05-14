<script setup>
import { ref, nextTick, onMounted, onBeforeUnmount, watch, markRaw } from 'vue';
import * as pdfjsLib from 'pdfjs-dist';

// 配置 Worker（使用本地文件避免CORS问题）
pdfjsLib.GlobalWorkerOptions.workerSrc = '/pdf.worker.min.js';

const props = defineProps({
  pdfSource: String,
});

const emit = defineEmits(['loaded', 'pagechange']);

// 状态管理
const readerContainer = ref(null);
const pdfCanvas = ref(null);
const pdfDoc = ref(null);
const totalPages = ref(0);
const pageWidth = ref(600);
const currentPages = ref(new Set()); // 当前可见的页码
const renderingPages = ref(new Set()); // 正在渲染的页码，避免重复渲染
const currentPage = ref(1); // 当前显示的页码（用于翻页）

// 动态 refs：存储每一页的 canvas、textLayer 和 pageContainer，用 Vue refs 替代 querySelector
const pageCanvasRefs = {};
const pageTextLayerRefs = {};
const pageContainerRefs = {};

const PAGE_GAP = 20;
const RENDER_THROTTLE = 200;
const ZOOM_STEP = 1.1;
const MIN_PAGE_WIDTH = 200;
const MAX_PAGE_WIDTH = 1200;
let resizeTimeout = null;
let renderTimeout = null;
let scrollTimeout = null;
let lastScrollTop = 0;
let pageObserver = null;

// 加载 PDF 文档
const loadPdf = async (source) => {
  try {
    const pdf = await pdfjsLib.getDocument(source).promise;
    pdfDoc.value = markRaw(pdf);
    totalPages.value = pdf.numPages;
    currentPage.value = 1;
    currentPages.value.clear();
    renderingPages.value.clear();
    lastScrollTop = 0;
    
    // 清理旧的 refs
    Object.keys(pageCanvasRefs).forEach(key => delete pageCanvasRefs[key]);
    Object.keys(pageTextLayerRefs).forEach(key => delete pageTextLayerRefs[key]);
    Object.keys(pageContainerRefs).forEach(key => delete pageContainerRefs[key]);
    
    console.log(`PDF 加载成功，共 ${pdf.numPages} 页`);

    // 页面壳子现在交给 Vue 的模板渲染；这里等它完成挂载，再继续做尺寸和观察器初始化
    await initializePages();
    fitWidth();

    // 使用第一页高度预先设置所有页面容器高度，确保滚动条位置正确
    await prepareInitialHeights();

    // 等页面尺寸就位后再挂观察器，避免它过早读取到不完整的布局
    setupIntersectionObserver();

    emit('loaded', pdf);
  } catch (err) {
    console.error('PDF 加载失败:', err);
  }
};

// 初始化页面容器：现在只负责等 Vue 把模板里的页面壳子渲染出来
const initializePages = async () => {
  if (!pdfCanvas.value || !totalPages.value) return;

  await nextTick();
};

// 设置交叉观察器：只负责“哪些页进入可视范围后要开始渲染”，不负责决定当前页码
const setupIntersectionObserver = () => {
  if (pageObserver) {
    pageObserver.disconnect();
  }

  pageObserver = new IntersectionObserver((entries) => {
    entries.forEach(entry => {
      const pageNum = parseInt(entry.target.dataset.pageNum);
      
      if (entry.isIntersecting && !renderingPages.value.has(pageNum)) {
        currentPages.value.add(pageNum);
        renderPage(pageNum);
      } else if (!entry.isIntersecting) {
        currentPages.value.delete(pageNum);
      }
    });
  }, {
    rootMargin: '200px' // 提前200px开始加载
  });
  
  const pageContainers = pdfCanvas.value?.querySelectorAll('.pdf-page-container');
  pageContainers?.forEach(container => pageObserver.observe(container));
};

// 更新当前页码并通知外部：这是页码状态的唯一入口，避免多个地方各自改 currentPage
const setCurrentPage = (pageNum, notify = true) => {
  const nextPage = Math.min(Math.max(pageNum, 1), totalPages.value || 1);

  if (nextPage === currentPage.value) return;

  currentPage.value = nextPage;

  if (notify) {
    emit('pagechange', nextPage);
  }
};

// 根据滚动方向和页边界计算当前页码：向下时等上一页完全消失再切到下一页；向上时同理
const updateCurrentPageFromScroll = () => {
  if (!readerContainer.value || !pdfCanvas.value) return;

  const scrollTop = readerContainer.value.scrollTop;
  const maxScrollTop = Math.max(readerContainer.value.scrollHeight - readerContainer.value.clientHeight, 0);
  const containerRect = readerContainer.value.getBoundingClientRect();
  const scrollingDown = scrollTop > lastScrollTop;
  const scrollingUp = scrollTop < lastScrollTop;

  lastScrollTop = scrollTop;

  // 向下滚动时，只有当前页完全离开视口上边界，才进入下一页
  if (scrollingDown) {
    // 最后一页没有“下一页”可等它完全消失，所以滚动到底部时直接切到最后一页
    if (scrollTop >= maxScrollTop - 1) {
      setCurrentPage(totalPages.value);
      return;
    }

    while (currentPage.value < totalPages.value) {
      // 用 Vue refs 而不是 querySelector 获取当前页容器
      const currentContainer = pageContainerRefs[currentPage.value];
      if (!currentContainer) break;

      const currentRect = currentContainer.getBoundingClientRect();
      if (currentRect.bottom <= containerRect.top + 1) {
        setCurrentPage(currentPage.value + 1);
      } else {
        break;
      }
    }

    return;
  }

  // 向上滚动时，只有当前页完全离开视口下边界，才回到上一页
  if (scrollingUp) {
    // 第一页没有“上一页”可等它完全消失，所以滚动到顶部时直接切回第一页
    if (scrollTop <= 0) {
      setCurrentPage(1);
      return;
    }

    while (currentPage.value > 1) {
      // 用 Vue refs 而不是 querySelector 获取当前页容器
      const currentContainer = pageContainerRefs[currentPage.value];
      if (!currentContainer) break;

      const currentRect = currentContainer.getBoundingClientRect();
      if (currentRect.top >= containerRect.bottom - 1) {
        setCurrentPage(currentPage.value - 1);
      } else {
        break;
      }
    }
  }
};

const handleScroll = () => {
  // 滚动事件很密集，先取消上一帧的计算，把页码判断压到下一帧执行
  if (scrollTimeout) cancelAnimationFrame(scrollTimeout);
  scrollTimeout = requestAnimationFrame(updateCurrentPageFromScroll);
};

// 处理 Ctrl+滚轮缩放
let wheelTimeout = null;
const handleWheel = (event) => {
  // 检测 Ctrl 或 Cmd 键
  if (!event.ctrlKey && !event.metaKey) return;
  
  event.preventDefault();
  
  // 节流：避免频繁重新渲染
  if (wheelTimeout) return;
  
  wheelTimeout = setTimeout(() => {
    wheelTimeout = null;
  }, 100);
  
  if (event.deltaY < 0) {
    zoomIn();
  } else {
    zoomOut();
  }
};

const setPageWidth = (nextWidth) => {
  // 所有缩放路径都收敛到这里：先夹紧，再重渲染可见页，最后通知外部当前缩放结果。
  const clamped = Math.min(Math.max(nextWidth, MIN_PAGE_WIDTH), MAX_PAGE_WIDTH);
  if (clamped === pageWidth.value) return;
  pageWidth.value = clamped;
  rerenderVisiblePages();
  emit('rescale', clamped);
};

const zoomIn = () => {
  // 放大按钮与滚轮向上共用同一套缩放步长。
  setPageWidth(pageWidth.value * ZOOM_STEP);
};

const zoomOut = () => {
  // 缩小按钮与滚轮向下共用同一套缩放步长。
  setPageWidth(pageWidth.value / ZOOM_STEP);
};

// 计算页面的 viewport 和 outputScale
const computePageViewport = (page) => {
  const pageWidthInPoints = page.view[2] - page.view[0];  // PDF 页面宽度（单位：点, 也就是1/72英寸）
  const scale = pageWidth.value / pageWidthInPoints; // 每个点对应的像素数，pageWidth是我们设定的页面宽度（单位：像素）
  // 根据我们提供的scale计算viewport，viewport.width应该等于pageWidth.value
  //  viewport包含了页面的尺寸和变换信息，比如页面的宽高（单位：像素）
  const viewport = page.getViewport({ scale });
  // 高分辨率渲染（考虑 devicePixelRatio）
  // devicePixelRatio 是一个表示设备像素与CSS像素之间关系的值。对于普通屏幕，这个值通常是1；
  // 对于高DPI屏幕（如Retina显示屏），这个值可能是2或更高。这意味着在高DPI屏幕上，每个CSS像素实际上由多个物理像素组成。
  const outputScale = window.devicePixelRatio || 1;
  return { viewport, outputScale };
};

// 使用第1页的高度为所有页面容器预先设置高度，避免首屏滚动条先“跳一下”再稳定
const prepareInitialHeights = async () => {
  if (!pdfDoc.value || totalPages.value === 0) return;

  try {
    const firstPage = await pdfDoc.value.getPage(1);
    const { viewport } = computePageViewport(firstPage);

    // 用 Vue refs 遍历所有页面容器，而不是 querySelectorAll
    for (let i = 1; i <= totalPages.value; i++) {
      const container = pageContainerRefs[i];
      const canvas = pageCanvasRefs[i];
      if (container && canvas) {
        container.style.height = viewport.height + 'px';
        canvas.style.width = viewport.width + 'px';
        canvas.style.height = viewport.height + 'px';
      }
    }
  } catch (e) {
    console.warn('prepareInitialHeights failed', e);
  }
};

// 渲染单一页面：先算尺寸，再画 canvas，最后再补文本层
const renderPage = async (pageNum) => {
  if (renderingPages.value.has(pageNum) || !pdfDoc.value) return;
  
  // 标记正在渲染的页码，避免重复渲染
  renderingPages.value.add(pageNum);
  
  try {
    const page = await pdfDoc.value.getPage(pageNum);
    // 用 Vue refs 获取元素，而不是 querySelector
    const canvas = pageCanvasRefs[pageNum];
    const textLayer = pageTextLayerRefs[pageNum];
    const pageContainer = pageContainerRefs[pageNum];

    if (!canvas) return;

    const { viewport, outputScale } = computePageViewport(page);
    console.log(viewport);
    // canvas.width和canvas.height是画布的实际像素尺寸，canvas.style.width和canvas.style.height是画布在页面上的显示尺寸
    // 即前者决定了渲染的清晰度，后者决定了画布在页面上的大小。
    // 通过设置canvas.width和canvas.height为viewport的尺寸乘以outputScale，我们确保了在高DPI屏幕上渲染的清晰度。
    canvas.width = Math.floor(viewport.width * outputScale);
    canvas.height = Math.floor(viewport.height * outputScale);
    canvas.style.width = viewport.width + 'px';
    canvas.style.height = viewport.height + 'px';

    // context.setTransform()方法用于设置当前的变换矩阵。通过将outputScale应用于x和y轴，我们确保了在高DPI屏幕上渲染的内容被正确缩放，从而保持清晰度。
    const context = canvas.getContext('2d');
    // context.setTransform()的函数原型是：context.setTransform(a, b, c, d, e, f)，其中a、b、c、d、e、f分别对应变换矩阵的元素。通过将outputScale应用于a和d，我们实现了在x和y轴上的缩放。
    // 即，canvas.width和canvas.height调整了画布大小，而context.setTransform()确保了绘制的内容按照正确的比例缩放，从而在高DPI屏幕上保持清晰。
    context.setTransform(outputScale, 0, 0, outputScale, 0, 0);
    
    // 渲染页面，renderContext包含了canvas的2D渲染上下文和页面的viewport信息，pdf.js会根据这些信息将PDF页面渲染到canvas上。
    const renderContext = {
      canvasContext: context,
      viewport: viewport,
    };

    await page.render(renderContext).promise;

    // 渲染完成后再回写容器高度，防止页面之间重叠
    if (pageContainer) {
      pageContainer.style.height = canvas.style.height;
    }

    // 渲染文本层用于选择
    if (textLayer) {
      try {
        const textContent = await page.getTextContent();
        renderTextLayer(textContent, viewport, textLayer);
      } catch (e) {
        console.warn('renderTextLayer failed', e);
      }
    }

    console.log(`第 ${pageNum} 页渲染完成`);
  } catch (err) {
    console.error(`渲染第 ${pageNum} 页失败:`, err);
  } finally {
    renderingPages.value.delete(pageNum);
  }
};

/**
 * 渲染 PDF 文本层（DOM，用于选择/复制）
 * @param {Object} textContent - page.getTextContent() 的结果
 * @param {PDFPageViewport} viewport - 与 canvas 使用的同一个 viewport
 * @param {HTMLElement} textLayer - .pdf-text-layer 元素
 */
const renderTextLayer = (textContent, viewport, textLayer) => {
  if (!textLayer) return;

  textLayer.innerHTML = '';
  textLayer.style.width = viewport.width + 'px';
  textLayer.style.height = viewport.height + 'px';

  const viewportTransform = viewport.transform;

  for (const item of textContent.items) {
    if (!item.str) continue;

    const span = document.createElement('span');
    span.textContent = item.str;

    // PDF → viewport
    const tx = pdfjsLib.Util.transform(
      viewportTransform,
      item.transform
    );

    let [a, b, c, d, e, f] = tx;

    // ⭐ 关键：基线 → 左上角修正
    const ascent = item.height * 0.8;
    f -= ascent;
    // ⭐ 核心：只修正文字方向，不翻整页
     c = -c;
     d = -d;

    span.style.transform = `matrix(${a}, ${b}, ${c}, ${d}, ${e}, ${f})`;
    span.style.transformOrigin = '0 0';

    const font = textContent.styles[item.fontName];
    if (font?.fontFamily) {
      span.style.fontFamily = font.fontFamily;
    }

    span.style.fontSize = '1px';
    span.style.lineHeight = '1';
    span.style.position = 'absolute';
    span.style.whiteSpace = 'pre';
    span.style.userSelect = 'text';
    span.style.webkitUserSelect = 'text';

    textLayer.appendChild(span);
  }
};

// 根据窗口大小自适应画布宽度
const fitWidth = () => {
  if (!readerContainer.value) return;
  
  // fitWidth 只负责把“页面宽度”贴合当前阅读器容器，页面高度仍由每页 viewport 动态计算。
  const containerWidth = readerContainer.value.clientWidth - 40;
  setPageWidth(containerWidth);
};

// 滚动到指定页面
const goToPage = (pageNum) => {
  if (pageNum < 1 || pageNum > totalPages.value) return;
  
  // 用 Vue refs 而不是 querySelector 获取页面容器
  const pageContainer = pageContainerRefs[pageNum];
  if (pageContainer && readerContainer.value) {
    // 翻页接口直接滚动到目标容器；页码显示交给 setCurrentPage 和滚动回调保持一致
    pageContainer.scrollIntoView({ behavior: 'smooth', block: 'start' });
    setCurrentPage(pageNum);
  }
};

// 翻到下一页
const goToNextPage = () => {
  if (currentPage.value < totalPages.value) {
    goToPage(currentPage.value + 1);
  }
};

// 翻到上一页
const goToPrevPage = () => {
  if (currentPage.value > 1) {
    goToPage(currentPage.value - 1);
  }
};

// 重新渲染可见页面
const rerenderVisiblePages = async () => {
  if (renderTimeout) clearTimeout(renderTimeout);

  renderTimeout = setTimeout(() => {
    currentPages.value.forEach(pageNum => {
      renderingPages.value.delete(pageNum);
      renderPage(pageNum);
    });
  }, RENDER_THROTTLE);
};

// 处理窗口大小改变
const handleResize = () => {
  if (resizeTimeout) clearTimeout(resizeTimeout);
  
  resizeTimeout = setTimeout(() => {
    if (props.pdfSource && pdfDoc.value) {
      fitWidth();
    }
  }, 200);
};

// 监听 pdfSource 变化
watch(() => props.pdfSource, async (newSource) => {
  if (newSource) {
    await loadPdf(newSource);
  }
});

// 生命周期
onMounted(() => {
  window.addEventListener('resize', handleResize);
  readerContainer.value?.addEventListener('scroll', handleScroll, { passive: true });
  readerContainer.value?.addEventListener('wheel', handleWheel, { passive: false });
  
  if (props.pdfSource) {
    loadPdf(props.pdfSource);
  }
});

onBeforeUnmount(() => {
  window.removeEventListener('resize', handleResize);
  readerContainer.value?.removeEventListener('scroll', handleScroll);
  readerContainer.value?.removeEventListener('wheel', handleWheel);
  if (pageObserver) pageObserver.disconnect();
  if (resizeTimeout) clearTimeout(resizeTimeout);
  if (renderTimeout) clearTimeout(renderTimeout);
  if (scrollTimeout) cancelAnimationFrame(scrollTimeout);
  if (wheelTimeout) clearTimeout(wheelTimeout);
});

defineExpose({
  pdfDoc,
  totalPages,
  currentPage,
  renderPage,
  zoomIn,
  zoomOut,
  fitWidth,
  goToPage,
  goToNextPage,
  goToPrevPage,
});
</script>
<template>
  <div class="reader-view" ref="readerContainer">
    <div class="pdf-canvas-container" ref="pdfCanvas">
      <div
        v-for="pageNum in totalPages"
        :key="pageNum"
        class="pdf-page-container"
        :data-page-num="pageNum"
        :style="{ marginBottom: PAGE_GAP + 'px' }"
        :ref="el =>
          el
            ? pageContainerRefs[pageNum] = el
            : delete pageContainerRefs[pageNum]
        "
      >
        <canvas
          class="pdf-page-canvas"
          :data-page-num="pageNum"
          :ref="el =>
            el
              ? pageCanvasRefs[pageNum] = el
              : delete pageCanvasRefs[pageNum]
          "
        ></canvas>

        <div
          class="pdf-text-layer"
          :data-page-num="pageNum"
          :ref="el =>
            el
              ? pageTextLayerRefs[pageNum] = el
              : delete pageTextLayerRefs[pageNum]
          "
        ></div>
      </div>
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

.pdf-page-container {
  background: white;
  margin-bottom: 20px;
  box-shadow: 0 4px 12px rgba(0,0,0,0.3);
  width: fit-content;
  
  position: relative;
  left: 0;
  top: 0;
  right: 0;
  bottom: 0;
}

.pdf-page-canvas {
  display: block;
  z-index: 1;
}

.pdf-text-layer {
  position: absolute;
  left: 0;
  top: 0;
  width: 100%;
  height: 100%;
  pointer-events: auto;
  user-select: text;
  -webkit-user-select: text;
  z-index: 2;
  color: transparent;
}

.pdf-text-layer span {
  position: absolute;
  white-space: pre;
  transform-origin: 0 0;
  line-height: 1;
  user-select: text;
  -webkit-user-select: text;
  cursor: text;
}


</style>