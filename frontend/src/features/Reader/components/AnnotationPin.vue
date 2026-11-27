<script setup>
/**
 * AnnotationPin.vue - PDF 大头针组件
 * 
 * 该组件负责在 PDF 页面上显示一个大头针图标，支持：
 * - 双击导航到对应的 AI 对话
 * - 右键菜单删除大头针
 * - Hover 显示提示信息
 */
import { nextTick, onBeforeUnmount, ref } from 'vue';
import { confirmDelete } from '../../../composables/useToast';

// 定义组件属性
const props = defineProps({
    annotation: {
        type: Object,
        required: true, // 大头针数据对象
    },
    x: {
        type: Number,
        required: true, // X 坐标（像素）
    },
    y: {
        type: Number,
        required: true, // Y 坐标（像素）
    },
});

// 定义组件事件
const emit = defineEmits(['dblclick', 'remove']);

const pin = ref(null);
const tooltip = ref(null);
const tooltipPosition = ref(null);
const isHovered = ref(false);

function placeTooltip() {
    if (!pin.value || !tooltip.value) return;
    const anchor = pin.value.getBoundingClientRect();
    const { offsetWidth: width, offsetHeight: height } = tooltip.value;
    const gap = 8;
    const above = anchor.top - height - gap;
    const top = above >= gap ? above : anchor.bottom + gap;
    const left = anchor.left + (anchor.width - width) / 2;
    tooltipPosition.value = {
        top: `${Math.max(gap, Math.min(top, window.innerHeight - height - gap))}px`,
        left: `${Math.max(gap, Math.min(left, window.innerWidth - width - gap))}px`,
    };
}

async function showTooltip() {
    isHovered.value = true;
    window.addEventListener('resize', placeTooltip);
    window.addEventListener('scroll', placeTooltip, true);
    await nextTick();
    placeTooltip();
}

function hideTooltip() {
    isHovered.value = false;
    tooltipPosition.value = null;
    window.removeEventListener('resize', placeTooltip);
    window.removeEventListener('scroll', placeTooltip, true);
}

onBeforeUnmount(hideTooltip);

/**
 * 处理双击事件
 */
const handleDoubleClick = (event) => {
    event.stopPropagation();
    emit('dblclick', props.annotation);
};

/**
 * 处理右键菜单
 */
const handleContextMenu = async (event) => {
    event.preventDefault();
    event.stopPropagation();
    
    const confirmed = await confirmDelete('这个大头针');
    if (confirmed) {
        emit('remove', props.annotation.id);
    }
};
</script>

<template>
    <div
        ref="pin"
        class="annotation-pin pdf-page-marker"
        :style="{ 
            left: x + 'px', 
            top: y + 'px'
        }"
        @dblclick="handleDoubleClick"
        @contextmenu="handleContextMenu"
        @mouseenter="showTooltip"
        @mouseleave="hideTooltip"
    >
        <!-- 大头针图标 -->
        <div class="pdf-marker-icon">📌</div>

    </div>
    <Teleport to="body">
        <div v-if="isHovered" ref="tooltip" class="pin-tooltip" :style="tooltipPosition || { visibility: 'hidden' }">
            <div class="tooltip-title">{{ annotation.note || 'AI 对话标记' }}</div>
            <div class="tooltip-info">双击跳转到对话</div>
            <div class="tooltip-info">右键删除标记</div>
        </div>
    </Teleport>
</template>

<style scoped>
.annotation-pin {
    position: absolute;
    z-index: 100;
    cursor: pointer;
    transform: translate(-50%, -50%); /* 居中定位 */
}

.pin-tooltip {
    position: fixed;
    z-index: 10000;
    box-sizing: border-box;
    width: max-content;
    max-width: min(280px, calc(100vw - 16px));
    max-height: calc(100vh - 16px);
    overflow: hidden;
    padding: 8px 12px;
    border: 1px solid #d8e0e9;
    border-radius: 8px;
    background: #fff;
    color: #202731;
    font-size: 12px;
    overflow-wrap: anywhere;
    pointer-events: none;
    box-shadow: 0 4px 16px rgb(0 0 0 / 14%);
}

.tooltip-title {
    display: -webkit-box;
    overflow: hidden;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 5;
    white-space: pre-line;
    font-weight: 600;
    margin-bottom: 4px;
}

.tooltip-info {
    color: #526579;
    font-size: 11px;
}
</style>
