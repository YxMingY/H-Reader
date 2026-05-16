<template>
  <!--
    MessageDisplay - 消息展示组件
    
    职责：
    - 渲染用户和 AI 的对话消息
    - 支持 Markdown 格式渲染（AI 消息）
    - 提供滚动容器，支持自动滚动到底部
    - 处理加载状态和空状态
  -->
  <div ref="messageViewportRef" class="chat-messages">
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
        <!-- 用户消息：纯文本显示 -->
        <div v-else class="chat-message-content">{{ message.content }}</div>
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
  }
});

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
  renderedHtml.value[index] = props.renderMarkdown(message.content);
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

/**
 * 获取消息的渲染结果
 * 
 * @param {number} index - 消息索引
 * @returns {string} 渲染后的 HTML
 */
const getRenderedHtml = (index) => {
  return renderedHtml.value[index] || '';
};

// ========================================
// 公开 API
// ========================================

defineExpose({
  /** 消息滚动容器 DOM 引用 */
  messageViewportRef
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
</style>
