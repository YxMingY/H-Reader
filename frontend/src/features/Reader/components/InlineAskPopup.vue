<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { useTools } from '../../ChatPanel/composables/useTools';

const props = defineProps({
  conversation: { type: Object, required: true },
  pageCanvas: { type: Object, default: null },
});
const emit = defineEmits(['close', 'send', 'update:question', 'retry', 'save-note']);
const { renderMarkdown } = useTools();

const dialog = ref(null);
const input = ref(null);
const viewport = ref(null);
const position = ref({ visibility: 'hidden' });

const imagePreview = ref(null);
const imagePanel = ref(null);
const closeImageButton = ref(null);
const imagePosition = ref({});
let previewTrigger;

function placeImagePreview() {
  if (!imagePreview.value || !dialog.value || !imagePanel.value) return;
  const anchor = dialog.value.getBoundingClientRect();
  const gap = 8;
  const roomRight = window.innerWidth - anchor.right - 2 * gap;
  const roomLeft = anchor.left - 2 * gap;
  const openRight = roomRight >= 360 || roomRight >= roomLeft;
  const sideRoom = openRight ? roomRight : roomLeft;
  const width = Math.min(360, window.innerWidth - 2 * gap, Math.max(180, sideRoom));
  const left = openRight ? anchor.right + gap : anchor.left - width - gap;
  const height = imagePanel.value.offsetHeight;
  imagePosition.value = {
    width: `${width}px`,
    left: `${Math.max(gap, Math.min(left, window.innerWidth - width - gap))}px`,
    top: `${Math.max(gap, Math.min(anchor.top, window.innerHeight - height - gap))}px`,
  };
}

async function openImagePreview(src, alt, event) {
  previewTrigger = event.currentTarget;
  imagePreview.value = { src, alt };
  await nextTick();
  placeImagePreview();
  closeImageButton.value?.focus();
}

function closeImagePreview(restoreFocus = true) {
  imagePreview.value = null;
  if (restoreFocus) previewTrigger?.focus();
  previewTrigger = null;
}

function dismissImageOnOutsideClick(event) {
  if (imagePreview.value && !imagePanel.value?.contains(event.target)) {
    closeImagePreview(false);
  }
}

function handlePreviewEscape(event) {
  if (event.key !== 'Escape' || !imagePreview.value) return;
  event.preventDefault();
  event.stopPropagation();
  closeImagePreview();
}

watch(position, placeImagePreview, { flush: 'post' });

const messages = computed(() => {
  const { messages, pending } = props.conversation;
  return (pending ? [...messages, pending.question, pending.answer] : messages).map(message => ({
    ...message,
    html: message.role === 'assistant' ? renderMarkdown(message.content, message.streaming) : '',
  }));
});

function placePopup() {
  if (!dialog.value || !props.pageCanvas) return;
  const rect = props.pageCanvas.getBoundingClientRect();
  const { x, y } = props.conversation.anchor;
  const pointX = rect.left + x * rect.width;
  const pointY = rect.top + y * rect.height;
  const { offsetWidth, offsetHeight } = dialog.value;
  const gap = 8;
  const preferredX = pointX + gap + offsetWidth <= window.innerWidth
    ? pointX + gap
    : pointX - offsetWidth - gap;

  position.value = {
    left: `${Math.max(gap, Math.min(preferredX, window.innerWidth - offsetWidth - gap))}px`,
    top: `${Math.max(gap, Math.min(pointY, window.innerHeight - offsetHeight - gap))}px`,
  };
}

let observer;
onMounted(() => {
  observer = new ResizeObserver(placePopup);
  observer.observe(dialog.value);
  if (props.pageCanvas) observer.observe(props.pageCanvas);
  document.addEventListener('pointerdown', dismissImageOnOutsideClick, true);
  document.addEventListener('keydown', handlePreviewEscape, true);
  window.addEventListener('resize', placePopup);
  window.addEventListener('scroll', placePopup, true);
  placePopup();
});
onBeforeUnmount(() => {
  observer?.disconnect();
  document.removeEventListener('pointerdown', dismissImageOnOutsideClick, true);
  document.removeEventListener('keydown', handlePreviewEscape, true);
  window.removeEventListener('resize', placePopup);
  window.removeEventListener('scroll', placePopup, true);
});

