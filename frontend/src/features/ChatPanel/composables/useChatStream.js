import { ref, watch, onMounted, onBeforeUnmount } from 'vue';
import { Events } from '@wailsio/runtime';

export function useChatStream(
    activeSessionId, sending, messages, errorMessage, loadSession, refreshSessions, scrollToBottom
) {

// 当前流式响应的会话 ID：用于验证事件来源
const streamSessionId = ref('');
// 当前流式响应的 assistant 消息索引：用于追加内容
const streamAssistantIndex = ref(-1);

// 事件取消订阅函数列表：用于组件卸载时清理
const eventUnsubscribers = [];

/**
 * ========================================
 * 8. 流式通信模块
 * ========================================
 * 
 * 这是 ChatPanel 的核心模块，负责与后端的实时通信。
 * 
 * 工作流程：
 * 1. 用户发送消息 → sendMessage()
 * 2. 前端创建 user 和 assistant 占位消息
 * 3. 调用后端 SendMessageStreamInSession API
 * 4. 后端在后台 goroutine 中处理，通过事件推送分片
 * 5. 前端监听 chat_chunk 事件，逐块更新 assistant 消息
 * 6. 后端完成后发送 chat_done 事件
 * 7. 前端刷新会话数据，确保与后端同步
 * 
 * 事件类型：
 * - chat_chunk: 流式分片（字符串）
 * - chat_done: 完整回复（字符串）
 * - chat_stream_error: 错误消息（字符串）
 */

/**
 * 重置流式发送的前端追踪状态
 * 
 * 在以下情况调用：
 * - 发送完成（成功或失败）
 * - 发生错误
 * - 组件卸载
 */
const resetStreamingState = () => {
  streamSessionId.value = '';
  streamAssistantIndex.value = -1;
  sending.value = false;
};

/**
 * 处理流式分片事件
 * 
 * 安全检查：
 * 1. 验证 streamSessionId 是否存在
 * 2. 验证事件来自当前活跃会话
 * 3. 验证 assistant 消息索引有效
 * 4. 验证目标消息角色为 assistant
 * 
 * 追加策略：
 * - 直接将分片拼接到现有内容后面
 * - 每次追加后滚动到底部
 * 
 * @param {Object} event - Wails 事件对象，data 字段包含文本分片
 */
const onStreamChunk = async (event) => {
  if (!streamSessionId.value) return;
  if (activeSessionId.value !== streamSessionId.value) return;

  const chunk = String(event?.data || '');
  if (!chunk) return;

  const index = streamAssistantIndex.value;
  if (index < 0 || index >= messages.value.length) return;

  const target = messages.value[index];
  if (!target || target.role !== 'assistant') return;

  target.content = (target.content || '') + chunk;
  await scrollToBottom();
};

/**
 * 处理流式完成事件
 * 
 * 流程：
 * 1. 更新 assistant 消息的最终内容（以防分片拼接不完整）
 * 2. 记录完成的会话 ID
 * 3. 重置流式状态
 * 4. 如果完成的是当前活跃会话，重新加载以确保数据一致
 * 5. 刷新会话列表（更新消息计数和时间戳）
 * 
 * @param {Object} event - Wails 事件对象，data 字段包含完整回复
 */
const onStreamDone = async (event) => {
  if (!streamSessionId.value) return;

  const full = String(event?.data || '');
  const index = streamAssistantIndex.value;
  if (
    activeSessionId.value === streamSessionId.value &&
    index >= 0 &&
    index < messages.value.length &&
    messages.value[index]?.role === 'assistant'
  ) {
    messages.value[index].content = full || messages.value[index].content || '';
  }

  const doneSessionId = streamSessionId.value;
  const shouldReloadActive = activeSessionId.value === doneSessionId;
  resetStreamingState();
  if (shouldReloadActive) {
    await loadSession(doneSessionId, { keepSelection: true });
  }
  await refreshSessions();
};

/**
 * 处理流式错误事件
 * 
 * 清理策略：
 * 1. 显示错误消息
 * 2. 如果 assistant 消息为空，删除占位消息（避免空白气泡）
 * 3. 重置流式状态
 * 4. 刷新会话列表
 * 
 * @param {Object} event - Wails 事件对象，data 字段包含错误信息
 */
const onStreamError = async (event) => {
  const msg = String(event?.data || '流式回复失败');
  console.error('Stream error:', msg);
  console.log('Setting errorMessage to:', msg);
  console.log('Current activeSessionId:', activeSessionId.value);
  console.log('Current streamSessionId:', streamSessionId.value);
  
  // 首先设置错误消息，确保它不会被后续操作清除
  errorMessage.value = msg;
  console.log('errorMessage.value after set:', errorMessage.value);

  const index = streamAssistantIndex.value;
  if (
    activeSessionId.value === streamSessionId.value &&
    index >= 0 &&
    index < messages.value.length &&
    messages.value[index]?.role === 'assistant'
  ) {
    if (!messages.value[index].content) {
      messages.value.splice(index, 1);
    }
  }

  resetStreamingState();
  console.log('After resetStreamingState, errorMessage.value:', errorMessage.value);
  // 注意：不要立即调用 refreshSessions，因为它可能会清除 errorMessage
  // 让错误消息保持显示，用户可以手动刷新或重新发送
};

/**
 * 监听消息内容变化
 * 
 * 触发条件：
 * - 流式响应中每个分片的追加
 * 
 * 实现技巧：
 * - 将所有消息内容用特殊字符连接成字符串
 * - 任何内容变化都会改变这个字符串，从而触发 watch
 * 
 * 目的：流式响应时持续滚动，确保用户能看到最新的回复内容
 */
watch(
  () => messages.value.map((item) => item.content || '').join('\u0001'),
  async () => {
    await scrollToBottom();
  }
);


/**
 * 组件挂载时注册事件监听器
 * 
 * 注册的三个事件：
 * 1. chat_chunk: 接收流式分片
 * 2. chat_done: 接收完整回复
 * 3. chat_stream_error: 接收错误信息
 * 
 * 每个监听器都会返回一个取消订阅函数，
 * 存储在 eventUnsubscribers 数组中供后续清理。
 */
onMounted(() => {
  eventUnsubscribers.push(Events.On('chat_chunk', onStreamChunk));
  eventUnsubscribers.push(Events.On('chat_done', onStreamDone));
  eventUnsubscribers.push(Events.On('chat_stream_error', onStreamError));
});

/**
 * 组件卸载前清理事件监听器
 * 
 * 重要性：
 * - 防止内存泄漏（未清理的事件监听器会阻止 GC）
 * - 避免重复回调（组件重新挂载时会注册新的监听器）
 * - 确保组件生命周期的一致性
 * 
 * 清理策略：
 * - 遍历所有取消订阅函数并执行
 * - 从数组中移除已执行的函数
 */
onBeforeUnmount(() => {
  while (eventUnsubscribers.length) {
    const off = eventUnsubscribers.pop();
    if (typeof off === 'function') {
      off();
    }
  }
});


return {
  streamSessionId,
  streamAssistantIndex,
  resetStreamingState,
  onStreamChunk,
  onStreamDone,
  onStreamError,
};  


}