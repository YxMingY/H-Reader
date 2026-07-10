/**
 * useScreenshot - 截图功能 Composable
 * 
 * 功能：
 * - 整页截图并发送到剪贴板
 * - 框选区域截图并发送到剪贴板
 * - 自动将截图添加到聊天附件（如果聊天面板打开）
 * 
 * @param {Object} options - 配置选项
 * @param {Ref<boolean>} options.chatOpen - 聊天面板开关状态
 * @param {Ref} options.readerRef - Reader 组件引用
 * @param {Function} options.addImageToChat - 添加图片到聊天的方法
 * @returns {Object} 截图方法
 */
export function useScreenshot({
  chatOpen,
  readerRef,
  addImageToChat,
}) {
  /**
   * 发送图片数据到系统剪贴板
   * @param {string} imageDataUrl - Base64 编码的图片数据
   */
  const sendToClipboard = async (imageDataUrl) => {
    try {
      // 将 data URL 转换为 Blob
      const response = await fetch(imageDataUrl);
      const blob = await response.blob();
      
      // 写入剪贴板
      await navigator.clipboard.write([
        new ClipboardItem({
          'image/png': blob,
        }),
      ]);
      
      console.log('已复制到剪贴板');
    } catch (err) {
      console.error('复制到剪贴板失败:', err);
      throw err;
    }
  };

  /**
   * 截取当前页面并发送到剪贴板
   * - 如果聊天面板开着，自动添加到附件列表
   */
  const captureCurrentPage = async () => {
    try {
      const imageDataUrl = await readerRef.value?.captureCurrentPage();
      if (!imageDataUrl) {
        alert('截图失败');
        return;
      }

      // 发送到剪贴板
      await sendToClipboard(imageDataUrl);

      // 如果聊天面板开着，自动添加到附件列表
      if (chatOpen.value) {
        await addImageToChat(imageDataUrl);
      }

      console.log('当前页截图成功');
    } catch (err) {
      console.error('截图失败:', err);
      alert('截图失败: ' + err.message);
    }
  };

  /**
   * 开始框选区域截图
   * - 如果聊天面板开着，自动添加到附件列表
   */
  const startAreaSelection = async () => {
    try {
      const imageDataUrl = await readerRef.value?.startAreaSelection();
      if (!imageDataUrl) {
        console.log('用户取消了框选');
        return;
      }

      // 发送到剪贴板
      await sendToClipboard(imageDataUrl);

      // 如果聊天面板开着，自动添加到附件列表
      if (chatOpen.value) {
        await addImageToChat(imageDataUrl);
      }

      console.log('框选截图成功');
    } catch (err) {
      console.error('框选截图失败:', err);
      alert('框选截图失败: ' + err.message);
    }
  };

  return {
    captureCurrentPage,
    startAreaSelection,
  };
}
