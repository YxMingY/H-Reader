import { ref, onMounted, onBeforeUnmount } from "vue";

/**
 * useScaleAdjust - 页面缩放管理 Composable
 * 
 * 负责处理 PDF 页面的缩放功能，包括：
 * - 放大/缩小控制
 * - 适应宽度
 * - 滚轮缩放
 * - 窗口大小调整
 */
export function useScaleAdjust(
    RerenderVisiblePages, // 重新渲染可见页面的函数
    readerContainer,      // 阅读器容器元素
    pageWidth,            // 页面宽度响应式变量
    emit,                 // 事件发射器
) {
    const MIN_PAGE_WIDTH = 200;   // 最小页面宽度（像素）
    const MAX_PAGE_WIDTH = 1200;  // 最大页面宽度（像素）
    const ZOOM_STEP = 1.1;        // 缩放步长（10%）

    let resizeTimeout = null;  // 窗口调整节流定时器
    let wheelTimeout = null;   // 滚轮事件节流定时器

    /**
     * 设置页面宽度（带范围限制）
     * @param {number} nextWidth - 新的页面宽度
     */
    const SetPageWidth = (nextWidth) => {
        // 所有缩放路径都收敛到这里：先夹紧，再重渲染可见页，最后通知外部当前缩放结果。
        const clamped = Math.min(Math.max(nextWidth, MIN_PAGE_WIDTH), MAX_PAGE_WIDTH);
        if (clamped === pageWidth.value) return;
        pageWidth.value = clamped;
        RerenderVisiblePages();
        // emit('rescale', clamped);
    };


    /**
     * 放大页面
     */
    const ZoomIn = () => {
        // 放大按钮与滚轮向上共用同一套缩放步长。
        SetPageWidth(pageWidth.value * ZOOM_STEP);
    };
    
    /**
     * 缩小页面
     */
    const ZoomOut = () => {
        // 缩小按钮与滚轮向下共用同一套缩放步长。
        SetPageWidth(pageWidth.value / ZOOM_STEP);
    };
    
    /**
     * 根据窗口大小自适应画布宽度
     */
    const FitWidth = () => {
        if (!readerContainer.value) return;
        console.log("FitWidth");
        // fitWidth 只负责把"页面宽度"贴合当前阅读器容器，页面高度仍由每页 viewport 动态计算。
        const containerWidth = readerContainer.value.clientWidth - 40;
        console.log("containerWidth", containerWidth);
        SetPageWidth(containerWidth);
    };

    /**
     * 处理 Ctrl+滚轮缩放
     * @param {WheelEvent} event - 滚轮事件对象
     */
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
            ZoomIn();  // 向上滚动，放大
        } else {
            ZoomOut(); // 向下滚动，缩小
        }
    };

    /**
     * 处理窗口大小改变
     */
    const HandleResize = () => {
        if (resizeTimeout) clearTimeout(resizeTimeout);
        
        // 使用节流避免频繁调整
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