<script setup>
/**
 * FormulaPreview.vue - 公式预览悬浮窗
 * 
 * 当用户选中 PDF 中的公式编号时弹出，显示：
 * - 公式编号
 * - 公式所在页码
 * - 跳转按钮（跳转到公式位置）
 * - 返回按钮（跳转后显示，返回之前的阅读位置）
 */

const props = defineProps({
  formula: {
    type: Object,
    required: true,
  },
  position: {
    type: Object,
    required: true,
    default: () => ({ x: 0, y: 0 }),
  },
  hasJumped: {
    type: Boolean,
    default: false,
  },
});

const emit = defineEmits(['jump', 'return', 'close']);
</script>

<template>
  <div
    class="formula-preview-popup"
    :class="{ 'docked': hasJumped }"
    :style="hasJumped ? {} : { left: position.x + 'px', top: position.y + 'px' }"
    @mousedown.stop
  >
    <div class="formula-header">
      <span class="formula-icon">📐</span>
      <span class="formula-number">公式 ({{ formula.number }})</span>
      <button class="close-btn" @click="emit('close')" title="关闭">×</button>
    </div>
    <div class="formula-page">第 {{ formula.pageNum }} 页</div>
    <div class="formula-actions">
      <button v-if="!hasJumped" class="action-btn jump-btn" @click="emit('jump')">
        🔗 跳转
      </button>
      <button v-if="hasJumped" class="action-btn return-btn" @click="emit('return')">
        ↩️ 返回原位
      </button>
    </div>
  </div>
</template>

<style scoped>
.formula-preview-popup {
  position: fixed;
  z-index: 10000;
  background: white;
  border-radius: 10px;
  box-shadow: 0 8px 30px rgba(0, 0, 0, 0.18), 0 2px 8px rgba(0, 0, 0, 0.08);
  padding: 14px 18px;
  min-width: 180px;
  font-size: 13px;
  border: 1px solid #e8e8e8;
  animation: formulaFadeIn 0.18s ease-out;
  pointer-events: auto;
  transition: all 0.3s ease;
}

.formula-preview-popup.docked {
  left: auto !important;
  top: auto !important;
  right: 24px;
  bottom: 24px;
}

@keyframes formulaFadeIn {
  from {
    opacity: 0;
    transform: translateY(-6px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.formula-header {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 8px;
}

.formula-icon {
  font-size: 15px;
}

.formula-number {
  font-weight: 700;
  font-size: 14px;
  color: #1a1a1a;
  flex: 1;
}

.close-btn {
  background: none;
  border: none;
  cursor: pointer;
  color: #bbb;
  font-size: 18px;
  line-height: 1;
  padding: 0 2px;
  border-radius: 4px;
  transition: color 0.15s, background 0.15s;
}

.close-btn:hover {
  color: #666;
  background: #f0f0f0;
}

.formula-page {
  color: #888;
  font-size: 12px;
  margin-bottom: 12px;
  padding-left: 22px;
}

.formula-actions {
  display: flex;
  gap: 8px;
}

.action-btn {
  padding: 7px 14px;
  border: none;
  border-radius: 6px;
  cursor: pointer;
  font-size: 12px;
  font-weight: 600;
  transition: all 0.18s;
  white-space: nowrap;
}

.jump-btn {
  background: linear-gradient(135deg, #e8f4ff, #d0e8ff);
  color: #007acc;
  border: 1px solid rgba(0, 122, 204, 0.2);
}

.jump-btn:hover {
  background: linear-gradient(135deg, #d0e8ff, #b8dcff);
  border-color: rgba(0, 122, 204, 0.35);
  transform: translateY(-1px);
  box-shadow: 0 2px 6px rgba(0, 122, 204, 0.2);
}

.return-btn {
  background: linear-gradient(135deg, #fff3e0, #ffe0b2);
  color: #e65100;
  border: 1px solid rgba(230, 81, 0, 0.2);
}

.return-btn:hover {
  background: linear-gradient(135deg, #ffe0b2, #ffcc80);
  border-color: rgba(230, 81, 0, 0.35);
  transform: translateY(-1px);
  box-shadow: 0 2px 6px rgba(230, 81, 0, 0.2);
}
</style>
