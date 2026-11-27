import { computed, reactive, ref, watch } from 'vue';
import { Events } from '@wailsio/runtime';
import { useChatInput } from '../../ChatPanel/composables/useChatInput';
import { ChatService } from '../../../../bindings/hreader/services/chat';

export function useInlineAsk(bookPath) {
  const active = ref(null);
  const markers = ref([]);
  const loadError = ref('');
  const sessions = new Map();
  const deletedIds = new Set();
  let bookVersion = 0;
  let nextViewKey = 0;

  watch(bookPath, async (path) => {
    const version = ++bookVersion;
    active.value = null;
    markers.value = [];
    sessions.clear();
    deletedIds.clear();
    loadError.value = '';
    if (!path) return;
    try {
      const saved = await ChatService.ListInlineConversations(path);
      if (version !== bookVersion) return;
      const known = new Map(saved.filter(marker => !deletedIds.has(marker.id)).map(marker => [marker.id, marker]));
      for (const marker of markers.value) known.set(marker.id, marker);
      markers.value = [...known.values()];
    } catch (error) {
      if (version === bookVersion) loadError.value = `读取原地问标记失败：${error?.message || error}`;
    }
  }, { immediate: true });

  const markersByPage = computed(() => {
    const groups = new Map();
    for (const marker of markers.value) {
      if (!groups.has(marker.pageNum)) groups.set(marker.pageNum, []);
      groups.get(marker.pageNum).push(marker);
    }
    return groups;
  });

  function createView(anchor, id = '') {
    const sending = ref(false);
    const pasteFeedback = ref('');
    const { attachments, addAttachmentFromDataUrl, handlePaste, removeAttachment } = useChatInput(sending, pasteFeedback);
    const processingPaste = ref(false);

    async function pasteImage(event) {
      if (sending.value || processingPaste.value) {
        event.preventDefault();
        return;
      }
      processingPaste.value = true;
      try {
        await handlePaste(event);
      } finally {
        processingPaste.value = false;
      }
    }

    return reactive({
      key: ++nextViewKey, id, anchor, messages: [], question: '', error: '',
      path: bookPath.value, version: bookVersion,
      loading: false, loaded: !id, sending,
      attachments, addAttachmentFromDataUrl, pasteImage, removeAttachment, pasteFeedback, processingPaste, pending: null,
    });
  }

  function openNew(anchor, quote) {
    active.value = createView({ ...anchor, quote });
  }

  async function openScreenshot(anchor, dataUrl) {
    const view = createView({ ...anchor, quote: '' });
    await view.addAttachmentFromDataUrl(dataUrl);
    if (view.version === bookVersion) active.value = view;
  }

  async function load(view) {
    if (view.loading || view.loaded) return;
    view.loading = true;
    view.error = '';
    try {
      const saved = await ChatService.LoadInlineConversation(view.path, view.id);
      view.anchor = saved.anchor;
      view.messages = saved.messages;
      view.loaded = true;
    } catch (error) {
      view.error = `读取对话失败：${error?.message || error}`;
    } finally {
      view.loading = false;
    }
  }

  function openSaved(marker) {
    if (deletedIds.has(marker.id)) return;
    let view = sessions.get(marker.id);
    if (!view) {
      const { id, pageNum, x, y } = marker;
      view = createView({ pageNum, x, y, quote: '' }, id);
      sessions.set(id, view);
    }
    active.value = view;
    void load(view);
  }

  async function send(view) {
    const images = view.attachments.map(image => image.compressedDataUrl);
    const question = view.question.trim() || (images.length ? '请解释图片' : '');
    if (!question || view.sending || view.processingPaste || !view.loaded || deletedIds.has(view.id)) return;
    view.sending = true;
    view.error = '';
    view.pasteFeedback = '';
    let unsubscribe;

    try {
      if (!view.id) {
        const created = await ChatService.CreateInlineConversation(view.path, view.anchor);
        view.id = created.id;
        if (view.version === bookVersion) {
          sessions.set(view.id, view);
          const { pageNum, x, y } = created.anchor;
          markers.value = [...markers.value, { id: view.id, pageNum, x, y }];
        }
      }
      const requestID = Array.from(crypto.getRandomValues(new Uint32Array(4))).join('-');
      view.pending = {
        question: { role: 'user', content: question, attachments: images },
        answer: { role: 'assistant', content: '', streaming: true },
      };
      unsubscribe = Events.On(`inline_chat_chunk:${requestID}`, event => {
        view.pending.answer.content += event.data;
      });
      const updated = await ChatService.StreamInlineQuestion(view.path, view.id, question, images, requestID);
      view.messages = updated.messages;
      view.question = '';
      view.attachments = [];
    } catch (error) {
      view.error = `原地问失败：${error?.message || error}`;
    } finally {
      unsubscribe?.();
      view.pending = null;
      view.sending = false;
    }
  }

  async function remove(marker) {
    const path = bookPath.value;
    const version = bookVersion;
    await ChatService.DeleteInlineConversation(path, marker.id);
    if (version !== bookVersion || path !== bookPath.value) return false;
    deletedIds.add(marker.id);
    sessions.delete(marker.id);
    markers.value = markers.value.filter(item => item.id !== marker.id);
    if (active.value?.id === marker.id) close();
    return true;
  }

  function close() {
    active.value = null;
  }

  return { active, markersByPage, loadError, openNew, openScreenshot, openSaved, close, send, load, remove };
}
