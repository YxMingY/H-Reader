import { ref, onMounted } from 'vue';
import { BookService } from '../../../../bindings/hreader';

/**
 * useLibrary - 图书库管理 Composable
 * 
 * 负责管理 PDF 图书库，包括：
 * - 扫描目录中的 PDF 文件
 * - 选择扫描目录
 * - 加载和刷新图书列表
 */
export function useLibrary() {
    const books = ref([]);        // 图书列表
    const loading = ref(false);   // 加载状态
    const scanDir = ref("");      // 当前扫描目录

    /**
     * 初始化：获取扫描目录并加载图书
     */
    const initLibrary = async () => {
        try {
            scanDir.value = await BookService.GetScanDir();
            console.log("扫描目录:", scanDir.value);
            await loadLibrary();
        } catch (err) {
            console.error("初始化图书库失败", err);
        }
    };

    /**
     * 选择扫描目录
     * @returns {Promise<void>}
     */
    const chooseDir = async () => {
        try {
            const selectedDir = await BookService.ChooseDir();
            if (selectedDir) {
                scanDir.value = selectedDir;
                console.log("选择的文件夹:", scanDir.value);
                await loadLibrary(scanDir.value);
            }
        } catch (err) {
            console.error("选择文件夹失败", err);
            throw err;
        }
    };

    /**
     * 加载图书列表
     * @param {string} dir - 可选的目录路径，不传则使用当前 scanDir
     * @returns {Promise<void>}
     */
    const loadLibrary = async (dir) => {
        loading.value = true;
        try {
            const result = await BookService.ScanBooks(dir);
            books.value = result || [];
            console.log("扫描结果:", books.value);
        } catch (err) {
            console.error("扫描失败", err);
            books.value = [];
            throw err;
        } finally {
            loading.value = false;
        }
    };

    /**
     * 刷新图书列表（重新扫描当前目录）
     * @returns {Promise<void>}
     */
    const refreshLibrary = async () => {
        await loadLibrary(scanDir.value);
    };

    // 组件挂载时自动初始化
    onMounted(() => {
        initLibrary();
    });

    return {
        books,          // 图书列表
        loading,        // 加载状态
        scanDir,        // 扫描目录
        chooseDir,      // 选择目录方法
        loadLibrary,    // 加载图书方法
        refreshLibrary, // 刷新图书方法
    };
}
