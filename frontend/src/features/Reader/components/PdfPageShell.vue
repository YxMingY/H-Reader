<script setup>
/**
 * PdfPageShell.vue - PDF 页面容器组件
 * 
 * 该组件负责创建单个 PDF 页面的 DOM 结构，包括：
 * - Canvas 元素用于渲染 PDF 内容
 * - 文本层用于支持文本选择和复制
 * - 向父组件注册 DOM 引用以便管理
 */
import { ref, onMounted, onBeforeUnmount } from 'vue';

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
});

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
</script>

<template>
  <div ref="container" class="pdf-page-container" :data-page-num="pageNum">
    <canvas ref="canvas" class="pdf-page-canvas"></canvas>
    <div ref="textLayer" class="pdf-text-layer"></div>
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

.pdf-text-layer {
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
}

.pdf-text-layer span {
  position: absolute;
  white-space: pre;
  transform-origin: 0 0;
  line-height: 1;
  user-select: text;
  -webkit-user-select: text;
  cursor: text;
}

</style>