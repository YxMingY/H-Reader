<template>
  <!--
    ChatPanel - AI 对话面板主组件
    
    职责：
    - 协调会话列表、消息展示和输入区域三个子组件
    - 管理作用域切换（书架级/书籍级）
    - 处理消息发送和流式响应
    - 提供统一的聊天界面入口
  -->
  <div class="chat-panel">
    <!-- 头部区域：显示当前作用域信息和操作按钮 -->
    <div class="chat-header">
      <!-- 作用域副标题：显示当前是书架级还是书籍级对话 -->
      <div class="chat-header-copy">
        <p class="chat-subtitle">{{ scopeSubtitle }}</p>
      </div>

      <!-- 操作按钮组：根据当前状态显示不同按钮 -->
      <div class="chat-header-actions">
        <!-- 在会话详情页时显示"返回会话列表"按钮 -->
        <button
          v-if="activeSessionId"
          class="chat-header-btn"
          type="button"
          @click="backToSessions"
        >
          返回
        </button>
        <!-- 在会话列表页时显示"刷新"按钮 -->
        <button
          style="display: none;"
          v-else
          class="chat-header-btn"
          type="button"
          @click="refreshSessions"
          :disabled="loadingSessions"
        >
          刷新
        </button>
        <!-- 仅在会话列表页时显示"新会话"按钮 -->
        <button v-if="!activeSessionId" class="chat-header-btn primary" type="button" @click="createSessionAndEnter" @mousedown="clearErrorBeforeAction">
          新会话
        </button>
      </div>
    </div>

    <!-- 会话列表视图：当没有活跃会话时显示 -->
    <SessionList
      v-if="!activeSessionId"
      :sessions="sessions"
      :active-session-id="activeSessionId"
      :loading-sessions="loadingSessions"
      :error-message="errorMessage"
      @load-session="loadSession"
      @delete-session="deleteSession"
    />

    <!-- 会话详情视图：当有活跃会话时显示 -->
    <div v-else class="chat-body">
      <!-- 消息滚动容器：包含所有对话消息 -->
      <MessageDisplay
        ref="messageDisplayRef"
        :messages="messages"
        :loading-messages="loadingMessages"
        :render-markdown="renderMarkdown"
        @scroll="handleScroll"
      />
    
      <!-- 消息输入区域：包含附件预览、输入框和操作按钮 -->
      <InputArea
        :draft="draft"
        @update:draft="draft = $event"
        :attachments="attachments"
        :sending="sending"
        :error-message="errorMessage"
        :clear-error-on-input="clearErrorOnInput"
        :handle-paste="handlePaste"
        :remove-attachment="removeAttachment"
        :handle-file-input="handleFileInput"
        @send-message="sendMessage"
      />
    </div>
  </div>
</template>

<script setup>
/**
 * ChatPanel - AI 对话面板主组件
 * 
 * 架构说明：
 * 本组件采用组合式 API + 子组件拆分的架构模式
 * - 业务逻辑通过 composables 封装（useSession, useChatInput, useChatStream）
 * - UI 拆分为三个独立子组件（SessionList, MessageDisplay, InputArea）
 * - 主组件负责状态管理和组件协调
 * 
 * 作用域系统：
 * - library: 书架级会话，所有书籍共享同一个会话空间
 * - book: 书籍级会话，每本书有独立的会话空间
 * - 使用 bookKey（文件哈希）而非 bookPath 标识书籍，避免路径变化导致会话丢失
 */

import { computed, nextTick, ref, watch } from 'vue';
import { ChatService } from '../../../bindings/hreader/services/chat';
import { useChatInput, useChatStream, useSession, useTools, useScrollBottom } from './composables';
import { SessionList, MessageDisplay, InputArea } from './components';

// ========================================
// Props 定义
// ========================================

const props = defineProps({
  /** 作用域类型：'library' | 'book' */
  scopeType: {
    type: String,
    default: 'library',
  },
  /** 书籍文件路径（仅 book 作用域有效） */
  bookPath: {
    type: String,
    default: '',
  },
  /** 书籍显示标题 */
  bookTitle: {
    type: String,
    default: '',
  },
  /** 书籍唯一标识（基于文件内容哈希） */
  bookKey: {
    type: String,
    default: '',
  },
});

// 事件定义
const emit = defineEmits(['close']);

// ========================================
// 响应式状态
// ========================================

/** 消息发送状态标志 */
const sending = ref(false);
/** 统一错误消息存储 */
const errorMessage = ref('');
/** 消息显示组件引用（用于获取滚动容器） */
const messageDisplayRef = ref(null);

// ========================================
// 计算属性 - 作用域相关
// ========================================

/** 作用域标题：根据类型显示书籍名称或“全局书架” */
const scopeTitle = computed(() => 
  props.scopeType === 'book' 
    ? props.bookTitle || props.bookPath || '当前书籍' 
    : '全局书架'
);

/** 作用域副标题：显示在头部，帮助用户理解当前对话范围 */
const scopeSubtitle = computed(() => 
  props.scopeType === 'book' 
    ? `当前书籍：${scopeTitle.value}` 
    : '当前作用域：书架'
);

/** 作用域书籍路径：仅在书籍级作用域时有值 */
const scopeBookPath = computed(() => 
  props.scopeType === 'book' ? props.bookPath : ''
);

