import { ref, markRaw } from 'vue';

/**
 * useFormulaNavigation - 公式导航 Composable
 * 
 * 功能：
 * - PDF 加载后使用 page.getTextContent() 提取全部页面文本
 * - 扫描公式编号（括号包裹的数字），记录首次出现的位置
 * - 监听用户文本选择，检测是否选中了公式编号
 * - 显示悬浮窗，支持跳转到公式位置和返回原位置
 * 
 * @param {Object} options
 * @param {Ref} options.pdfDoc - PDF 文档对象
 * @param {Ref<number>} options.totalPages - 总页数
 * @param {Ref} options.readerContainer - 阅读器容器 ref
 * @param {Object} options.pageContainerRefs - 页面容器引用
 * @param {Object} options.pageCanvasRefs - 页面 Canvas 引用
 * @param {Function} options.GoToPage - 跳转页面方法
 */
export function useFormulaNavigation({
  pdfDoc,
  totalPages,
  readerContainer,
  pageContainerRefs,
  pageCanvasRefs,
  GoToPage,
}) {
  // 公式编号 → 首次出现位置 { pageNum, x, y (PDF 坐标) }
  const formulaMap = ref(new Map());
  // 当前选中的公式信息
  const selectedFormula = ref(null);
  // 悬浮窗显示状态
  const showFormulaPopup = ref(false);
  // 悬浮窗位置（视口坐标）
  const popupPosition = ref({ x: 0, y: 0 });
  // 跳转前的滚动位置
  const previousScrollPosition = ref(null);
  // 是否已跳转到公式位置（用于显示"返回"按钮）
  const hasJumped = ref(false);

  // 内部状态
  let selectionListener = null;
  let outsideClickHandler = null;
  let isJumping = false; // 跳转进行中的标志，防止 selectionchange 干扰

  // 匹配公式编号的正则：只匹配英文括号包裹的数字
  // \( (\d+(?:\.\d+)*) \)  → (1), (2.3), (12)
  const FORMULA_RE = /\((\d+(?:\.\d+)*)\)/g;

  /**
   * 从 PDF 全部页面提取公式编号及其首次出现位置
   * 使用 page.getTextContent() API，不依赖 DOM 渲染
   */
  async function extractFormulas() {
    if (!pdfDoc.value) return;

    formulaMap.value = new Map();
    const numPages = totalPages.value;
    console.log(`[公式提取] 开始扫描 ${numPages} 页...`);
    const startTime = performance.now();

    for (let pageNum = 1; pageNum <= numPages; pageNum++) {
      try {
        const page = await pdfDoc.value.getPage(pageNum);
        const textContent = await page.getTextContent();
        const pageHeight = page.view[3] - page.view[1];

        // 拼接本页全部文本，并记录每个 text item 的字符偏移
        let pageText = '';
        const items = [];

        for (const item of textContent.items) {
          if (!item.str) continue;
          items.push({ str: item.str, offset: pageText.length, transform: item.transform });
          pageText += item.str;
        }

        // 扫描公式编号
        FORMULA_RE.lastIndex = 0;
        let match;

        while ((match = FORMULA_RE.exec(pageText)) !== null) {
          const formulaNumber = match[1]; // 捕获组 1 就是括号内的数字
          if (!formulaNumber) continue;

          // 只记录首次出现
          if (formulaMap.value.has(formulaNumber)) continue;

          // 定位匹配文本起始处所在的 text item
          const matchStart = match.index;
          let targetItem = items[0];

          for (let i = 0; i < items.length; i++) {
            if (items[i].offset <= matchStart) {
              targetItem = items[i];
            } else {
              break;
            }
          }

          if (!targetItem) continue;

          const x = targetItem.transform[4];
          const y = targetItem.transform[5];

          formulaMap.value.set(formulaNumber, {
            pageNum,
            x,
            y,
            pageHeight,
          });
        }

        // 每处理一页让出一次主线程，避免阻塞 UI
        if (pageNum % 5 === 0) {
          await new Promise(resolve => setTimeout(resolve, 0));
        }
      } catch (err) {
        console.warn(`[公式提取] 第 ${pageNum} 页提取失败:`, err);
      }
    }

    const elapsed = (performance.now() - startTime).toFixed(0);
    console.log(`[公式提取] 完成，共 ${formulaMap.value.size} 个公式编号，耗时 ${elapsed}ms`);
  }

  /**
   * 设置文本选择监听（在 PDF 加载完成后调用）
   */
  function setupSelectionListener() {
    // 清除旧监听
    cleanupSelectionListener();

    selectionListener = () => {
      // 跳转进行中不处理选择变化
      if (isJumping) return;

      const selection = window.getSelection();
      if (!selection || selection.isCollapsed || !selection.toString().trim()) {
        return;
      }

      const selectedText = selection.toString().trim();
      // 去掉多余空白，便于匹配
      const normalized = selectedText.replace(/\s+/g, '');

      // 用户选择检测宽松模式：英文括号、中文括号、裸数字均可
      const enParenMatch = normalized.match(/^\((\d+(?:\.\d+)*)\)$/);       // (1)
      const zhParenMatch = !enParenMatch ? normalized.match(/^\uff08(\d+(?:\.\d+)*)\uff09$/) : null; // （1）
      const bareMatch = (!enParenMatch && !zhParenMatch) ? normalized.match(/^(\d+(?:\.\d+)*)$/) : null; // 1
      const theMatch = enParenMatch || zhParenMatch || bareMatch;

      if (!theMatch) {
        return;
      }

      const formulaNumber = theMatch[1];

      const formulaInfo = formulaMap.value.get(formulaNumber);
      if (!formulaInfo) return;

      // 获取选区位置，用于定位悬浮窗
      const range = selection.getRangeAt(0);
      const rect = range.getBoundingClientRect();

      popupPosition.value = {
        x: Math.min(rect.left + rect.width / 2, window.innerWidth - 160),
        y: rect.bottom + 10,
      };

      selectedFormula.value = {
        number: formulaNumber,
        pageNum: formulaInfo.pageNum,
        x: formulaInfo.x,
        y: formulaInfo.y,
        pageHeight: formulaInfo.pageHeight,
      };

      showFormulaPopup.value = true;
      hasJumped.value = false;
      previousScrollPosition.value = null;

      // 添加外部点击关闭
      addOutsideClickListener();
    };

    document.addEventListener('selectionchange', selectionListener);
  }

  /**
   * 清除选择监听
   */
  function cleanupSelectionListener() {
    if (selectionListener) {
      document.removeEventListener('selectionchange', selectionListener);
      selectionListener = null;
    }
    removeOutsideClickListener();
  }

  /**
   * 添加外部点击关闭处理器
   */
  function addOutsideClickListener() {
    removeOutsideClickListener();
    outsideClickHandler = (e) => {
      if (e.target.closest('.formula-preview-popup')) return;
      // 跳转后仅允许通过"返回"按钮关闭，外部点击不关闭
      if (hasJumped.value) return;
      showFormulaPopup.value = false;
    };
    // 用 setTimeout 避免当前 selection 事件触发关闭
    setTimeout(() => {
      if (outsideClickHandler) {
        document.addEventListener('mousedown', outsideClickHandler);
      }
    }, 50);
  }

  function removeOutsideClickListener() {
    if (outsideClickHandler) {
      document.removeEventListener('mousedown', outsideClickHandler);
      outsideClickHandler = null;
    }
  }

  /**
   * 跳转到公式位置
   * 
   * 策略：
   * 1. 先 GoToPage 确保目标页已渲染且可见
   * 2. 等待平滑滚动完成
   * 3. 重新获取页面/canvas 尺寸，精确滚动到公式位置
   * 4. 保持悬浮窗可见（固定在右下角），显示"返回"按钮
   */
  async function jumpToFormula() {
    if (!selectedFormula.value || !readerContainer.value) return;

    const formula = selectedFormula.value;

    // 标记跳转开始，防止 selectionchange 干扰状态
    isJumping = true;

    // 保存当前位置（用于返回）
    previousScrollPosition.value = {
      scrollTop: readerContainer.value.scrollTop,
      scrollLeft: readerContainer.value.scrollLeft,
    };

    hasJumped.value = true;

    // 第一步：立即跳转到目标页（确保页面已渲染，使用 instant 无动画）
    const targetPageContainer = pageContainerRefs[formula.pageNum];
    if (targetPageContainer) {
      targetPageContainer.scrollIntoView({ behavior: 'instant', block: 'start' });
    }

    // 第二步：等待页面渲染完成
    await new Promise(resolve => setTimeout(resolve, 150));

    // 第三步：在页面已渲染的前提下，精确滚动到公式位置
    const pageContainer = pageContainerRefs[formula.pageNum];
    const canvas = pageCanvasRefs[formula.pageNum];

    if (pageContainer && canvas) {
      const canvasDisplayHeight = parseFloat(canvas.style.height) || canvas.clientHeight;

      if (canvasDisplayHeight > 0 && formula.pageHeight > 0) {
        // 重新获取页面相对于阅读器的当前位置
        const readerRect = readerContainer.value.getBoundingClientRect();
        const pageTop = pageContainer.getBoundingClientRect().top - readerRect.top + readerContainer.value.scrollTop;

        // PDF 坐标 → Canvas 显示坐标
        const canvasY = (formula.pageHeight - formula.y) * (canvasDisplayHeight / formula.pageHeight);
        const targetScroll = pageTop + canvasY - readerContainer.value.clientHeight / 3;

        readerContainer.value.scrollTo({
          top: Math.max(0, targetScroll),
          behavior: 'instant',
        });
      }
    }

    // 不关闭悬浮窗 —— 它会自动移动到右下角显示"返回"按钮

    // 跳转完成，解除 selectionchange 屏蔽
    isJumping = false;
  }

  /**
   * 返回跳转前的位置
   */
  function returnToPreviousPosition() {
    if (!previousScrollPosition.value || !readerContainer.value) return;

    readerContainer.value.scrollTo({
      top: previousScrollPosition.value.scrollTop,
      left: previousScrollPosition.value.scrollLeft,
      behavior: 'instant',
    });

    hasJumped.value = false;
    previousScrollPosition.value = null;
    showFormulaPopup.value = false; // 返回后关闭悬浮窗
  }

  /**
   * 清理所有资源
   */
  function cleanup() {
    cleanupSelectionListener();
    formulaMap.value = new Map();
    selectedFormula.value = null;
    showFormulaPopup.value = false;
    hasJumped.value = false;
    previousScrollPosition.value = null;
  }

  return {
    formulaMap,
    selectedFormula,
    showFormulaPopup,
    popupPosition,
    hasJumped,
    extractFormulas,
    setupSelectionListener,
    jumpToFormula,
    returnToPreviousPosition,
    cleanup,
  };
}
