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
      >
        <!-- 会话主要信息区 -->
        <div class="chat-session-main">
          <!-- 会话标题（支持内联编辑） -->
          <div 
            v-if="editingSessionId !== session.session_id"
            class="chat-session-title"
          >
            {{ session.title }}
          </div>
          <input
            v-else
            :ref="(el) => setTitleInputRef(el, session.session_id)"
            v-model="editingTitle"
            class="chat-session-title-input"
            type="text"
            @blur="finishEditing(session.session_id)"
            @keydown.enter="finishEditing(session.session_id)"
            @keydown.esc="cancelEditing()"
            @click.stop
          />
          <!-- 会话元数据：消息数量和更新时间 -->
          <div class="chat-session-meta">
            <span>{{ session.message_count }} 条消息</span>
            <span>{{ session.updated_at }}</span>
          </div>
        </div>
        <!-- 操作菜单按钮 -->
        <div class="chat-session-actions" @click.stop>
          <button
            class="chat-session-menu-btn"
            type="button"
            title="更多操作"
            @click="toggleMenu(session.session_id, $event)"
          >
            ⋯
          </button>
          <!-- 下拉菜单 -->
          <div
            v-if="activeMenuId === session.session_id"
            class="chat-session-menu"
            @click.stop
          >
            <button
              class="chat-menu-item"
              type="button"
              @click="startEditingFromMenu(session.session_id, session.title)"
            >
              编辑标题
            </button>
            <button
              class="chat-menu-item chat-menu-item-danger"
              type="button"
              @click="deleteSession(session.session_id)"
            >
              删除会话
            </button>
          </div>
        </div>
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
 * - 支持双击编辑会话标题
 * - 提供加载状态和空状态提示
 */

import { ref, nextTick, onMounted, onBeforeUnmount } from 'vue';

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
  'delete-session',
  /** 用户更新会话标题时触发 */
  'update-session-title'
]);

// ========================================
// 响应式状态
// ========================================

/** 正在编辑的会话 ID */
const editingSessionId = ref('');
/** 编辑中的标题文本 */
const editingTitle = ref('');
/** 输入框引用映射（sessionId -> input element） */
const titleInputRefs = new Map();
/** 当前激活的菜单 ID */
const activeMenuId = ref('');

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

/**
 * 设置输入框引用（动态 ref）
 * @param {HTMLElement|null} el - 输入框元素
 * @param {string} sessionId - 会话 ID
 */
const setTitleInputRef = (el, sessionId) => {
  if (el) {
    titleInputRefs.set(sessionId, el);
  } else {
    titleInputRefs.delete(sessionId);
  }
};

/**
 * 开始编辑会话标题
 * @param {string} sessionId - 会话 ID
 * @param {string} currentTitle - 当前标题
 */
const startEditing = async (sessionId, currentTitle) => {
  editingSessionId.value = sessionId;
  editingTitle.value = currentTitle;
  activeMenuId.value = ''; // 关闭菜单
  
  // 等待 DOM 更新后聚焦输入框
  await nextTick();
  const inputEl = titleInputRefs.get(sessionId);
  if (inputEl) {
    inputEl.focus();
    inputEl.select();
  }
};

/**
 * 从菜单中开始编辑（包装函数）
 * @param {string} sessionId - 会话 ID
 * @param {string} currentTitle - 当前标题
 */
const startEditingFromMenu = (sessionId, currentTitle) => {
  startEditing(sessionId, currentTitle);
};

/**
 * 切换菜单显示状态
 * @param {string} sessionId - 会话 ID
 * @param {Event} event - 点击事件
 */
const toggleMenu = (sessionId, event) => {
  event.stopPropagation();
  if (activeMenuId.value === sessionId) {
    activeMenuId.value = '';
  } else {
    activeMenuId.value = sessionId;
  }
};

/**
 * 完成编辑并保存
 * @param {string} sessionId - 会话 ID
 */
const finishEditing = (sessionId) => {
  const newTitle = editingTitle.value.trim();
  if (newTitle && newTitle !== props.sessions.find(s => s.session_id === sessionId)?.title) {
    emit('update-session-title', sessionId, newTitle);
  }
  cancelEditing();
};

/**
 * 取消编辑
 */
const cancelEditing = () => {
  editingSessionId.value = '';
  editingTitle.value = '';
};

/**
 * 点击外部关闭菜单
 */
const handleClickOutside = (event) => {
  if (!event.target.closest('.chat-session-actions')) {
    activeMenuId.value = '';
  }
};

// ========================================
// 生命周期钩子
// ========================================

/**
 * 组件挂载时添加全局点击监听器
 */
onMounted(() => {
  document.addEventListener('click', handleClickOutside);
});

/**
 * 组件卸载前移除事件监听器
 */
onBeforeUnmount(() => {
  document.removeEventListener('click', handleClickOutside);
});
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

.chat-session-title-input {
  width: 100%;
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary);
  border: 1px solid rgba(0, 122, 204, 0.4);
  border-radius: 4px;
  padding: 2px 6px;
  background: rgba(255, 255, 255, 0.95);
  outline: none;
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

/* 操作按钮容器 */
.chat-session-actions {
  position: relative;
  flex: 0 0 24px;
}

/* 菜单按钮 */
.chat-session-menu-btn {
  width: 24px;
  height: 24px;
  border: none;
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  font-size: 20px;
  line-height: 1;
  padding: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
  transition: background-color 0.2s;
}

.chat-session-menu-btn:hover {
  background: rgba(0, 0, 0, 0.05);
}

/* 下拉菜单 */
.chat-session-menu {
  position: absolute;
  top: 100%;
  right: 0;
  margin-top: 4px;
  background: white;
  border: 1px solid rgba(0, 0, 0, 0.1);
  border-radius: 8px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  min-width: 120px;
  z-index: 100;
  overflow: hidden;
}

/* 菜单项 */
.chat-menu-item {
  width: 100%;
  padding: 10px 16px;
  border: none;
  background: transparent;
  text-align: left;
  font-size: 13px;
  color: var(--text-primary);
  cursor: pointer;
  transition: background-color 0.2s;
}

.chat-menu-item:hover {
  background: rgba(0, 0, 0, 0.05);
}

/* 危险操作（删除） */
.chat-menu-item-danger {
  color: #dc2626;
}

.chat-menu-item-danger:hover {
  background: rgba(220, 38, 38, 0.05);
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
