/**
 * usePageDetection.ts - 页面检测工具 Composable
 * 
 * 提供根据坐标或点击事件判断目标所在页面的功能
 */

interface PageInfo {
    pageNum: number;
    canvas: HTMLCanvasElement;
    rect: DOMRect;
}

/**
 * 根据屏幕坐标查找对应的页面
 * 
 * @param clientX - 鼠标 X 坐标（屏幕坐标）
 * @param clientY - 鼠标 Y 坐标（屏幕坐标）
 * @param pageCanvasRefs - 页面 Canvas 引用集合
 * @param totalPages - 总页数
 * @returns 包含 pageNum, canvas, rect 的对象，未找到返回 null
 */
export function findPageByPoint(
    clientX: number,
    clientY: number,
    pageCanvasRefs: Record<number, HTMLCanvasElement | undefined>,
    totalPages: number
): PageInfo | null {
    for (let pageNum = 1; pageNum <= totalPages; pageNum++) {
        const canvas = pageCanvasRefs[pageNum];
        if (!canvas) continue;
        
        const rect = canvas.getBoundingClientRect();
        
        // 检查坐标是否在这个 canvas 范围内
        if (clientX >= rect.left && clientX <= rect.right &&
            clientY >= rect.top && clientY <= rect.bottom) {
            return {
                pageNum,
                canvas,
                rect
            };
        }
    }
    
    return null;
}

/**
 * 根据选择区域查找对应的页面（使用区域中心点）
 * 
 * @param left - 选择区域左边界（屏幕坐标）
 * @param top - 选择区域上边界（屏幕坐标）
 * @param width - 选择区域宽度
 * @param height - 选择区域高度
 * @param pageCanvasRefs - 页面 Canvas 引用集合
 * @param totalPages - 总页数
 * @returns 包含 pageNum, canvas, rect 的对象，未找到返回 null
 */
export function findPageBySelection(
    left: number,
    top: number,
    width: number,
    height: number,
    pageCanvasRefs: Record<number, HTMLCanvasElement | undefined>,
    totalPages: number
): PageInfo | null {
    // 计算选择区域的中心点
    const centerX = left + width / 2;
    const centerY = top + height / 2;
    
    return findPageByPoint(centerX, centerY, pageCanvasRefs, totalPages);
}
