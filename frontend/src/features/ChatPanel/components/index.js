/**
 * ChatPanel 子组件导出
 * 
 * 提供统一的组件导入入口，方便在主组件中使用
 */

/** 会话列表组件 - 展示和管理会话 */
export { default as SessionList } from './SessionList.vue';

/** 消息展示组件 - 渲染对话消息（支持 Markdown） */
export { default as MessageDisplay } from './MessageDisplay.vue';

/** 输入区域组件 - 文本输入和附件管理 */
export { default as InputArea } from './InputArea.vue';
