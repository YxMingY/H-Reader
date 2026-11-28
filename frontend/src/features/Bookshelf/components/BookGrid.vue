<template>
  <div class="book-grid" :class="{ 'list-view': listView }">
    <div v-for="{ book, folder, filename } in books" :key="book.id" class="book-card"
      role="button" tabindex="0" :aria-label="folder ? folder + '/' + filename : filename"
      @dblclick="$emit('select', book)" @keydown.enter.prevent="$emit('select', book)"
      @keydown.space.prevent="$emit('select', book)">
      <BookCover v-if="!listView" :path="book.path" :cache="coverCache" />
      <div class="book-meta">
        <div class="title" :title="filename">{{ listView ? filename : book.title }}</div>
        <div class="book-folder" :title="folder">{{ folder || '扫描根目录' }}</div>
      </div>
    </div>
  </div>
</template>

<script setup>
import BookCover from './BookCover.vue';

defineProps({
  books: { type: Array, required: true },
  coverCache: { type: Map, required: true },
  listView: { type: Boolean, default: false },
});

defineEmits(['select']);
</script>

<style scoped>
.book-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  gap: 25px;
}

.book-card {
  background: white;
  border-radius: 12px;
  padding: 15px;
  cursor: pointer;
  transition: transform 0.2s, box-shadow 0.2s;
  border: 1px solid transparent;
  display: flex;
  flex-direction: column;
  align-items: center;
  box-shadow: var(--shadow-sm);
}

.book-card:hover {
  transform: translateY(-5px);
  box-shadow: var(--shadow-md);
  border-color: var(--accent-color);
}

.book-meta {
  text-align: center;
  width: 100%;
}

.title {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-primary);
  word-break: break-all;
  display: -webkit-box;
  line-clamp: 2;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.book-folder {
  margin-top: 4px;
  color: var(--text-secondary);
  font-size: 12px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.list-view { grid-template-columns: minmax(0, 1fr); gap: 6px; }
.list-view .book-card:hover { transform: none; }
.list-view .book-meta { text-align: start; }
.list-view .title, .list-view .book-folder { display: block; white-space: normal; overflow-wrap: anywhere; }
</style>
