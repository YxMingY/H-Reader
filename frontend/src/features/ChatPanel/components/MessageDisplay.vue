<template>
  <!--
    MessageDisplay - 消息展示组件
    
    职责：
    - 渲染用户和 AI 的对话消息
    - 支持 Markdown 格式渲染（AI 消息）
    - 提供滚动容器，支持自动滚动到底部
    - 处理加载状态和空状态
  -->
  <div ref="messageViewportRef" class="chat-messages" @scroll="handleScroll">
    <!-- 加载状态 -->
    <div v-if="loadingMessages" class="chat-empty">正在加载对话...</div>
    <!-- 空状态 -->
    <div v-else-if="messages.length === 0" class="chat-empty">
      当前会话还没有消息，输入内容开始对话
    </div>
    <!-- 消息列表 -->
    <div v-else class="chat-message-list">
      <div
        v-for="(message, index) in messages"
        :key="`${message.created_at}-${index}`"
        class="chat-message"
        :class="message.role"
      >
        <!-- AI 助手消息：使用 Markdown 渲染（带防抖） -->
        <div
          v-if="message.role === 'assistant'"
          class="chat-message-content markdown-body"
          v-html="getRenderedHtml(index)"
        ></div>
        <!-- 用户消息：显示文本和图片附件 -->
        <div v-else class="chat-message-user">
          <!-- 图片附件列表 -->
          <div v-if="message.attachments && message.attachments.length > 0" class="user-attachments">
            <img
              v-for="(img, imgIndex) in message.attachments"
              :key="imgIndex"
              :src="getImageSrc(img)"
              :alt="'附件图片 ' + (imgIndex + 1)"
              class="attachment-image"
              @click="openImage(img)"
            />
          </div>
          <!-- 文本内容 -->
          <div v-if="message.content" class="chat-message-content">{{ message.content }}</div>
        </div>
        
        <!-- 📌 标记位置按钮（仅在有 readerRef 时显示） -->
        <div class="message-actions">
          <button
            v-if="readerRef"
            class="action-btn"
            @click="startPinMode(index)"
            title="在 PDF 中标记此对话的位置"
          >
            📌
          </button>
          <!-- 用户消息：显示重新编辑和删除按钮 -->
          <template v-if="message.role === 'user' && isLastRound(index)">
            <button
              class="action-btn"
              @click="onEditMessage(index)"
              title="重新编辑这条消息"
            >
              ✏️
            </button>
            <button
              class="action-btn"
              @click="onDeleteRound(index)"
              title="删除本轮对话"
            >
              🗑️
            </button>
          </template>
          <!-- AI 消息：只显示删除按钮 -->
          <template v-else-if="message.role === 'assistant' && isLastRound(index)">
            <button
              class="action-btn"
              @click="onDeleteRound(index)"
              title="删除本轮对话"
            >
              🗑️
            </button>
          </template>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
/**
 * MessageDisplay - 消息展示组件
 * 
 * 功能说明：
 * - 以气泡形式展示对话消息
 * - 用户消息右对齐，AI 消息左对齐
 * - AI 消息支持 Markdown 渲染（代码块、列表、引用等）
 * - 暴露 messageViewportRef 供父组件控制滚动
 * - 使用防抖优化流式输出的公式渲染
 */

import { ref, watch, onBeforeUnmount } from 'vue';
import { confirmDelete, confirmEdit } from '../../../composables/useToast';
import { previewImage } from '../../../composables/useImagePreview';

// ========================================
// Props 定义
// ========================================

const props = defineProps({
  /** 消息列表数据 */
  messages: {
    type: Array,
    required: true
  },
  /** 是否正在加载消息 */
  loadingMessages: {
    type: Boolean,
    default: false
  },
  /** Markdown 渲染函数 */
  renderMarkdown: {
    type: Function,
    required: true
  },
  streamingMessageIndex: {
    type: Number,
    default: -1
  },
  /** Reader 组件引用（用于调用 startSelectionMode） */
  readerRef: {
    type: Object,
    default: null
  },
  /** 当前会话 ID */
  sessionId: {
    type: String,
    default: ''
  },
  /** 判断是否是最新一轮对话的函数 */
  isLastRound: {
    type: Function,
    default: null
  },
  /** 删除本轮对话的处理函数 */
  handleDeleteRound: {
    type: Function,
    default: null
  },
  /** 重新编辑消息的处理函数 */
  handleEditMessage: {
    type: Function,
    default: null
  }
});

// ========================================
// Emits 定义
// ========================================

const emit = defineEmits([
  /** 滚动事件，向父组件传递 */
  'scroll',
  /** 编辑消息事件，传递消息内容 */
  'edit-message'
]);

// ========================================
// 响应式引用
// ========================================

