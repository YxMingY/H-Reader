// Reader Composables 导出

// PDF 文档管理
export { usePdfDocument } from './usePdfDocument'

// 页面渲染管理
export { usePdfPages } from './usePdfPages'

// 缩放控制
export { useScaleAdjust } from './useScaleAdjust'

// 滚动与导航
export { useScrollPage } from './useScrollPage'

// 截图功能
export { useScreenshot } from './useScreenshot'

// 阅读进度管理
export { useReadingProgress } from './useReadingProgress'

// 大头针标注管理
export { useAnnotations } from './useAnnotations'

// 公式导航
export { useFormulaNavigation } from './useFormulaNavigation'

// 页面检测工具
export { findPageByPoint, findPageBySelection } from './usePageDetection.ts'
