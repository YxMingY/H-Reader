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

      <div class="header-right">
        <div v-if="view === 'library'" class="toolbar library-toolbar">
          <div class="settings-anchor">
            <button
              class="btn-icon btn-square settings-btn"
              @click="toggleSettings"
              title="设置"
              aria-label="设置"
            >
              <svg viewBox="0 0 24 24" aria-hidden="true">
                <path d="M12 8.75a3.25 3.25 0 1 0 0 6.5 3.25 3.25 0 0 0 0-6.5Zm8.08 3.43-.95-.55c.05-.4.07-.8.07-1.2s-.02-.8-.07-1.2l.95-.55a1.5 1.5 0 0 0 .55-2.05l-1.15-1.99a1.5 1.5 0 0 0-2.05-.55l-.96.55a8.3 8.3 0 0 0-2.07-1.2V2.86A1.5 1.5 0 0 0 12.5 1.36h-2.3a1.5 1.5 0 0 0-1.5 1.5v1.08c-.73.24-1.43.64-2.07 1.2l-.96-.55a1.5 1.5 0 0 0-2.05.55L2.47 7.13a1.5 1.5 0 0 0 .55 2.05l.95.55c-.05.4-.07.8-.07 1.2s.02.8.07 1.2l-.95.55a1.5 1.5 0 0 0-.55 2.05l1.15 1.99a1.5 1.5 0 0 0 2.05.55l.96-.55c.64.56 1.34.96 2.07 1.2v1.08a1.5 1.5 0 0 0 1.5 1.5h2.3a1.5 1.5 0 0 0 1.5-1.5v-1.08c.73-.24 1.43-.64 2.07-1.2l.96.55a1.5 1.5 0 0 0 2.05-.55l1.15-1.99a1.5 1.5 0 0 0-.55-2.05Zm-6.08 1.82a5.25 5.25 0 1 1 0-10.5 5.25 5.25 0 0 1 0 10.5Z"/>
              </svg>
            </button>
            <SettingsMenu
              v-model:open="settingsOpen"
              :api-key="apiKey"
              @save="saveApiKey"
            />
          </div>
        </div>

        <div v-if="view === 'reader'" class="toolbar reader-toolbar">
          <div class="reader-controls">
            <!-- 截图功能按钮 -->
            <div class="screenshot-controls">
              <button @click="captureCurrentPage" title="截取当前页" class="nav-btn screenshot-btn" aria-label="截取当前页">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <rect x="3" y="3" width="18" height="18" rx="2" ry="2"></rect>
                  <circle cx="8.5" cy="8.5" r="1.5"></circle>
                  <polyline points="21 15 16 10 5 21"></polyline>
                </svg>
              </button>
              <button @click="startAreaSelection" title="框选区域截图" class="nav-btn screenshot-btn" aria-label="框选区域截图">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M3 7V5a2 2 0 0 1 2-2h2"></path>
                  <path d="M17 3h2a2 2 0 0 1 2 2v2"></path>
                  <path d="M21 17v2a2 2 0 0 1-2 2h-2"></path>
                  <path d="M7 21H5a2 2 0 0 1-2-2v-2"></path>
                  <line x1="7" y1="12" x2="17" y2="12"></line>
                </svg>
              </button>
            </div>

            <div class="page-navigation">
              <button @click="goToPrevPage" title="上一页" class="nav-btn">←</button>
              <div class="page-display">
                <input
                  ref="pageInputRef"
                  class="page-input"
                  v-model.number="currentPage"
                  @keyup.enter="goToPageInput($event)"
                  @blur="goToPageInput"
                  :min="1"
                  :max="totalPages"
                  aria-label="页码"
                />
                <span class="page-delimiter">/</span>
                <span class="page-total">{{ totalPages }}</span>
              </div>
              <button @click="goToNextPage" title="下一页" class="nav-btn">→</button>
            </div>

            <div class="zoom-controls">
              <button @click="zoomOut" title="缩小" class="nav-btn">−</button>
              <button @click="zoomIn" title="放大" class="nav-btn">+</button>
            </div>
          </div>
        </div>

        <button
          class="btn-icon btn-square ai-btn"
          :class="{ active: chatOpen }"
          @click="toggleChatPanel"
          title="AI 对话"
          aria-label="AI 对话"
        >
          AI
        </button>
      </div>
    </header>

    <div class="workspace-shell" :style="workspaceStyle" :class="{ 'chat-open': chatOpen }">
      <main class="main-content content-pane">
        <Bookshelf 
          v-if="view === 'library'" 
          ref="bookshelfRef"
          :has-api-key="!!apiKey"
          @select="openBook" 
          @settings-opened="toggleSettings"
        />

        <Reader
          v-if="view === 'reader'"
          ref="readerRef"
          :pdfSource="pdfSource"
          @loaded="onPDFLoaded"
          @pagechange="currentPage = $event"
          @rescale="onPDFRescale"
        ></Reader>
      </main>

      <div
        v-show="chatOpen"
        class="chat-resizer"
        @pointerdown="startChatResize"
        aria-hidden="true"
      ></div>

      <aside v-show="chatOpen" class="chat-pane">
        <ChatPanel
          ref="chatPanelRef"
          :scope-type="chatScopeType"
          :book-path="currentBookPath"
          :book-title="currentBookTitle"
          :book-key="chatScopeKey"
          @close="chatOpen = false"
        />
      </aside>
    </div>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue';

