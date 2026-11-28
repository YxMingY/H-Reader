<template>
  <div ref="cover" class="book-cover">
    <img v-if="thumbnail" :src="thumbnail" alt="">
    <span v-else aria-hidden="true">📄</span>
  </div>
</template>

<script>
import * as pdfjs from 'pdfjs-dist';
import { BookService } from '../../../../bindings/hreader/services/book';

// Limit concurrent full-file reads and keep only a bounded set of previews.
let previewQueue = Promise.resolve();
const maxPreviews = 200;
</script>

<script setup>
import { onBeforeUnmount, onMounted, ref } from 'vue';

const props = defineProps({
  path: { type: String, required: true },
  cache: { type: Map, required: true },
});

const cover = ref(null);
const thumbnail = ref('');
let observer;
let mounted = true;

onMounted(() => {
  observer = new IntersectionObserver(entries => {
    if (!entries.some(entry => entry.isIntersecting)) return;
    observer.disconnect();
    previewQueue = previewQueue.then(async () => {
      if (!mounted) return;

      const { path, cache } = props;
      if (cache.has(path)) {
        thumbnail.value = cache.get(path);
        return;
      }

      let task;
      try {
        const encoded = await BookService.LoadPDF(path);
        if (!mounted) return;
        const data = Uint8Array.from(atob(encoded), character => character.charCodeAt(0));
        task = pdfjs.getDocument({ data, isEvalSupported: false });
        const document = await task.promise;
        if (!mounted) return;
        const page = await document.getPage(1);
        const original = page.getViewport({ scale: 1 });
        const viewport = page.getViewport({ scale: Math.min(180 / original.width, 228 / original.height) });
        const canvas = window.document.createElement('canvas');
        canvas.width = Math.ceil(viewport.width);
        canvas.height = Math.ceil(viewport.height);
        await page.render({ canvas, viewport }).promise;
        const image = canvas.toDataURL('image/jpeg', 0.82);
        if (cache.size >= maxPreviews) cache.delete(cache.keys().next().value);
        cache.set(path, image);
        if (mounted) thumbnail.value = image;
      } catch {
        // The file card remains usable when the first page cannot be rendered.
      } finally {
        await task?.destroy().catch(() => {});
      }
    });
  }, { rootMargin: '160px' });
  observer.observe(cover.value);
});

onBeforeUnmount(() => {
  mounted = false;
  observer?.disconnect();
});
</script>

<style scoped>
.book-cover {
  width: 90px;
  height: 114px;
  background: #f0f2f5;
  border-radius: 8px;
  display: grid;
  place-items: center;
  margin-bottom: 15px;
  overflow: hidden;
  font-size: 32px;
}
img { display: block; width: 100%; height: 100%; object-fit: contain; }
</style>
