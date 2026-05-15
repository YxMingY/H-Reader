import { ref, markRaw } from 'vue';
import * as pdfjsLib from 'pdfjs-dist';

// 配置 Worker（使用本地文件避免CORS问题）
pdfjsLib.GlobalWorkerOptions.workerSrc = '/pdf.worker.min.js';

// 从给定路径调用pdfjsLib加载PDF文档，并将其存储在响应式变量中，以便在组件中使用。
export function usePdfDocument() {
    const pdfDoc = ref(null);
    const totalPages = ref(0);
    
    const LoadPdfDocument = async (pdfPath) => {
        const pdf = await pdfjsLib.getDocument(pdfPath).promise;
        pdfDoc.value = markRaw(pdf);
        //将会触发组件中对 totalPages 的响应式更新，生成页面容器
        totalPages.value = pdf.numPages; 
        console.log(`PDF 读取成功，共 ${pdf.numPages} 页`);
    };

    return {
        pdfDoc,
        totalPages,
        LoadPdfDocument,
    };
}
