import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { NoteService } from '../../../../bindings/hreader/services/book';

export function useNotes(bookPath) {
  const notes = ref([]);
  const editing = ref(null);
  const saving = ref(false);
  const saveError = ref('');
  const loadError = ref('');
  let loadPromise;
  let savePromise;

  watch(bookPath, (path, _, onCleanup) => {
    let stale = false;
    onCleanup(() => { stale = true; });
    notes.value = [];
    loadError.value = '';
    loadPromise = path ? NoteService.GetNotes(path).then(result => {
      if (!stale) notes.value = result;
    }).catch(error => {
      if (!stale) loadError.value = `读取笔记失败：${error?.message || error}`;
    }) : undefined;
  }, { immediate: true });

  const notesByPage = computed(() => {
    const groups = new Map();
    const entries = editing.value?.isNew ? [...notes.value, editing.value.note] : notes.value;
    for (const note of entries) {
      if (!groups.has(note.pageNum)) groups.set(note.pageNum, []);
      groups.get(note.pageNum).push(note);
    }
    return groups;
  });

  function flushNote() {
    if (savePromise) return savePromise;
    const current = editing.value;
    if (!current) return Promise.resolve(true);

    if ((current.isNew && !current.text.trim()) ||
        (!current.isNew && current.text === current.note.text)) {
      editing.value = null;
      return Promise.resolve(true);
    }

    saving.value = true;
    saveError.value = '';
    savePromise = (async () => {
      await loadPromise;
      try {
        const saved = await NoteService.SaveNote(current.path, { ...current.note, text: current.text });
        if (current.path === bookPath.value) {
          notes.value = [...notes.value.filter(note => note.id !== saved.id), saved];
        }
        if (editing.value === current) editing.value = null;
        return true;
      } catch (error) {
        saveError.value = `保存笔记失败：${error?.message || error}`;
        return false;
      }
    })().finally(() => {
      saving.value = false;
      savePromise = null;
    });
    return savePromise;
  }

  async function editNote(note, isNew = false) {
    if (editing.value?.note.id === note.id) return true;
    const path = bookPath.value;
    if (!path || !await flushNote() || path !== bookPath.value) return false;
    saveError.value = '';
    editing.value = { path, note, text: note.text, isNew };
    return true;
  }

  function addNote(anchor, quote = '', { text = '', inlineConversationId = '' } = {}) {
    const id = crypto.randomUUID?.() ?? Array.from(crypto.getRandomValues(new Uint32Array(4))).join('-');
    return editNote({ ...anchor, id, text, quote, inlineConversationId, createdAt: 0, updatedAt: 0 }, true);
  }

  async function removeNote(note) {
    const path = bookPath.value;
    if (!path) return;
    if (editing.value?.note.id === note.id) {
      await savePromise;
      if (editing.value?.note.id === note.id && editing.value.isNew) {
        editing.value = null;
        return;
      }
    }
    await loadPromise;
    await NoteService.RemoveNote(path, note.id);
    if (path === bookPath.value) {
      notes.value = notes.value.filter(item => item.id !== note.id);
      if (editing.value?.note.id === note.id) editing.value = null;
    }
  }

  async function detachInlineNoteLinks(id) {
    const path = bookPath.value;
    await loadPromise;
    if (path !== bookPath.value) return;
    notes.value = notes.value.map(note => note.inlineConversationId === id
      ? { ...note, inlineConversationId: '' }
      : note);
    if (editing.value?.note.inlineConversationId === id) {
      editing.value.note = { ...editing.value.note, inlineConversationId: '' };
    }
  }

  function saveOnOutsideClick(event) {
    if (editing.value && !event.target.closest?.('.pdf-note-editor, .pdf-note-marker[data-note-editing]')) void flushNote();
  }

  onMounted(() => document.addEventListener('pointerdown', saveOnOutsideClick, true));
  onBeforeUnmount(() => document.removeEventListener('pointerdown', saveOnOutsideClick, true));

  return { notesByPage, editing, saving, saveError, loadError, addNote, editNote, removeNote, flushNote, detachInlineNoteLinks };
}