/** 
 * 消息视口 DOM 引用
 * 用于父组件控制滚动位置（scrollToBottom）
 */
const messageViewportRef = ref(null);

/**
 * 渲染后的 HTML 缓存
 * 键：消息索引，值：渲染后的 HTML 字符串
 */
const renderedHtml = ref({});

/**
 * 标记正在流式输出的消息索引集合
 * 用于跟踪哪些消息还在接收新内容
 */
const streamingMessages = ref(new Set());

/**
 * 立即渲染消息内容
 * 
 * @param {Object} message - 消息对象
 * @param {number} index - 消息索引
 */
const renderMessage = (message, index) => {
  // 用户消息不需要 Markdown 渲染
  if (message.role !== 'assistant') {
    renderedHtml.value[index] = message.content;
    return;
  }

  // 直接渲染，不做防抖
  renderedHtml.value[index] = props.renderMarkdown(message.content, index === props.streamingMessageIndex);
};

/**
 * 监听消息列表变化
 * - 检测新增消息或内容更新
 * - 标记流式输出状态
 * - 在输出结束后重新渲染以确保公式完整
 */
watch(
  () => props.messages,
  (newMessages, oldMessages) => {
    newMessages.forEach((message, index) => {
      const oldMessage = oldMessages?.[index];
      
      // 检测是否是流式输出（内容在变化）
      if (oldMessage && message.content !== oldMessage.content) {
        // 标记为正在流式输出
        streamingMessages.value.add(index);
        
        // 立即渲染（保持实时性）
        renderMessage(message, index);
      } else if (!oldMessage || message.content !== renderedHtml.value[index]) {
        // 新消息或内容未同步，立即渲染
        renderMessage(message, index);
        
        // 如果不是流式输出，清除标记
        if (!streamingMessages.value.has(index)) {
          // 延迟一小段时间后检查是否需要重新渲染
          setTimeout(() => {
            // 如果内容没有继续变化，说明输出已结束
            if (props.messages[index]?.content === message.content) {
              streamingMessages.value.delete(index);
              // 重新渲染一次，确保公式完整
              renderMessage(props.messages[index], index);
            }
          }, 300);
        }
      }
    });
  },
  { deep: true, immediate: true }
);

// A stream can finish without changing the text of its last chunk.
watch(() => props.streamingMessageIndex, (index, previousIndex) => {
  for (const messageIndex of [previousIndex, index]) {
    const message = props.messages[messageIndex];
    if (message) renderMessage(message, messageIndex);
  }
});

/**
 * 获取消息的渲染结果
 * 
 * @param {number} index - 消息索引
 * @returns {string} 渲染后的 HTML
 */
const getRenderedHtml = (index) => {
  return renderedHtml.value[index] || '';
};

/**
 * 获取图片的 src
 * 
 * @param {string} imgPath - 图片路径或 base64
 * @returns {string} 图片的 src
 */
const getImageSrc = (imgPath) => {
  // 如果是 base64 或完整 URL，直接返回
  if (imgPath.startsWith('data:') || imgPath.startsWith('http://') || imgPath.startsWith('https://')) {
    return imgPath;
  }
  // 否则假设是本地文件路径，需要根据实际情况处理
  // TODO: 如果后端返回的是文件路径，可能需要转换为可访问的 URL
  return imgPath;
};

/**
 * 点击图片时打开大图
 * 
 * @param {string} imgPath - 图片路径
 */
const openImage = (imgPath) => {
  previewImage(imgPath, '消息附件图片');
};

/**
 * 处理滚动事件
 * 向父组件传递 scroll 事件
 */
const handleScroll = (event) => {
  emit('scroll', event);
};

/**
 * 开始大头针选择模式
 * @param {number} messageIndex - 消息索引
 */
const startPinMode = (messageIndex) => {
  if (!props.readerRef || !props.sessionId) {
    console.warn('无法启动大头针模式：缺少 readerRef 或 sessionId');
    return;
  }
  
  // 获取消息内容预览（前 20 个字）
  const message = props.messages[messageIndex];
  let messagePreview = '';
  if (message && message.content) {
    // 提取前 20 个字符，如果超过则添加省略号
    const content = message.content.trim();
    messagePreview = content.length > 20 ? content.substring(0, 20) + '...' : content;
  }
  
  console.log('启动大头针选择模式，消息索引:', messageIndex, '预览:', messagePreview);
  props.readerRef.startSelectionMode(props.sessionId, messageIndex, messagePreview);
};

/**
 * 处理重新编辑消息（包装函数）
 * @param {number} messageIndex - 消息索引
 */
const onEditMessage = async (messageIndex) => {
  if (!props.handleEditMessage) {
    console.warn('handleEditMessage 未提供');
    return;
  }
  
  // 显示确认框
  const confirmed = await confirmEdit();
  if (!confirmed) {
    return;
  }
  
  const content = await props.handleEditMessage(messageIndex);
  if (content) {
    emit('edit-message', content);
  }
};

