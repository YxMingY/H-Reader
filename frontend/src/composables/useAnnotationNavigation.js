import { nextTick } from 'vue';

/**
 * useAnnotationNavigation - 大头针导航 Composable
 * 
 * 功能：
 * - 处理双击大头针时的导航逻辑
 * - 自动打开聊天面板并调整布局
 * - 切换到对应会话并滚动到目标消息
 * 
 * @param {Object} options - 配置选项
 * @param {Ref<boolean>} options.chatOpen - 聊天面板开关状态
 * @param {Ref} options.chatPanelRef - ChatPanel 组件引用
 * @param {Function} options.toggleChatPanel - 切换聊天面板的方法
 * @returns {Object} 导航方法
 */
export function useAnnotationNavigation({
  chatOpen,
  chatPanelRef,
  toggleChatPanel,
}) {
  /**
   * 处理从大头针导航到 AI 对话
   * 
   * 工作流程：
   * 1. 打开聊天面板（如果未打开），复用 toggleChatPanel 的自适应逻辑
   * 2. 切换到对应的 session（不自动滚动到底部）
   * 3. 滚动到目标消息并高亮显示
   * 
   * @param {Object} data - 导航数据
   * @param {string} data.sessionId - 目标会话 ID
   * @param {number} data.messageIndex - 目标消息索引
   */
  const handleNavigateToMessage = async ({ sessionId, messageIndex }) => {
    console.log('导航到会话:', sessionId, '消息索引:', messageIndex);
    
    // 1. 如果聊天面板未打开，先打开（复用 toggleChatPanel 的自适应逻辑）
    if (!chatOpen.value) {
      toggleChatPanel();
      // 等待 DOM 更新
      await nextTick();
    }
    
    // 2. 切换到对应的 session（不使用自动滚动）
    if (chatPanelRef.value && typeof chatPanelRef.value.loadSession === 'function') {
      // 注意：这里需要直接调用 loadSession，而不是 switchSession
      // 因为 switchSession 会自动滚动到底部，干扰我们的定位
      await chatPanelRef.value.loadSession(sessionId);
      console.log('已切换到会话:', sessionId);
    } else {
      console.warn('ChatPanel 未暴露 loadSession 方法');
    }
    
    // 3. 滚动到对应的消息
    if (chatPanelRef.value && typeof chatPanelRef.value.scrollToMessage === 'function') {
      setTimeout(() => {
        chatPanelRef.value.scrollToMessage(messageIndex);
        console.log('已滚动到消息索引:', messageIndex);
      }, 100); // 减少延迟，因为不需要等待滚动到底部
    } else {
      console.warn('ChatPanel 未暴露 scrollToMessage 方法');
    }
  };

  return {
    handleNavigateToMessage,
  };
}
