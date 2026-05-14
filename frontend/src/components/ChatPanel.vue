<template>
  <div class="chat-panel">
    <div class="chat-header">
      <div class="chat-header-copy">
        <p class="chat-subtitle">{{ scopeSubtitle }}</p>
      </div>

      <div class="chat-header-actions">
        <button
          v-if="activeSessionId"
          class="chat-header-btn"
          type="button"
          @click="backToSessions"
        >
          会话列表
        </button>
        <button
          v-else
          class="chat-header-btn"
          type="button"
          @click="refreshSessions"
          :disabled="loadingSessions"
        >
          刷新
        </button>
        <button class="chat-header-btn primary" type="button" @click="createSessionAndEnter">
          新会话
        </button>
        <button class="chat-close" type="button" @click="emit('close')" aria-label="关闭聊天框">×</button>
      </div>
    </div>

    <div v-if="!activeSessionId" class="chat-sessions">
      <div class="chat-section-head">
        <h4>会话</h4>
        <span class="chat-section-count">{{ sessions.length }}</span>
      </div>

      <div v-if="loadingSessions" class="chat-empty compact">正在加载会话...</div>
      <div v-else-if="sessions.length === 0" class="chat-empty compact">
        当前作用域还没有会话
      </div>
      <div v-else class="chat-session-list">
        <div
          v-for="session in sessions"
          :key="session.session_id"
          class="chat-session-item"
          :class="{ active: session.session_id === activeSessionId }"
          role="button"
          tabindex="0"
          @click="loadSession(session.session_id)"
          @keydown.enter.prevent="loadSession(session.session_id)"
          @keydown.space.prevent="loadSession(session.session_id)"
        >
          <div class="chat-session-main">
            <div class="chat-session-title">{{ session.title }}</div>
            <div class="chat-session-meta">
              <span>{{ session.message_count }} 条消息</span>
              <span>{{ session.updated_at }}</span>
            </div>
          </div>
          <button
            class="chat-session-delete"
            type="button"
            title="删除会话"
            @click.stop="deleteSession(session.session_id)"
          >
            ×
          </button>
        </div>
      </div>

      <p v-if="errorMessage" class="chat-error selector">{{ errorMessage }}</p>
    </div>

    <div v-else class="chat-body">
      <div ref="messageViewportRef" class="chat-messages">
        <div v-if="loadingMessages" class="chat-empty">正在加载对话...</div>
        <div v-else-if="messages.length === 0" class="chat-empty">
          当前会话还没有消息，输入内容开始对话
        </div>
        <div v-else class="chat-message-list">
          <div
            v-for="(message, index) in messages"
            :key="`${message.created_at}-${index}`"
            class="chat-message"
            :class="message.role"
          >
            <div
              v-if="message.role === 'assistant'"
              class="chat-message-content markdown-body"
              v-html="renderMarkdown(message.content)"
            ></div>
            <div v-else class="chat-message-content">{{ message.content }}</div>
          </div>
        </div>
      </div>

      <div class="chat-composer">
        <div v-if="attachments.length" class="attachment-strip">
          <div v-for="item in attachments" :key="item.id" class="attachment-thumb">
            <img :src="item.dataUrl" :alt="item.name" />
            <button type="button" class="attachment-remove" @click="removeAttachment(item.id)">×</button>
          </div>
        </div>

        <textarea
          v-model="draft"
          class="chat-input"
          rows="3"
          placeholder="输入文本，支持多张图片一起发送"
          @keydown.meta.enter.prevent="sendMessage"
          @keydown.ctrl.enter.prevent="sendMessage"
        ></textarea>

        <div class="composer-actions">
          <input
            ref="fileInputRef"
            class="file-input"
            type="file"
            accept="image/*"
            multiple
            @change="handleFileInput"
          />
          <button class="chat-header-btn" type="button" @click="pickImages">添加图片</button>
          <button class="chat-header-btn primary" type="button" @click="sendMessage" :disabled="sending">
            {{ sending ? '发送中…' : '发送' }}
          </button>
        </div>

        <p v-if="errorMessage" class="chat-error">{{ errorMessage }}</p>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import MarkdownIt from 'markdown-it';
