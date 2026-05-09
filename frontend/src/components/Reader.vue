<script setup>
import { ref, onMounted, onBeforeUnmount, watch, markRaw } from 'vue';
import * as pdfjsLib from 'pdfjs-dist';

// 配置 Worker（使用本地文件避免CORS问题）
pdfjsLib.GlobalWorkerOptions.workerSrc = '/pdf.worker.min.js';

const props = defineProps({
  pdfSource: String,
});

const emit = defineEmits(['loaded']);

// 状态管理
const readerContainer = ref(null);
const pdfCanvas = ref(null);
const pdfDoc = ref(null);
const totalPages = ref(0);
const pageWidth = ref(600);
const currentPages = ref(new Set()); // 当前可见的页码
const renderingPages = ref(new Set()); // 正在渲染的页码
const pageHeights = ref({}); // 存储每页的高度缓存

const PAGE_GAP = 20;
const RENDER_THROTTLE = 200;
let resizeTimeout = null;
let renderTimeout = null;

// 加载 PDF 文档
const loadPdf = async (source) => {
  try {
    const pdf = await pdfjsLib.getDocument(source).promise;
    pdfDoc.value = markRaw(pdf);
    totalPages.value = pdf.numPages;
    console.log(`PDF 加载成功，共 ${pdf.numPages} 页`);

    // 清空容器
    if (pdfCanvas.value) {
      pdfCanvas.value.innerHTML = '';
    }

    // 初始化所有页面容器并适配宽度
    await initializePages();
    fitWidth();

    emit('loaded', pdf);
  } catch (err) {
    console.error('PDF 加载失败:', err);
  }
};

// 初始化页面容器
const initializePages = async () => {
  if (!pdfCanvas.value || !totalPages.value) return;

  const fragment = document.createDocumentFragment();

  for (let pageNum = 1; pageNum <= totalPages.value; pageNum++) {
    const pageContainer = document.createElement('div');
    pageContainer.className = 'pdf-page-container';
    pageContainer.dataset.pageNum = pageNum;
    pageContainer.style.marginBottom = PAGE_GAP + 'px';

    const canvas = document.createElement('canvas');
    canvas.className = 'pdf-page-canvas';
    canvas.dataset.pageNum = pageNum;

    const textLayer = document.createElement('div');
    textLayer.className = 'pdf-text-layer';
    textLayer.dataset.pageNum = pageNum;

    pageContainer.style.position = 'relative';
    pageContainer.appendChild(canvas);
    pageContainer.appendChild(textLayer);
    fragment.appendChild(pageContainer);
  }

  pdfCanvas.value.appendChild(fragment);
  setupIntersectionObserver();
};

// 设置交叉观察器
const setupIntersectionObserver = () => {
  const observer = new IntersectionObserver((entries) => {
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
  pageContainers?.forEach(container => observer.observe(container));
};

// 渲染单一页面
const renderPage = async (pageNum) => {
  if (renderingPages.value.has(pageNum) || !pdfDoc.value) return;
  
  renderingPages.value.add(pageNum);
  
  try {
    const page = await pdfDoc.value.getPage(pageNum);
    const canvas = document.querySelector(`canvas[data-page-num="${pageNum}"]`);
    const textLayer = document.querySelector(`.pdf-text-layer[data-page-num="${pageNum}"]`);

    if (!canvas) return;

    const pageWidthInPoints = page.view[2] - page.view[0];
    const scale = pageWidth.value / pageWidthInPoints;
    const viewport = page.getViewport({ scale });

    // 高分辨率渲染（考虑 devicePixelRatio）
    const outputScale = window.devicePixelRatio || 1;
    canvas.width = Math.floor(viewport.width * outputScale);
    canvas.height = Math.floor(viewport.height * outputScale);
    canvas.style.width = viewport.width + 'px';
    canvas.style.height = viewport.height + 'px';

    const context = canvas.getContext('2d');
    context.setTransform(outputScale, 0, 0, outputScale, 0, 0);

    const renderContext = {
      canvasContext: context,
      viewport: viewport,
    };

    await page.render(renderContext).promise;

    // 缓存高度
    pageHeights.value[pageNum] = viewport.height;

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

// 手动渲染文本层
const renderTextLayer = (textContent, viewport, container) => {
  container.innerHTML = '';
  const vt = viewport.transform;
  const scale = viewport.scale || 1;

  textContent.items.forEach((item) => {
    let t;
    if (item.transform) {
      t = pdfjsLib.Util.transform(vt, item.transform);
    } else if (item.x !== undefined && item.y !== undefined) {
      // fallback when items provide x/y
      const x0 = item.x;
      const y0 = item.y;
      t = [vt[0], vt[1], vt[2], vt[3], vt[0] * x0 + vt[4], vt[3] * y0 + vt[5]];
    } else {
      // can't place this item
      console.warn('Text item missing transform and x/y:', item);
      return;
    }

    const x = t[4];
    const y = t[5];
    const fontHeight = Math.abs((item.height || 10) * scale);


    console.log('Rendering text item:', {
      str: item.str,
      x: x.toFixed(2),
      y: y.toFixed(2),
      fontHeight: fontHeight.toFixed(2),
      fontName: item.fontName,
    });
    const div = document.createElement('span');
    div.textContent = item.str;
    div.className = 'pdf-text-div';
    div.style.position = 'absolute';
    div.style.left = x + 'px';
    div.style.top = (y - fontHeight * 0.85) + 'px';
    div.style.fontSize = fontHeight + 'px';
    div.style.lineHeight = fontHeight + 'px';
    div.style.fontFamily = item.fontName || 'sans-serif';
    div.style.whiteSpace = 'pre';
    div.style.cursor = 'text';
    div.style.color = 'transparent';
    div.style.userSelect = 'text';
    container.appendChild(div);
  });
};

// 自适应宽度
const fitWidth = () => {
  if (!readerContainer.value) return;
  
  const containerWidth = Math.max(readerContainer.value.clientWidth - 40, 200);
  pageWidth.value = containerWidth;
  
  // 重新渲染所有可见页面
  rerenderVisiblePages();
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
  
  if (props.pdfSource) {
    loadPdf(props.pdfSource);
  }
});

onBeforeUnmount(() => {
  window.removeEventListener('resize', handleResize);
  if (resizeTimeout) clearTimeout(resizeTimeout);
  if (renderTimeout) clearTimeout(renderTimeout);
});

defineExpose({
  pdfDoc,
  totalPages,
  renderPage,
});
</script>
<template>
  <div class="reader-view" ref="readerContainer">
    <div class="pdf-canvas-container" ref="pdfCanvas"></div>
  </div>
</template>


<style scoped>
.reader-view {
  height: 100%;
  width: 100vw;
  box-sizing: border-box;
  overflow-y: auto;
  overflow-x: hidden;
  background: #525659;
  display: flex;
  justify-content: center; /* 居中显示 */
  padding: 20px;
  position: relative;
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
}

.pdf-page-canvas {
  display: block;
}

.pdf-text-layer {
  position: absolute;
  left: 0;
  top: 0;
  right: 0;
  bottom: 0;
  pointer-events: auto;
  user-select: text;
  -webkit-user-select: text;
}

.pdf-text-div {
  color: transparent;
  position: absolute;
  white-space: pre;
  user-select: text;
  -webkit-user-select: text;
  cursor: text;
}


</style>