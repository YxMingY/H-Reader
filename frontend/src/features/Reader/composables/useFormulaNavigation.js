import { ref } from 'vue';

/**
 * useFormulaNavigation - 公式导航 Composable
 * 
 * 功能：
 * - PDF 加载后读取 PDF.js 文本流
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
 */
export function useFormulaNavigation({
  pdfDoc,
  totalPages,
  readerContainer,
  pageContainerRefs,
  pageCanvasRefs,
}) {
  // 公式编号 → 首次出现位置 { pageNum, x, y (PDF 坐标) }
  const formulaMap = ref(new Map());
  // 当前选中的公式信息
  const selectedFormula = ref(null);
  const selectedText = ref('');
  const selectionPoint = ref(null);
  const selectionTopLeft = ref(null);
  const selectedIsFormulaNumber = ref(false);
  const selectionActionPending = ref(false);
  const selectionInputActive = ref(false);
  // 悬浮窗显示状态
  const showFormulaPopup = ref(false);
  // 悬浮窗位置（视口坐标）
  const popupPosition = ref({ x: 0, y: 0 });
  // 跳转前的滚动位置
  const previousScrollPosition = ref(null);
  // 是否已跳转到公式位置（用于显示"返回"按钮）
  const hasJumped = ref(false);

  // 内部状态
  let selectionEvents;
  let isJumping = false; // 跳转进行中的标志，防止 selectionchange 干扰

  // 匹配公式编号的正则：只匹配英文括号包裹的数字
  // \( (\d+(?:\.\d+)*) \)  → (1), (2.3), (12)
  const FORMULA_RE = /\((\d+(?:\.\d+)*)\)/g;

  /**
   * 从 PDF 全部页面提取公式编号及其首次出现位置
   * 直接读取 PDF.js 文本流，不依赖 DOM 渲染
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
        const pageHeight = page.view[3] - page.view[1];

        // 拼接本页全部文本，并记录每个 text item 的字符偏移
        let pageText = '';
        const items = [];

        // 通过 reader 读取，兼容尚未实现 ReadableStream 异步迭代的 WebKit。
        const reader = page.streamTextContent().getReader();
        try {
          while (true) {
            const { done, value } = await reader.read();
            if (done) break;

            for (const item of value.items) {
              if (!item.str) continue;
              items.push({ offset: pageText.length, transform: item.transform });
              pageText += item.str;
            }
          }
        } finally {
          reader.releaseLock();
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
    cleanupSelectionListener();
    selectionEvents = new AbortController();
    const options = { signal: selectionEvents.signal };
    let pointerDown = false;
    let pointerEnd = null;

    function updateSelection() {
      if (isJumping || hasJumped.value || pointerDown || selectionActionPending.value || selectionInputActive.value) return;

      const selection = window.getSelection();
      const text = selection?.toString().trim();
      if (!text || selection.isCollapsed || !selection.rangeCount) {
        showFormulaPopup.value = false;
        return;
      }

      const range = selection.getRangeAt(0);
      const reader = readerContainer.value;
      // WebKit can anchor a whole text item at its enclosing page element.
      if (!reader?.contains(range.commonAncestorContainer) ||
          !Array.from(reader.querySelectorAll('.textLayer')).some(layer => range.intersectsNode(layer))) {
        showFormulaPopup.value = false;
        return;
      }

      const normalized = text.replace(/\s+/g, '');
      const formulaNumber = normalized.match(/^\((\d+(?:\.\d+)*)\)$/)?.[1]
        ?? normalized.match(/^（(\d+(?:\.\d+)*)）$/)?.[1]
        ?? normalized.match(/^(\d+(?:\.\d+)*)$/)?.[1];
      const formula = formulaMap.value.get(formulaNumber);
      const rect = range.getBoundingClientRect();
      selectionTopLeft.value = { x: rect.left, y: rect.top };
      const focusRange = range.cloneRange();
      focusRange.collapse(selection.focusNode === range.startContainer && selection.focusOffset === range.startOffset);
      const focusRect = focusRange.getBoundingClientRect();
      selectionPoint.value = pointerEnd ?? {
        x: focusRect.height ? focusRect.left : rect.right,
        y: focusRect.height ? focusRect.top + focusRect.height / 2 : rect.bottom,
      };
      popupPosition.value = { x: rect.left + rect.width / 2, top: rect.top, bottom: rect.bottom };
      selectedText.value = text;
      selectedIsFormulaNumber.value = Boolean(formulaNumber);
      selectedFormula.value = formula ? { number: formulaNumber, ...formula } : null;
      showFormulaPopup.value = true;
    }

    document.addEventListener('pointerdown', (event) => {
      if (event.target.closest('.selection-action-menu') || selectionActionPending.value) return;
      pointerDown = true;
      pointerEnd = null;
      if (!hasJumped.value) showFormulaPopup.value = false;
    }, options);
    document.addEventListener('pointerup', (event) => {
      pointerDown = false;
      if (event.button === 0 && readerContainer.value?.contains(event.target)) {
        pointerEnd = { x: event.clientX, y: event.clientY };
        updateSelection();
      }
    }, options);
    document.addEventListener('pointercancel', () => { pointerDown = false; }, options);
    document.addEventListener('selectionchange', updateSelection, options);
    document.addEventListener('keydown', (event) => {
      if (event.shiftKey && event.key.startsWith('Arrow')) pointerEnd = null;
      if (event.key === 'Escape' && !hasJumped.value && !selectionActionPending.value && !selectionInputActive.value) {
        showFormulaPopup.value = false;
      }
    }, options);
  }

  function cleanupSelectionListener() {
    selectionEvents?.abort();
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

  function dismissSelectionMenu() {
    showFormulaPopup.value = false;
    hasJumped.value = false;
    previousScrollPosition.value = null;
  }

  /**
   * 清理所有资源
   */
  function cleanup() {
    cleanupSelectionListener();
    formulaMap.value = new Map();
    selectedFormula.value = null;
    selectedText.value = '';
    showFormulaPopup.value = false;
    hasJumped.value = false;
    previousScrollPosition.value = null;
  }

  return {
    formulaMap,
    selectedFormula,
    selectedText,
    selectionPoint,
    selectionTopLeft,
    selectedIsFormulaNumber,
    selectionActionPending,
    selectionInputActive,
    showFormulaPopup,
    popupPosition,
    hasJumped,
    extractFormulas,
    setupSelectionListener,
    jumpToFormula,
    returnToPreviousPosition,
    dismissSelectionMenu,
    cleanup,
  };
}
