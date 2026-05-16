import { ref } from 'vue'; 

export function useChatInput(
    sending, errorMessage
) {
// 输入框草稿文本
const draft = ref('');
// 附件列表：待发送的图片文件
const attachments = ref([]);
// 文件选择器 DOM 引用：用于触发文件选择
const fileInputRef = ref(null);

const clearInput = () => {
  draft.value = '';
  attachments.value = [];
};

/**
 * ========================================
 * 7. 附件处理模块
 * ========================================
 * 
 * 功能：
 * - 读取本地图片文件并转换为 data URL
 * - 管理附件列表（添加、删除）
 * - 提供文件选择器触发接口
 * 
 * 为什么使用 data URL？
 * - 便于直接嵌入到消息中发送给后端
 * - 避免临时文件管理的复杂性
 * - 适合小图片（大图片应考虑上传到服务器）
 */

/**
 * 将文件读取为 data URL
 * 
 * 生成的 ID 包含：
 * - 文件名、大小、修改时间：确保唯一性
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
 * 触发文件选择器
 * 通过编程方式点击隐藏的 input 元素
 */
const pickImages = () => {
  fileInputRef.value?.click();
};

/**
 * 处理文件选择事件
 * 
 * 流程：
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
  console.log('Paste event triggered');
  const items = event.clipboardData?.items;
  if (!items) {
    console.log('No clipboard data');
    return;
  }

  const imageFiles = [];
  
  // 遍历剪贴板中的所有项目
  for (let i = 0; i < items.length; i++) {
    const item = items[i];
    console.log(`Clipboard item ${i}: type=${item.type}, kind=${item.kind}`);
    
    // 检查是否是图片类型
    if (item.type.indexOf('image') !== -1) {
      const blob = item.getAsFile();
      if (blob) {
        console.log(`Found image: size=${blob.size}, type=${blob.type}`);
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
    console.log(`Processing ${imageFiles.length} pasted image(s)`);
    try {
      const nextItems = await Promise.all(imageFiles.map((file) => readFileAsDataUrl(file)));
      attachments.value = attachments.value.concat(nextItems);
      console.log(`Successfully pasted ${nextItems.length} image(s)`);
      
      // 显示成功提示（可选）
      if (nextItems.length === 1) {
        errorMessage.value = '✅ 已添加 1 张图片';
        // 2秒后自动清除提示
        setTimeout(() => {
          if (errorMessage.value === '✅ 已添加 1 张图片') {
            errorMessage.value = '';
          }
        }, 2000);
      } else {
        errorMessage.value = `✅ 已添加 ${nextItems.length} 张图片`;
        setTimeout(() => {
          if (errorMessage.value.startsWith('✅ 已添加')) {
            errorMessage.value = '';
          }
        }, 2000);
      }
    } catch (err) {
      console.error('Failed to process pasted images:', err);
      errorMessage.value = `粘贴图片失败：${err?.message || err}`;
    }
  } else {
    console.log('No images found in clipboard');
  }
};

/**
 * 当用户开始输入时清除错误消息
 * 提供更好的用户体验，避免错误消息一直显示
 * 
 * 注意：
 * - 只在发送成功后，用户开始输入新消息时清除
 * - 如果正在发送或刚刚失败，不要立即清除
 * - 不清除成功提示（以 ✅ 开头的消息）
 */
const clearErrorOnInput = () => {
  // 只有在没有正在发送的消息时才清除错误
  // 并且不清除成功提示
  if (errorMessage.value && !sending.value && !errorMessage.value.startsWith('✅')) {
    console.log('Clearing error message on input');
    errorMessage.value = '';
  }
};

/**
 * 当用户点击新会话按钮时清除错误消息
 */
const clearErrorBeforeAction = () => {
  errorMessage.value = '';
};

  /**
   * 从 data URL 添加图片到附件列表
   * @param {string} dataUrl - 图片的 data URL
   * @returns {Promise<void>}
   */
  const addAttachmentFromDataUrl = async (dataUrl) => {
    try {
      // 将 data URL 转换为 File 对象
      const response = await fetch(dataUrl);
      const blob = await response.blob();
      const fileName = `screenshot-${Date.now()}.png`;
      const file = new File([blob], fileName, { type: blob.type });
      
      // 读取为 data URL 并添加到附件列表
      const item = await readFileAsDataUrl(file);
      attachments.value.push(item);
      console.log('已添加截图到附件列表');
    } catch (err) {
      console.error('添加截图失败:', err);
      errorMessage.value = `添加截图失败：${err?.message || err}`;
    }
  };

  return {
  draft,
  attachments,
  fileInputRef,
  clearInput,
  pickImages,
  handleFileInput,
  addAttachmentFromDataUrl,
  handlePaste,
  clearErrorOnInput,
  clearErrorBeforeAction,
  removeAttachment,
};

}