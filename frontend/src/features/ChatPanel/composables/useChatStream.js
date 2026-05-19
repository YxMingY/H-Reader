/**
 * useChatStream - 流式响应管理 Composable
 * 
 * 职责：
 * - 监听后端推送的流式消息事件（chat_chunk, chat_done, chat_error）
 * - 实时更新 assistant 消息内容
 * - 处理流式响应的生命周期（开始、进行中、完成、错误）
 * - 管理事件订阅和清理
 * - 发送消息并管理流式响应
 * 
 * 工作流程：
 * 1. 用户发送消息 → 创建占位消息 → 调用后端 API
 * 2. 后端通过事件推送分片数据（chat_chunk）
 * 3. 前端逐块更新 assistant 消息内容
 * 4. 后端完成后发送 chat_done 事件
 * 5. 前端刷新会话数据，确保与后端同步
 * 
 * @param {Ref<string>} activeSessionId - 活跃会话 ID
 * @param {Ref<boolean>} sending - 发送状态
 * @param {Ref<Array>} messages - 消息列表
 * @param {Ref<string>} errorMessage - 错误消息
 * @param {Function} loadSession - 加载会话函数
 * @param {Function} refreshSessions - 刷新会话列表函数
 * @param {Function} scrollToBottom - 滚动到底部函数
 * @param {Function} createSession - 创建会话函数
 * @param {Object} props - 组件 props（scopeType, bookPath 等）
 * @param {Ref<string>} draft - 草稿文本
 * @param {Ref<Array>} attachments - 附件列表
 * @returns {Object} 流式响应相关的状态和方法
 */

import { ref, watch, onMounted, onBeforeUnmount } from 'vue';
import { Events } from '@wailsio/runtime';
import { ChatService } from '../../../../bindings/hreader';

export function useChatStream(activeSessionId, sending, messages, errorMessage, loadSession, refreshSessions, scrollToBottom, createSession, props, draft, attachments) {
  // ========================================
  // 响应式状态
  // ========================================

  /** 当前流式响应的会话 ID：用于验证事件来源 */
  const streamSessionId = ref('');
  
  /** 当前流式响应的 assistant 消息索引：用于追加内容 */
  const streamAssistantIndex = ref(-1);

  /** 事件取消订阅函数列表：用于组件卸载时清理 */
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
 * 发送消息 - 核心业务逻辑
 * 
 * 执行流程：
 * 1. 验证输入：文本或附件至少有一个
 * 2. 会话检查：如果没有活跃会话，自动创建新会话
 * 3. 乐观更新：在本地创建 user 和 assistant 占位消息，提供即时反馈
 * 4. 流式追踪：设置 streamSessionId 和 streamAssistantIndex
 * 5. 清理输入：清空草稿和附件列表
 * 6. 滚动定位：滚动到消息底部
 * 7. API 调用：调用后端流式接口 SendMessageStreamInSession
 * 8. 错误处理：失败时回滚占位消息，显示错误提示
 * 
 * 设计原则：
 * - 乐观 UI：提前显示占位消息，提升用户体验
 * - 失败回滚：API 调用失败时恢复 UI 状态，保持一致性
 * - 多模态支持：同时支持纯文本、纯图片、图文混合发送
 * 
 * 注意事项：
 * - API 调用可能立即失败（如 API key 未配置）
 * - 也可能在后台异步失败（通过网络事件监听器捕获）
 */
const sendMessage = async () => {
  // 步骤 1: 验证输入
  let text = draft.value.trim();
  
  // 如果只有图片没有文字，自动添加默认提示
  if (!text && attachments.value.length > 0) {
    text = '请解释图片';
  }
  
  if (!text && attachments.value.length === 0) return;

  // 步骤 2: 初始化状态
  errorMessage.value = '';
  sending.value = true;
  
  try {
    // 步骤 3: 确保有活跃会话
    let sessionId = activeSessionId.value;
    if (!sessionId) {
      const summary = await createSession();
      if (!summary) {
        throw new Error('创建会话失败');
      }
      sessionId = summary.session_id;
      activeSessionId.value = sessionId;
    }

    // 步骤 4: 乐观更新 - 创建占位消息
    const localNow = new Date().toISOString();
    
    // 准备附件数据（用于立即显示）
    const pendingImages = attachments.value.map((item) => item.dataUrl);
    
    // 创建用户消息（包含附件）
    messages.value.push({ 
      role: 'user', 
      content: text, 
      attachments: pendingImages.length > 0 ? pendingImages : undefined,
      created_at: localNow 
    });
    
    // 创建 assistant 占位消息
    messages.value.push({ role: 'assistant', content: '', created_at: localNow });
    
    // 步骤 5: 设置流式追踪状态
    streamSessionId.value = sessionId;
    streamAssistantIndex.value = messages.value.length - 1;

    // 步骤 6: 清理输入
    draft.value = '';
    attachments.value = [];
    
    // 步骤 7: 滚动到底部
    await scrollToBottom();

    // 步骤 8: 调用后端流式 API
    await ChatService.SendMessageStreamInSession(
      props.scopeType,
      props.scopeType === 'book' ? props.bookPath : '',
      sessionId,
      text,
      pendingImages
    );
  } catch (err) {
    // 错误处理：回滚占位消息
    console.error('sendMessage catch error:', err);
    
    // 移除 assistant 占位消息
    if (streamAssistantIndex.value >= 0 && streamAssistantIndex.value < messages.value.length) {
      messages.value.splice(streamAssistantIndex.value, 1);
      streamAssistantIndex.value = -1;
    }
    
    // 移除 user 占位消息（如果存在）
    if (messages.value.length && messages.value[messages.value.length - 1]?.role === 'user' && messages.value[messages.value.length - 1]?.content === text) {
      messages.value.splice(messages.value.length - 1, 1);
    }
    
    // 重置流式状态
    resetStreamingState();
    
    // 显示错误消息
    const errorMsg = err?.message || String(err) || '发送失败';
    errorMessage.value = `发送失败：${errorMsg}`;
    console.error('sendMessage error:', err);
  } finally {
    // 无论成功与否，都重置发送状态
    sending.value = false;
  }
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
  sendMessage,
  onStreamChunk,
  onStreamDone,
  onStreamError,
};  


}