<script setup>
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue';
import VuePdfEmbed from 'vue-pdf-embed';
import * as pdfjsLib from 'pdfjs-dist';
import 'pdfjs-dist/legacy/web/pdf_viewer.css';
// // 配置 Worker (保持你之前成功的配置)
// // 如果本地 public 下有文件，用 '/pdf.worker.min.js'，否则用 CDN

pdfjsLib.GlobalWorkerOptions.workerSrc = 'https://unpkg.com/pdfjs-dist@3.11.174/build/pdf.worker.min.js';

const basePageWidth = ref(0); // 记录第一页在 scale=1 时的原始宽度
const basePageHeight = ref(0); // 记录第一页在 scale=1 时的原始高度
const pageWidth = ref(600); // 通过 width 控制视觉缩放
const pdfInstance = ref(null); // 存储 PDF.js 实例
const scale = ref(1.0); // 实则为清晰度控制，视觉尺寸由 pageWidth 控制
const previewZoom = ref(1); // 交互期的轻量 CSS 预览缩放
const readerContainer = ref(null);
const showScrollHint = ref(false);
const zoomLevel = ref(1);
const committedZoomLevel = ref(1);
const zoomCommitDelay = 120;
const renderScaleMax = 1.8;
const totalPages = ref(0);
const visibleStartPage = ref(1);
const visibleEndPage = ref(1);
const pageGap = 20;
const virtualOverscanPages = 3;
let zoomCommitTimer = null;
let scrollRafId = null;
let pendingAnchor = null; // { page: number, offset: px }
const perPageHeights = ref([]);
const props = defineProps({
  pdfSource: String,
});
const emit = defineEmits(['loaded','rescale']);
const renderKey = ref(0); // 用于强制重新渲染组件

const estimatedPageBlockHeight = computed(() => {
  const fallbackBaseHeight = 842;
  const baseHeight = basePageHeight.value || fallbackBaseHeight;
  return baseHeight * committedZoomLevel.value + pageGap;
});

const visiblePages = computed(() => {
  if (!totalPages.value) return [];
  const start = Math.max(1, visibleStartPage.value);
  const end = Math.min(totalPages.value, Math.max(start, visibleEndPage.value));
  return Array.from({ length: end - start + 1 }, (_, i) => start + i);
});

const topSpacerHeight = computed(() => {
  return Math.max(0, (visibleStartPage.value - 1) * estimatedPageBlockHeight.value);
});

const bottomSpacerHeight = computed(() => {
  return Math.max(0, (totalPages.value - visibleEndPage.value) * estimatedPageBlockHeight.value);
});

// --- 生命周期 ---
onMounted(async () => {
  window.addEventListener('resize', handleResize);
});

onBeforeUnmount(() => {
  window.removeEventListener('resize', handleResize);
  if (zoomCommitTimer) {
    clearTimeout(zoomCommitTimer);
  }
  if (scrollRafId) {
    cancelAnimationFrame(scrollRafId);
  }
});

watch(() => props.pdfSource, () => {
  basePageWidth.value = 0;
  basePageHeight.value = 0;
  pageWidth.value = 600;
  pdfInstance.value = null;
  scale.value = 1.0;
  previewZoom.value = 1;
  zoomLevel.value = 1;
  committedZoomLevel.value = 1;
  totalPages.value = 0;
  visibleStartPage.value = 1;
  visibleEndPage.value = 1;
  pendingAnchor = null;
  perPageHeights.value = [];
});

// 监听 pageWidth 变化，强制组件重新渲染
watch(pageWidth, () => {
  //renderKey.value++;
});
const handleLoaded = (pdf) => {
  if (pdfInstance.value) {
    requestVirtualWindowUpdate();
    return;
  }

  pdfInstance.value = pdf; // 存储 PDF.js 实例，后续可能需要
  totalPages.value = pdf.numPages;
  console.log(`PDF 加载成功，共 ${pdf.numPages} 页`);

  // 用第一页真实宽度作为基准，避免不同 PDF 初始显示大小不一致
  pdf.getPage(1)
    .then((page) => {
      const viewport = page.getViewport({ scale: 1 });
      basePageWidth.value = viewport.width;
      basePageHeight.value = viewport.height;
      pageWidth.value = viewport.width;
      fitWidth();
      updateVirtualWindow();
    })
    .catch((err) => {
      console.error('读取第一页尺寸失败:', err);
      basePageWidth.value = 595;
      basePageHeight.value = 842;
      fitWidth();
      updateVirtualWindow();
    });
  emit('loaded', pdf);
};

