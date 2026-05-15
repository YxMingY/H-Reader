import { ref, onMounted, onBeforeUnmount } from "vue";

export function useScaleAdjust(
    RerenderVisiblePages,
    readerContainer,
    pageWidth,
    emit,
) {
    const MIN_PAGE_WIDTH = 200;
    const MAX_PAGE_WIDTH = 1200;
    const ZOOM_STEP = 1.1;

    let resizeTimeout = null;
    let wheelTimeout = null;
    const SetPageWidth = (nextWidth) => {
        // 所有缩放路径都收敛到这里：先夹紧，再重渲染可见页，最后通知外部当前缩放结果。
        const clamped = Math.min(Math.max(nextWidth, MIN_PAGE_WIDTH), MAX_PAGE_WIDTH);
        if (clamped === pageWidth.value) return;
        pageWidth.value = clamped;
        RerenderVisiblePages();
        // emit('rescale', clamped);
    };


    const ZoomIn = () => {
        // 放大按钮与滚轮向上共用同一套缩放步长。
        SetPageWidth(pageWidth.value * ZOOM_STEP);
    };

    const ZoomOut = () => {
        // 缩小按钮与滚轮向下共用同一套缩放步长。
        SetPageWidth(pageWidth.value / ZOOM_STEP);
    };

    // 根据窗口大小自适应画布宽度
    const FitWidth = () => {
        if (!readerContainer.value) return;
        console.log("FitWidth");
        // fitWidth 只负责把“页面宽度”贴合当前阅读器容器，页面高度仍由每页 viewport 动态计算。
        const containerWidth = readerContainer.value.clientWidth - 40;
        console.log("containerWidth", containerWidth);
        SetPageWidth(containerWidth);
    };

    // 处理 Ctrl+滚轮缩放
    const HandleWheel = (event) => {
        // 检测 Ctrl 或 Cmd 键
        if (!event.ctrlKey && !event.metaKey) return;
        
        event.preventDefault();
        
        // 节流：避免频繁重新渲染
        if (wheelTimeout) return;
        
        wheelTimeout = setTimeout(() => {
            wheelTimeout = null;
        }, 100);
        
        if (event.deltaY < 0) {
            ZoomIn();
        } else {
            ZoomOut();
        }
    };

    // 处理窗口大小改变
    const HandleResize = () => {
        if (resizeTimeout) clearTimeout(resizeTimeout);
        
        resizeTimeout = setTimeout(() => {
            //if (props.pdfSource && pdfDoc.value) {
                FitWidth();
            //}
        }, 200);
    };

    onMounted(() => { 
        window.addEventListener('resize', HandleResize);
        readerContainer.value?.addEventListener('wheel', HandleWheel, { passive: false });
    });

    onBeforeUnmount(() => {
        window.removeEventListener('resize', HandleResize);
        readerContainer.value?.removeEventListener('wheel', HandleWheel);
        if (resizeTimeout) clearTimeout(resizeTimeout);
        if (wheelTimeout) clearTimeout(wheelTimeout);
    });

    return {
        ZoomIn,
        ZoomOut,
        FitWidth,
        HandleWheel,
        HandleResize,
    }
};