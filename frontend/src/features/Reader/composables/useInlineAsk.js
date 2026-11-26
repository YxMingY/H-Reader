import { computed, reactive, ref, watch } from 'vue';
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
    return reactive({
      key: ++nextViewKey, id, anchor, messages: [], question: '', error: '',
      path: bookPath.value, version: bookVersion,
      loading: false, loaded: !id, sending: false,
    });
  }

  function openNew(anchor, quote) {
    active.value = createView({ ...anchor, quote });
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
    const question = view.question.trim();
    if (!question || view.sending || !view.loaded || deletedIds.has(view.id)) return;
    view.sending = true;
    view.error = '';

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
      const updated = await ChatService.SendInlineQuestion(view.path, view.id, question);
      view.messages = updated.messages;
      view.question = '';
    } catch (error) {
      view.error = `原地问失败：${error?.message || error}`;
    } finally {
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

  return { active, markersByPage, loadError, openNew, openSaved, close, send, load, remove };
}