const handleError = (err) => {
  console.error("PDF 渲染错误:", err);
};
// --- 缩放与滚动逻辑 ---
const clampZoom = (value) => Math.max(0.5, Math.min(value, 3.0));

const commitScale = (newScale) => {
  // 视觉尺寸由 width 控制；scale 主要提升渲染清晰度
  if (basePageWidth.value) {
    pageWidth.value = basePageWidth.value * newScale;
  }
  // 限制渲染清晰度上限，避免高倍缩放时重绘过慢
  scale.value = Math.min(Math.max(1.0, newScale), renderScaleMax);
  committedZoomLevel.value = newScale;
  previewZoom.value = 1;
  requestVirtualWindowUpdate();
};

const measureRenderedPageHeights = () => {
  // Measure DOM heights of rendered pages and update perPageHeights
  if (!readerContainer.value) return;
  const container = readerContainer.value;
  const pageEls = container.querySelectorAll('.vue-pdf-embed__page');
  const vp = visiblePages.value;
  pageEls.forEach((el, i) => {
    const pageNum = vp[i];
    if (!pageNum) return;
    const h = Math.max(1, Math.round(el.getBoundingClientRect().height));
    perPageHeights.value[pageNum] = h + pageGap;
  });
};

const prefixHeights = () => {
  // Build prefix sum array: prefix[0]=0, prefix[i]=sum heights of pages 1..i
  const pref = [0];
  const est = Math.max(estimatedPageBlockHeight.value, 1);
  for (let i = 1; i <= totalPages.value; i++) {
    const h = perPageHeights.value[i] || est;
    pref[i] = pref[i - 1] + h;
  }
  return pref;
};

const locatePageByAbs = (abs, pref) => {
  // find smallest i such that pref[i] > abs => page is i
  let lo = 1, hi = totalPages.value, mid, page = totalPages.value;
  while (lo <= hi) {
    mid = (lo + hi) >> 1;
    if (pref[mid] > abs) {
      page = mid;
      hi = mid - 1;
    } else {
      lo = mid + 1;
    }
  }
  return page;
};

const setZoom = (newScale, options = { deferRender: true }) => {
  const clamped = clampZoom(newScale);
  zoomLevel.value = clamped;
  emit('rescale', zoomLevel.value);

  if (!options.deferRender) {
    if (zoomCommitTimer) {
      clearTimeout(zoomCommitTimer);
      zoomCommitTimer = null;
    }
    commitScale(clamped);
    return;
  }

  const baseZoom = committedZoomLevel.value || 1;
  previewZoom.value = clamped / baseZoom;

  if (zoomCommitTimer) {
    clearTimeout(zoomCommitTimer);
  }
  zoomCommitTimer = setTimeout(() => {
    // Before committing, capture an exact pixel anchor (page + offset)
    const container = readerContainer.value;
    if (container && totalPages.value) {
      measureRenderedPageHeights();
      const pref = prefixHeights();
      const centerAbs = container.scrollTop + container.clientHeight / 2;
      const page = locatePageByAbs(centerAbs, pref);
      const pageTop = pref[page - 1] || 0;
      const offsetInPage = Math.max(0, Math.min(centerAbs - pageTop, pref[page] - pageTop));
      pendingAnchor = { page, offset: offsetInPage };
    } else {
      pendingAnchor = null;
    }
    commitScale(clamped);
    zoomCommitTimer = null;
  }, zoomCommitDelay);
};

const zoomIn = () => {
  setZoom(zoomLevel.value + 0.1, { deferRender: true }); // 最大 300%
};

const zoomOut = () => {
  setZoom(zoomLevel.value - 0.1, { deferRender: true }); // 最小 50%
};

const fitWidth = () => {
  if (!readerContainer.value || !pdfInstance.value || !basePageWidth.value) return;

  // 获取容器的可用宽度（减去 padding）
  const containerWidth = Math.max(readerContainer.value.clientWidth - 40, 200);
  let newScale = containerWidth / basePageWidth.value;

  // 适应宽度是一次性动作，直接提交重绘
  setZoom(newScale, { deferRender: false });
  console.log('Fit Width Scale:', newScale);
};

