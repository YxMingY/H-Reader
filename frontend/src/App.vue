<template>
  <div class="app-container">
    <!-- 顶部栏：现代简洁风格 -->
    <header class="header">
      <div class="header-left">
        <button v-if="view === 'reader'" class="btn-icon" @click="goBack" title="返回书架">
          ←
        </button>
        <h2 class="app-title">{{ view === 'library' ? 'H-Reader' : currentBookTitle }}</h2>
      </div>
      
      <!-- 阅读器工具栏：仅在阅读模式下显示 -->
      <div v-if="view === 'reader'" class="toolbar">
        <div class="zoom-controls">
          <button @click="zoomOut" title="缩小">−</button>
          <span class="zoom-level">{{ Math.round(zoomLevel * 100) }}%</span>
          <button @click="zoomIn" title="放大">+</button>
          <button @click="fitWidth" title="适应宽度">Fit</button>
        </div>
        
        <div class="page-info">
          <span>{{ currentPage }} / {{ totalPages }}</span>
        </div>
      </div>
    </header>

    <main class="main-content">
      
      <!-- 书架视图 -->
      <div v-if="view === 'library'" class="library-view">
        <div v-if="loading" class="loading-state">正在扫描书籍...</div>
        
        <div v-else-if="books.length === 0" class="empty-msg">
          <div class="empty-icon">📚</div>
          <p>暂无书籍</p>
          <p class="hint">请把 PDF 放到 Documents/Papers 文件夹</p>
        </div>

        <div class="book-grid" v-else>
          <div 
            v-for="book in books" 
            :key="book.id" 
            class="book-card"
            @dblclick="openBook(book)"
          >
            <div class="book-cover">
              <div class="icon">📄</div>
            </div>
            <div class="book-meta">
              <div class="title" :title="book.title">{{ book.title }}</div>
            </div>
          </div>
        </div>
      </div>

      <!-- 阅读器视图：连续滚动模式 -->
      <div v-if="view === 'reader'" class="reader-view" ref="readerContainer" @wheel="handleWheel">
        <div class="pdf-canvas-container">
          <!-- 
            vue-pdf-embed 在连续模式下会渲染所有页面。
            :scale 控制缩放比例。
            :annotation-layer-enabled 和 :text-layer-enabled 提升体验（可选）
          -->
          <vue-pdf-embed
            v-if="pdfSource"
            ref="pdfRef"
            :source="pdfSource"
            :scale="scale"
            :width="pageWidth"
            :annotation-layer="true"
            :text-layer="true"
            @loaded="handleLoaded"
            @rendered="enableTextSelection"
            @error="handleError"
            class="pdf-document"  
          />
        </div> 
        
        <!-- 底部悬浮提示（可选） -->
        <div class="scroll-hint" v-if="showScrollHint">
          使用滚轮或拖动右侧滑块浏览
        </div>
      </div>
    </main>
  </div>
</template>

<script setup>
import { computed, ref, onMounted, onBeforeUnmount } from 'vue';
import VuePdfEmbed from 'vue-pdf-embed';
import * as pdfjsLib from 'pdfjs-dist';
import 'pdfjs-dist/legacy/web/pdf_viewer.css';
 
// 配置 Worker (保持你之前成功的配置)
// 如果本地 public 下有文件，用 '/pdf.worker.min.js'，否则用 CDN
pdfjsLib.GlobalWorkerOptions.workerSrc = 'https://unpkg.com/pdfjs-dist@3.11.174/build/pdf.worker.min.js';

import { BookService } from '../bindings/changeme'; //这个路径是正确的，不要修改

// --- 状态定义 ---
const view = ref('library');
const books = ref([]);
const loading = ref(false);
const currentBookTitle = ref('');
const pdfSource = ref(null);
const pdfInstance = ref(null); // 存储 PDF.js 实例
const currentPage = ref(1); // 在连续模式下，这个值主要用于显示“当前可视区域大致页码”，实际滚动由浏览器原生处理
const totalPages = ref(0);
const scale = ref(1.0); // 缩放比例
const readerContainer = ref(null);
const showScrollHint = ref(false);
const basePageWidth = ref(0); // 记录第一页在 scale=1 时的原始宽度
const pageWidth = ref(600); // 通过 width 控制视觉缩放
const zoomLevel = computed(() => {
  if (!basePageWidth.value) return 1.0;
  return pageWidth.value / basePageWidth.value;
});
 