watch(() => [props.conversation.loaded, props.conversation.sending], async ([loaded, sending]) => {
  if (!loaded || sending) return;
  await nextTick();
  input.value?.focus();
}, { immediate: true });

watch(() => [messages.value.length, props.conversation.pending?.answer.content], async () => {
  await nextTick();
  if (viewport.value) viewport.value.scrollTop = viewport.value.scrollHeight;
}, { immediate: true });

function handleKeydown(event) {
  if (event.isComposing || event.keyCode === 229) return;
  if (event.key === 'Escape') {
    event.stopPropagation();
    emit('close');
  } else if (event.key === 'Enter' && !event.shiftKey && event.target === input.value) {
    event.preventDefault();
    emit('send');
  }
}
</script>

<template>
  <Teleport to="body">
    <section
      ref="dialog"
      class="inline-ask-popup"
      role="dialog"
      aria-label="原地问"
      :style="position"
      @keydown="handleKeydown"
    >
      <header>
        <strong>原地问</strong>
        <button class="inline-close" type="button" aria-label="收起原地问" @click="emit('close')">×</button>
      </header>
      <details v-if="conversation.anchor.quote" class="inline-quote">
        <summary>论文摘录</summary>
        <blockquote>{{ conversation.anchor.quote }}</blockquote>
      </details>
      <div v-if="conversation.loading" role="status">正在读取问答…</div>
      <div v-else-if="!conversation.loaded" class="inline-error" role="alert">
        {{ conversation.error }}
        <button type="button" @click="emit('retry')">重试</button>
      </div>
      <template v-else>
        <div v-if="messages.length" ref="viewport" class="inline-messages">
          <article
            v-for="(message, index) in messages"
            :key="index"
            :class="message.role === 'assistant' ? 'inline-answer' : 'inline-question'"
          >
            <template v-if="message.role === 'assistant'">
              <div v-if="message.content" class="inline-markdown" v-html="message.html"></div>
              <span v-else role="status">等待回答…</span>
              <button v-if="!message.streaming" class="inline-save-note" type="button" @click="emit('save-note', message.content)">记为笔记</button>
            </template>
            <template v-else>
              <div v-if="message.attachments?.length" class="inline-attachments">
                <button
                  v-for="(src, index) in message.attachments"
                  :key="index"
                  type="button"
                  class="inline-image-preview"
                  :aria-label="`预览已发送图片 ${index + 1}`"
                  @click="openImagePreview(src, `已发送图片 ${index + 1}`, $event)"
                >
                  <img :src="src" :alt="`已发送图片 ${index + 1}`" />
                </button>
              </div>
              {{ message.content }}
            </template>
          </article>
        </div>
        <div v-if="conversation.error" class="inline-error" role="alert">{{ conversation.error }}</div>
        <div v-if="!conversation.sending && conversation.attachments.length" class="inline-attachments">
          <div v-for="(image, index) in conversation.attachments" :key="image.id" class="inline-draft-image">
            <button
              type="button"
              class="inline-image-preview"
              :aria-label="`预览待发送图片 ${index + 1}`"
              @click="openImagePreview(image.compressedDataUrl, `待发送图片 ${index + 1}`, $event)"
            >
              <img :src="image.compressedDataUrl" :alt="`待发送图片 ${index + 1}`" />
            </button>
            <button type="button" class="inline-image-remove" :aria-label="`移除图片 ${index + 1}`" @click="conversation.removeAttachment(image.id)">×</button>
          </div>
        </div>
        <div v-if="conversation.pasteFeedback" role="status">{{ conversation.pasteFeedback }}</div>
        <form @submit.prevent="emit('send')">
          <textarea
            ref="input"
            :value="conversation.question"
            :readonly="conversation.sending"
            :placeholder="messages.length ? '继续追问' : '输入问题'"
            aria-label="原地问问题"
            rows="2"
            @input="emit('update:question', $event.target.value)"
            @paste="conversation.pasteImage"
          />
          <button class="inline-send" type="submit" :disabled="conversation.sending || conversation.processingPaste || (!conversation.question.trim() && !conversation.attachments.length)">
            {{ conversation.sending ? '发送中…' : '发送' }}
          </button>
        </form>
      </template>
    </section>
    <section
      v-if="imagePreview"
      ref="imagePanel"
      class="inline-image-panel"
      role="dialog"
      aria-label="原地问图片预览"
      :style="imagePosition"
    >
      <header>
        <span>{{ imagePreview.alt }}</span>
        <button ref="closeImageButton" type="button" class="inline-close" aria-label="关闭图片预览" @click="closeImagePreview()">×</button>
      </header>
      <img :src="imagePreview.src" :alt="imagePreview.alt" />
    </section>
  </Teleport>