/**
 * 处理删除本轮对话（包装函数）
 * @param {number} messageIndex - 消息索引
 */
const onDeleteRound = async (messageIndex) => {
  if (!props.handleDeleteRound) {
    console.warn('handleDeleteRound 未提供');
    return;
  }
  
  // 显示确认框
  const confirmed = await confirmDelete('本轮对话');
  if (!confirmed) {
    return;
  }
  
  props.handleDeleteRound(messageIndex);
};

/**
 * 滚动到指定消息（通过索引）
 * @param {number} messageIndex - 消息索引
 */
const scrollToMessage = (messageIndex) => {
  if (!messageViewportRef.value) {
    console.warn('无法滚动到消息：视口引用不存在');
    return;
  }
  
  const viewport = messageViewportRef.value;
  
  // 等待 DOM 更新完成
  setTimeout(() => {
    // 找到目标消息元素
    const messageElements = viewport.querySelectorAll('.chat-message');
    if (messageIndex >= messageElements.length) {
      console.warn('消息元素不存在', { messageIndex, elementCount: messageElements.length });
      return;
    }
    
    const targetElement = messageElements[messageIndex];
    if (!targetElement) {
      console.warn('目标消息元素不存在');
      return;
    }
    
    // 计算滚动位置，将消息滚动到视口中间
    const elementTop = targetElement.offsetTop;
    const elementHeight = targetElement.offsetHeight;
    const viewportHeight = viewport.clientHeight;
    const scrollTop = elementTop - (viewportHeight / 2) + (elementHeight / 2);
    
    // 直接滚动（不使用平滑效果）
    viewport.scrollTo({
      top: scrollTop,
      behavior: 'auto'
    });
    
    console.log('已滚动到消息索引:', messageIndex, '滚动位置:', scrollTop);
    
    // 添加高亮效果（可选）
    targetElement.classList.add('highlighted-message');
    setTimeout(() => {
      targetElement.classList.remove('highlighted-message');
    }, 2000);
  }, 50); // 短暂延迟确保 DOM 完全渲染
};

// ========================================
// 公开 API
// ========================================

defineExpose({
  /** 消息滚动容器 DOM 引用 */
  messageViewportRef,
  /** 滚动到指定消息（通过索引） */
  scrollToMessage
});

// ========================================
// 生命周期钩子
// ========================================

/**
 * 组件卸载时清理状态
 */
onBeforeUnmount(() => {
  // 清空缓存和状态
  renderedHtml.value = {};
  streamingMessages.value.clear();
});
</script>

<style scoped>
.chat-messages {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  overflow-x: hidden;
  padding-right: 4px;
  user-select: text;
  -webkit-user-select: text;
}

.chat-message-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-width: 0;
}

.chat-message {
  box-sizing: border-box;
  max-width: 100%;
  min-width: 0;
  width: auto;
  align-self: flex-start;
  padding: 12px 13px;
  border-radius: 16px;
  border: 1px solid rgba(0, 0, 0, 0.06);
  background: rgba(255, 255, 255, 0.88);
}

.chat-message.user {
  align-self: flex-end;
  background: linear-gradient(180deg, rgba(0, 122, 204, 0.08), rgba(0, 122, 204, 0.04));
  border-color: rgba(0, 122, 204, 0.14);
}

