<script setup>
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue';

const props = defineProps({
  note: { type: Object, required: true },
  editing: Boolean,
  draft: { type: String, default: '' },
  saving: Boolean,
  error: { type: String, default: '' },
});
const emit = defineEmits(['edit', 'update:draft', 'contextmenu', 'open-conversation']);

const marker = ref(null);
const popup = ref(null);
const textarea = ref(null);
const reading = ref(false);
const position = ref({});
const visible = computed(() => props.editing || reading.value);
let hideTimeout;

function showReading() {
  clearTimeout(hideTimeout);
  if (!props.editing) reading.value = true;
}

function hideReading() {
  clearTimeout(hideTimeout);
  hideTimeout = setTimeout(() => { reading.value = false; }, 120);
}

function edit() {
  reading.value = false;
  emit('edit', props.note);
}

function openContext(event) {
  reading.value = false;
  emit('contextmenu', event, props.note);
}

function placePopup() {
  if (!marker.value || !popup.value) return;
  const rect = marker.value.getBoundingClientRect();
  const { offsetWidth, offsetHeight } = popup.value;
  const margin = 8;
  const left = rect.right + offsetWidth <= window.innerWidth - margin
    ? rect.right
    : rect.left - offsetWidth;
  position.value = {
    left: `${Math.max(margin, Math.min(left, window.innerWidth - offsetWidth - margin))}px`,
    top: `${Math.max(margin, Math.min(rect.top, window.innerHeight - offsetHeight - margin))}px`,
  };
}

watch(visible, async (open) => {
  if (open) {
    window.addEventListener('resize', placePopup);
    window.addEventListener('scroll', placePopup, true);
    await nextTick();
    placePopup();
  } else {
    window.removeEventListener('resize', placePopup);
    window.removeEventListener('scroll', placePopup, true);
  }
}, { immediate: true });

watch(() => props.editing, async (editing) => {
  if (!editing) return;
  reading.value = false;
  await nextTick();
  placePopup();
  textarea.value?.focus();
}, { immediate: true });
watch(() => props.error, placePopup, { flush: 'post' });
onBeforeUnmount(() => {
  clearTimeout(hideTimeout);
  window.removeEventListener('resize', placePopup);
  window.removeEventListener('scroll', placePopup, true);
});
</script>

<template>
  <button
    ref="marker"
    type="button"
    class="pdf-note-marker pdf-page-marker"
    :style="{ left: `${note.x * 100}%`, top: `${note.y * 100}%` }"
    aria-label="笔记"
    :aria-expanded="visible"
    @pointerenter="showReading"
    @pointerleave="hideReading"
    @focus="showReading"
    @blur="hideReading"
    @click.stop="edit"
    @contextmenu.prevent.stop="openContext"
  ><span class="pdf-marker-icon" aria-hidden="true">✏️</span></button>
  <Teleport to="body">
    <div
      v-if="visible"
      ref="popup"
      class="pdf-note-popup"
      :class="{ 'pdf-note-editor': editing }"
      :style="position"
      @pointerenter="showReading"
      @pointerleave="hideReading"
    >
      <template v-if="editing">
        <textarea
          ref="textarea"
          aria-label="编辑笔记"
          placeholder="输入笔记"
          :value="draft"
          :readonly="saving"
          @input="emit('update:draft', $event.target.value)"
        />
        <div v-if="error" class="note-error" role="alert">{{ error }}</div>
      </template>
      <button v-else type="button" class="note-content" @click="edit">{{ note.text || '空笔记' }}</button>
      <button
        v-if="note.inlineConversationId"
        type="button"
        class="note-conversation-link"
        :disabled="saving"
        @click="reading = false; emit('open-conversation', note)"
      >查看原问答</button>
    </div>
  </Teleport>
</template>

<style scoped>
.pdf-note-marker {
  position: absolute;
  z-index: 101;
  transform: translate(-50%, -50%);
  padding: 0;
  border: 0;
  background: transparent;
  font: inherit;
  cursor: pointer;
}

.pdf-note-popup {
  position: fixed;
  z-index: 10001;
  box-sizing: border-box;
  width: min(280px, calc(100vw - 16px));
  padding: 8px;
  border: 1px solid #e6c66e;
  border-radius: 8px;
  background: #fffaf0;
  color: #29251e;
  box-shadow: 0 4px 18px rgb(0 0 0 / 18%);
}

.pdf-note-popup textarea,
.note-content {
  box-sizing: border-box;
  width: 100%;
  padding: 6px;
  border: 0;
  background: transparent;
  color: inherit;
  font: 14px/1.5 system-ui, sans-serif;
  text-align: left;
}

.pdf-note-popup textarea {
  display: block;
  height: min(110px, 45vh);
  resize: none;
}

.note-content {
  display: block;
  max-height: min(180px, 45vh);
  overflow: auto;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  cursor: text;
}

.note-conversation-link {
  margin: 4px 6px 0;
  padding: 4px 7px;
  border: 1px solid #e6c66e;
  border-radius: 5px;
  background: #fff5d3;
  color: #6b4915;
  cursor: pointer;
}

.note-error {
  padding: 4px 6px;
  color: #a12622;
  overflow-wrap: anywhere;
}
</style>
