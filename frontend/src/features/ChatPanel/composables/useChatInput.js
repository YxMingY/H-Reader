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
  
  /** 
   * 附件列表：每个附件包含原始图片和压缩副本
   * 结构：{ id, name, originalDataUrl, compressedDataUrl, quality, size }
   */
  const attachments = ref([]);
  
  /** 文件选择器 DOM 引用：用于触发文件选择 */
  const fileInputRef = ref(null);
  
  /**
   * 估算 data URL 的实际字节大小
   * Base64 编码后的大小约为原始大小的 4/3
   * @param {string} dataUrl - base64 data URL
   * @returns {number} 估算的字节数
   */
  const estimateDataUrlSize = (dataUrl) => {
    if (!dataUrl) return 0;
    const base64String = dataUrl.split(',')[1];
    if (!base64String) return 0;
    // Base64 字符串长度 * 3/4 得到原始字节数
    return Math.floor(base64String.length * 0.75);
  };

  /**
   * 将 Canvas 转换为压缩的 JPEG data URL
   * @param {HTMLCanvasElement} canvas - 源 canvas
   * @param {number} quality - JPEG 质量 (0-1)
   * @param {number} maxSize - 最大边长限制
   * @returns {string} 压缩后的 JPEG data URL
   */
  const compressImageToJpeg = (canvas, quality = 0.75, maxSize = 2000) => {
    let width = canvas.width;
    let height = canvas.height;
    
    // 如果超过最大尺寸，等比例缩放
    if (width > maxSize || height > maxSize) {
      const ratio = Math.min(maxSize / width, maxSize / height);
      width = Math.floor(width * ratio);
      height = Math.floor(height * ratio);
    }
    
    // 创建临时 canvas 进行缩放
    const tempCanvas = document.createElement('canvas');
    tempCanvas.width = width;
    tempCanvas.height = height;
    const ctx = tempCanvas.getContext('2d');
    
    // 填充白色背景（JPG 不支持透明）
    ctx.fillStyle = '#FFFFFF';
    ctx.fillRect(0, 0, width, height);
    
    // 绘制原始图像
    ctx.drawImage(canvas, 0, 0, width, height);
    
    // 转换为 JPEG
    return tempCanvas.toDataURL('image/jpeg', quality);
  };

  /**
   * 将图片文件压缩为 JPEG
   * @param {File|Blob} file - 原始图片文件
   * @param {number} quality - JPEG 质量 (0-1)，默认 0.75
   * @param {number} maxSize - 最大边长，默认 2000px
   * @returns {Promise<string>} 压缩后的 JPEG data URL
   */
  const compressFileToJpeg = (file, quality = 0.75, maxSize = 2000) => {
    return new Promise((resolve, reject) => {
      const img = new Image();
      const url = URL.createObjectURL(file);
      
      img.onload = () => {
        URL.revokeObjectURL(url);
        
        // 创建 canvas
        const canvas = document.createElement('canvas');
        canvas.width = img.width;
        canvas.height = img.height;
        const ctx = canvas.getContext('2d');
        
        // 填充白色背景
        ctx.fillStyle = '#FFFFFF';
        ctx.fillRect(0, 0, canvas.width, canvas.height);
        
        // 绘制图像
        ctx.drawImage(img, 0, 0);
        
        // 压缩为 JPEG
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
   * 读取文件并生成原始 PNG 和压缩 JPEG 副本
   * 
   * ID 生成策略：
   * - 文件名 + 大小 + 修改时间：确保唯一性
   * - 随机字符串：避免同名同大小文件的冲突
   * 
   * @param {File} file - 文件对象
   * @param {number} quality - JPEG 压缩质量，默认 0.75
   * @returns {Promise<Object>} 包含 id, name, originalDataUrl, compressedDataUrl, quality, size 的对象
   */
  const readFileAsDualFormat = async (file, quality = 0.75) => {
    // 读取原始文件为 data URL
    const originalDataUrl = await new Promise((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => resolve(reader.result);
      reader.onerror = () => reject(reader.error || new Error('读取图片失败'));
      reader.readAsDataURL(file);
    });
    
    // 压缩为 JPEG
    const compressedDataUrl = await compressFileToJpeg(file, quality);
    
    // 计算压缩后的大小
    const compressedSize = estimateDataUrlSize(compressedDataUrl);
    
    return {
      id: `${file.name}-${file.size}-${file.lastModified}-${Math.random().toString(36).slice(2)}`,
      name: file.name,
      originalSize: file.size,
      originalDataUrl,
      compressedDataUrl,
      quality,
      size: compressedSize, // 使用压缩后的大小作为显示大小
    };
  };

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
      const nextItems = await Promise.all(files.map((file) => readFileAsDualFormat(file)));
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
        const nextItems = await Promise.all(imageFiles.map((file) => readFileAsDualFormat(file)));
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
   * 更新附件的压缩质量
   * @param {string} id - 附件 ID
   * @param {number} quality - 新的压缩质量 (0.5-1.0)
   */
  const updateAttachmentQuality = async (id, quality) => {
    const attachment = attachments.value.find(item => item.id === id);
    if (!attachment) return;
    
    try {
      // 将原始 data URL 转换为 Blob
      const response = await fetch(attachment.originalDataUrl);
      const blob = await response.blob();
      const file = new File([blob], attachment.name, { type: blob.type });
      
      // 重新压缩
      const compressedDataUrl = await compressFileToJpeg(file, quality);
      const compressedSize = estimateDataUrlSize(compressedDataUrl);
      
      // 更新附件
      attachment.compressedDataUrl = compressedDataUrl;
      attachment.quality = quality;
      attachment.size = compressedSize;
    } catch (err) {
      console.error('更新压缩质量失败:', err);
      errorMessage.value = `压缩失败：${err?.message || err}`;
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
   * 从 data URL 添加图片到附件列表（截图调用）
   * 
   * 使用场景：
   * - 截图功能：将截图直接添加到附件
   * - 外部图片：从其他来源获取的 data URL 图片
   * 
   * @param {string} dataUrl - 图片的 data URL（PNG 格式）
   * @returns {Promise<void>}
   */
  const addAttachmentFromDataUrl = async (dataUrl, { onSaved, isCurrent } = {}) => {
    const response = await fetch(dataUrl);
    const blob = await response.blob();
    const file = new File([blob], `screenshot-${Date.now()}.png`, { type: blob.type });
    const attachment = await readFileAsDualFormat(file);
    if (isCurrent && !isCurrent()) throw new Error('聊天会话已切换，请重试。');
    if (onSaved) attachment.onSaved = onSaved;
    attachments.value.push(attachment);
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
    updateAttachmentQuality,
  };
}