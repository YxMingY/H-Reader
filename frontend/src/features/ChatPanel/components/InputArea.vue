<template>
  <!--
    InputArea - 消息输入区域组件
    
    职责：
    - 提供文本输入框（支持多行、快捷键发送）
    - 显示和管理图片附件（预览、删除）
    - 支持粘贴截图和选择文件
    - 提供发送按钮和操作按钮
    - 显示错误消息
  -->
  <div class="chat-composer">
    <!-- 附件预览区：显示已选择的图片缩略图 -->
    <div v-if="attachments.length" class="attachment-strip">
      <div v-for="item in attachments" :key="item.id" class="attachment-thumb">
        <img :src="item.dataUrl" :alt="item.name" />
        <!-- 移除附件按钮 -->
        <button type="button" class="attachment-remove" @click="removeAttachment(item.id)">×</button>
      </div>
    </div>
    

    <!-- 文本输入框：支持多行输入、快捷键发送和粘贴图片 -->
    <textarea
      :value="draft"
      class="chat-input"
      rows="3"
      placeholder="输入文本，或直接粘贴截图（Ctrl+V）"
      @input="(e) => { $emit('update:draft', e.target.value); clearErrorOnInput(); }"
      @keydown.meta.enter.prevent="sendMessage"
      @keydown.ctrl.enter.prevent="sendMessage"
      @keydown.alt.enter.prevent="sendMessage"
      @keydown.shift.enter.prevent="sendMessage"
      @paste="handlePaste"
    ></textarea>

    <!-- 操作按钮区：图片选择和发送按钮 -->
    <div class="composer-actions">
      <!-- 隐藏的文件选择器 -->
      <input
        ref="localFileInputRef"
        class="file-input"
        type="file"
        accept="image/*"
        multiple
        @change="handleFileInput"
      />
      <!-- 触发文件选择器 -->
      <button class="chat-header-btn" type="button" @click="handlePickImages">添加图片</button>
      <!-- 发送按钮：发送中时禁用 -->
      <button class="chat-header-btn primary" type="button" @click="sendMessage" :disabled="sending">
        {{ sending ? '发送中…' : '发送' }}
      </button>
    </div>

    <!-- 错误消息显示 -->
    <p v-if="errorMessage" class="chat-error">{{ errorMessage }}</p>
  </div>
</template>

<script setup>
/**
 * InputArea - 消息输入区域组件
 * 
 * 功能说明：
 * - 多行文本输入，支持 Ctrl/Alt/Meta + Enter 快捷发送
 * - 图片附件管理：添加、预览、删除
 * - 支持剪贴板粘贴截图（通过 handlePaste）
 * - 使用 :value + @input 模式实现 prop 双向绑定
 */

import { ref } from 'vue';

// ========================================
// Props 定义
// ========================================

const props = defineProps({
  /** 当前输入的文本内容（使用 :value + @update:draft 实现双向绑定） */
  draft: {
    type: String,
    required: true
  },
  /** 已选择的图片附件列表 */
  attachments: {
    type: Array,
    required: true
  },
  /** 是否正在发送消息 */
  sending: {
    type: Boolean,
    default: false
  },
  /** 错误消息 */
  errorMessage: {
    type: String,
    default: ''
  },
  /** 输入时清除错误的回调函数 */
  clearErrorOnInput: {
    type: Function,
    required: true
  },
  /** 处理粘贴事件的回调函数 */
  handlePaste: {
    type: Function,
    required: true
  },
  /** 移除附件的回调函数 */
  removeAttachment: {
    type: Function,
    required: true
  },
  /** 处理文件选择的回调函数 */
  handleFileInput: {
    type: Function,
    required: true
  }
});

// ========================================
// 本地响应式状态
// ========================================

/** 本地文件选择器 ref（用于模板绑定） */
const localFileInputRef = ref(null);

// ========================================
// 事件定义
// ========================================

const emit = defineEmits([
  /** 用户点击发送按钮时触发 */
  'send-message',
  /** 输入内容变化时触发（用于 v-model 双向绑定） */
  'update:draft'
]);

// ========================================
// 事件处理函数
// ========================================

/**
 * 处理点击“添加图片”按钮
 * 使用本地 ref 触发文件选择器
 */
const handlePickImages = () => {
  console.log('InputArea: 触发文件选择器');
  console.log('localFileInputRef.value:', localFileInputRef.value);
  localFileInputRef.value?.click();
};

/**
 * 发送消息
 * 触发 send-message 事件，由父组件处理实际发送逻辑
 */
const sendMessage = () => {
  emit('send-message');
};
</script>

<style scoped>
.chat-composer {
  flex-shrink: 0;
  border-top: 1px solid rgba(0, 0, 0, 0.06);
  padding-top: 10px;
}

.chat-header-btn {
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

.attachment-strip {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(64px, 1fr));
  gap: 8px;
  margin-bottom: 10px;
}

.attachment-thumb {
  position: relative;
  aspect-ratio: 1 / 1;
  border-radius: 14px;
  overflow: hidden;
  border: 1px solid rgba(0, 0, 0, 0.08);
  background: #fff;
}

.attachment-thumb img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.attachment-remove {
  position: absolute;
  top: 4px;
  right: 4px;
  width: 20px;
  height: 20px;
  border: none;
  border-radius: 999px;
  background: rgba(0, 0, 0, 0.58);
  color: #fff;
  cursor: pointer;
}

.chat-input {
  width: 100%;
  box-sizing: border-box;
  resize: vertical;
  min-height: 76px;
  max-height: 140px;
  border-radius: 12px;
  border: 1px solid var(--border-color);
  padding: 9px 10px;
  font: inherit;
  font-size: 13px;
  color: var(--text-primary);
  background: rgba(255, 255, 255, 0.96);
  outline: none;
}

.chat-input:focus {
  border-color: var(--accent-color);
  box-shadow: 0 0 0 3px rgba(0, 122, 204, 0.12);
}

.composer-actions {
  margin-top: 8px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
}

.file-input {
  display: none;
}

.chat-error {
  margin: 10px 0 0;
  padding: 8px 12px;
  font-size: 13px;
  line-height: 1.5;
  color: #b42318;
  background: rgba(180, 35, 24, 0.08);
  border: 1px solid rgba(180, 35, 24, 0.2);
  border-radius: 8px;
  word-break: break-word;
  animation: errorFadeIn 0.3s ease-in;
}

@keyframes errorFadeIn {
  from {
    opacity: 0;
    transform: translateY(-5px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}
</style>
