<template>
  <Teleport to="body">
    <Transition name="image-preview-fade">
      <div 
        v-if="visible" 
        class="image-preview-overlay"
        @click="handleOverlayClick"
      >
        <div class="image-preview-container" @click.stop>
          <!-- 关闭按钮 -->
          <button class="preview-close-btn" @click="close" title="关闭预览 (Esc)">
            ✕
          </button>
          
          <!-- 图片容器 -->
          <div class="preview-image-wrapper">
            <img 
              :src="currentImage" 
              :alt="imageAlt"
              class="preview-image"
              @click.stop
            />
          </div>
          
          <!-- 底部信息栏 -->
          <div class="preview-info-bar">
            <span class="preview-filename">{{ imageAlt || '图片预览' }}</span>
            <span class="preview-hint">点击背景或按 Esc 关闭</span>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup>
import { ref, onMounted, onBeforeUnmount } from 'vue';

const visible = ref(false);
const currentImage = ref('');
const imageAlt = ref('');

/**
 * 显示图片预览
 * @param {string} imageUrl - 图片 URL 或 data URL
 * @param {string} alt - 图片描述（可选）
 */
const show = (imageUrl, alt = '') => {
  currentImage.value = imageUrl;
  imageAlt.value = alt;
  visible.value = true;
  
  // 禁止页面滚动
  document.body.style.overflow = 'hidden';
};

/**
 * 关闭预览
 */
const close = () => {
  visible.value = false;
  currentImage.value = '';
  imageAlt.value = '';
  
  // 恢复页面滚动
  document.body.style.overflow = '';
};

/**
 * 点击遮罩层关闭
 */
const handleOverlayClick = () => {
  close();
};

/**
 * 键盘事件处理（Esc 键关闭）
 */
const handleKeyDown = (event) => {
  if (event.key === 'Escape' && visible.value) {
    close();
  }
};

// 生命周期钩子
onMounted(() => {
  document.addEventListener('keydown', handleKeyDown);
});

onBeforeUnmount(() => {
  document.removeEventListener('keydown', handleKeyDown);
  // 确保恢复页面滚动
  document.body.style.overflow = '';
});

// 暴露方法给外部使用
defineExpose({
  show,
  close
});
</script>

<style scoped>
.image-preview-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.85);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 10000;
  backdrop-filter: blur(8px);
  animation: fadeIn 0.2s ease-out;
}

@keyframes fadeIn {
  from {
    opacity: 0;
  }
  to {
    opacity: 1;
  }
}

.image-preview-container {
  position: relative;
  max-width: 90vw;
  max-height: 90vh;
  display: flex;
  flex-direction: column;
  animation: zoomIn 0.3s ease-out;
}

@keyframes zoomIn {
  from {
    opacity: 0;
    transform: scale(0.9);
  }
  to {
    opacity: 1;
    transform: scale(1);
  }
}

.preview-close-btn {
  position: absolute;
  top: -40px;
  right: 0;
  width: 36px;
  height: 36px;
  border: none;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.2);
  color: white;
  font-size: 20px;
  cursor: pointer;
  transition: all 0.2s ease;
  display: flex;
  align-items: center;
  justify-content: center;
}

.preview-close-btn:hover {
  background: rgba(255, 255, 255, 0.3);
  transform: scale(1.1);
}

.preview-image-wrapper {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  max-width: 90vw;
  max-height: calc(90vh - 80px);
}

.preview-image {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
  border-radius: 8px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.4);
  user-select: none;
}

.preview-info-bar {
  margin-top: 16px;
  padding: 12px 20px;
  background: rgba(255, 255, 255, 0.1);
  border-radius: 8px;
  backdrop-filter: blur(10px);
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
}

.preview-filename {
  color: white;
  font-size: 14px;
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 60%;
}

.preview-hint {
  color: rgba(255, 255, 255, 0.7);
  font-size: 12px;
}

/* Transition 动画 */
.image-preview-fade-enter-active,
.image-preview-fade-leave-active {
  transition: opacity 0.3s ease;
}

.image-preview-fade-enter-from,
.image-preview-fade-leave-to {
  opacity: 0;
}

.image-preview-fade-enter-active .image-preview-container {
  animation: zoomIn 0.3s ease-out;
}

.image-preview-fade-leave-active .image-preview-container {
  animation: zoomOut 0.2s ease-in;
}

@keyframes zoomOut {
  from {
    opacity: 1;
    transform: scale(1);
  }
  to {
    opacity: 0;
    transform: scale(0.9);
  }
}
</style>
