/**
 * useScrollBottom - 智能滚动到底部 Composable
 * 
 * 功能说明：
 * - 管理聊天消息容器的自动滚动行为
 * - 检测用户是否主动离开底部
 * - 只在用户在底部时自动滚动，避免打断用户浏览历史消息
 * - 提供手动控制滚动的能力
 * 
 * 使用场景：
 * - ChatPanel 组件中的消息列表滚动管理
 * - 任何需要智能滚动行为的长列表场景
 * 
 * @returns {Object} 滚动控制方法和状态
 */

import { ref, nextTick } from 'vue';

export function useScrollBottom() {
  // ========================================
  // 状态管理
  // ========================================

  /**
   * 标记用户是否主动离开底部
   * true = 用户已上拉，不应自动滚动
   * false = 用户在底部，应自动滚动
   */
  const userScrolledAway = ref(false);

  /**
   * 滚动阈值配置
   * 使用容器高度按比例计算，再限制在合理区间内，避免不同设备上体感差异过大
   */
  const SCROLL_THRESHOLD_RATIO = 0.06;
  const MIN_SCROLL_THRESHOLD = 20;
  const MAX_SCROLL_THRESHOLD = 50;

  /**
   * 计算当前容器的底部判定阈值
   * @param {HTMLElement} container - 滚动容器 DOM 元素
   * @returns {number} 动态阈值（像素）
   */
  const getScrollThreshold = (container) => {
    const clientHeight = container?.clientHeight || 0;
    const threshold = Math.round(clientHeight * SCROLL_THRESHOLD_RATIO);
    return Math.min(MAX_SCROLL_THRESHOLD, Math.max(MIN_SCROLL_THRESHOLD, threshold));
  };

  // ========================================
  // 核心方法
  // ========================================

  /**
   * 检查容器是否在底部
   * 
   * @param {HTMLElement} container - 滚动容器 DOM 元素
   * @returns {boolean} true = 在底部，false = 不在底部
   */
  const isAtBottom = (container) => {
    if (!container) return true;
    
    const { scrollTop, scrollHeight, clientHeight } = container;
    // 计算距离底部的距离
    const distanceToBottom = scrollHeight - scrollTop - clientHeight;
    return distanceToBottom <= getScrollThreshold(container);
  };

  /**
   * 记录当前滚动位置
   *
   * @param {HTMLElement} container - 滚动容器 DOM 元素
   * @returns {number} 当前滚动高度（scrollTop）
   */
  const captureScrollPosition = (container) => {
    if (!container) return 0;

    return Math.max(0, container.scrollTop || 0);
  };

  /**
   * 恢复到指定滚动位置
   *
   * 用于重新加载消息后保持用户原先阅读的位置，而不是强制到底部。
   *
   * @param {HTMLElement} container - 滚动容器 DOM 元素
   * @param {number} scrollTop - 需要恢复的滚动高度
   */
  const restoreScrollPosition = async (container, scrollTop) => {
    if (!container || scrollTop === undefined || scrollTop === null) return;

    await nextTick();

    const maxScrollTop = Math.max(0, container.scrollHeight - container.clientHeight);
    const targetScrollTop = Math.min(Math.max(0, scrollTop), maxScrollTop);

    container.scrollTop = targetScrollTop;

    requestAnimationFrame(() => {
      container.scrollTop = targetScrollTop;

      requestAnimationFrame(() => {
        container.scrollTop = targetScrollTop;
      });
    });
  };

  /**
   * 处理用户滚动事件
   * 检测用户是否主动离开底部，并更新状态
   * 
   * @param {Event} event - 滚动事件对象
   */
  const handleScroll = (event) => {
    const container = event.target;
    const atBottom = isAtBottom(container);
    
    if (atBottom) {
      // 用户滚动到底部，重置标记
      userScrolledAway.value = false;
    } else {
      // 用户离开底部，设置标记
      userScrolledAway.value = true;
    }
  };

  /**
   * 智能滚动到底部
   * 
   * 策略：
   * - 如果用户主动上拉（userScrolledAway = true），不强制滚动
   * - 如果用户在底部（userScrolledAway = false），自动滚动
   * - 使用 nextTick 等待 DOM 更新完成
   * - 使用多层 requestAnimationFrame 确保异步渲染完成后仍能正确滚动
   * 
   * @param {HTMLElement} container - 滚动容器 DOM 元素
   */
  const scrollToBottom = async (container) => {
    // 如果用户主动离开底部，不执行自动滚动
    if (userScrolledAway.value) {
      return;
    }
    
    if (!container) return;
    
    await nextTick();
    
    // 第一次滚动：立即执行
    container.scrollTop = container.scrollHeight;
    
    // 第二次滚动：下一帧执行，等待部分异步渲染
    requestAnimationFrame(() => {
      container.scrollTop = container.scrollHeight;
      
      // 第三次滚动：再下一帧执行，确保所有内容渲染完成
      requestAnimationFrame(() => {
        container.scrollTop = container.scrollHeight;
      });
    });
  };

  /**
   * 强制滚动到底部（忽略用户状态）
   * 
   * 用于特殊场景，如：
   * - 切换会话时
   * - 加载历史消息后
   * - 用户点击"回到底部"按钮
   * 
   * @param {HTMLElement} container - 滚动容器 DOM 元素
   */
  const forceScrollToBottom = async (container) => {
    if (!container) return;
    
    await nextTick();
    
    container.scrollTop = container.scrollHeight;
    
    requestAnimationFrame(() => {
      container.scrollTop = container.scrollHeight;
      
      requestAnimationFrame(() => {
        container.scrollTop = container.scrollHeight;
        // 强制滚动后，重置用户状态
        userScrolledAway.value = false;
      });
    });
  };

  /**
   * 重置滚动状态
   * 通常在切换会话或清空消息时调用
   */
  const resetScrollState = () => {
    userScrolledAway.value = false;
  };

  // ========================================
  // 公开 API
  // ========================================

  return {
    // 状态
    userScrolledAway,
    
    // 方法
    isAtBottom,
    handleScroll,
    captureScrollPosition,
    restoreScrollPosition,
    scrollToBottom,
    forceScrollToBottom,
    resetScrollState,
  };
}
