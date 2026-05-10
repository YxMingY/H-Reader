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

    const pageWidthInPoints = page.view[2] - page.view[0];  // PDF 页面宽度（单位：点, 也就是1/72英寸）
    const scale = pageWidth.value / pageWidthInPoints; // 每个点对应的像素数，pageWidth是我们设定的页面宽度（单位：像素）
    // 根据我们提供的scale计算viewport，viewport.width应该等于pageWidth.value
    //  viewport包含了页面的尺寸和变换信息，比如页面的宽高（单位：像素）
    const viewport = page.getViewport({ scale }); 
    console.log(viewport);
    // 高分辨率渲染（考虑 devicePixelRatio）
    // devicePixelRatio 是一个表示设备像素与CSS像素之间关系的值。对于普通屏幕，这个值通常是1；
    // 对于高DPI屏幕（如Retina显示屏），这个值可能是2或更高。这意味着在高DPI屏幕上，每个CSS像素实际上由多个物理像素组成。
    const outputScale = window.devicePixelRatio || 1;
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


<style>
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
  -webkit-user-select: text;
  cursor: text;
}


</style>