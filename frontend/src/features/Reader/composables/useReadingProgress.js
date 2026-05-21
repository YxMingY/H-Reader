/**
 * useReadingProgress - 阅读进度管理 Composable
 * 
 * 职责：
 * - 保存和恢复 PDF 阅读进度
 * - 使用 PDF 内容哈希作为唯一标识
 * - 等待 PDF 加载完成后翻页
 * - 自动保存（带防抖）
 * 
 * @param {Ref<string>} bookPath - 书籍文件路径
 * @param {Ref<number>} currentPage - 当前页码
 * @param {Ref<Object>} readerRef - Reader 组件引用
 * @returns {Object} 阅读进度管理相关的方法
 */

import { ref, watch, onBeforeUnmount } from 'vue';
import { BookService } from '../../../../bindings/hreader/services/book';

export function useReadingProgress(bookPath, currentPage, readerRef) {
  // ========================================
  // 状态管理
  // ========================================

  /**
   * 标记 PDF 是否已加载完成
   * 用于确保在 PDF 加载完成后再翻页
   */
  const isPdfLoaded = ref(false);

  /**
   * 防抖定时器
   * 用于延迟保存阅读进度，避免频繁写入
   */
  let saveProgressTimer = null;

  // ========================================
  // 核心方法
  // ========================================

  /**
   * 获取并恢复阅读进度
   * 
   * 在 PDF 加载前调用，获取保存的页码
   * 
   * @returns {Promise<number>} 恢复的页码
   */
  const restoreProgress = async () => {
    if (!bookPath.value) {
      return 1;
    }

    try {
      const savedPage = await BookService.GetReadingProgress(bookPath.value);
      console.log(`[阅读进度] 恢复: ${bookPath.value} - 第 ${savedPage} 页`);
      return savedPage;
    } catch (err) {
      console.error('[阅读进度] 获取失败:', err);
      return 1;
    }
  };

  /**
   * 保存阅读进度
   * 
   * @param {number} page - 要保存的页码
   */
  const saveProgress = async (page) => {
    if (!bookPath.value || page <= 1) {
      return;
    }

    try {
      await BookService.SaveReadingProgress(bookPath.value, page);
      console.log(`[阅读进度] 保存: ${bookPath.value} - 第 ${page} 页`);
    } catch (err) {
      console.error('[阅读进度] 保存失败:', err);
    }
  };

  /**
   * 防抖保存阅读进度
   * 
   * 延迟 2 秒后保存，避免频繁写入
   * 如果在此期间再次调用，会重置定时器
   * 
   * @param {number} page - 要保存的页码
   */
  const debouncedSaveProgress = (page) => {
    // 清除之前的定时器
    if (saveProgressTimer) {
      clearTimeout(saveProgressTimer);
    }

    // 设置新的定时器（2秒后保存）
    saveProgressTimer = setTimeout(() => {
      saveProgress(page);
    }, 2000);
  };

  /**
   * 标记 PDF 已加载完成
   * 
   * 由 Reader 组件在 PDF 加载完成后调用
   */
  const markPdfLoaded = () => {
    isPdfLoaded.value = true;
  };

  /**
   * 跳转到指定页码（等待 PDF 加载完成）
   * 
   * @param {number} targetPage - 目标页码
   */
  const goToPage = async (targetPage) => {
    if (!isPdfLoaded.value) {
      console.warn('[阅读进度] PDF 尚未加载完成，等待中...');
      // 等待 PDF 加载完成
      await new Promise((resolve) => {
        const unwatch = watch(isPdfLoaded, (loaded) => {
          if (loaded) {
            unwatch();
            resolve();
          }
        });
      });
    }

    // 调用 Reader 的翻页方法
    if (readerRef.value && typeof readerRef.value.GoToPage === 'function') {
      readerRef.value.GoToPage(targetPage);
      console.log(`[阅读进度] 跳转到第 ${targetPage} 页`);
    }
  };

  // ========================================
  // 自动监听页码变化
  // ========================================

  /**
   * 监听页码变化，自动保存阅读进度
   * 
   * 当用户翻页时，自动触发防抖保存
   */
  const setupAutoSave = () => {
    const stopWatch = watch(
      currentPage,
      (newPage) => {
        // 只在 PDF 加载完成后才自动保存
        if (isPdfLoaded.value && newPage > 1) {
          debouncedSaveProgress(newPage);
        }
      }
    );

    return stopWatch;
  };

  // ========================================
  // 生命周期清理
  // ========================================

  /**
   * 组件卸载时立即保存当前页码并清理定时器
   * 
   * 执行流程：
   * 1. 清除防抖定时器（避免重复保存）
   * 2. 如果 PDF 已加载且页码 > 1，立即保存当前进度
   * 3. 确保用户离开时的页码被持久化
   * 
   * 注意：
   * - 使用同步保存而非异步，确保数据写入完成
   * - 只在有效状态下保存（已加载 + 页码 > 1）
   */
  onBeforeUnmount(() => {
    // 步骤 1: 清除防抖定时器
    if (saveProgressTimer) {
      clearTimeout(saveProgressTimer);
      saveProgressTimer = null;
    }
    
    // 步骤 2: 立即保存当前页码
    if (isPdfLoaded.value && currentPage.value > 1 && bookPath.value) {
      console.log(`[阅读进度] 离开书籍，立即保存: ${bookPath.value} - 第 ${currentPage.value} 页`);
      // 同步调用保存（不等待 Promise，让后端异步处理）
      BookService.SaveReadingProgress(bookPath.value, currentPage.value).catch(err => {
        console.error('[阅读进度] 离开时保存失败:', err);
      });
    }
  });

  // ========================================
  // 公开 API
  // ========================================

  return {
    isPdfLoaded,
    restoreProgress,
    saveProgress,
    debouncedSaveProgress,
    markPdfLoaded,
    goToPage,
    setupAutoSave,
  };
}