const updateVirtualWindow = () => {
  if (!readerContainer.value || !totalPages.value) {
    visibleStartPage.value = 1;
    visibleEndPage.value = 1;
    return;
  }

  const container = readerContainer.value;
  const blockHeight = Math.max(estimatedPageBlockHeight.value, 1);
  const startFromScroll = Math.floor(container.scrollTop / blockHeight) + 1;
  const endFromScroll = Math.ceil((container.scrollTop + container.clientHeight) / blockHeight);

  visibleStartPage.value = Math.max(1, startFromScroll - virtualOverscanPages);
  visibleEndPage.value = Math.min(totalPages.value, endFromScroll + virtualOverscanPages);
};

const requestVirtualWindowUpdate = () => {
  if (scrollRafId) return;
  scrollRafId = requestAnimationFrame(() => {
    scrollRafId = null;
    updateVirtualWindow();

    // measure rendered heights then restore precise anchor if present
    measureRenderedPageHeights();
    if (pendingAnchor !== null && readerContainer.value && totalPages.value) {
      const container = readerContainer.value;
      const pref = prefixHeights();
      const page = Math.min(Math.max(1, pendingAnchor.page), totalPages.value);
      const pageTop = pref[page - 1] || 0;
      const contentCenter = pageTop + Math.max(0, Math.min(pendingAnchor.offset, pref[page] - pageTop));
      const targetScrollTop = Math.max(0, contentCenter - container.clientHeight / 2);
      container.scrollTop = targetScrollTop;
      pendingAnchor = null;
      updateVirtualWindow();
    }
  });
};

const handleScroll = () => {
  requestVirtualWindowUpdate();
};

// 简单的滚轮辅助（可选，浏览器原生滚动通常已经足够好）
const handleWheel = (e) => {
  if (!e.ctrlKey) return;

  // Ctrl + 滚轮时拦截浏览器默认缩放，改为 PDF 缩放
  e.preventDefault();

  if (e.deltaY < 0) {
    zoomIn();
  } else {
    zoomOut();
  }
};

const handleResize = () => {
  console.log('窗口大小改变，重新适应宽度');
  if (props.pdfSource && pdfInstance.value) {
    fitWidth();
    requestVirtualWindowUpdate();
  }
};

defineExpose({
  zoomIn,
  zoomOut,
  fitWidth,
});
</script>
<template>
  <div class="reader-view" ref="readerContainer" @wheel="handleWheel" @scroll.passive="handleScroll">
        <div class="pdf-canvas-container">
        <div class="pdf-preview-layer" :style="{ transform: `scale(${previewZoom})` }">
      <div class="virtual-spacer" :style="{ height: `${topSpacerHeight}px` }"></div>
            <!-- 
            vue-pdf-embed 在连续模式下会渲染所有页面。
            :scale 控制缩放比例。
            :annotation-layer-enabled 和 :text-layer-enabled 提升体验（可选）
            -->
            <vue-pdf-embed
            v-if="pdfSource"
       
            ref="pdfRef"
            :source="pdfSource"
      :page="visiblePages"
            :scale="scale"
            :width="pageWidth"
            :annotation-layer="true"
            :text-layer="true"
            @loaded="handleLoaded"
            @error="handleError"
            class="pdf-document"  
            />
      <div class="virtual-spacer" :style="{ height: `${bottomSpacerHeight}px` }"></div>
            </div>
        </div> 
        
        <!-- 底部悬浮提示（可选） -->
        <div class="scroll-hint" v-if="showScrollHint">
            使用滚轮或拖动右侧滑块浏览
        </div>
    </div>
</template>


<style>

/* --- 阅读器样式 --- */
/* --- 阅读器样式修正 --- */
.reader-view {
  height: 100%;
  width: 100vw;
  box-sizing: border-box;
  overflow-y: auto;
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

.pdf-preview-layer {
  transform-origin: top center;
  will-change: transform;
}

.virtual-spacer {
  width: 100%;
}

/* 强制每一页的容器自适应内容 */
:deep(.vue-pdf-embed__page) {
  margin-bottom: 20px;
  box-shadow: 0 4px 12px rgba(0,0,0,0.3);
  background: white;
  width: auto !important;
  height: auto !important;
}

/* 彻底解放 Canvas */
:deep(canvas) {
  display: block;
  max-width: 100% !important;
  width: auto !important;
  height: auto !important;
}

/* PDF text layer selection moved to global style.css for better specificity */


.scroll-hint {
  position: absolute;
  bottom: 20px;
  left: 50%;
  transform: translateX(-50%);
  background: rgba(0,0,0,0.7);
  color: white;
  padding: 8px 16px;
  border-radius: 20px;
  font-size: 12px;
  pointer-events: none;
  opacity: 0;
  transition: opacity 0.5s;
}

</style>