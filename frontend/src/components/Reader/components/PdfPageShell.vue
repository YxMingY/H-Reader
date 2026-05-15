<script setup>
import { ref, onMounted, onBeforeUnmount } from 'vue';

const props = defineProps({
  pageNum: {
    type: Number,
    required: true,
  },
  register: {
    type: Function,
    required: true,
  },
});

const container = ref(null);
const canvas = ref(null);
const textLayer = ref(null);

onMounted(() => {
  props.register(props.pageNum, {
    container: container.value,
    canvas: canvas.value,
    textLayer: textLayer.value,
  });
});

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