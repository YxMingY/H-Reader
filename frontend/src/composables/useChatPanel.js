import { ref, nextTick } from 'vue';

/**
 * useChatPanel - 聊天面板管理 Composable
 * 
 * 功能：
 * - 控制聊天面板的打开/关闭
 * - 管理聊天面板宽度（初始化、同步、调整）
 * - 自动调整 PDF 缩放以适应布局变化
 * 
 * @param {Object} options - 配置选项
 * @param {Ref<boolean>} options.chatOpen - 聊天面板开关状态
 * @param {Ref<number>} options.chatPanelWidth - 聊天面板宽度
 * @param {Ref<boolean>} options.hasCustomChatWidth - 是否自定义过宽度
 * @param {Ref<string>} options.view - 当前视图类型 ('library' | 'reader')
 * @param {Ref} options.readerRef - Reader 组件引用
 * @returns {Object} 聊天面板管理方法
 */
export function useChatPanel({
  chatOpen,
  chatPanelWidth,
  hasCustomChatWidth,
  view,
  readerRef,
}) {
  const isResizingChat = ref(false);
  const resizeStartX = ref(0);
  const resizeStartWidth = ref(420);

  /**
   * 获取视口宽度
   */
  const getViewportWidth = () => (typeof window !== 'undefined' ? window.innerWidth : 0);

  /**
   * 同步聊天面板宽度到 CSS 变量
   */
  const syncChatWidth = () => {
    const viewportWidth = getViewportWidth();
    if (!viewportWidth) return;
    const width = Math.min(chatPanelWidth.value, Math.floor(viewportWidth * 0.6));
    document.documentElement.style.setProperty('--chat-width', `${width}px`);
  };

  /**
   * 初始化聊天面板宽度（默认占视口的 30%）
   */
  const initChatWidth = () => {
    const viewportWidth = getViewportWidth();
    if (!viewportWidth) return;
    chatPanelWidth.value = Math.floor(viewportWidth * 0.3);
    syncChatWidth();
  };

  /**
   * 切换聊天面板显示状态
   * - 打开时自动调整 PDF 缩放（如果聊天面板宽度 ≤ 33% 视口）
   * - 关闭时无额外操作
   */
  const toggleChatPanel = () => {
    const willOpen = !chatOpen.value;
    chatOpen.value = willOpen;
    
    if (willOpen) {
      // 用户如果没有拖拽过，就按默认比例初始化；拖拽过则保留用户偏好。
      if (!hasCustomChatWidth.value || !chatPanelWidth.value) {
        initChatWidth();
      } else {
        syncChatWidth();
      }
      
      // 只有当聊天面板实际宽度没有超过 33% 时，才自动调用 Reader 的 fitWidth。
      const viewportWidth = getViewportWidth();
      const shouldAutoFitReader =
        view.value === 'reader' &&
        viewportWidth > 0 &&
        chatPanelWidth.value <= Math.floor(viewportWidth * 0.33);

      if (shouldAutoFitReader) {
        nextTick(() => {
          readerRef.value?.FitWidth();
        });
      }
    }
  };

  /**
   * 开始调整聊天面板宽度
   */
  const startChatResize = (event) => {
    if (!chatOpen.value) return;
    event.preventDefault();
    isResizingChat.value = true;
    resizeStartX.value = event.clientX;
    resizeStartWidth.value = chatPanelWidth.value;
    window.addEventListener('pointermove', handleChatResizeMove);
    window.addEventListener('pointerup', stopChatResize);
  };

  /**
   * 处理聊天面板宽度调整（拖动中）
   */
  const handleChatResizeMove = (event) => {
    if (!isResizingChat.value) return;
    const delta = resizeStartX.value - event.clientX;
    const newWidth = resizeStartWidth.value + delta;
    const viewportWidth = getViewportWidth();
    
    // 限制最小宽度 280px，最大宽度 60% 视口
    const minWidth = 280;
    const maxWidth = Math.floor(viewportWidth * 0.6);
    chatPanelWidth.value = Math.max(minWidth, Math.min(newWidth, maxWidth));
    
    hasCustomChatWidth.value = true;
    syncChatWidth();
  };

  /**
   * 停止调整聊天面板宽度
   */
  const stopChatResize = () => {
    if (!isResizingChat.value) return;
    isResizingChat.value = false;
    window.removeEventListener('pointermove', handleChatResizeMove);
    window.removeEventListener('pointerup', stopChatResize);
  };

  /**
   * 清理资源（组件卸载时调用）
   */
  const cleanup = () => {
    window.removeEventListener('pointermove', handleChatResizeMove);
    window.removeEventListener('pointerup', stopChatResize);
  };

  return {
    isResizingChat,
    getViewportWidth,
    syncChatWidth,
    initChatWidth,
    toggleChatPanel,
    startChatResize,
    handleChatResizeMove,
    stopChatResize,
    cleanup,
  };
}