/** 
 * 作用域唯一键：用于 watch 检测作用域切换
 * 格式："library:library" 或 "book:<bookKey>"
 */
const scopeKey = computed(() => 
  `${props.scopeType}:${props.bookKey || scopeBookPath.value || 'library'}`
);

// ========================================
// 工具函数
// ========================================

/** Markdown 渲染工具 */
const { renderMarkdown } = useTools();

/** 智能滚动管理 */
const {
  userScrolledAway,
  handleScroll,
  scrollToBottom: smartScrollToBottom,
  forceScrollToBottom,
  resetScrollState,
} = useScrollBottom();

/**
 * 将消息视口滚动到底部（包装函数）
 * 
 * 调用 useScrollBottom composable 中的智能滚动方法
 * 自动获取消息容器并传递
 */
const scrollToBottom = async () => {
  const container = messageDisplayRef.value?.messageViewportRef;
  await smartScrollToBottom(container);
};

// ========================================
// Composables 初始化
// ========================================

/** 输入管理：处理草稿、附件、文件选择等 */
const { 
  draft,
  attachments,
  fileInputRef,
  clearInput,
  handleFileInput,
  addAttachmentFromDataUrl,
  handlePaste,
  clearErrorOnInput,
  clearErrorBeforeAction,
  removeAttachment,
} = useChatInput(sending, errorMessage);

/** 会话管理：处理会话列表、创建、加载、删除等操作 */
const { 
  sessions, 
  activeSessionId, 
  messages, 
  loadingSessions, 
  loadingMessages, 
  refreshSessions, 
  loadSession, 
  createSession, 
  createSessionAndEnter, 
  backToSessions, 
  deleteSession 
} = useSession(props, scopeTitle, scopeBookPath, errorMessage, scrollToBottom, resetScrollState, clearInput);

/** 流式响应管理：处理后端推送的实时消息更新 */
const { 
  streamSessionId,
  streamAssistantIndex,
  resetStreamingState,
  sendMessage,
  onStreamChunk,
  onStreamDone,
  onStreamError,
} = useChatStream(activeSessionId, sending, messages, errorMessage, loadSession, refreshSessions, scrollToBottom, createSession, props, draft, attachments);



// ========================================
// 生命周期和监听器
// ========================================

/**
 * 作用域切换监听器
 * 
 * 触发场景：
 * - 用户从书架切换到书籍阅读界面
 * - 用户从书籍阅读返回书架
 * - 用户在不同的书籍之间切换
 * 
 * 清理策略：
 * 1. 清除活跃会话 ID（避免跨作用域污染）
 * 2. 清空消息列表（防止显示错误的数据）
 * 3. 清空输入状态（草稿、附件等）
 * 4. 重新加载新作用域的会话列表
 * 
 * 技术细节：
 * - 使用 immediate: true 确保组件挂载时立即执行初始化
 * - 监听 scopeKey 而非单个 prop，避免多次触发
 */
watch(
  () => scopeKey.value,
  async () => {
    // 清理旧作用域的状态
    activeSessionId.value = '';
    messages.value = [];
    clearInput();
    
    // 加载新作用域的会话列表
    await refreshSessions();
  },
  { immediate: true }
);

/**
 * 错误消息监听器（调试用）
 * 用于追踪错误状态变化，方便开发调试
 */
watch(
  () => errorMessage.value,
  (newVal, oldVal) => {
    console.log('errorMessage changed:', { old: oldVal, new: newVal });
  }
);

// ========================================
// 公开 API
// ========================================

/**
 * 暴露给父组件的方法
 * 
 * 可通过 ref 访问：
 * const chatPanelRef = ref(null);
 * chatPanelRef.value?.addAttachmentFromDataUrl(dataUrl, fileName);
 */
defineExpose({
  /** 从 Data URL 添加附件（用于截图等功能） */
  addAttachmentFromDataUrl,
});

</script>

<style scoped>
.chat-panel {
  height: 100%;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(246, 248, 252, 0.98));
  border-left: 1px solid rgba(0, 0, 0, 0.08);
  box-shadow: inset 1px 0 0 rgba(255, 255, 255, 0.7);
}

.chat-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding: 18px 18px 14px;
  border-bottom: 1px solid rgba(0, 0, 0, 0.06);
}

.chat-header-copy {
  min-width: 0;
}

.chat-kicker {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  height: 22px;
  padding: 0 9px;
  border-radius: 999px;
  background: rgba(0, 122, 204, 0.1);
  color: var(--accent-color);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.08em;
}

.chat-title {
  margin: 8px 0 0;
  font-size: 18px;
  line-height: 1.2;
}

.chat-subtitle {
  margin: 6px 0 0;
  font-size: 12px;
  color: var(--text-secondary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.chat-header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.chat-header-btn,
.chat-close {
  border: 1px solid rgba(0, 0, 0, 0.08);
  background: rgba(255, 255, 255, 0.9);
  color: var(--text-primary);
  border-radius: 10px;
  height: 30px;
  padding: 0 10px;
  font-size: 12px;
  cursor: pointer;
}

.chat-header-btn.primary {
  background: var(--accent-color);
  color: #fff;
  border-color: transparent;
}

.chat-close {
  width: 30px;
  padding: 0;
  font-size: 20px;
  line-height: 1;
}

.chat-body {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  padding: 16px 18px 18px;
  gap: 14px;
}
</style>
