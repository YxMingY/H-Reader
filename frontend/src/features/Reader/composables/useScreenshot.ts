import { ref, Ref } from "vue";

/**
 * useScreenshot - PDF 页面截图功能 Composable
 * 
 * 提供两种截图方式：
 * 1. 截取当前完整页面
 * 2. 框选区域截图
 */
export function useScreenshot(
    currentPage: Ref<number>,                                    // 当前页码响应式变量
    pageCanvasRefs: Record<number, HTMLCanvasElement | undefined>, // 页面 Canvas 引用集合
) {
    /**
     * 截取当前页为图片
     * 
     * @returns {Promise<string|null>} 图片的 data URL，失败时返回 null
     */
    const captureCurrentPage = async () => {
        try {
            const pageNum = currentPage.value;
            const canvas = pageCanvasRefs[pageNum];
            
            if (!canvas) {
                console.error(`第 ${pageNum} 页的 canvas 不存在`);
                return null;
            }

            // 将 canvas 转换为 PNG 格式的 data URL
            const dataUrl = canvas.toDataURL('image/png');
            console.log(`已截取第 ${pageNum} 页`);
            return dataUrl;
        } catch (err) {
            console.error('截取当前页失败:', err);
            return null;
        }
    };

    /**
     * 开始框选区域截图
     * 
     * @returns {Promise<string|null>} 用户选择的区域的图片 data URL，取消时返回 null
     */
    const startAreaSelection = async () => {
        return new Promise((resolve) => {
            // 创建遮罩层，覆盖整个屏幕
            const overlay = document.createElement('div');
            overlay.style.cssText = `
            position: fixed;
            top: 0;
            left: 0;
            width: 100%;
            height: 100%;
            background: rgba(0, 0, 0, 0.3);
            cursor: crosshair;
            z-index: 9999;
            `;

            // 创建选择框，用于显示用户拖拽的区域
            const selectionBox = document.createElement('div');
            selectionBox.style.cssText = `
            position: absolute;
            border: 2px dashed #007acc;
            background: rgba(0, 122, 204, 0.1);
            display: none;
            pointer-events: none;
            `;
            overlay.appendChild(selectionBox);

            let startX = 0;      // 鼠标按下时的 X 坐标
            let startY = 0;      // 鼠标按下时的 Y 坐标
            let isSelecting = false; // 是否正在选择

            /**
             * 处理鼠标按下事件，开始选择区域
             */
            const handleMouseDown = (e: MouseEvent) => {
                startX = e.clientX;
                startY = e.clientY;
                isSelecting = true;
                
                selectionBox.style.left = startX + 'px';
                selectionBox.style.top = startY + 'px';
                selectionBox.style.width = '0px';
                selectionBox.style.height = '0px';
                selectionBox.style.display = 'block';
            };

            /**
             * 处理鼠标移动事件，更新选择框大小
             */
            const handleMouseMove = (e: MouseEvent) => {
                if (!isSelecting) return;

                const currentX = e.clientX;
                const currentY = e.clientY;

                const left = Math.min(startX, currentX);
                const top = Math.min(startY, currentY);
                const width = Math.abs(currentX - startX);
                const height = Math.abs(currentY - startY);

                selectionBox.style.left = left + 'px';
                selectionBox.style.top = top + 'px';
                selectionBox.style.width = width + 'px';
                selectionBox.style.height = height + 'px';
            };

            /**
             * 处理鼠标释放事件，完成区域选择并截图
             */
            const handleMouseUp = async (e: MouseEvent) => {
                if (!isSelecting) return;
                isSelecting = false;

                const endX = e.clientX;
                const endY = e.clientY;

                // 计算选择区域
                const left = Math.min(startX, endX);
                const top = Math.min(startY, endY);
                const width = Math.abs(endX - startX);
                const height = Math.abs(endY - startY);

                // 如果选择区域太小，视为取消
                if (width < 10 || height < 10) {
                    cleanup();
                    resolve(null);
                    return;
                }

                try {
                    // 获取当前可见页面的 canvas
                    const pageNum = currentPage.value;
                    const sourceCanvas = pageCanvasRefs[pageNum];

                    if (!sourceCanvas) {
                        console.error(`第 ${pageNum} 页的 canvas 不存在`);
                        cleanup();
                        resolve(null);
                        return;
                    }

                    // 计算 canvas 相对于视口的位置
                    const rect = sourceCanvas.getBoundingClientRect();
                    
                    // 计算选择区域在 canvas 中的坐标（考虑缩放）
                    const scaleX = sourceCanvas.width / rect.width;
                    const scaleY = sourceCanvas.height / rect.height;

                    const canvasX = (left - rect.left) * scaleX;
                    const canvasY = (top - rect.top) * scaleY;
                    const canvasWidth = width * scaleX;
                    const canvasHeight = height * scaleY;

                    // 创建一个新的 canvas 来裁剪选区
                    const cropCanvas = document.createElement('canvas');
                    cropCanvas.width = canvasWidth;
                    cropCanvas.height = canvasHeight;
                    const ctx = cropCanvas.getContext('2d');

                    // 从源 canvas 中裁剪选区
                    if (ctx) 
                        ctx.drawImage(
                            sourceCanvas,
                            canvasX,
                            canvasY,
                            canvasWidth,
                            canvasHeight,
                            0,
                            0,
                            canvasWidth,
                            canvasHeight
                        );

                    // 转换为 data URL
                    const dataUrl = cropCanvas.toDataURL('image/png');
                    console.log(`已截取区域: ${Math.round(width)}x${Math.round(height)}`);
                    
                    cleanup();
                    resolve(dataUrl);
                } catch (err) {
                    console.error('框选截图失败:', err);
                    cleanup();
                    resolve(null);
                }
            };

            /**
             * 处理 ESC 键按下事件，取消选择
             */
            const handleEscape = (e: KeyboardEvent) => {
                if (e.key === 'Escape') {
                    cleanup();
                    resolve(null);
                }
            };

            /**
             * 处理窗口失焦事件，确保清理
             */
            const handleWindowBlur = () => {
                cleanup();
                resolve(null);
            };

            // 使用 let 而不是 const，以便后续可以重新赋值
            let hintElement: HTMLElement | null = null;
            
            /**
             * 清理函数：移除所有事件监听器和 DOM 元素
             */
            const cleanup = () => {
                overlay.removeEventListener('mousedown', handleMouseDown);
                overlay.removeEventListener('mousemove', handleMouseMove);
                overlay.removeEventListener('mouseup', handleMouseUp);
                document.removeEventListener('keydown', handleEscape);
                window.removeEventListener('blur', handleWindowBlur);
                
                // 移除提示条
                if (hintElement && hintElement.parentNode) {
                    document.body.removeChild(hintElement);
                    hintElement = null;
                }
                
                // 移除遮罩层
                if (overlay.parentNode) {
                    document.body.removeChild(overlay);
                }
            };

            // 添加事件监听器
            overlay.addEventListener('mousedown', handleMouseDown);
            overlay.addEventListener('mousemove', handleMouseMove);
            overlay.addEventListener('mouseup', handleMouseUp);
            document.addEventListener('keydown', handleEscape);
            window.addEventListener('blur', handleWindowBlur);

            // 添加到 body
            document.body.appendChild(overlay);

            // 显示提示信息
            hintElement = document.createElement('div');
            hintElement.textContent = '拖动鼠标选择区域，按 ESC 取消';
            hintElement.style.cssText = `
            position: fixed;
            top: 20px;
            left: 50%;
            transform: translateX(-50%);
            background: rgba(0, 0, 0, 0.8);
            color: white;
            padding: 10px 20px;
            border-radius: 8px;
            font-size: 14px;
            z-index: 10000;
            pointer-events: none;
            `;
            document.body.appendChild(hintElement);
        });
    };

    return {
        captureCurrentPage,   // 截取当前页方法
        startAreaSelection,   // 开始区域选择方法
    };
}

