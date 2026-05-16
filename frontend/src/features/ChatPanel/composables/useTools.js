/**
 * useTools - 工具函数 Composable
 * 
 * 提供聊天面板所需的工具函数：
 * - Markdown 渲染（支持代码高亮、链接识别、自动换行）
 * - LaTeX 数学公式渲染（支持 $...$、$$...$$、\(...\)、\[...\] 等多种语法，使用 KaTeX）
 * - 集成 highlight.js 实现语法高亮
 * 
 * @returns {Object} 工具函数集合
 */

import MarkdownIt from 'markdown-it';
import texmath from 'markdown-it-texmath';
import katex from 'katex';
import hljs from 'highlight.js';
import 'highlight.js/styles/github.css';
import 'katex/dist/katex.min.css';

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
   * 插件：
   * - markdown-it-texmath - 支持 LaTeX 数学公式渲染（基于 KaTeX）
   *   - 行内公式：$...$ 或 \(...\)
   *   - 块级公式：$$...$$ 或 \[...\]
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

  // 启用 texmath 插件以支持 LaTeX 数学公式
  // 配置说明：
  // - engine: katex - 使用 KaTeX 作为渲染引擎
  // - delimiters: 支持的分隔符类型
  //   - 'dollars': $...$ (行内) 和 $$...$$ (块级)
  //   - 'brackets': \(...\) (行内) 和 \[...\] (块级)
  //   - 'doxygen': \f$...\f$ (行内), \f(...\f) (行内), \f[...\f] (块级)
  //   - 可以使用数组组合多种类型，如 ['dollars', 'brackets']
  // - throwOnError: false - 渲染错误时不抛出异常，而是显示原始文本（适合流式输出）
  // - errorColor: '#cc0000' - 错误文本的颜色
  // - katexOptions: KaTeX 渲染选项
  //   - macros: {} - 自定义宏定义
  //   - strict: false - 允许非标准语法
  // 
  // 流式输出适配：
  // - throwOnError: false 确保不完整的公式不会导致渲染失败
  // - 未闭合的公式符号会被当作普通文本处理，等后续内容到达后重新渲染
  markdownRenderer.use(texmath, {
    engine: katex,
    delimiters: ['dollars', 'brackets'], // 同时支持 $...$ 和 \(...\) 两种语法
    throwOnError: false,
    errorColor: '#cc0000',
    katexOptions: {
      macros: {},
      strict: false, // 允许更宽松的语法解析
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
  const renderMarkdown = (content) => {
    const text = String(content || '');
    
    // 步骤 1：保护代码块和行内代码中的内容
    // 使用占位符替换代码，避免公式预处理误伤
    const codeBlocks = [];
    const inlineCodes = [];
    
    // 保护多行代码块 ```...```
    let processedText = text.replace(/```[\s\S]*?```/g, (match) => {
      codeBlocks.push(match);
      return `__CODE_BLOCK_${codeBlocks.length - 1}__`;
    });
    
    // 保护行内代码 `...`
    processedText = processedText.replace(/`[^`]+`/g, (match) => {
      inlineCodes.push(match);
      return `__INLINE_CODE_${inlineCodes.length - 1}__`;
    });
    
    // 步骤 2：预处理公式（此时代码已被保护，不会被误伤）
    processedText = processedText.replace(/\$\s*([^\n$]+?)\s*\$/g, (_, formula) => {
      const trimmedFormula = formula.trim();
      return `$${trimmedFormula}$`;
    });
    
    // 步骤 3：恢复代码块
    processedText = processedText.replace(/__CODE_BLOCK_(\d+)__/g, (_, index) => {
      return codeBlocks[parseInt(index)];
    });
    
    // 步骤 4：恢复行内代码
    processedText = processedText.replace(/__INLINE_CODE_(\d+)__/g, (_, index) => {
      return inlineCodes[parseInt(index)];
    });
    
    return markdownRenderer.render(processedText);
  };

  return {
    renderMarkdown,
  };
}