import hljs from 'highlight.js';
import 'highlight.js/styles/github.css';
import { Events } from '@wailsio/runtime';
import { ChatService } from '../../bindings/hreader';

// 组件输入：决定会话作用域（书架级/书籍级）及上下文书籍信息。
const props = defineProps({
  scopeType: {
    type: String,
    default: 'library',
  },
  bookPath: {
    type: String,
    default: '',
  },
  bookTitle: {
    type: String,
    default: '',
  },
  bookKey: {
    type: String,
    default: '',
  },
});

const emit = defineEmits(['close']);

// 核心状态：会话列表、当前会话消息、输入草稿、附件、加载/发送状态等。
const sessions = ref([]);
const activeSessionId = ref('');
const messages = ref([]);
const draft = ref('');
const attachments = ref([]);
const loadingSessions = ref(false);
const loadingMessages = ref(false);
const sending = ref(false);
const errorMessage = ref('');
const messageViewportRef = ref(null);
const fileInputRef = ref(null);
const streamSessionId = ref('');
const streamAssistantIndex = ref(-1);
const eventUnsubscribers = [];

// Markdown 渲染器：用于 assistant 消息渲染，并接入代码高亮。
const markdownRenderer = new MarkdownIt({
  breaks: true,
  linkify: true,
  typographer: true,
  highlight(code, language) {
    if (language && hljs.getLanguage(language)) {
      try {
        const highlighted = hljs.highlight(code, { language, ignoreIllegals: true }).value;
        return `<pre class="hljs"><code class="language-${language}">${highlighted}</code></pre>`;
      } catch (err) {
        // 回退到纯文本输出，避免高亮失败影响消息渲染。
      }
    }

    try {
      const highlighted = hljs.highlightAuto(code).value;
      return `<pre class="hljs"><code>${highlighted}</code></pre>`;
    } catch (err) {
      return '';
    }
  },
});

const scopeTitle = computed(() => (props.scopeType === 'book' ? props.bookTitle || props.bookPath || '当前书籍' : '全局书架'));
const scopeSubtitle = computed(() => (props.scopeType === 'book' ? `当前书籍：${scopeTitle.value}` : '当前作用域：书架'));
const scopeBookPath = computed(() => (props.scopeType === 'book' ? props.bookPath : ''));
const scopeKey = computed(() => `${props.scopeType}:${props.bookKey || scopeBookPath.value || 'library'}`);

// 将消息视口滚动到底部。包含多帧滚动以覆盖异步渲染（Markdown/图片）导致的高度延后变化。
const scrollToBottom = async () => {
  // 在切换会话/渲染 Markdown 后再滚动，避免只执行一次 nextTick 仍停在中部。
  await nextTick();
  const container = messageViewportRef.value;
  if (container) {
    container.scrollTop = container.scrollHeight;
    requestAnimationFrame(() => {
      container.scrollTop = container.scrollHeight;
      requestAnimationFrame(() => {
        container.scrollTop = container.scrollHeight;
      });
    });
  }
};

const renderMarkdown = (content) => markdownRenderer.render(String(content || ''));

const normalizeSessions = (result) => (Array.isArray(result) ? result : []);

// 拉取当前作用域会话列表；若已有活跃会话，则尝试保持选中并刷新其消息。
const refreshSessions = async () => {
  loadingSessions.value = true;
  errorMessage.value = '';
  try {
    sessions.value = normalizeSessions(await ChatService.ListSessions(props.scopeType, scopeBookPath.value));
    if (activeSessionId.value) {
      const existing = sessions.value.find((item) => item.session_id === activeSessionId.value);
      if (existing) {
        await loadSession(existing.session_id, { keepSelection: true });
      } else {
        activeSessionId.value = '';
        messages.value = [];
      }
    }
  } catch (err) {
    errorMessage.value = `加载会话失败：${err?.message || err}`;
  } finally {
    loadingSessions.value = false;
  }
};

const upsertSessionSummary = (summary) => {
  const next = sessions.value.filter((item) => item.session_id !== summary.session_id);
  next.unshift(summary);
  sessions.value = next;
};