</template>

<style scoped>
.inline-ask-popup {
  position: fixed;
  z-index: 10002;
  display: grid;
  gap: 8px;
  box-sizing: border-box;
  width: min(380px, calc(100vw - 16px));
  max-height: calc(100vh - 16px);
  overflow: auto;
  padding: 12px;
  border: 1px solid #d8e0e9;
  border-radius: 10px;
  background: #fff;
  color: #202731;
  box-shadow: 0 8px 28px rgb(0 0 0 / 22%);
  font-size: 14px;
}

header,
form {
  display: flex;
  align-items: center;
  gap: 8px;
}

header {
  justify-content: space-between;
}

button {
  padding: 5px 8px;
  border: 1px solid #ccdbe9;
  border-radius: 6px;
  background: #edf5fc;
  color: #235071;
  cursor: pointer;
}

button:disabled {
  opacity: 0.55;
  cursor: default;
}

.inline-close {
  border: 0;
  background: transparent;
  color: #4a596a;
  font-size: 18px;
}

.inline-quote {
  padding: 6px 8px;
  border-left: 3px solid #b9c7d7;
  background: #f2f5f8;
  color: #43566c;
}

summary {
  cursor: pointer;
}

blockquote {
  max-height: 100px;
  margin: 5px 0 0;
  overflow: auto;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.inline-messages {
  display: grid;
  gap: 6px;
  max-height: min(330px, 47vh);
  overflow: auto;
  overflow-wrap: anywhere;
}

.inline-question,
.inline-answer {
  min-width: 0;
  padding: 8px;
  border-radius: 6px;
}

.inline-question {
  justify-self: end;
  max-width: 90%;
  background: #eaf4ff;
  white-space: pre-wrap;
}

.inline-answer {
  background: #f5f7fa;
}

.inline-markdown :deep(p) {
  margin: 0 0 6px;
}

.inline-markdown :deep(pre),
.inline-markdown :deep(.katex-display),
.inline-markdown :deep(table) {
  max-width: 100%;
  overflow: auto;
}

.inline-markdown :deep(img) {
  max-width: 100%;
}

.inline-attachments {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-bottom: 4px;
}

.inline-attachments img {
  display: block;
  max-width: 100%;
  max-height: 100px;
  border-radius: 4px;
}

.inline-draft-image {
  position: relative;
}

.inline-image-preview {
  max-width: 100%;
  padding: 0;
  border: 0;
  background: transparent;
  cursor: zoom-in;
}

.inline-image-panel {
  position: fixed;
  z-index: 10003;
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  gap: 8px;
  box-sizing: border-box;
  width: min(360px, calc(100vw - 16px));
  height: min(340px, calc(100vh - 16px));
  padding: 10px;
  border: 1px solid #d8e0e9;
  border-radius: 10px;
  background: #fff;
  color: #202731;
  box-shadow: 0 8px 28px rgb(0 0 0 / 18%);
  font-size: 14px;
}

.inline-image-panel img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.inline-image-remove {
  position: absolute;
  top: 2px;
  right: 2px;
}

.inline-save-note {
  margin-top: 4px;
}

form {
  align-items: flex-end;
}

textarea {
  flex: 1;
  min-width: 0;
  padding: 7px;
  resize: vertical;
  border: 1px solid #bccad9;
  border-radius: 6px;
  font: inherit;
}

.inline-send {
  flex: none;
}

.inline-error {
  color: #a02422;
  overflow-wrap: anywhere;
}
</style>
