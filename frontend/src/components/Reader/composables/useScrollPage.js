import { ref, onMounted, onBeforeUnmount } from "vue";
export function useScrollPage({ 
    readerContainer, totalPages, pdfCanvas, pageContainerRefs, emit 
}) {
    const currentPage = ref(1); // 当前显示的页码（用于翻页）

    let scrollTimeout = null;
    let lastScrollTop = 0;

    const ResetLastScrollTop = () => {
        lastScrollTop = 0;
    };

  // 更新当前页码并通知外部：这是页码状态的唯一入口，避免多个地方各自改 currentPage
    const setCurrentPage = (pageNum, notify = true) => {
        const nextPage = Math.min(Math.max(pageNum, 1), totalPages.value || 1);

        if (nextPage === currentPage.value) return;

        currentPage.value = nextPage;

        if (notify) {
            emit('pagechange', nextPage);
        }
    };

    // 根据滚动方向和页边界计算当前页码：向下时等上一页完全消失再切到下一页；向上时同理
    const updateCurrentPageFromScroll = () => {
        if (!readerContainer.value || !pdfCanvas.value) return;

        const scrollTop = readerContainer.value.scrollTop;
        const maxScrollTop = Math.max(readerContainer.value.scrollHeight - readerContainer.value.clientHeight, 0);
        const containerRect = readerContainer.value.getBoundingClientRect();
        const scrollingDown = scrollTop > lastScrollTop;
        const scrollingUp = scrollTop < lastScrollTop;

        lastScrollTop = scrollTop;

        // 向下滚动时，只有当前页完全离开视口上边界，才进入下一页
        if (scrollingDown) {
            // 最后一页没有“下一页”可等它完全消失，所以滚动到底部时直接切到最后一页
            if (scrollTop >= maxScrollTop - 1) {
            setCurrentPage(totalPages.value);
            return;
            }

            while (currentPage.value < totalPages.value) {
                // 用 Vue refs 而不是 querySelector 获取当前页容器
                const currentContainer = pageContainerRefs[currentPage.value];
                if (!currentContainer) break;

                const currentRect = currentContainer.getBoundingClientRect();
                if (currentRect.bottom <= containerRect.top + 1) {
                    setCurrentPage(currentPage.value + 1);
                } else {
                    break;
                }
            }

            return;
        }
        // 向上滚动时，只有当前页完全离开视口下边界，才回到上一页
        if (scrollingUp) {
            // 第一页没有“上一页”可等它完全消失，所以滚动到顶部时直接切回第一页
            if (scrollTop <= 0) {
                setCurrentPage(1);
                return;
            }

            while (currentPage.value > 1) {
                // 用 Vue refs 而不是 querySelector 获取当前页容器
                const currentContainer = pageContainerRefs[currentPage.value];
                if (!currentContainer) break;

                const currentRect = currentContainer.getBoundingClientRect();
                if (currentRect.top >= containerRect.bottom - 1) {
                    setCurrentPage(currentPage.value - 1);
                } else {
                    break;
                }
            }
        }
    };

    const handleScroll = () => {
        // 滚动事件很密集，先取消上一帧的计算，把页码判断压到下一帧执行
        if (scrollTimeout) cancelAnimationFrame(scrollTimeout);
        scrollTimeout = requestAnimationFrame(updateCurrentPageFromScroll);
    };

    // 滚动到指定页面
    const GoToPage = (pageNum) => {
        if (pageNum < 1 || pageNum > totalPages.value) return;
        
        // 用 Vue refs 而不是 querySelector 获取页面容器
        const pageContainer = pageContainerRefs[pageNum];
        if (pageContainer && readerContainer.value) {
            // 翻页接口直接滚动到目标容器；页码显示交给 setCurrentPage 和滚动回调保持一致
            pageContainer.scrollIntoView({ behavior: 'smooth', block: 'start' });
            setCurrentPage(pageNum);
        }
    };

    // 翻到下一页
    const GoToNextPage = () => {
        if (currentPage.value < totalPages.value) {
            GoToPage(currentPage.value + 1);
        }
    };

    // 翻到上一页
    const GoToPrevPage = () => {
        if (currentPage.value > 1) {
            GoToPage(currentPage.value - 1);
        }
    };

    onMounted(() => {
        readerContainer.value?.addEventListener('scroll', handleScroll, { passive: true });
    });

    
    onBeforeUnmount(() => {
        readerContainer.value?.removeEventListener('scroll', handleScroll);
        if (scrollTimeout) cancelAnimationFrame(scrollTimeout);
    });

    return {
        currentPage,
        GoToPage,
        GoToNextPage,
        GoToPrevPage,
        ResetLastScrollTop,
    };
}