// 加载指定会话详情（summary + messages），并在成功后滚动到底部。
const loadSession = async (sessionId, options = {}) => {
  if (!sessionId) return;
  loadingMessages.value = true;
  errorMessage.value = '';
  try {
    const detail = await ChatService.LoadSession(props.scopeType, scopeBookPath.value, sessionId);
    activeSessionId.value = detail.summary.session_id;
    messages.value = detail.messages || [];
    if (!options.keepSelection) {
      upsertSessionSummary(detail.summary);
    }
    await scrollToBottom();
  } catch (err) {
    errorMessage.value = `加载对话失败：${err?.message || err}`;
  } finally {
    loadingMessages.value = false;
  }
};

// 创建会话但不自动发送消息，返回创建后的 summary。
const createSession = async () => {
  errorMessage.value = '';
  try {
    const title = props.scopeType === 'book' ? `${scopeTitle.value} 对话` : '新会话';
    const summary = await ChatService.CreateSession(props.scopeType, scopeBookPath.value, title);
    upsertSessionSummary(summary);
    return summary;
  } catch (err) {
    errorMessage.value = `创建会话失败：${err?.message || err}`;
    return null;
  }
};

// 进入新建会话：重置输入态并切换到会话详情页。
const createSessionAndEnter = async () => {
  const summary = await createSession();
  if (!summary) return;
  activeSessionId.value = summary.session_id;
  messages.value = [];
  draft.value = '';
  attachments.value = [];
  await scrollToBottom();
};

// 返回会话列表页：仅清理当前会话视图相关状态。
const backToSessions = () => {
  activeSessionId.value = '';
  messages.value = [];
  draft.value = '';
  attachments.value = [];
  errorMessage.value = '';
};

// 删除会话并同步列表；若删除的是当前会话则回退到列表态。
const deleteSession = async (sessionId) => {
  if (!window.confirm('确定删除这个会话吗？')) return;
  errorMessage.value = '';
  try {
    await ChatService.DeleteSession(props.scopeType, scopeBookPath.value, sessionId);
    sessions.value = sessions.value.filter((item) => item.session_id !== sessionId);
    if (activeSessionId.value === sessionId) {
      activeSessionId.value = '';
      messages.value = [];
      if (sessions.value.length > 0) {
        await loadSession(sessions.value[0].session_id, { keepSelection: true });
      }
    }
  } catch (err) {
    errorMessage.value = `删除会话失败：${err?.message || err}`;
  }
};

// 将本地图片文件转为 data URL，便于与文本一起发送给后端。
const readFileAsDataUrl = (file) => new Promise((resolve, reject) => {
  const reader = new FileReader();
  reader.onload = () => resolve({
    id: `${file.name}-${file.size}-${file.lastModified}-${Math.random().toString(36).slice(2)}`,
    name: file.name,
    dataUrl: reader.result,
  });
  reader.onerror = () => reject(reader.error || new Error('读取图片失败'));
  reader.readAsDataURL(file);
});

const pickImages = () => {
  fileInputRef.value?.click();
};

// 读取多选图片并追加到附件列表。
const handleFileInput = async (event) => {
  const files = Array.from(event.target.files || []);
  event.target.value = '';
  if (!files.length) return;

  try {
    const nextItems = await Promise.all(files.map((file) => readFileAsDataUrl(file)));
    attachments.value = attachments.value.concat(nextItems);
  } catch (err) {
    errorMessage.value = `添加图片失败：${err?.message || err}`;
  }
};

const removeAttachment = (id) => {
  attachments.value = attachments.value.filter((item) => item.id !== id);
};

// 重置一次流式发送的前端追踪状态。
const resetStreamingState = () => {
  streamSessionId.value = '';
  streamAssistantIndex.value = -1;
  sending.value = false;
};

// 接收后端流式分片并追加到“当前 assistant 占位消息”。
const onStreamChunk = async (event) => {
  if (!streamSessionId.value) return;
  if (activeSessionId.value !== streamSessionId.value) return;

  const chunk = String(event?.data || '');
  if (!chunk) return;

  const index = streamAssistantIndex.value;
  if (index < 0 || index >= messages.value.length) return;

  const target = messages.value[index];
  if (!target || target.role !== 'assistant') return;

  target.content = (target.content || '') + chunk;
  await scrollToBottom();
};

