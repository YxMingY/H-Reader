<script setup>
/**
 * PdfPageShell.vue - PDF 页面容器组件
 * 
 * 该组件负责创建单个 PDF 页面的 DOM 结构，包括：
 * - Canvas 元素用于渲染 PDF 内容
 * - 文本层用于支持文本选择和复制
 * - 大头针标注显示
 * - 向父组件注册 DOM 引用以便管理
 */
import { ref, onMounted, onBeforeUnmount } from 'vue';
import AnnotationPin from './AnnotationPin.vue';

// 定义组件属性
const props = defineProps({
  pageNum: {
    type: Number,
    required: true, // 页码，必须提供
  },
  register: {
    type: Function,
    required: true, // 注册函数，用于向父组件注册 DOM 引用
  },
  annotations: {
    type: Array,
    default: () => [], // 当前页的大头针列表
  },
  pageWidth: {
    type: Number,
    default: 600, // 页面宽度
  },
  pageHeight: {
    type: Number,
    default: 840, // 页面高度（默认 A4 比例）
  },
});

// 定义组件事件
const emit = defineEmits(['pin-dblclick', 'pin-remove']);

// DOM 元素引用
const container = ref(null); // 页面容器元素
const canvas = ref(null);    // Canvas 元素，用于渲染 PDF 页面
const textLayer = ref(null); // 文本层元素，用于文本选择

// 组件挂载时注册 DOM 引用
onMounted(() => {
  props.register(props.pageNum, {
    container: container.value,
    canvas: canvas.value,
    textLayer: textLayer.value,
  });
});

// 组件卸载时清除引用
onBeforeUnmount(() => {
  props.register(props.pageNum, null);
});

/**
 * 计算大头针的实际像素坐标
 */
const getPinPosition = (annotation) => {
  // 直接使用 props.pageWidth 和 props.pageHeight
  // 这两个值是响应式的，会随缩放正确更新
  const pixelX = annotation.x * props.pageWidth;
  const pixelY = annotation.y * props.pageHeight;
  
  
  return {
    x: pixelX,
    y: pixelY
  };
};

/**
 * 计算大头针图标的缩放比例
 * 基于当前页面宽度与标准宽度（732px）的比例
 */
const getPinScale = () => {
  const standardWidth = 732; // 标准页面宽度（首次渲染时的宽度）
  const scale = Math.max(0.5, Math.min(2.0, props.pageWidth / standardWidth));
  return scale;
};

/**
 * 处理大头针双击
 */
const handlePinDoubleClick = (annotation) => {
  emit('pin-dblclick', annotation);
};

/**
 * 处理大头针删除
 */
const handlePinRemove = (annotationId) => {
  emit('pin-remove', annotationId);
};
</script>

<template>
  <div ref="container" class="pdf-page-container" :data-page-num="pageNum">
    <canvas ref="canvas" class="pdf-page-canvas"></canvas>
    <div ref="textLayer" class="textLayer"></div>
    
    <!-- 渲染当前页的大头针 -->
    <AnnotationPin
      v-for="annotation in annotations"
      :key="annotation.id"
      :annotation="annotation"
      :x="getPinPosition(annotation).x"
      :y="getPinPosition(annotation).y"
      :scale="getPinScale()"
      @dblclick="handlePinDoubleClick"
      @remove="handlePinRemove"
    />
  </div>
</template>

<style scoped>
  
.pdf-page-container {
  background: white;
  margin-bottom: 20px;
  box-shadow: 0 4px 12px rgba(0,0,0,0.3);
  width: fit-content;
  
  position: relative;
  left: 0;
  top: 0;
  right: 0;
  bottom: 0;
}

.pdf-page-canvas {
  display: block;
  z-index: 1;
}

.textLayer {
  position: absolute;
  left: 0;
  top: 0;
  width: 100%;
  height: 100%;
  pointer-events: auto;
  user-select: text;
  -webkit-user-select: text;
  z-index: 2;
  color: transparent;
  /* PDF.js TextLayer 必需样式 */
  --csstools-color-scheme--light: initial;
  color-scheme: only light;
  text-align: initial;
  inset: 0;
  overflow: clip;
  opacity: 1;
  line-height: 1;
  -webkit-text-size-adjust: none;
  -moz-text-size-adjust: none;
  text-size-adjust: none;
  forced-color-adjust: none;
  transform-origin: 0 0;
  caret-color: CanvasText;
  /* PDF.js TextLayer 字体缩放变量 */
  --min-font-size: 1;
  --text-scale-factor: calc(var(--total-scale-factor, 1) * var(--min-font-size));
  --min-font-size-inv: calc(1 / var(--min-font-size));
}

.textLayer :is(span, br) {
  color: transparent;
  position: absolute;
  white-space: pre;
  cursor: text;
  transform-origin: 0% 0%;
  user-select: text;
  -webkit-user-select: text;
  /* PDF.js TextLayer 字体大小控制 */
  z-index: 1;
  --font-height: 0;
  font-size: calc(var(--text-scale-factor) * var(--font-height));
  --scale-x: 1;
  --rotate: 0deg;
  transform: rotate(var(--rotate)) scaleX(var(--scale-x)) scale(var(--min-font-size-inv));
}

</style>