// --- 书架逻辑 ---
onMounted(async () => {
  loadLibrary();
  window.addEventListener('resize', handleResize);
});

onBeforeUnmount(() => {
  window.removeEventListener('resize', handleResize);
});

const loadLibrary = async () => {
  loading.value = true;
  try {
    const result = await BookService.ScanBooks("");
    books.value = result;
  } catch (err) {
    console.error("扫描失败", err);
  } finally {
    loading.value = false;
  }
};

// --- 阅读器逻辑 ---
const openBook = async (book) => {
  currentBookTitle.value = book.title;
  view.value = 'reader';
  currentPage.value = 1;
  totalPages.value = 0;
  pdfSource.value = null;
  basePageWidth.value = 0;
  pageWidth.value = 600;
  scale.value = 1.0; // 重置缩放

  try {
    const data = await BookService.LoadPDF(book.path);
    
    // Wails v3 byte[] -> Base64 转换
    const byteCharacters = atob(data);
    const byteNumbers = new Array(byteCharacters.length);
    for (let i = 0; i < byteCharacters.length; i++) {
      byteNumbers[i] = byteCharacters.charCodeAt(i);
    }
    const byteArray = new Uint8Array(byteNumbers);
    const blob = new Blob([byteArray], { type: 'application/pdf' });

    if (blob.size === 0) throw new Error("Empty PDF");

    pdfSource.value = URL.createObjectURL(blob);

  } catch (err) {
    alert("无法打开文件：" + err.message);
    goBack();
  }
};

const goBack = () => {
  view.value = 'library';
  pdfSource.value = null;
};

const handleLoaded = (pdf) => {
  totalPages.value = pdf.numPages;
  pdfInstance.value = pdf; // 存储 PDF.js 实例，后续可能需要
  console.log(`PDF 加载成功，共 ${pdf.numPages} 页`);

  // 用第一页真实宽度作为基准，避免不同 PDF 初始显示大小不一致
  pdf.getPage(1)
    .then((page) => {
      const viewport = page.getViewport({ scale: 1 });
      basePageWidth.value = viewport.width;
      pageWidth.value = viewport.width;
      fitWidth();
    })
    .catch((err) => {
      console.error('读取第一页尺寸失败:', err);
      basePageWidth.value = 595;
      fitWidth();
    });
};

const handleError = (err) => {
  console.error("PDF 渲染错误:", err);
};
// --- 缩放与滚动逻辑 ---
const adjustScale = (newScale) => {
  // 视觉尺寸由 width 控制；scale 主要提升渲染清晰度
  if (basePageWidth.value) {
    pageWidth.value = basePageWidth.value * newScale;
  }

  scale.value = Math.max(1.0, newScale);
};
const zoomIn = () => {
  adjustScale(Math.min(zoomLevel.value + 0.1, 3.0)); // 最大 300%
};

const zoomOut = () => {
  adjustScale(Math.max(zoomLevel.value - 0.1, 0.5)); // 最小 50%
};

const fitWidth = () => {
  if (!readerContainer.value || !pdfInstance.value || !basePageWidth.value) return;

  // 获取容器的可用宽度（减去 padding）
  const containerWidth = Math.max(readerContainer.value.clientWidth - 40, 200);
  let newScale = containerWidth / basePageWidth.value;

  // 限制缩放范围，防止过大或过小
  newScale = Math.max(0.5, Math.min(newScale, 3.0));
  adjustScale(newScale);
  console.log('Fit Width Scale:', newScale);
};

