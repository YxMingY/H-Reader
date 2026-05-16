/**
 * useSession - 会话管理 Composable
 * 
 * 职责：
 * - 管理会话列表（获取、刷新）
 * - 管理会话详情（加载消息、创建、删除）
 * - 维护活跃会话状态
 * - 处理会话切换和清理
 * 
 * @param {Object} props - 组件 props（scopeType, bookPath, bookKey）
 * @param {ComputedRef<string>} scopeTitle - 作用域标题
 * @param {ComputedRef<string>} scopeBookPath - 作用域书籍路径
 * @param {Ref<string>} errorMessage - 错误消息引用
 * @param {Function} scrollToBottom - 滚动到底部函数
 * @param {Function} clearInput - 清空输入函数
 * @returns {Object} 会话管理相关的状态和方法
 */

import { ref, watch } from 'vue';
import { ChatService } from '../../../../bindings/hreader';

export function useSession(props, scopeTitle, scopeBookPath, errorMessage, scrollToBottom, clearInput) {
  // ========================================
  // 响应式状态
  // ========================================

  /** 会话列表：存储当前作用域下的所有会话摘要 */
  const sessions = ref([]);
  
  /** 当前活跃的会话 ID：空字符串表示在会话列表页 */
  const activeSessionId = ref('');
  
  /** 当前会话的消息列表：按时间顺序排列 */
  const messages = ref([]);
  
  /** 加载会话列表的状态标志 */
  const loadingSessions = ref(false);
  
  /** 加载会话详情的状态标志 */
  const loadingMessages = ref(false);

/**
 * 规范化会话列表结果
 * 确保返回值始终是数组，避免 null/undefined 导致的错误
 * @param {*} result - 后端返回的会话列表
 * @returns {Array} 会话数组
 */
const normalizeSessions = (result) => (Array.isArray(result) ? result : []);
/**
 * ========================================
 * 6. 会话管理模块
 * ========================================
 * 
 * 这一组函数负责会话的 CRUD 操作，包括：
 * - refreshSessions: 拉取会话列表
 * - loadSession: 加载会话详情
 * - createSession: 创建新会话
 * - deleteSession: 删除会话
 * - upsertSessionSummary: 更新会话列表中的单个会话
 */

/**
 * 刷新会话列表
 * 
 * 流程：
 * 1. 调用后端 API 获取当前作用域的所有会话
 * 2. 如果有活跃会话，检查它是否仍在列表中
 * 3. 如果存在，重新加载该会话的消息
 * 4. 如果不存在，清空活跃会话状态
 * 
 * 用途：
 * - 组件初始化时
 * - 用户点击"刷新"按钮
 * - 发送消息后同步最新状态
 */
const refreshSessions = async () => {
  loadingSessions.value = true;
  errorMessage.value = '';
  try {
    sessions.value = normalizeSessions(await ChatService.ListSessions(props.scopeType, scopeBookPath.value));
    if (activeSessionId.value) {
      const existing = sessions.value.find((item) => item.session_id === activeSessionId.value);
      if (existing) {
        await loadSession(existing.session_id, { keepSelection: true });
      } else {
        activeSessionId.value = '';
        messages.value = [];
      }
    }
  } catch (err) {
    errorMessage.value = `加载会话失败：${err?.message || err}`;
  } finally {
    loadingSessions.value = false;
  }
};

/**
 * 插入或更新会话摘要到列表顶部
 * 
 * 策略：
 * - 先移除旧的相同 ID 的会话（避免重复）
 * - 将新的会话添加到列表开头（最新优先）
 * 
 * @param {Object} summary - 会话摘要对象
 */
const upsertSessionSummary = (summary) => {
  const next = sessions.value.filter((item) => item.session_id !== summary.session_id);
  next.unshift(summary);
  sessions.value = next;
};

/**
 * 加载指定会话的详细信息
 * 
 * 流程：
 * 1. 调用后端 API 获取会话详情（包括消息历史和 LLM 状态）
 * 2. 设置活跃会话 ID
 * 3. 填充消息列表
 * 4. 更新会话列表中的摘要（除非是保持选中状态）
 * 5. 滚动到底部显示最新消息
 * 
 * @param {string} sessionId - 会话 ID
 * @param {Object} options - 可选参数
 * @param {boolean} options.keepSelection - 是否保持当前选中状态（不更新列表）
 */
const loadSession = async (sessionId, options = {}) => {
  if (!sessionId) return;
  loadingMessages.value = true;
  errorMessage.value = '';
  try {
    const detail = await ChatService.LoadSession(props.scopeType, scopeBookPath.value, sessionId);
    activeSessionId.value = detail.summary.session_id;
    messages.value = detail.messages || [];
    if (!options.keepSelection) {
      upsertSessionSummary(detail.summary);
    }
    await scrollToBottom();
  } catch (err) {
    errorMessage.value = `加载对话失败：${err?.message || err}`;
  } finally {
    loadingMessages.value = false;
  }
};

/**
 * 创建新会话
 * 
 * 命名规则：
 * - 书籍级："{书名} 对话"
 * - 书架级："新会话"
 * 
 * @returns {Object|null} 创建的会话摘要，失败返回 null
 */
const createSession = async () => {
  errorMessage.value = '';
  try {
    const title = props.scopeType === 'book' ? `${scopeTitle.value} 对话` : '新会话';
    const summary = await ChatService.CreateSession(props.scopeType, scopeBookPath.value, title);
    upsertSessionSummary(summary);
    return summary;
  } catch (err) {
    errorMessage.value = `创建会话失败：${err?.message || err}`;
    return null;
  }
};

/**
 * 创建新会话并立即进入
 * 
 * 流程：
 * 1. 调用 createSession 创建会话
 * 2. 切换到会话详情页
 * 3. 清空输入状态
 * 4. 滚动到底部
 */
const createSessionAndEnter = async () => {
  const summary = await createSession();
  if (!summary) return;
  activeSessionId.value = summary.session_id;
  messages.value = [];
  clearInput();
  await scrollToBottom();
};

/**
 * 返回会话列表页
 * 
 * 清理状态：
 * - 清除活跃会话 ID
 * - 清空消息列表
 * - 清空输入草稿和附件
 * - 清除错误消息
 */
const backToSessions = () => {
  activeSessionId.value = '';
  messages.value = [];
  clearInput();
  errorMessage.value = '';
};

/**
 * 删除指定会话
 * 
 * 流程：
 * 1. 用户确认删除
 * 2. 调用后端 API 删除会话文件
 * 3. 从本地列表中移除
 * 4. 如果删除的是当前会话，切换到第一个会话或返回列表页
 * 
 * @param {string} sessionId - 要删除的会话 ID
 */
const deleteSession = async (sessionId) => {
  if (!window.confirm('确定删除这个会话吗？')) return;
  errorMessage.value = '';
  try {
    await ChatService.DeleteSession(props.scopeType, scopeBookPath.value, sessionId);
    sessions.value = sessions.value.filter((item) => item.session_id !== sessionId);
    if (activeSessionId.value === sessionId) {
      activeSessionId.value = '';
      messages.value = [];
      if (sessions.value.length > 0) {
        await loadSession(sessions.value[0].session_id, { keepSelection: true });
      }
    }
  } catch (err) {
    errorMessage.value = `删除会话失败：${err?.message || err}`;
  }
};

/**
 * 监听活跃会话变化
 * 
 * 触发条件：
 * - 用户点击会话列表项
 * - 创建新会话后自动进入
 * 
 * 目的：进入会话后立即滚动到底部，显示最新消息
 */
watch(
  () => activeSessionId.value,
  async (sessionId) => {
    if (!sessionId) return;
    await scrollToBottom();
  }
);

/**
 * 监听消息数量变化
 * 
 * 触发条件：
 * - 新增用户消息
 * - 新增 assistant 消息
 * - 加载历史消息
 * 
 * 目的：确保新消息出现时自动滚动到底部
 */
watch(
  () => messages.value.length,
  async () => {
    await scrollToBottom();
  }
);

return {
  sessions,
  activeSessionId,
  messages,
  loadingSessions,
  loadingMessages,
  refreshSessions,
  loadSession,
  createSession,
  createSessionAndEnter,
  backToSessions,
  deleteSession,
}



}