import { nextTick } from 'vue';
import MarkdownIt from 'markdown-it';
import hljs from 'highlight.js';
import 'highlight.js/styles/github.css';

export function useTools(

) {

/**
 * ========================================
 * 3. Markdown 渲染器配置
 * ========================================
 * 
 * 功能：
 * - 自动换行（breaks: true）
 * - 链接识别（linkify: true）
 * - 排版优化（typographer: true）
 * - 代码高亮（集成 highlight.js）
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
        // 回退到纯文本输出，避免高亮失败影响消息渲染。
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
 * @param {string} content - 原始文本内容
 * @returns {string} 渲染后的 HTML 字符串
 */
const renderMarkdown = (content) => markdownRenderer.render(String(content || ''));

return {
  renderMarkdown,
};

}