const handleResize = () => {
  if (view.value !== 'reader') return;
  fitWidth();
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

</script>

<style>
/* --- 全局变量与重置 --- */
:root {
  --bg-color: #f5f7fa;       /* 浅灰背景 */
  --header-bg: #ffffff;      /* 白色头部 */
  --text-primary: #333333;   /* 深灰文字 */
  --text-secondary: #666666; /* 浅灰文字 */
  --accent-color: #007acc;   /* 强调色（蓝） */
  --border-color: #e0e0e0;   /* 边框色 */
  --shadow-sm: 0 2px 8px rgba(0,0,0,0.05);
  --shadow-md: 0 4px 12px rgba(0,0,0,0.1);
}

body { 
  margin: 0; 
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; 
  background: var(--bg-color); 
  color: var(--text-primary); 
  overflow: hidden; /* 防止整个页面滚动 */
}

.app-container { 
  display: flex; 
  flex-direction: column; 
  height: 100vh; 
  width: 100%;
}

/* --- 头部样式 --- */
.header {
  height: 60px;
  width: 100%;
  box-sizing: border-box;
  background: var(--header-bg);
  border-bottom: 1px solid var(--border-color);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
  box-shadow: var(--shadow-sm);
  z-index: 10;
}

.header-left {
  display: flex;
  flex: 1;
  min-width: 0;
  align-items: center;
  gap: 15px;
  overflow: hidden; /* 关键：防止标题溢出 */
}

.btn-icon {
  background: transparent;
  border: 1px solid var(--border-color);
  color: var(--text-primary);
  width: 32px;
  height: 32px;
  border-radius: 6px;
  cursor: pointer;
  font-size: 18px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s;
  flex-shrink: 0; /* 防止按钮被压缩 */
}

.btn-icon:hover {
  background: #f0f0f0;
  border-color: #ccc;
}

.app-title {
  font-size: 18px;
  font-weight: 600;
  margin: 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis; /* 关键：长文件名显示省略号 */
  color: var(--text-primary);
}

/* --- 工具栏样式 --- */
.toolbar {
  display: flex;
  align-items: center;
  gap: 20px;
  margin-left: 16px;
  flex-shrink: 0;
}

.zoom-controls {
  display: flex;
  align-items: center;
  background: #f0f2f5;
  border-radius: 8px;
  padding: 4px;
  gap: 5px;
}

.zoom-controls button {
  border: none;
  background: white;
  width: 28px;
  height: 28px;
  border-radius: 4px;
  cursor: pointer;
  font-weight: bold;
  color: var(--text-primary);
  box-shadow: 0 1px 3px rgba(0,0,0,0.1);
}

.zoom-controls button:hover {
  background: #e6e6e6;
}

.zoom-level {
  font-size: 13px;
  min-width: 40px;
  text-align: center;
  color: var(--text-secondary);
}

.page-info {
  font-size: 14px;
  color: var(--text-secondary);
  font-variant-numeric: tabular-nums; /* 数字等宽，防止跳动 */
}

/* --- 主内容区 --- */
.main-content { 
  flex: 1; 
  overflow: hidden; 
  position: relative; 
}

/* --- 书架样式 --- */
.library-view { 
  height: 100%; 
  overflow-y: auto; 
  padding: 30px; 
  background: var(--bg-color);
}

.book-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  gap: 25px;
}

.book-card {
  background: white;
  border-radius: 12px;
  padding: 15px;
  cursor: pointer;
  transition: transform 0.2s, box-shadow 0.2s;
  border: 1px solid transparent;
  display: flex;
  flex-direction: column;
  align-items: center;
  box-shadow: var(--shadow-sm);
}

.book-card:hover {
  transform: translateY(-5px);
  box-shadow: var(--shadow-md);
  border-color: var(--accent-color);
}

.book-cover {
  width: 80px;
  height: 100px;
  background: #f0f2f5;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 15px;
  font-size: 32px;
}

.book-meta {
  text-align: center;
  width: 100%;
}

.title {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-primary);
  word-break: break-all;
  display: -webkit-box;
  line-clamp: 2;
  -webkit-line-clamp: 2; /* 最多显示两行 */
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.empty-msg {
  text-align: center;
  color: var(--text-secondary);
  margin-top: 100px;
}
.empty-icon { font-size: 48px; margin-bottom: 10px; opacity: 0.5; }
.hint { font-size: 12px; opacity: 0.7; }

/* --- 阅读器样式 --- */
/* --- 阅读器样式修正 --- */
.reader-view {
  height: 100%;
  width: 100%;
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
  width: 100%;
  min-width: 0;
}

/* 强制每一页的容器自适应内容 */
:deep(.vue-pdf-embed__page) {
  margin-bottom: 20px;
  box-shadow: 0 4px 12px rgba(0,0,0,0.3);
  background: white;
  width: auto !important;
  max-width: 100%;
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