import Bookshelf from './features/Bookshelf/Bookshelf.vue';
import Reader from './features/Reader/Reader.vue';
import ChatPanel from './features/ChatPanel/ChatPanel.vue';
import SettingsMenu from './features/SettingsMenu.vue';

import { BookService, ChatService } from '../bindings/hreader'; //这个路径是正确的，不要修改

// --- 状态定义 ---
const view = ref('library');
const currentBookTitle = ref('');
const currentBookPath = ref('');
const pdfSource = ref(null);
const apiKey = ref('');
const settingsOpen = ref(false);
const chatOpen = ref(false);
const chatPanelWidth = ref(0);
const hasCustomChatWidth = ref(false);
const isResizingChat = ref(false);
const resizeStartX = ref(0);
const resizeStartWidth = ref(420);

// Bookshelf 组件引用
const bookshelfRef = ref(null);

const currentPage = ref(1); // 在连续模式下，这个值主要用于显示“当前可视区域大致页码”，实际滚动由浏览器原生处理
const totalPages = ref(0);

const zoomLevel = ref(1);
const readerRef = ref(null);
const chatPanelRef = ref(null);
const pageInputRef = ref(null);

const getViewportWidth = () => (typeof window !== 'undefined' ? window.innerWidth : 0);

const clampChatWidth = (value) => {
  const viewportWidth = getViewportWidth();
  const minWidth = 280;
  const maxWidth = Math.max(360, Math.min(800, Math.floor(viewportWidth * 0.55)));
  // 聊天面板既要有可用下限，也要防止在大屏上无限拉宽。
  return Math.min(Math.max(Math.floor(value), minWidth), maxWidth);
};

const syncChatWidth = () => {
  const viewportWidth = getViewportWidth();
  if (!viewportWidth) return;
  // 已经有宽度时沿用当前值，仅在窗口变化时重新夹紧。
  const nextWidth = chatPanelWidth.value || Math.round(viewportWidth * 0.3);
  chatPanelWidth.value = clampChatWidth(nextWidth);
};

const initChatWidth = () => {
  const viewportWidth = getViewportWidth();
  if (!viewportWidth) return;
  // 第一次打开时按当前窗口宽度的 30% 初始化，给出默认侧栏占比。
  chatPanelWidth.value = clampChatWidth(Math.round(viewportWidth * 0.3));
};

const workspaceStyle = computed(() => ({
  '--chat-panel-width': `${chatPanelWidth.value}px`,
}));

const chatScopeType = computed(() => (view.value === 'reader' ? 'book' : 'library'));
const chatScopeKey = computed(() => (view.value === 'reader' ? currentBookPath.value : 'library'));

const loadApiKey = async () => {
  try {
    apiKey.value = await ChatService.GetAPIKey();
    return apiKey.value;
  } catch (err) {
    console.error('读取 API Key 失败', err);
    return '';
  }
};

