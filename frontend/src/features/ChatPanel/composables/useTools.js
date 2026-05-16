/**
 * useTools - 工具函数 Composable
 * 
 * 提供聊天面板所需的工具函数：
 * - Markdown 渲染（支持代码高亮、链接识别、自动换行）
 * - 集成 highlight.js 实现语法高亮
 * 
 * @returns {Object} 工具函数集合
 */

import MarkdownIt from 'markdown-it';
import hljs from 'highlight.js';
import 'highlight.js/styles/github.css';

export function useTools() {
  // ========================================
  // Markdown 渲染器配置
  // ========================================

  /**
   * Markdown 渲染器实例
   * 
   * 配置说明：
   * - breaks: true - 将换行符转换为 <br> 标签，实现自动换行
   * - linkify: true - 自动识别并转换 URL 为可点击链接
   * - typographer: true - 启用排版优化（如引号转换）
   * - highlight - 自定义代码高亮函数
   * 
   * 高亮策略：
   * 1. 优先使用指定的语言进行高亮
   * 2. 如果指定语言失败，回退到自动检测
   * 3. 如果都失败，返回空字符串（不影响消息渲染）
   */
  const markdownRenderer = new MarkdownIt({
    breaks: true,
    linkify: true,
    typographer: true,
    highlight(code, language) {
      // 尝试使用指定语言高亮
      if (language && hljs.getLanguage(language)) {
        try {
          const highlighted = hljs.highlight(code, { language, ignoreIllegals: true }).value;
          return `<pre class="hljs"><code class="language-${language}">${highlighted}</code></pre>`;
        } catch (err) {
          // 回退到纯文本输出，避免高亮失败影响消息渲染
        }
      }

      // 回退到自动检测语言
      try {
        const highlighted = hljs.highlightAuto(code).value;
        return `<pre class="hljs"><code>${highlighted}</code></pre>`;
      } catch (err) {
        return '';
      }
    },
  });

  /**
   * 渲染 Markdown 内容为 HTML
   * 
   * @param {string} content - 原始文本内容
   * @returns {string} 渲染后的 HTML 字符串
   * 
   * @example
   * const html = renderMarkdown('# Hello\n**World**');
   * // 返回: "<h1>Hello</h1><p><strong>World</strong></p>"
   */
  const renderMarkdown = (content) => markdownRenderer.render(String(content || ''));

  return {
    renderMarkdown,
  };
}