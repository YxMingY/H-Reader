<template>
  <Teleport to="body">
    <Transition name="toast-fade">
      <div v-if="visible" class="toast-overlay">
        <div class="toast-confirm">
          <div class="toast-header">
            <span class="toast-icon">{{ options.icon || '⚠️' }}</span>
            <h3 class="toast-title">{{ options.title || '确认操作' }}</h3>
          </div>
          
          <div class="toast-content">
            <p>{{ options.message }}</p>
          </div>
          
          <div class="toast-actions">
            <button 
              class="toast-btn toast-btn-cancel"
              @click="handleCancel"
            >
              {{ options.cancelText || '取消' }}
            </button>
            <button 
              class="toast-btn toast-btn-confirm"
              @click="handleConfirm"
            >
              {{ options.confirmText || '确定' }}
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup>
import { ref } from 'vue';

const visible = ref(false);
const options = ref({});
let resolvePromise = null;

/**
 * 显示确认框
 * @param {Object} opts - 配置选项
 * @param {string} opts.title - 标题（可选）
 * @param {string} opts.message - 消息内容
 * @param {string} opts.icon - 图标 emoji（可选，默认 ⚠️）
 * @param {string} opts.confirmText - 确认按钮文本（可选，默认 "确定"）
 * @param {string} opts.cancelText - 取消按钮文本（可选，默认 "取消"）
 * @returns {Promise<boolean>} 用户点击确定返回 true，取消返回 false
 */
const show = (opts) => {
  return new Promise((resolve) => {
    options.value = {
      title: opts.title || '确认操作',
      message: opts.message,
      icon: opts.icon || '⚠️',
      confirmText: opts.confirmText || '确定',
      cancelText: opts.cancelText || '取消'
    };
    visible.value = true;
    resolvePromise = resolve;
  });
};

const handleConfirm = () => {
  visible.value = false;
  if (resolvePromise) {
    resolvePromise(true);
    resolvePromise = null;
  }
};

const handleCancel = () => {
  visible.value = false;
  if (resolvePromise) {
    resolvePromise(false);
    resolvePromise = null;
  }
};

// 暴露方法给外部使用
defineExpose({
  show
});
</script>

<style scoped>
.toast-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 9999;
  backdrop-filter: blur(2px);
}

.toast-confirm {
  background: white;
  border-radius: 12px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.2);
  min-width: 320px;
  max-width: 480px;
  animation: slideIn 0.3s ease-out;
}

@keyframes slideIn {
  from {
    opacity: 0;
    transform: translateY(-20px) scale(0.95);
  }
  to {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

.toast-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 20px 24px 12px;
  border-bottom: 1px solid #f0f0f0;
}

.toast-icon {
  font-size: 24px;
}

.toast-title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: #1a1a1a;
}

.toast-content {
  padding: 20px 24px;
}

.toast-content p {
  margin: 0;
  font-size: 14px;
  line-height: 1.6;
  color: #4a4a4a;
}

.toast-actions {
  display: flex;
  gap: 12px;
  padding: 16px 24px 20px;
  justify-content: flex-end;
}

.toast-btn {
  padding: 8px 20px;
  font-size: 14px;
  font-weight: 500;
  border: none;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s ease;
}

.toast-btn-cancel {
  background: #f5f5f5;
  color: #666;
}

.toast-btn-cancel:hover {
  background: #e8e8e8;
}

.toast-btn-confirm {
  background: #1890ff;
  color: white;
}

.toast-btn-confirm:hover {
  background: #40a9ff;
  transform: translateY(-1px);
  box-shadow: 0 2px 8px rgba(24, 144, 255, 0.3);
}

/* Transition 动画 */
.toast-fade-enter-active,
.toast-fade-leave-active {
  transition: opacity 0.3s ease;
}

.toast-fade-enter-from,
.toast-fade-leave-to {
  opacity: 0;
}

.toast-fade-enter-active .toast-confirm {
  animation: slideIn 0.3s ease-out;
}

.toast-fade-leave-active .toast-confirm {
  animation: slideOut 0.3s ease-in;
}

@keyframes slideOut {
  from {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
  to {
    opacity: 0;
    transform: translateY(-20px) scale(0.95);
  }
}
</style>
