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
import { computed, ref, onMounted, onBeforeUnmount } from 'vue';
import AnnotationPin from './AnnotationPin.vue';
import NoteMarker from './NoteMarker.vue';

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
  notes: { type: Array, default: () => [] },
  editingNote: { type: Object, default: null },
  savingNote: Boolean,
  noteSaveError: { type: String, default: '' },
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
const emit = defineEmits(['pin-dblclick', 'pin-remove', 'note-edit', 'note-draft', 'note-contextmenu']);

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

// Both PDF markers use the same 12px icon size at a page width of 732px.
const markerSize = computed(() => 12 * Math.max(0.5, Math.min(2, props.pageWidth / 732)));

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
  <div
    ref="container"
    class="pdf-page-container"
    :data-page-num="pageNum"
    :style="{ '--pdf-marker-size': markerSize + 'px' }"
  >
    <canvas ref="canvas" class="pdf-page-canvas"></canvas>
    <div ref="textLayer" class="pdf-text-layer-host"></div>
    
    <!-- 渲染当前页的大头针 -->
    <AnnotationPin
      v-for="annotation in annotations"
      :key="annotation.id"
      :annotation="annotation"
      :x="getPinPosition(annotation).x"
      :y="getPinPosition(annotation).y"
      @dblclick="handlePinDoubleClick"
      @remove="handlePinRemove"
    />
    <NoteMarker
      v-for="note in notes"
      :key="note.id"
      :note="note"
      :editing="editingNote?.note.id === note.id"
      :draft="editingNote?.text"
      :saving="savingNote"
      :error="noteSaveError"
      @edit="emit('note-edit', $event)"
      @update:draft="emit('note-draft', $event)"
      @contextmenu="(event, item) => emit('note-contextmenu', event, item)"
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

.pdf-page-container :deep(.pdf-page-marker) {
  transition: filter 0.2s ease;
}

.pdf-page-container :deep(.pdf-page-marker:hover) {
  filter: brightness(1.2) drop-shadow(0 4px 8px rgba(0, 0, 0, 0.4));
}

.pdf-page-container :deep(.pdf-marker-icon) {
  display: block;
  font-size: var(--pdf-marker-size);
  filter: drop-shadow(0 2px 4px rgba(0, 0, 0, 0.3));
  user-select: none;
  opacity: 0.7;
}

.pdf-page-canvas {
  display: block;
  z-index: 1;
}

.pdf-text-layer-host {
  position: absolute;
  inset: 0;
  z-index: 2;
}

.pdf-text-layer-host :deep(.textLayer) {
  position: absolute;
  left: 0;
  top: 0;
  width: 100%;
  height: 100%;
  pointer-events: auto;
  user-select: text;
  -webkit-user-select: text;
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
  --scale-round-x: 1px;
  --scale-round-y: 1px;
  --min-font-size: 1;
  --text-scale-factor: calc(var(--total-scale-factor, 1) * var(--min-font-size));
  --min-font-size-inv: calc(1 / var(--min-font-size));
}

.pdf-text-layer-host :deep(.textLayer :is(span, br)) {
  color: transparent;
  position: absolute;
  white-space: pre;
  cursor: text;
  transform-origin: 0% 0%;
  user-select: text;
  -webkit-user-select: text;
}

.pdf-text-layer-host :deep(.textLayer > :not(.markedContent, .endOfContent)),
.pdf-text-layer-host :deep(.textLayer .markedContent span:not(.markedContent)) {
  z-index: 1;
  --font-height: 0;
  font-size: calc(var(--text-scale-factor) * var(--font-height));
  --scale-x: 1;
  --rotate: 0deg;
  transform: rotate(var(--rotate)) scaleX(var(--scale-x)) scale(var(--min-font-size-inv));
}

.pdf-text-layer-host :deep(.markedContent) {
  display: contents;
}

.pdf-text-layer-host :deep(.textLayer ::selection) {
  background: rgba(54, 122, 255, 0.32);
}

.pdf-text-layer-host :deep(.textLayer br::selection) {
  background: transparent;
}

.pdf-text-layer-host :deep(.endOfContent) {
  position: absolute;
  inset: 100% 0 0;
  z-index: 0;
  -webkit-user-select: none;
  user-select: none;
}

.pdf-text-layer-host :deep(.textLayer.selecting .endOfContent) {
  top: 0;
}

.pdf-text-layer-host :deep([data-main-rotation="90"]) {
  transform: rotate(90deg) translateY(-100%);
}

.pdf-text-layer-host :deep([data-main-rotation="180"]) {
  transform: rotate(180deg) translate(-100%, -100%);
}

.pdf-text-layer-host :deep([data-main-rotation="270"]) {
  transform: rotate(270deg) translateX(-100%);
}
</style>