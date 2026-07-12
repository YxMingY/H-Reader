/**
 * useImagePreview - 全局图片预览 Composable
 * 
 * 提供统一的图片预览功能，支持：
 * - 点击任意图片打开全屏预览
 * - ESC 键关闭预览
 * - 点击背景关闭预览
 * - 优雅的动画过渡效果
 * 
 * @example
 * const { previewImage } = useImagePreview();
 * previewImage('data:image/png;base64,...', '截图');
 */

// 全局单例引用
let previewInstance = null;

/**
 * 设置预览实例（由 App.vue 调用）
 * @param {Object} instance - ImagePreview 组件的 ref
 */
export function setPreviewInstance(instance) {
  previewInstance = instance;
}

/**
 * 显示图片预览
 * @param {string} imageUrl - 图片 URL 或 data URL
 * @param {string} alt - 图片描述（可选）
 */
export function previewImage(imageUrl, alt = '') {
  if (!previewInstance) {
    console.warn('预览实例未初始化，降级使用新窗口打开');
    window.open(imageUrl, '_blank');
    return;
  }
  
  previewInstance.show(imageUrl, alt);
}

/**
 * 关闭图片预览
 */
export function closePreview() {
  if (previewInstance) {
    previewInstance.close();
  }
}

/**
 * useImagePreview hook（保持 API 一致性）
 * @returns {Object}
 */
export function useImagePreview() {
  return {
    previewImage,
    closePreview
  };
}
