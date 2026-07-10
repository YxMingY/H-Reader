<script setup>
/**
 * AnnotationPin.vue - PDF 大头针组件
 * 
 * 该组件负责在 PDF 页面上显示一个大头针图标，支持：
 * - 双击导航到对应的 AI 对话
 * - 右键菜单删除大头针
 * - Hover 显示提示信息
 */
import { ref, onMounted } from 'vue';

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
    scale: {
        type: Number,
        default: 1, // 缩放比例
    },
});

// 定义组件事件
const emit = defineEmits(['dblclick', 'remove']);

// 状态管理
const isHovered = ref(false);

// 组件挂载时输出调试信息
onMounted(() => {
    console.log('大头针渲染:', {
        id: props.annotation.id,
        x: props.x,
        y: props.y,
        pageNum: props.annotation.pageNum
    });
});

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
const handleContextMenu = (event) => {
    event.preventDefault();
    event.stopPropagation();
    
    if (confirm('确定要删除这个大头针吗？')) {
        emit('remove', props.annotation.id);
    }
};
</script>

<template>
    <div
        class="annotation-pin"
        :style="{ 
            left: x + 'px', 
            top: y + 'px'
        }"
        @dblclick="handleDoubleClick"
        @contextmenu="handleContextMenu"
        @mouseenter="isHovered = true"
        @mouseleave="isHovered = false"
    >
        <!-- 大头针图标 -->
        <div class="pin-icon" :style="{ fontSize: (12 * scale) + 'px' }">📌</div>
        
        <!-- Hover 提示框 -->
        <div v-if="isHovered" class="pin-tooltip">
            <div class="tooltip-title">{{ annotation.note || 'AI 对话标记' }}</div>
            <div class="tooltip-info">双击跳转到对话</div>
            <div class="tooltip-info">右键删除标记</div>
        </div>
    </div>
</template>

<style scoped>
.annotation-pin {
    position: absolute;
    z-index: 100;
    cursor: pointer;
    transform: translate(-50%, -50%); /* 居中定位 */
    transition: filter 0.2s ease;
}

.annotation-pin:hover {
    filter: brightness(1.2) drop-shadow(0 4px 8px rgba(0, 0, 0, 0.4));
}

.pin-icon {
    font-size: 8px;
    filter: drop-shadow(0 2px 4px rgba(0, 0, 0, 0.3));
    user-select: none;
    opacity: 0.7;
}

.pin-tooltip {
    position: absolute;
    bottom: 100%;
    left: 50%;
    transform: translateX(-50%);
    margin-bottom: 8px;
    padding: 8px 12px;
    background: rgba(0, 0, 0, 0.85);
    color: white;
    border-radius: 6px;
    font-size: 12px;
    white-space: nowrap;
    pointer-events: none;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.3);
}

.tooltip-title {
    font-weight: bold;
    margin-bottom: 4px;
}

.tooltip-info {
    opacity: 0.8;
    font-size: 11px;
}
</style>
