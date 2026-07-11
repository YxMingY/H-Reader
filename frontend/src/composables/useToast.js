/**
 * useToast - 全局 Toast 确认框 Composable
 * 
 * 提供统一的确认对话框，替代 window.confirm
 * 支持 Promise 风格的调用方式
 * 
 * @example
 * const { confirm } = useToast();
 * const result = await confirm({
 *   title: '删除会话',
 *   message: '确定要删除这个会话吗？此操作不可恢复。',
 *   icon: '🗑️',
 *   confirmText: '删除',
 *   cancelText: '取消'
 * });
 * if (result) { /* 执行删除 * / }
 */

import { ref } from 'vue';

// 全局单例引用
let toastInstance = null;

/**
 * 设置 Toast 实例（由 App.vue 调用）
 * @param {Object} instance - ToastConfirm 组件的 ref
 */
export function setToastInstance(instance) {
  toastInstance = instance;
}

/**
 * 显示确认框
 * @param {Object} options - 配置选项
 * @param {string} options.title - 标题（可选）
 * @param {string} options.message - 消息内容（必填）
 * @param {string} options.icon - 图标 emoji（可选，默认 ⚠️）
 * @param {string} options.confirmText - 确认按钮文本（可选，默认 "确定"）
 * @param {string} options.cancelText - 取消按钮文本（可选，默认 "取消"）
 * @returns {Promise<boolean>} 用户点击确定返回 true，取消返回 false
 */
export async function confirm(options) {
  if (!toastInstance) {
    console.warn('Toast 实例未初始化，降级使用 window.confirm');
    return window.confirm(options.message);
  }
  
  return await toastInstance.show(options);
}

/**
 * 快捷方法：删除确认
 * @param {string} itemName - 被删除的项目名称
 * @param {boolean} irreversible - 是否不可恢复（默认 true）
 * @returns {Promise<boolean>}
 */
export async function confirmDelete(itemName, irreversible = true) {
  return await confirm({
    title: '删除确认',
    message: `确定要删除「${itemName}」吗？${irreversible ? '此操作不可恢复。' : ''}`,
    icon: '🗑️',
    confirmText: '删除',
    cancelText: '取消'
  });
}

/**
 * 快捷方法：编辑确认
 * @param {string} message - 额外说明（可选）
 * @returns {Promise<boolean>}
 */
export async function confirmEdit(message = '') {
  return await confirm({
    title: '重新编辑',
    message: message || '确定要重新编辑这条消息吗？这将删除本轮对话。',
    icon: '✏️',
    confirmText: '确定',
    cancelText: '取消'
  });
}

/**
 * 快捷方法：危险操作确认
 * @param {Object} options - 配置选项
 * @returns {Promise<boolean>}
 */
export async function confirmDanger(options) {
  return await confirm({
    ...options,
    icon: options.icon || '⚠️'
  });
}

/**
 * useToast hook（保持 API 一致性）
 * @returns {Object}
 */
export function useToast() {
  return {
    confirm,
    confirmDelete,
    confirmEdit,
    confirmDanger
  };
}
