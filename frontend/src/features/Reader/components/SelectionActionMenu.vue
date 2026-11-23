<script setup>
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';

const props = defineProps({
  heading: { type: String, default: '' },
  position: { type: Object, required: true },
  actions: { type: Array, required: true },
  docked: Boolean,
});

const emit = defineEmits(['close']);
const menu = ref(null);
const placement = ref({ left: '0px', top: '0px' });
const pending = ref(false);
const error = ref('');

function placeMenu() {
  if (!menu.value || props.docked) return;
  const { offsetWidth, offsetHeight } = menu.value;
  const { x, top, bottom } = props.position;
  const margin = 8;
  const below = bottom + margin;
  const preferredTop = below + offsetHeight <= window.innerHeight - margin
    ? below
    : top - offsetHeight - margin;
  placement.value = {
    left: `${Math.max(margin, Math.min(x - offsetWidth / 2, window.innerWidth - offsetWidth - margin))}px`,
    top: `${Math.max(margin, preferredTop)}px`,
  };
}

async function runAction(action) {
  if (pending.value) return;
  pending.value = true;
  error.value = '';
  try {
    await action.run();
  } catch (err) {
    error.value = err?.message || '操作失败，请重试。';
  } finally {
    pending.value = false;
  }
}

watch(() => props.position, () => {
  error.value = '';
  nextTick(placeMenu);
});
watch(error, placeMenu, { flush: 'post' });
onMounted(() => {
  placeMenu();
  window.addEventListener('resize', placeMenu);
});
onBeforeUnmount(() => window.removeEventListener('resize', placeMenu));
</script>

<template>
  <Teleport to="body">
    <div
      ref="menu"
      class="selection-action-menu"
      :class="{ docked }"
      :style="docked ? undefined : placement"
      role="group"
      aria-label="PDF 选区操作"
      :aria-busy="pending"
      @mousedown.prevent
    >
      <div v-if="heading" class="selection-heading">{{ heading }}</div>
      <div class="selection-actions">
        <button
          v-for="action in actions"
          :key="action.id"
          type="button"
          :disabled="pending"
          @click="runAction(action)"
        >{{ action.label }}</button>
        <button type="button" aria-label="关闭选区菜单" :disabled="pending" @click="emit('close')">×</button>
      </div>
      <div v-if="error" class="selection-error" role="alert">{{ error }}</div>
    </div>
  </Teleport>
</template>

<style scoped>
.selection-action-menu {
  position: fixed;
  z-index: 10000;
  box-sizing: border-box;
  max-width: min(340px, calc(100vw - 16px));
  padding: 8px;
  border: 1px solid #ddd;
  border-radius: 8px;
  background: white;
  color: #222;
  box-shadow: 0 4px 18px rgba(0, 0, 0, 0.18);
  font-size: 13px;
}

.selection-action-menu.docked {
  right: 24px;
  bottom: 24px;
}

.selection-heading,
.selection-error {
  margin: 4px;
}

.selection-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.selection-actions button {
  padding: 6px 9px;
  border: 1px solid #d4e0ea;
  border-radius: 5px;
  background: #edf5fc;
  color: #17476b;
  cursor: pointer;
}

.selection-actions button:hover:not(:disabled) {
  background: #dcecf9;
}

.selection-actions button:disabled {
  opacity: 0.65;
  cursor: wait;
}

.selection-error {
  color: #a12622;
  overflow-wrap: anywhere;
}
</style>