// 流式结束：落最终内容、重置流式状态，并回源刷新会话文件数据。
const onStreamDone = async (event) => {
  if (!streamSessionId.value) return;

  const full = String(event?.data || '');
  const index = streamAssistantIndex.value;
  if (
    activeSessionId.value === streamSessionId.value &&
    index >= 0 &&
    index < messages.value.length &&
    messages.value[index]?.role === 'assistant'
  ) {
    messages.value[index].content = full || messages.value[index].content || '';
  }

  const doneSessionId = streamSessionId.value;
  const shouldReloadActive = activeSessionId.value === doneSessionId;
  resetStreamingState();
  if (shouldReloadActive) {
    await loadSession(doneSessionId, { keepSelection: true });
  }
  await refreshSessions();
};

// 流式错误：展示错误并清理空占位消息。
const onStreamError = async (event) => {
  const msg = String(event?.data || '流式回复失败');
  errorMessage.value = msg;

  const index = streamAssistantIndex.value;
  if (
    activeSessionId.value === streamSessionId.value &&
    index >= 0 &&
    index < messages.value.length &&
    messages.value[index]?.role === 'assistant'
  ) {
    if (!messages.value[index].content) {
      messages.value.splice(index, 1);
    }
  }

  resetStreamingState();
  await refreshSessions();
};

// 发送入口：创建本地 user/assistant 占位消息后，调用后端流式接口。
const sendMessage = async () => {
  const text = draft.value.trim();
  if (!text && attachments.value.length === 0) return;

  errorMessage.value = '';
  sending.value = true;
  try {
    let sessionId = activeSessionId.value;
    if (!sessionId) {
      const summary = await createSession();
      if (!summary) {
        throw new Error('创建会话失败');
      }
      sessionId = summary.session_id;
      activeSessionId.value = sessionId;
    }

    const localNow = new Date().toISOString();
    messages.value.push({ role: 'user', content: text, created_at: localNow });
    messages.value.push({ role: 'assistant', content: '', created_at: localNow });
    streamSessionId.value = sessionId;
    streamAssistantIndex.value = messages.value.length - 1;

    const pendingImages = attachments.value.map((item) => item.dataUrl);
    draft.value = '';
    attachments.value = [];
    await scrollToBottom();

    await ChatService.SendMessageStreamInSession(
      props.scopeType,
      scopeBookPath.value,
      sessionId,
      text,
      pendingImages
    );
  } catch (err) {
    if (streamAssistantIndex.value >= 0 && streamAssistantIndex.value < messages.value.length) {
      messages.value.splice(streamAssistantIndex.value, 1);
      streamAssistantIndex.value = -1;
    }
    if (messages.value.length && messages.value[messages.value.length - 1]?.role === 'user' && messages.value[messages.value.length - 1]?.content === text) {
      messages.value.splice(messages.value.length - 1, 1);
    }
    resetStreamingState();
    errorMessage.value = `发送失败：${err?.message || err}`;
  }
};

// 作用域变化（书架/书籍切换）时，清空旧态并重拉会话列表。
watch(
  () => scopeKey.value,
  async () => {
    activeSessionId.value = '';
    messages.value = [];
    draft.value = '';
    attachments.value = [];
    await refreshSessions();
  },
  { immediate: true }
);

// 消息数量变化通常意味着新增消息，自动贴底。
watch(
  () => messages.value.length,
  async () => {
    await scrollToBottom();
  }
);

// 消息内容变化（流式追加）时持续贴底。
watch(
  () => messages.value.map((item) => item.content || '').join('\u0001'),
  async () => {
    await scrollToBottom();
  }
);

// 打开会话后立即贴底，确保进入时看到最新消息。
watch(
  () => activeSessionId.value,
  async (sessionId) => {
    if (!sessionId) return;
    await scrollToBottom();
  }
);

// 注册流式事件监听（chat_chunk/chat_done/chat_stream_error）。
onMounted(() => {
  eventUnsubscribers.push(Events.On('chat_chunk', onStreamChunk));
  eventUnsubscribers.push(Events.On('chat_done', onStreamDone));
  eventUnsubscribers.push(Events.On('chat_stream_error', onStreamError));
});

// 组件卸载时取消所有事件订阅，避免内存泄漏和重复回调。
onBeforeUnmount(() => {
  while (eventUnsubscribers.length) {
    const off = eventUnsubscribers.pop();
    if (typeof off === 'function') {
      off();
    }
  }
});