const toggleSettings = async () => {
  if (!settingsOpen.value) {
    await loadApiKey();
  }
  settingsOpen.value = !settingsOpen.value;
};

const toggleChatPanel = () => {
  const willOpen = !chatOpen.value;
  chatOpen.value = willOpen;
  if (willOpen) {
    // 用户如果没有拖拽过，就按默认比例初始化；拖拽过则保留用户偏好。
    if (!hasCustomChatWidth.value || !chatPanelWidth.value) {
      initChatWidth();
    } else {
      syncChatWidth();
    }
    // 只有当聊天面板实际宽度没有超过 30% 时，才自动调用 Reader 的 fitWidth。
    const viewportWidth = getViewportWidth();
    const shouldAutoFitReader =
      view.value === 'reader' &&
      viewportWidth > 0 &&
      chatPanelWidth.value <= Math.floor(viewportWidth * 0.33);

    if (shouldAutoFitReader) {
      nextTick(() => {
        readerRef.value?.FitWidth();
      });
    }
  }
};

const saveApiKey = async (nextKey) => {
  try {
    await ChatService.SaveAPIKey(nextKey);
    apiKey.value = nextKey;
    settingsOpen.value = false;
    
    // API Key 保存后，检查是否需要显示/隐藏引导界面
    if (bookshelfRef.value) {
      bookshelfRef.value.checkOnboarding(nextKey);
    }
  } catch (err) {
    alert('保存 API Key 失败：' + err.message);
  }
};

const startChatResize = (event) => {
  if (!chatOpen.value) return;
  event.preventDefault();
  isResizingChat.value = true;
  resizeStartX.value = event.clientX;
  resizeStartWidth.value = chatPanelWidth.value;
  window.addEventListener('pointermove', handleChatResizeMove);
  window.addEventListener('pointerup', stopChatResize);
};

const handleChatResizeMove = (event) => {
  if (!isResizingChat.value) return;
  const delta = resizeStartX.value - event.clientX;
  // 一旦发生拖拽，就标记为用户自定义宽度，后续再次打开时不再强制恢复默认值。
  hasCustomChatWidth.value = true;
  chatPanelWidth.value = clampChatWidth(resizeStartWidth.value + delta);
};

const stopChatResize = () => {
  if (!isResizingChat.value) return;
  isResizingChat.value = false;
  window.removeEventListener('pointermove', handleChatResizeMove);
  window.removeEventListener('pointerup', stopChatResize);
};

onMounted(() => {
  loadApiKey().then(() => {
    // API Key 加载完成后，检查是否需要显示引导界面
    // 使用 nextTick 确保 bookshelfRef 已经挂载
    nextTick(() => {
      if (bookshelfRef.value) {
        bookshelfRef.value.checkOnboarding(apiKey.value);
      }
    });
  });
  window.addEventListener('resize', syncChatWidth);
});

onBeforeUnmount(() => {
  window.removeEventListener('resize', syncChatWidth);
  stopChatResize();
});

// --- 阅读器逻辑 ---
const openBook = async (book) => {
  currentBookTitle.value = book.title;
  currentBookPath.value = book.path;
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

    if (blob.size === 0) throw new Error('Empty PDF');

    pdfSource.value = URL.createObjectURL(blob);
  } catch (err) {
    alert('无法打开文件：' + err.message);
    goBack();
  }
};

const goBack = () => {
  view.value = 'library';
  currentBookPath.value = '';
  pdfSource.value = null;
};

const onPDFLoaded = (pdf) => {
  console.log(pdf.numPages); 
  totalPages.value = pdf.numPages;
};

const onPDFRescale = (newScale) => {
  zoomLevel.value = newScale;
};

const goToNextPage = () => {
  readerRef.value?.GoToNextPage();
};

const goToPrevPage = () => {
  readerRef.value?.GoToPrevPage();
};

const zoomIn = () => {
  readerRef.value?.ZoomIn();
};

const zoomOut = () => {
  readerRef.value?.ZoomOut();
};

const fitWidth = () => {
  readerRef.value?.FitWidth();
};

