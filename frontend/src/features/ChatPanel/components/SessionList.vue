<template>
  <!--
    SessionList - 会话列表组件
    
    职责：
    - 展示当前作用域下的所有会话
    - 支持加载、选择、删除会话
    - 显示会话元数据（标题、消息数、更新时间）
    - 处理空状态和加载状态
  -->
  <div class="chat-sessions">
    <!-- 会话列表标题和计数 -->
    <div class="chat-section-head">
      <h4>会话</h4>
      <span class="chat-section-count">{{ sessions.length }}</span>
    </div>

    <!-- 加载状态提示 -->
    <div v-if="loadingSessions" class="chat-empty compact">正在加载会话...</div>
    <!-- 空状态提示 -->
    <div v-else-if="sessions.length === 0" class="chat-empty compact">
      当前作用域还没有会话
    </div>
    <!-- 会话列表项 -->
    <div v-else class="chat-session-list">
      <div
        v-for="session in sessions"
        :key="session.session_id"
        class="chat-session-item"
        :class="{ active: session.session_id === activeSessionId }"
        role="button"
        tabindex="0"
        @click="loadSession(session.session_id)"
        @keydown.enter.prevent="loadSession(session.session_id)"
        @keydown.space.prevent="loadSession(session.session_id)"
      >
        <!-- 会话主要信息区 -->
        <div class="chat-session-main">
          <div class="chat-session-title">{{ session.title }}</div>
          <!-- 会话元数据：消息数量和更新时间 -->
          <div class="chat-session-meta">
            <span>{{ session.message_count }} 条消息</span>
            <span>{{ session.updated_at }}</span>
          </div>
        </div>
        <!-- 删除会话按钮（阻止事件冒泡，避免触发加载会话） -->
        <button
          class="chat-session-delete"
          type="button"
          title="删除会话"
          @click.stop="deleteSession(session.session_id)"
        >
          ×
        </button>
      </div>
    </div>

    <!-- 错误消息显示 -->
    <p v-if="errorMessage" class="chat-error selector">{{ errorMessage }}</p>
  </div>
</template>

<script setup>
/**
 * SessionList - 会话列表组件
 * 
 * 功能说明：
 * - 以列表形式展示所有可用会话
 * - 支持点击会话项加载对话内容
 * - 支持删除不需要的会话
 * - 提供加载状态和空状态提示
 */

import { defineProps, defineEmits } from 'vue';

// ========================================
// Props 定义
// ========================================

const props = defineProps({
  /** 会话列表数据 */
  sessions: {
    type: Array,
    required: true
  },
  /** 当前活跃的会话 ID */
  activeSessionId: {
    type: String,
    default: ''
  },
  /** 是否正在加载会话列表 */
  loadingSessions: {
    type: Boolean,
    default: false
  },
  /** 错误消息 */
  errorMessage: {
    type: String,
    default: ''
  }
});

// ========================================
// 事件定义
// ========================================

const emit = defineEmits([
  /** 用户点击会话项时触发 */
  'load-session',
  /** 用户点击删除按钮时触发 */
  'delete-session'
]);

// ========================================
// 事件处理函数
// ========================================

/**
 * 加载指定会话
 * @param {string} sessionId - 要加载的会话 ID
 */
const loadSession = (sessionId) => {
  emit('load-session', sessionId);
};

/**
 * 删除指定会话
 * @param {string} sessionId - 要删除的会话 ID
 */
const deleteSession = (sessionId) => {
  emit('delete-session', sessionId);
};
</script>

<style scoped>
.chat-sessions {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  padding: 16px 18px 18px;
}

.chat-section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}

.chat-section-head h4 {
  margin: 0;
  font-size: 13px;
}

.chat-section-count {
  font-size: 12px;
  color: var(--text-secondary);
}

.chat-session-list {
  flex: 1;
  min-height: 0;
  overflow: auto;
  display: grid;
  align-content: start;
  grid-auto-rows: max-content;
  gap: 10px;
  padding-right: 6px;
}

.chat-session-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  width: 100%;
  box-sizing: border-box;
  border: 1px solid rgba(0, 0, 0, 0.08);
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.9);
  padding: 12px;
  cursor: pointer;
  text-align: left;
  min-width: 0;
}

.chat-session-item.active {
  border-color: rgba(0, 122, 204, 0.4);
  box-shadow: 0 0 0 3px rgba(0, 122, 204, 0.08);
}

.chat-session-main {
  min-width: 0;
  flex: 1;
  overflow: hidden;
}

.chat-session-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.chat-session-meta {
  margin-top: 6px;
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  font-size: 11px;
  color: var(--text-secondary);
}

.chat-session-delete {
  flex: 0 0 24px;
  width: 24px;
  height: 24px;
  border: none;
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  font-size: 20px;
  line-height: 1;
  padding: 0;
}

.chat-empty.compact {
  min-height: 56px;
  padding: 14px 10px;
  justify-content: flex-start;
  border: 1px dashed rgba(0, 0, 0, 0.08);
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.55);
}

.chat-error.selector {
  margin-top: 12px;
}
</style>