</script>

<style scoped>
.chat-panel {
  height: 100%;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(246, 248, 252, 0.98));
  border-left: 1px solid rgba(0, 0, 0, 0.08);
  box-shadow: inset 1px 0 0 rgba(255, 255, 255, 0.7);
}

.chat-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding: 18px 18px 14px;
  border-bottom: 1px solid rgba(0, 0, 0, 0.06);
}

.chat-header-copy {
  min-width: 0;
}

.chat-kicker {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  height: 22px;
  padding: 0 9px;
  border-radius: 999px;
  background: rgba(0, 122, 204, 0.1);
  color: var(--accent-color);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.08em;
}

.chat-title {
  margin: 8px 0 0;
  font-size: 18px;
  line-height: 1.2;
}

.chat-subtitle {
  margin: 6px 0 0;
  font-size: 12px;
  color: var(--text-secondary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.chat-header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.chat-header-btn,
.chat-close {
  border: 1px solid rgba(0, 0, 0, 0.08);
  background: rgba(255, 255, 255, 0.9);
  color: var(--text-primary);
  border-radius: 10px;
  height: 30px;
  padding: 0 10px;
  font-size: 12px;
  cursor: pointer;
}

.chat-header-btn.primary {
  background: var(--accent-color);
  color: #fff;
  border-color: transparent;
}

.chat-close {
  width: 30px;
  padding: 0;
  font-size: 20px;
  line-height: 1;
}

.chat-sessions {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  padding: 16px 18px 18px;
}

.chat-section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}

.chat-section-head h4 {
  margin: 0;
  font-size: 13px;
}

.chat-section-count {
  font-size: 12px;
  color: var(--text-secondary);
}

.chat-session-list {
  flex: 1;
  min-height: 0;
  overflow: auto;
  display: grid;
  align-content: start;
  grid-auto-rows: max-content;
  gap: 10px;
  padding-right: 6px;
}

.chat-session-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  width: 100%;
  box-sizing: border-box;
  border: 1px solid rgba(0, 0, 0, 0.08);
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.9);
  padding: 12px;
  cursor: pointer;
  text-align: left;
  min-width: 0;
}

.chat-session-item.active {
  border-color: rgba(0, 122, 204, 0.4);
  box-shadow: 0 0 0 3px rgba(0, 122, 204, 0.08);
}

.chat-session-main {
  min-width: 0;
  flex: 1;
  overflow: hidden;
}

.chat-session-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.chat-session-meta {
  margin-top: 6px;
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  font-size: 11px;
  color: var(--text-secondary);
}

.chat-session-delete {
  flex: 0 0 24px;
  width: 24px;
  height: 24px;
  border: none;
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  font-size: 20px;
  line-height: 1;
  padding: 0;
}

.chat-body {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  padding: 16px 18px 18px;
  gap: 14px;
}

.chat-messages {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  overflow-x: hidden;
  padding-right: 4px;
  user-select: text;
  -webkit-user-select: text;
}

.chat-message-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-width: 0;
}

.chat-message {
  box-sizing: border-box;
  max-width: 100%;
  min-width: 0;
  width: auto;
  align-self: flex-start;
  padding: 12px 13px;
  border-radius: 16px;
  border: 1px solid rgba(0, 0, 0, 0.06);
  background: rgba(255, 255, 255, 0.88);
}

.chat-message.user {
  align-self: flex-end;
  background: linear-gradient(180deg, rgba(0, 122, 204, 0.08), rgba(0, 122, 204, 0.04));
  border-color: rgba(0, 122, 204, 0.14);
}

.chat-message.assistant {
  background: rgba(255, 255, 255, 0.96);
}

.chat-message-content {
  min-width: 0;
  max-width: 100%;
  white-space: pre-wrap;
  word-break: break-word;
  overflow-wrap: anywhere;
  font-size: 13px;
  line-height: 1.65;
  color: var(--text-primary);
  user-select: text;
  -webkit-user-select: text;
}

.chat-message-content.markdown-body {
  min-width: 0;
  max-width: 100%;
  white-space: normal;
}

.chat-message-content.markdown-body :deep(*) {
  max-width: 100%;
}

