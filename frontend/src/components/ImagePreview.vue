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
            <!-- 质量调节控件（仅当有附件对象时显示） -->
            <div v-if="currentAttachment" class="quality-control-inline">
              <label class="quality-label">质量:</label>
              <input 
                type="range" 
                min="50" 
                max="100" 
                :value="Math.round(currentQuality * 100)"
                @input="handleQualityChange($event.target.value)"
                class="quality-slider"
                title="拖动调整压缩质量"
              />
              <span class="quality-value">{{ Math.round(currentQuality * 100) }}%</span>
              <span class="quality-size">({{ formatFileSize(currentCompressedSize) }})</span>
            </div>
            <span v-else class="preview-hint">点击背景或按 Esc 关闭</span>
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
const currentAttachment = ref(null); // 当前附件对象
const currentQuality = ref(0.75); // 当前压缩质量
const currentCompressedSize = ref(0); // 当前压缩后大小

/**
 * 估算 data URL 的实际字节大小
 */
const estimateDataUrlSize = (dataUrl) => {
  if (!dataUrl) return 0;
  const base64String = dataUrl.split(',')[1];
  if (!base64String) return 0;
  return Math.floor(base64String.length * 0.75);
};

/**
 * 格式化文件大小
 */
const formatFileSize = (bytes) => {
  if (!bytes || bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB'];
  let size = bytes;
  let unitIndex = 0;
  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024;
    unitIndex++;
  }
  const formatted = size % 1 === 0 ? size.toFixed(0) : size.toFixed(1);
  return `${formatted} ${units[unitIndex]}`;
};

/**
 * 将 Canvas 转换为压缩的 JPEG data URL
 */
const compressImageToJpeg = (canvas, quality = 0.75, maxSize = 2000) => {
  let width = canvas.width;
  let height = canvas.height;
  
  if (width > maxSize || height > maxSize) {
    const ratio = Math.min(maxSize / width, maxSize / height);
    width = Math.floor(width * ratio);
    height = Math.floor(height * ratio);
  }
  
  const tempCanvas = document.createElement('canvas');
  tempCanvas.width = width;
  tempCanvas.height = height;
  const ctx = tempCanvas.getContext('2d');
  
  ctx.fillStyle = '#FFFFFF';
  ctx.fillRect(0, 0, width, height);
  ctx.drawImage(canvas, 0, 0, width, height);
  
  return tempCanvas.toDataURL('image/jpeg', quality);
};

/**
 * 压缩图片文件为 JPEG
 */
const compressFileToJpeg = (file, quality = 0.75, maxSize = 2000) => {
  return new Promise((resolve, reject) => {
    const img = new Image();
    const url = URL.createObjectURL(file);
    
    img.onload = () => {
      URL.revokeObjectURL(url);
      const canvas = document.createElement('canvas');
      canvas.width = img.width;
      canvas.height = img.height;
      const ctx = canvas.getContext('2d');
      
      ctx.fillStyle = '#FFFFFF';
      ctx.fillRect(0, 0, canvas.width, canvas.height);
      ctx.drawImage(img, 0, 0);
      
      const jpegDataUrl = compressImageToJpeg(canvas, quality, maxSize);
      resolve(jpegDataUrl);
    };
    
    img.onerror = () => {
      URL.revokeObjectURL(url);
      reject(new Error('图片加载失败'));
    };
    
    img.src = url;
  });
};

/**
 * 处理质量滑块变化
 */
const handleQualityChange = async (value) => {
  if (!currentAttachment.value) return;
  
  const quality = Math.max(0.5, Math.min(1.0, Number(value) / 100));
  currentQuality.value = quality;
  
  try {
    // 将原始 data URL 转换为 Blob
    const response = await fetch(currentAttachment.value.originalDataUrl);
    const blob = await response.blob();
    const file = new File([blob], currentAttachment.value.name, { type: blob.type });
    
    // 重新压缩
    const compressedDataUrl = await compressFileToJpeg(file, quality);
    const compressedSize = estimateDataUrlSize(compressedDataUrl);
    
    // 更新预览和附件
    currentImage.value = compressedDataUrl;
    currentCompressedSize.value = compressedSize;
    currentAttachment.value.compressedDataUrl = compressedDataUrl;
    currentAttachment.value.quality = quality;
    currentAttachment.value.size = compressedSize;
  } catch (err) {
    console.error('压缩失败:', err);
  }
};

/**
 * 显示图片预览
 * @param {string} imageUrl - 图片 URL 或 data URL
 * @param {string} alt - 图片描述（可选）
 * @param {Object} attachment - 附件对象（可选，用于质量调节）
 */
const show = (imageUrl, alt = '', attachment = null) => {
  currentImage.value = imageUrl;
  imageAlt.value = alt;
  currentAttachment.value = attachment;
  
  if (attachment) {
    currentQuality.value = attachment.quality || 0.75;
    currentCompressedSize.value = attachment.size || 0;
  } else {
    currentQuality.value = 0.75;
    currentCompressedSize.value = 0;
  }
  
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
  currentAttachment.value = null;
  currentQuality.value = 0.75;
  currentCompressedSize.value = 0;
  
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
  width: 90vw;
  height: calc(90vh - 80px);
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
  max-width: 40%;
}

.quality-control-inline {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  justify-content: center;
}

.quality-label {
  color: rgba(255, 255, 255, 0.9);
  font-size: 13px;
  font-weight: 500;
}

.quality-slider {
  width: 150px;
  height: 4px;
  -webkit-appearance: none;
  appearance: none;
  background: rgba(255, 255, 255, 0.3);
  border-radius: 2px;
  outline: none;
  cursor: pointer;
}

.quality-slider::-webkit-slider-thumb {
  -webkit-appearance: none;
  appearance: none;
  width: 14px;
  height: 14px;
  border-radius: 50%;
  background: #fff;
  cursor: pointer;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.3);
}

.quality-slider::-moz-range-thumb {
  width: 14px;
  height: 14px;
  border-radius: 50%;
  background: #fff;
  cursor: pointer;
  border: none;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.3);
}

.quality-value {
  color: white;
  font-size: 13px;
  font-weight: 600;
  min-width: 40px;
}

.quality-size {
  color: rgba(255, 255, 255, 0.7);
  font-size: 12px;
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
