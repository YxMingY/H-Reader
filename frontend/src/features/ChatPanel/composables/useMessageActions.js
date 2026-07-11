/**
 * useMessageActions - 消息操作管理 Composable
 * 
 * 职责：
 * - 管理消息的回滚（删除最新一轮对话）
 * - 处理消息的重新编辑
 * - 提供判断是否为最新一轮对话的工具函数
 * 
 * @param {Object} context - 上下文对象
 * @param {Ref<string>} context.activeSessionId - 当前活跃会话 ID
 * @param {Ref<Array>} context.messages - 消息列表
 * @param {Ref<string>} context.errorMessage - 错误消息引用
 * @param {Function} context.loadSession - 加载会话函数
 * @param {Object} context.props - 组件 props（包含 scopeType, bookPath）
 * @param {ComputedRef<string>} context.scopeBookPath - 作用域书籍路径
 * @param {Ref<string>} context.draft - 输入框草稿内容
 * @returns {Object} 消息操作相关的状态和方法
 */

import { ChatService } from '../../../../bindings/hreader/services/chat';

export function useMessageActions({
  activeSessionId,
  messages,
  errorMessage,
  loadSession,
  props,
  scopeBookPath,
  draft
}) {
  /**
   * 判断指定索引是否是最新的一轮对话（最后两条消息）
   * 
   * @param {number} index - 消息索引
   * @returns {boolean} 是否是最新一轮
   */
  const isLastRound = (index) => {
    const len = messages.value.length;
    // 最新的 user 和 assistant 消息是最后两条
    return index >= len - 2;
  };

  /**
   * 回滚最新的一轮对话（删除最新的 user + assistant 消息对）
   * 
   * 用途：
   * - 用户点击"删除"按钮时删除本轮对话
   * - 用户点击"重新编辑"时先回滚再填充输入框
   * 
   * 注意：
   * - 只能回滚最新的消息对
   * - 会同时删除 LLM 状态和 UI 消息列表
   * - 回滚后需要重新加载会话以同步状态
   * 
   * @returns {Promise<boolean>} 是否成功回滚
   */
  const rollbackLastRound = async () => {
    if (!activeSessionId.value) {
      errorMessage.value = '当前没有活跃会话';
      return false;
    }
    
    if (messages.value.length < 2) {
      errorMessage.value = '没有可删除的消息';
      return false;
    }
    
    errorMessage.value = '';
    try {
      // 调用后端 API 回滚一轮（n=1）
      const deletedPairs = await ChatService.RollbackSessionMessages(
        props.scopeType,
        scopeBookPath.value,
        activeSessionId.value,
        1
      );
      
      if (deletedPairs > 0) {
        // 重新加载会话以同步状态
        await loadSession(activeSessionId.value, { keepSelection: true });
        return true;
      } else {
        errorMessage.value = '没有可删除的消息对';
        return false;
      }
    } catch (err) {
      errorMessage.value = `回滚失败：${err?.message || err}`;
      return false;
    }
  };

  /**
   * 处理删除本轮对话
   * 
   * @param {number} messageIndex - 消息索引
   * @returns {Promise<void>}
   */
  const handleDeleteRound = async (messageIndex) => {
    if (!window.confirm('确定要删除本轮对话吗？')) {
      return;
    }
    
    try {
      const success = await rollbackLastRound();
      if (success) {
        console.log('成功删除本轮对话');
      }
    } catch (err) {
      console.error('删除失败:', err);
    }
  };

  /**
   * 处理重新编辑消息
   * 
   * @param {number} messageIndex - 消息索引
   * @returns {Promise<string|null>} 返回消息内容，失败返回 null
   */
  const handleEditMessage = async (messageIndex) => {
    const message = messages.value[messageIndex];
    if (!message || message.role !== 'user') {
      return null;
    }
    
    // 先确认删除
    if (!window.confirm('确定要重新编辑这条消息吗？这将删除本轮对话。')) {
      return null;
    }
    
    try {
      const success = await rollbackLastRound();
      if (success) {
        // 返回消息内容，由调用者填充到输入框
        console.log('已触发编辑，消息内容:', message.content);
        return message.content;
      }
      return null;
    } catch (err) {
      console.error('重新编辑失败:', err);
      return null;
    }
  };

  return {
    isLastRound,
    rollbackLastRound,
    handleDeleteRound,
    handleEditMessage
  };
}
