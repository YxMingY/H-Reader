import { ref, markRaw } from 'vue';
import * as pdfjsLib from 'pdfjs-dist';

/**
 * usePdfDocument - PDF 文档管理 Composable
 * 
 * 负责加载和解析 PDF 文件，提供文档信息和页面数量。
 * 使用 markRaw 包装 PDF 文档对象以避免 Vue 的响应式系统处理大型对象。
 */

// 配置 Worker（使用本地文件避免CORS问题）
pdfjsLib.GlobalWorkerOptions.workerSrc = '/pdf.worker.min.js';

/**
 * 从给定路径调用pdfjsLib加载PDF文档，并将其存储在响应式变量中，以便在组件中使用。
 * @returns {Object} 包含 pdfDoc、totalPages 和 LoadPdfDocument 的对象
 */
export function usePdfDocument() {
    const pdfDoc = ref(null);      // PDF 文档对象引用
    const totalPages = ref(0);     // PDF 总页数
    
    /**
     * 加载 PDF 文档
     * @param {string} pdfPath - PDF 文件路径或 URL
     */
    const LoadPdfDocument = async (pdfPath) => {
        // 使用 pdfjsLib 加载 PDF 文档
        const pdf = await pdfjsLib.getDocument(pdfPath).promise;
        // 使用 markRaw 包装 PDF 文档对象，避免 Vue 响应式系统处理大型对象
        pdfDoc.value = markRaw(pdf);
        // 设置总页数，将会触发组件中对 totalPages 的响应式更新，生成页面容器
        totalPages.value = pdf.numPages; 
        console.log(`PDF 读取成功，共 ${pdf.numPages} 页`);
    };

    return {
        pdfDoc,           // PDF 文档对象
        totalPages,       // 总页数
        LoadPdfDocument,  // 加载文档方法
    };
}
