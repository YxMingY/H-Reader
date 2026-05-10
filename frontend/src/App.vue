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
        <div class="page-navigation">
          <!-- 这里是快捷翻页入口，只负责发起动作，不直接操作滚动 DOM -->
          <button @click="goToPrevPage" title="上一页" class="nav-btn">←</button>
          <span class="page-counter">{{ currentPage }} / {{ totalPages }}</span>
          <button @click="goToNextPage" title="下一页" class="nav-btn">→</button>
        </div>
        
        <div class="page-info">
          <span>{{ currentPage }} / {{ totalPages }}</span>
        </div>
      </div>
    </header>

    <main class="main-content">
      
      <!-- 书架视图 -->
      <Bookshelf v-if="view === 'library'" @select="openBook" />

      <!-- 阅读器视图：连续滚动模式 -->
      <Reader v-if="view === 'reader'"
        ref="readerRef"
        :pdfSource="pdfSource"
        @loaded="onPDFLoaded"
        <!-- Reader 负责决定当前页，App 只接收这个结果并更新工具栏显示 -->
        @pagechange="currentPage = $event"
        @rescale="onPDFRescale"
      ></Reader>
    </main>
  </div>
</template>

<script setup>
import { computed, ref, onMounted, onBeforeUnmount, watch } from 'vue';

import Bookshelf from './components/Bookshelf.vue';
import Reader from './components/Reader.vue';
 
import { BookService } from '../bindings/changeme'; //这个路径是正确的，不要修改

// --- 状态定义 ---
const view = ref('library');
const currentBookTitle = ref('');
const pdfSource = ref(null);

const currentPage = ref(1); // 在连续模式下，这个值主要用于显示“当前可视区域大致页码”，实际滚动由浏览器原生处理
const totalPages = ref(0);

const zoomLevel = ref(1);
const readerRef = ref(null);

// --- 阅读器逻辑 ---
const openBook = async (book) => {
  currentBookTitle.value = book.title;
  view.value = 'reader';
  currentPage.value = 1;
  totalPages.value = 0;
  pdfSource.value = null;

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

const onPDFLoaded = (pdf) => {
  totalPages.value = pdf.numPages;
};

const onPDFRescale = (newScale) => {
  zoomLevel.value = newScale;
};

const goToNextPage = () => {
  readerRef.value?.goToNextPage();
};

const goToPrevPage = () => {
  readerRef.value?.goToPrevPage();
};

const zoomIn = () => {
  readerRef.value?.zoomIn();
};

const zoomOut = () => {
  readerRef.value?.zoomOut();
};

const fitWidth = () => {
  readerRef.value?.fitWidth();
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

.page-navigation {
  display: flex;
  align-items: center;
  background: #f0f2f5;
  border-radius: 8px;
  padding: 4px;
  gap: 5px;
}

.nav-btn {
  border: none;
  background: white;
  width: 28px;
  height: 28px;
  border-radius: 4px;
  cursor: pointer;
  font-weight: bold;
  color: var(--text-primary);
  box-shadow: 0 1px 3px rgba(0,0,0,0.1);
  font-size: 14px;
}

.nav-btn:hover {
  background: #e6e6e6;
}

.page-counter {
  font-size: 13px;
  min-width: 50px;
  text-align: center;
  color: var(--text-secondary);
  font-variant-numeric: tabular-nums;
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

</style>