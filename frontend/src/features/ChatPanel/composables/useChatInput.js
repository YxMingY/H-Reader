/**
 * useChatInput - 输入管理 Composable
 * 
 * 职责：
 * - 管理文本输入草稿
 * - 管理图片附件（添加、删除、预览）
 * - 处理文件选择和剪贴板粘贴
 * - 提供错误清除功能
 * 
 * @param {Ref<boolean>} sending - 发送状态引用
 * @param {Ref<string>} errorMessage - 错误消息引用
 * @returns {Object} 输入管理相关的状态和方法
 */

import { ref } from 'vue';

export function useChatInput(sending, errorMessage) {
  // ========================================
  // 响应式状态
  // ========================================

  /** 输入框草稿文本 */
  const draft = ref('');
  
  /** 附件列表：待发送的图片文件 */
  const attachments = ref([]);
  
  /** 文件选择器 DOM 引用：用于触发文件选择 */
  const fileInputRef = ref(null);
  /**
   * 清空输入状态
   * 重置草稿文本和附件列表
   */
  const clearInput = () => {
    draft.value = '';
    attachments.value = [];
  };

  // ========================================
  // 附件处理模块
  // ========================================

  /**
   * 将文件读取为 data URL
   * 
   * ID 生成策略：
   * - 文件名 + 大小 + 修改时间：确保唯一性
   * - 随机字符串：避免同名同大小文件的冲突
   * 
   * @param {File} file - 文件对象
   * @returns {Promise<Object>} 包含 id, name, dataUrl 的对象
   */
  const readFileAsDataUrl = (file) => new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve({
      id: `${file.name}-${file.size}-${file.lastModified}-${Math.random().toString(36).slice(2)}`,
      name: file.name,
      dataUrl: reader.result,
    });
    reader.onerror = () => reject(reader.error || new Error('读取图片失败'));
    reader.readAsDataURL(file);
  });

  /**
   * 处理文件选择事件
   * 
   * 执行流程：
   * 1. 获取选中的文件列表
   * 2. 清空 input value（允许重复选择同一文件）
   * 3. 并行读取所有文件为 data URL
   * 4. 追加到附件列表
   * 
   * @param {Event} event - 文件选择事件
   */
  const handleFileInput = async (event) => {
    const files = Array.from(event.target.files || []);
    event.target.value = '';
    if (!files.length) return;

    try {
      const nextItems = await Promise.all(files.map((file) => readFileAsDataUrl(file)));
      attachments.value = attachments.value.concat(nextItems);
    } catch (err) {
      errorMessage.value = `添加图片失败：${err?.message || err}`;
    }
  };

  /**
   * 移除指定附件
   * @param {string} id - 附件 ID
   */
  const removeAttachment = (id) => {
    attachments.value = attachments.value.filter((item) => item.id !== id);
  };


  /**
   * 处理粘贴事件：从剪贴板中提取图片并添加到附件列表
   * 
   * 支持的粘贴来源：
   * - 截图软件（微信、QQ、系统截图等）
   * - 复制的图片文件
   * - 网页上复制的图片
   * 
   * @param {ClipboardEvent} event - 粘贴事件对象
   */
  const handlePaste = async (event) => {
    const items = event.clipboardData?.items;
    if (!items) return;

    const imageFiles = [];
    
    // 遍历剪贴板中的所有项目，查找图片
    for (let i = 0; i < items.length; i++) {
      const item = items[i];
      
      // 检查是否是图片类型
      if (item.type.indexOf('image') !== -1) {
        const blob = item.getAsFile();
        if (blob) {
          // 生成一个文件名，使用时间戳
          const fileName = `pasted-image-${Date.now()}-${i}.png`;
          const file = new File([blob], fileName, { type: blob.type });
          imageFiles.push(file);
        }
      }
    }

    // 如果找到了图片，读取并添加到附件列表
    if (imageFiles.length > 0) {
      event.preventDefault(); // 阻止默认粘贴行为（避免粘贴图片的 base64 到文本框）
      
      try {
        const nextItems = await Promise.all(imageFiles.map((file) => readFileAsDataUrl(file)));
        attachments.value = attachments.value.concat(nextItems);
        
        // 显示成功提示（2秒后自动清除）
        const count = nextItems.length;
        errorMessage.value = `✅ 已添加 ${count} 张图片`;
        setTimeout(() => {
          if (errorMessage.value.startsWith('✅ 已添加')) {
            errorMessage.value = '';
          }
        }, 2000);
      } catch (err) {
        errorMessage.value = `粘贴图片失败：${err?.message || err}`;
      }
    }
  };

  /**
   * 当用户开始输入时清除错误消息
   * 
   * 清除策略：
   * - 只在没有正在发送的消息时清除
   * - 不清除成功提示（以 ✅ 开头的消息）
   * - 提供更好的用户体验，避免错误消息一直显示
   */
  const clearErrorOnInput = () => {
    if (errorMessage.value && !sending.value && !errorMessage.value.startsWith('✅')) {
      errorMessage.value = '';
    }
  };

  /**
   * 当用户点击新会话按钮时清除错误消息
   * 用于在创建新会话前清理之前的错误状态
   */
  const clearErrorBeforeAction = () => {
    errorMessage.value = '';
  };

  /**
   * 从 data URL 添加图片到附件列表
   * 
   * 使用场景：
   * - 截图功能：将截图直接添加到附件
   * - 外部图片：从其他来源获取的 data URL 图片
   * 
   * @param {string} dataUrl - 图片的 data URL
   * @returns {Promise<void>}
   */
  const addAttachmentFromDataUrl = async (dataUrl) => {
    try {
      // 将 data URL 转换为 Blob，再转换为 File 对象
      const response = await fetch(dataUrl);
      const blob = await response.blob();
      const fileName = `screenshot-${Date.now()}.png`;
      const file = new File([blob], fileName, { type: blob.type });
      
      // 读取为 data URL 并添加到附件列表
      const item = await readFileAsDataUrl(file);
      attachments.value.push(item);
    } catch (err) {
      errorMessage.value = `添加截图失败：${err?.message || err}`;
    }
  };

  // ========================================
  // 公开 API
  // ========================================

  return {
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
  };
}