.chat-message-content.markdown-body :deep(p) {
  margin: 0 0 0.75em;
}

.chat-message-content.markdown-body :deep(p:last-child) {
  margin-bottom: 0;
}

.chat-message-content.markdown-body :deep(ul),
.chat-message-content.markdown-body :deep(ol) {
  margin: 0.5em 0 0.75em;
  padding-left: 1.4em;
}

.chat-message-content.markdown-body :deep(li + li) {
  margin-top: 0.25em;
}

.chat-message-content.markdown-body :deep(blockquote) {
  margin: 0.5em 0;
  padding: 0.2em 0 0.2em 0.9em;
  border-left: 3px solid rgba(0, 122, 204, 0.3);
  color: var(--text-secondary);
}

.chat-message-content.markdown-body :deep(pre) {
  margin: 0.75em 0;
  width: auto;
  padding: 12px 14px;
  overflow: auto;
  max-width: 100%;
  box-sizing: border-box;
  border-radius: 12px;
  border: 1px solid #d0d7de;
  background: #f6f8fa;
  color: #24292f;
  white-space: pre;
}

.chat-message-content.markdown-body :deep(pre code) {
  display: block;
  min-width: max-content;
  padding: 0;
  background: transparent;
  color: inherit;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre;
  overflow-wrap: normal;
  word-break: normal;
}

.chat-message-content.markdown-body :deep(code) {
  padding: 0.12em 0.35em;
  border-radius: 6px;
  background: rgba(175, 184, 193, 0.2);
  color: #24292f;
  font-size: 0.95em;
  overflow-wrap: anywhere;
  word-break: break-word;
}

.chat-message-content.markdown-body :deep(.hljs) {
  color: #24292f;
  background: transparent;
}

.chat-message-content.markdown-body :deep(a) {
  color: var(--accent-color);
  text-decoration: none;
  word-break: break-word;
  overflow-wrap: anywhere;
}

.chat-message-content.markdown-body :deep(a:hover) {
  text-decoration: underline;
}

.chat-message-content.markdown-body :deep(hr) {
  border: none;
  border-top: 1px solid rgba(0, 0, 0, 0.1);
  margin: 0.9em 0;
}

.chat-composer {
  flex-shrink: 0;
  border-top: 1px solid rgba(0, 0, 0, 0.06);
  padding-top: 10px;
}

.attachment-strip {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(64px, 1fr));
  gap: 8px;
  margin-bottom: 10px;
}

.attachment-thumb {
  position: relative;
  aspect-ratio: 1 / 1;
  border-radius: 14px;
  overflow: hidden;
  border: 1px solid rgba(0, 0, 0, 0.08);
  background: #fff;
}

.attachment-thumb img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.attachment-remove {
  position: absolute;
  top: 4px;
  right: 4px;
  width: 20px;
  height: 20px;
  border: none;
  border-radius: 999px;
  background: rgba(0, 0, 0, 0.58);
  color: #fff;
  cursor: pointer;
}

.chat-input {
  width: 100%;
  box-sizing: border-box;
  resize: vertical;
  min-height: 76px;
  max-height: 140px;
  border-radius: 12px;
  border: 1px solid var(--border-color);
  padding: 9px 10px;
  font: inherit;
  font-size: 13px;
  color: var(--text-primary);
  background: rgba(255, 255, 255, 0.96);
  outline: none;
}

.chat-input:focus {
  border-color: var(--accent-color);
  box-shadow: 0 0 0 3px rgba(0, 122, 204, 0.12);
}

.composer-actions {
  margin-top: 8px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
}

.file-input {
  display: none;
}

.chat-empty {
  min-height: 180px;
  display: flex;
  align-items: center;
  justify-content: center;
  text-align: center;
  color: var(--text-secondary);
  padding: 20px;
  font-size: 13px;
  line-height: 1.6;
}

.chat-empty.compact {
  min-height: 56px;
  padding: 14px 10px;
  justify-content: flex-start;
  border: 1px dashed rgba(0, 0, 0, 0.08);
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.55);
}

.chat-error {
  margin: 10px 0 0;
  font-size: 12px;
  color: #b42318;
}

.chat-error.selector {
  margin-top: 12px;
}
</style>