/* 用户消息容器：支持文本和图片混合显示 */
.chat-message-user {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

/* 用户消息的图片附件列表 */
.user-attachments {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

/* 附件图片样式 */
.attachment-image {
  max-width: 200px;
  max-height: 200px;
  width: auto;
  height: auto;
  object-fit: contain;
  border-radius: 8px;
  cursor: pointer;
  transition: transform 0.2s ease;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

.attachment-image:hover {
  transform: scale(1.05);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

.chat-message.assistant {
  background: rgba(255, 255, 255, 0.96);
}

.chat-message-content {
  min-width: 0;
  max-width: 100%;
  white-space: pre-wrap;
  word-break: break-word;
  overflow-wrap: anywhere;
  font-size: 13px;
  line-height: 1.65;
  color: var(--text-primary);
  user-select: text;
  -webkit-user-select: text;
}

.chat-message-content.markdown-body {
  min-width: 0;
  max-width: 100%;
  white-space: normal;
}

.chat-message-content.markdown-body :deep(*) {
  max-width: 100%;
}

.chat-message-content.markdown-body :deep(p) {
  margin: 0 0 0.75em;
}

.chat-message-content.markdown-body :deep(p:last-child) {
  margin-bottom: 0;
}

/* 标题样式 */
.chat-message-content.markdown-body :deep(h1),
.chat-message-content.markdown-body :deep(h2),
.chat-message-content.markdown-body :deep(h3),
.chat-message-content.markdown-body :deep(h4),
.chat-message-content.markdown-body :deep(h5),
.chat-message-content.markdown-body :deep(h6) {
  margin: 1em 0 0.5em;
  font-weight: 600;
  line-height: 1.4;
  color: var(--text-primary);
}

.chat-message-content.markdown-body :deep(h1) {
  font-size: 1.5em;
}

.chat-message-content.markdown-body :deep(h2) {
  font-size: 1.3em;
}

.chat-message-content.markdown-body :deep(h3) {
  font-size: 1.15em;
}

.chat-message-content.markdown-body :deep(h4) {
  font-size: 1.05em;
}

.chat-message-content.markdown-body :deep(h5),
.chat-message-content.markdown-body :deep(h6) {
  font-size: 1em;
}

.chat-message-content.markdown-body :deep(ul),
.chat-message-content.markdown-body :deep(ol) {
  margin: 0.5em 0 0.75em;
  padding-left: 1.4em;
}

.chat-message-content.markdown-body :deep(li + li) {
  margin-top: 0.25em;
}

.chat-message-content.markdown-body :deep(blockquote) {
  margin: 0.5em 0;
  padding: 0.2em 0 0.2em 0.9em;
  border-left: 3px solid rgba(0, 122, 204, 0.3);
  color: var(--text-secondary);
}

.chat-message-content.markdown-body :deep(pre) {
  margin: 0.75em 0;
  width: auto;
  padding: 12px 14px;
  overflow: auto;
  max-width: 100%;
  box-sizing: border-box;
  border-radius: 12px;
  border: 1px solid #d0d7de;
  background: #f6f8fa;
  color: #24292f;
  white-space: pre;
}

.chat-message-content.markdown-body :deep(pre code) {
  display: block;
  min-width: max-content;
  padding: 0;
  background: transparent;
  color: inherit;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre;
  overflow-wrap: normal;
  word-break: normal;
}

.chat-message-content.markdown-body :deep(code) {
  padding: 0.12em 0.35em;
  border-radius: 6px;
  background: rgba(175, 184, 193, 0.2);
  color: #24292f;
  font-size: 0.95em;
  overflow-wrap: anywhere;
  word-break: break-word;
}

.chat-message-content.markdown-body :deep(.hljs) {
  color: #24292f;
  background: transparent;
}

.chat-message-content.markdown-body :deep(a) {
  color: var(--accent-color);
  text-decoration: none;
  word-break: break-word;
  overflow-wrap: anywhere;
}

.chat-message-content.markdown-body :deep(a:hover) {
  text-decoration: underline;
}

.chat-message-content.markdown-body :deep(hr) {
  border: none;
  border-top: 1px solid rgba(0, 0, 0, 0.1);
  margin: 0.9em 0;
}

/* ========================================
   数学公式样式（KaTeX）
   ======================================== */

.chat-message-content.markdown-body :deep(.katex) {
  font-size: 1.1em;
  line-height: 1.2;
}

.chat-message-content.markdown-body :deep(.katex-display) {
  margin: 1em 0;
  overflow-x: auto;
  overflow-y: hidden;
  text-align: center;
}

/* 确保所有内容都可以被选中和复制 */
.chat-message-content.markdown-body :deep(*) {
  user-select: text !important;
  -webkit-user-select: text !important;
}

.chat-empty {
  min-height: 180px;
  display: flex;
  align-items: center;
  justify-content: center;
  text-align: center;
  color: var(--text-secondary);
  padding: 20px;
  font-size: 13px;
  line-height: 1.6;
}

/* ========================================
   📌 大头针标记按钮样式
   ======================================== */

/* 消息操作按钮容器 */
.message-actions {
  margin-top: 8px;
  display: flex;
  gap: 6px;
  justify-content: flex-end;
}

/* 无边框图标按钮 */
.action-btn {
  padding: 4px 8px;
  font-size: 16px;
  color: #666;
  background: transparent;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  transition: all 0.2s ease;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 28px;
  height: 28px;
}

.action-btn:hover {
  background: rgba(0, 0, 0, 0.08);
  color: #333;
  transform: scale(1.1);
}

.action-btn:active {
  transform: scale(0.95);
}

/* 📌 大头针导航高亮效果 */
.highlighted-message {
  animation: highlight-pulse 2s ease-in-out;
  border: 2px solid #007acc;
  border-radius: 8px;
  box-shadow: 0 0 12px rgba(0, 122, 204, 0.3);
}

@keyframes highlight-pulse {
  0% {
    background-color: rgba(0, 122, 204, 0.2);
  }
  50% {
    background-color: rgba(0, 122, 204, 0.1);
  }
  100% {
    background-color: transparent;
  }
}
</style>