const goToPageInput = (event) => {
  let n = Number(currentPage.value);
  if (!Number.isFinite(n) || n <= 0) {
    currentPage.value = Math.min(Math.max(currentPage.value || 1, 1), totalPages.value || 1);
    return;
  }
  n = Math.min(Math.max(Math.floor(n), 1), totalPages.value || 1);
  currentPage.value = n;
  readerRef.value?.goToPage(n);

  // 如果由回车触发，则取消焦点（隐藏输入框）
  if (event && event.type === 'keyup') {
    pageInputRef.value?.blur();
  }
};

/**
 * 截取当前页并发送到剪贴板和聊天面板
 */
const captureCurrentPage = async () => {
  try {
    const imageDataUrl = await readerRef.value?.captureCurrentPage();
    if (!imageDataUrl) {
      alert('截图失败');
      return;
    }

    // 发送到剪贴板
    await sendToClipboard(imageDataUrl);

    // 如果聊天面板开着，自动添加到附件列表
    if (chatOpen.value) {
      await addImageToChat(imageDataUrl);
    }

    console.log('当前页截图成功');
  } catch (err) {
    console.error('截图失败:', err);
    alert('截图失败: ' + err.message);
  }
};

/**
 * 开始框选区域截图
 */
const startAreaSelection = async () => {
  try {
    const imageDataUrl = await readerRef.value?.startAreaSelection();
    if (!imageDataUrl) {
      console.log('用户取消了框选');
      return;
    }

    // 发送到剪贴板
    await sendToClipboard(imageDataUrl);

    // 如果聊天面板开着，自动添加到附件列表
    if (chatOpen.value) {
      await addImageToChat(imageDataUrl);
    }

    console.log('框选截图成功');
  } catch (err) {
    console.error('框选截图失败:', err);
    alert('框选截图失败: ' + err.message);
  }
};

/**
 * 将图片数据 URL 发送到剪贴板
 * @param {string} dataUrl - 图片的 data URL
 */
const sendToClipboard = async (dataUrl) => {
  try {
    // 将 data URL 转换为 Blob
    const response = await fetch(dataUrl);
    const blob = await response.blob();

    // 创建 ClipboardItem 并写入剪贴板
    const clipboardItem = new ClipboardItem({ [blob.type]: blob });
    await navigator.clipboard.write([clipboardItem]);
    console.log('已复制到剪贴板');
  } catch (err) {
    console.error('复制到剪贴板失败:', err);
    // 某些浏览器可能不支持，静默失败
  }
};

/**
 * 将图片添加到聊天面板的附件列表
 * @param {string} dataUrl - 图片的 data URL
 */
