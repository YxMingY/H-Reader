import { ref, computed } from 'vue';
import { BookService, AnnotationService } from '../../../../bindings/hreader/services/book';
import { v4 as uuidv4 } from 'uuid';

/**
 * useAnnotations - PDF 大头针管理 Composable
 * 
 * 负责管理 PDF 页面上的标注（大头针），包括：
 * - 加载和保存大头针数据
 * - 添加、删除、更新大头针
 * - 获取当前页的大头针列表
 * - 双击导航到对应的 AI 对话
 */
export function useAnnotations(bookPathRef, currentPage, emitNavigateToMessage) {
    const annotations = ref([]); // 所有大头针列表
    const isSelectingMode = ref(false); // 是否处于选择模式（等待用户点击 PDF）
    let pendingAnnotationData = null; // 待创建的大头针数据（在选择模式下暂存）

    /**
     * 加载当前 PDF 的所有大头针
     */
    const loadAnnotations = async () => {
        if (!bookPathRef.value) {
            annotations.value = [];
            return;
        }

        try {
            const result = await AnnotationService.GetAnnotations(bookPathRef.value);
            annotations.value = result || [];
            console.log(`加载了 ${annotations.value.length} 个大头针`);
        } catch (err) {
            console.error('加载大头针失败:', err);
            annotations.value = [];
        }
    };

    /**
     * 添加大头针
     * @param {number} pageNum - 页码
     * @param {number} x - X 坐标（像素，相对于页面）
     * @param {number} y - Y 坐标（像素，相对于页面）
     * @param {string} sessionId - 会话 ID
     * @param {number} messageIndex - 消息在会话中的索引位置
     * @param {number} pageWidth - 当前页面宽度（用于归一化）
     * @param {number} pageHeight - 当前页面高度（用于归一化）
     * @param {string} messagePreview - 消息内容预览（可选）
     */
    const addAnnotation = async (pageNum, x, y, sessionId, messageIndex, pageWidth, pageHeight, messagePreview = '') => {
        if (!bookPathRef.value) {
            console.warn('无法添加大头针：bookPath 为空');
            return;
        }

        // 归一化坐标（0-1 范围）
        const normalizedX = x / pageWidth;
        const normalizedY = y / pageHeight;

        const annotation = {
            id: uuidv4(),
            sessionId,
            messageIndex,
            pageNum,
            x: normalizedX,
            y: normalizedY,
            createdAt: Date.now(),
            note: messagePreview || '' // 使用消息预览作为备注
        };

        try {
            await AnnotationService.AddAnnotation(bookPathRef.value, annotation);
            annotations.value.push(annotation);
            console.log('大头针添加成功:', annotation.id);
            return annotation;
        } catch (err) {
            console.error('添加大头针失败:', err);
            throw err;
        }
    };

    /**
     * 删除大头针
     * @param {string} annotationId - 大头针 ID
     */
    const removeAnnotation = async (annotationId) => {
        if (!bookPathRef.value) return;

        try {
            await AnnotationService.RemoveAnnotation(bookPathRef.value, annotationId);
            annotations.value = annotations.value.filter(a => a.id !== annotationId);
            console.log('大头针删除成功:', annotationId);
        } catch (err) {
            console.error('删除大头针失败:', err);
            throw err;
        }
    };

    /**
     * 获取当前页的大头针列表
     */
    const getCurrentPageAnnotations = computed(() => {
        if (!currentPage.value) return [];
        return annotations.value.filter(a => a.pageNum === currentPage.value);
    });

    /**
     * 将归一化坐标转换为实际像素坐标
     * @param {number} normalizedX - 归一化 X (0-1)
     * @param {number} normalizedY - 归一化 Y (0-1)
     * @param {number} pageWidth - 当前页面宽度
     * @param {number} pageHeight - 当前页面高度
     * @returns {Object} { x, y } 实际像素坐标
     */
    const convertToPixelCoords = (normalizedX, normalizedY, pageWidth, pageHeight) => {
        return {
            x: normalizedX * pageWidth,
            y: normalizedY * pageHeight
        };
    };

    /**
     * 开始选择模式（等待用户点击 PDF）
     * @param {string} sessionId - 会话 ID
     * @param {number} messageIndex - 消息索引
     * @param {string} messagePreview - 消息内容预览（可选）
     */
    const startSelectionMode = (sessionId, messageIndex, messagePreview = '') => {
        isSelectingMode.value = true;
        pendingAnnotationData = { sessionId, messageIndex, messagePreview };
        console.log('进入大头针选择模式，请点击 PDF 页面');
    };

    /**
     * 取消选择模式
     */
    const cancelSelectionMode = () => {
        isSelectingMode.value = false;
        pendingAnnotationData = null;
    };

    /**
     * 处理 PDF 页面点击（在选择模式下创建大头针）
     * @param {number} pageNum - 页码
     * @param {number} clickX - 点击的 X 坐标（像素）
     * @param {number} clickY - 点击的 Y 坐标（像素）
     * @param {number} pageWidth - 页面宽度
     * @param {number} pageHeight - 页面高度
     */
    const handlePageClick = async (pageNum, clickX, clickY, pageWidth, pageHeight) => {
        if (!isSelectingMode.value || !pendingAnnotationData) {
            return false; // 不在选择模式下
        }

        try {
            const annotation = await addAnnotation(
                pageNum,
                clickX,
                clickY,
                pendingAnnotationData.sessionId,
                pendingAnnotationData.messageIndex,
                pageWidth,
                pageHeight,
                pendingAnnotationData.messagePreview // 传递消息预览
            );

            // 退出选择模式
            cancelSelectionMode();
            return true;
        } catch (err) {
            console.error('创建大头针失败:', err);
            cancelSelectionMode();
            return false;
        }
    };

    /**
     * 双击大头针，导航到对应的对话
     * @param {Object} annotation - 大头针对象
     */
    const handlePinDoubleClick = (annotation) => {
        console.log('双击大头针，导航到:', annotation.sessionId, annotation.messageIndex);
        
        // 通过 emit 通知父组件进行导航
        if (emitNavigateToMessage) {
            emitNavigateToMessage({
                sessionId: annotation.sessionId,
                messageIndex: annotation.messageIndex
            });
        }
    };

    return {
        annotations,
        isSelectingMode,
        getCurrentPageAnnotations,
        loadAnnotations,
        addAnnotation,
        removeAnnotation,
        startSelectionMode,
        cancelSelectionMode,
        handlePageClick,
        handlePinDoubleClick,
        convertToPixelCoords
    };
}
