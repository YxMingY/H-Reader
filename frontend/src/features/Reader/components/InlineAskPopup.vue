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

const messages = computed(() => props.conversation.messages.map(message => ({
  ...message,
  html: message.role === 'assistant' ? renderMarkdown(message.content) : '',
})));

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
  window.addEventListener('resize', placePopup);
  window.addEventListener('scroll', placePopup, true);
  placePopup();
});
onBeforeUnmount(() => {
  observer?.disconnect();
  window.removeEventListener('resize', placePopup);
  window.removeEventListener('scroll', placePopup, true);
});

watch(() => [props.conversation.loaded, props.conversation.sending], async ([loaded, sending]) => {
  if (!loaded || sending) return;
  await nextTick();
  input.value?.focus();
}, { immediate: true });

watch(() => props.conversation.messages.length, async () => {
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
              <div class="inline-markdown" v-html="message.html"></div>
              <button class="inline-save-note" type="button" @click="emit('save-note', message.content)">记为笔记</button>
            </template>
            <template v-else>{{ message.content }}</template>
          </article>
        </div>
        <div v-if="conversation.error" class="inline-error" role="alert">{{ conversation.error }}</div>
        <form @submit.prevent="emit('send')">
          <textarea
            ref="input"
            :value="conversation.question"
            :readonly="conversation.sending"
            :placeholder="messages.length ? '继续追问' : '输入问题'"
            aria-label="原地问问题"
            rows="2"
            @input="emit('update:question', $event.target.value)"
          />
          <button class="inline-send" type="submit" :disabled="conversation.sending || !conversation.question.trim()">
            {{ conversation.sending ? '等待回答…' : '发送' }}
          </button>
        </form>
      </template>
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