const addImageToChat = async (dataUrl) => {
  try {
    // 通过 chatPanelRef 访问 ChatPanel 组件的方法
    if (chatPanelRef.value && typeof chatPanelRef.value.addAttachmentFromDataUrl === 'function') {
      await chatPanelRef.value.addAttachmentFromDataUrl(dataUrl);
      console.log('已添加图片到聊天面板');
    } else {
      console.warn('ChatPanel 未暴露 addAttachmentFromDataUrl 方法');
    }
  } catch (err) {
    console.error('添加图片到聊天面板失败:', err);
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

.btn-square {
  padding: 0;
}

.settings-btn svg {
  width: 16px;
  height: 16px;
  fill: currentColor;
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

.header-right {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-left: auto;
  min-width: 0;
}

.library-toolbar {
  margin-left: auto;
}

.reader-toolbar {
  margin-left: 0;
}

.settings-anchor {
  position: relative;
}

.ai-btn {
  background: linear-gradient(180deg, #1457c7, #0f4aa8);
  border-color: rgba(0, 0, 0, 0.08);
  color: #fff;
  font-size: 13px;
  font-weight: 700;
  letter-spacing: 0.04em;
}

.ai-btn:hover,
.ai-btn.active {
  background: linear-gradient(180deg, #0f4aa8, #0b3b85);
  border-color: rgba(0, 0, 0, 0.08);
}

.page-navigation {
  display: flex;
  align-items: center;
  background: #f0f2f5;
  border-radius: 8px;
  padding: 2px 8px; /* 紧凑一点 */
  gap: 8px;
}

.reader-controls {
  display: flex;
  align-items: center;
  gap: 8px;
}

.screenshot-controls {
  display: flex;
  align-items: center;
  gap: 6px;
}

.screenshot-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  padding: 0;
  background: linear-gradient(180deg, #e8f4ff, #d0e8ff);
  border: 1px solid rgba(0, 122, 204, 0.3);
  color: #007acc;
  border-radius: 6px;
  transition: all 0.2s ease;
  cursor: pointer;
}

.screenshot-btn:hover {
  background: linear-gradient(180deg, #d0e8ff, #b8dcff);
  border-color: rgba(0, 122, 204, 0.5);
  transform: translateY(-1px);
  box-shadow: 0 2px 4px rgba(0, 122, 204, 0.2);
}

.screenshot-btn:active {
  transform: translateY(0);
  box-shadow: 0 1px 2px rgba(0, 122, 204, 0.15);
}

.screenshot-btn svg {
  display: block;
  width: 16px;
  height: 16px;
}

.zoom-controls {
  display: flex;
  align-items: center;
  background: #f0f2f5;
  border-radius: 8px;
  padding: 2px 6px;
  gap: 6px;
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

.toolbar { --toolbar-font-size: 13px; }

.page-display {
  display: flex;
  align-items: center;
  gap: 0; /* 不要额外间距，由分隔符内部 margin 控制 */
}

.page-total {
  width: 28px; /* 与 page-input 宽度相同 */
  height: 28px;
  margin: 0 4px; /* 分隔符左右间距 */
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: var(--toolbar-font-size);
  color: var(--text-primary);
  font-variant-numeric: tabular-nums;
  border-radius: 4px;
  background: transparent;
  border: 1px solid transparent;
}

.page-input {
  width: 28px; /* 固定宽度 */
  height: 28px;
  margin: 0 4px; /* 与 page-total 间距 */
  text-align: center;
  border-radius: 4px;
  border: 1px solid transparent;
  background: transparent;
  padding: 0 6px;
  transition: background-color 0.12s ease, box-shadow 0.12s ease, border-color 0.12s ease;
  font-size: var(--toolbar-font-size);
  color: var(--text-primary);
}

.page-input:focus {
  border-color: var(--border-color);
  background: white;
  box-shadow: 0 0 0 3px rgba(0,122,204,0.08);
  outline: none;
}

/* 悬停时也显示输入框的边框和背景，但不改变宽度 */
.page-navigation:hover .page-input {
  border-color: var(--border-color);
  background: white;
  box-shadow: 0 0 0 3px rgba(0,122,204,0.04);
}

/* 分隔符：确保左右间距完全对称 */
.page-delimiter {
  display: inline-block;
  margin: 0; /* 紧凑但对称 */
  color: var(--text-secondary);
  font-size: var(--toolbar-font-size);
  line-height: 1;
  width: auto;
}

.page-info {
  font-size: 14px;
  color: var(--text-secondary);
  font-variant-numeric: tabular-nums; /* 数字等宽，防止跳动 */
}

/* --- 主内容区 --- */
.main-content {
  flex: 1;
  min-height: 0;
  position: relative;
}

.workspace-shell {
  flex: 1;
  min-height: 0;
  display: flex;
  align-items: stretch;
  overflow: hidden;
  --chat-panel-width: 420px;
}

.content-pane {
  flex: 1 1 auto;
  min-width: 0;
}

.chat-pane {
  flex: 0 0 var(--chat-panel-width);
  width: var(--chat-panel-width);
  min-width: 280px;
  max-width: 800px;
  min-height: 0;
}

.chat-resizer {
  width: 8px;
  flex: 0 0 8px;
  cursor: col-resize;
  position: relative;
  z-index: 5;
  background: linear-gradient(180deg, rgba(0, 0, 0, 0.02), rgba(0, 0, 0, 0.04));
}

.chat-resizer::before {
  content: '';
  position: absolute;
  top: 50%;
  left: 50%;
  width: 2px;
  height: 42px;
  transform: translate(-50%, -50%);
  border-radius: 999px;
  background: rgba(0, 0, 0, 0.12);
}

.chat-resizer:hover::before {
  background: rgba(0, 122, 204, 0.55);
}

</style>