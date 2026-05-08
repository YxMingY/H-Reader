<template>
  <div class="library-view">
    <div v-if="loading" class="loading-state">正在扫描书籍...</div>
    
    <div v-else-if="books.length === 0" class="empty-msg">
      <div class="empty-icon">📚</div>
      <p>暂无书籍</p>
      <p class="hint">请把 PDF 放到 Documents/Papers 文件夹</p>
    </div>

    <div class="book-grid" v-else>
      <div 
        v-for="book in books" 
        :key="book.id" 
        class="book-card"
        @dblclick="selectBook(book)"
      >
        <div class="book-cover">
          <div class="icon">📄</div>
        </div>
        <div class="book-meta">
          <div class="title" :title="book.title">{{ book.title }}</div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue';
import { BookService } from '../../bindings/changeme';

const books = ref([]);
const loading = ref(false);

const emit = defineEmits(['select']);

onMounted(async () => {
  await loadLibrary();
});

const loadLibrary = async () => {
  loading.value = true;
  try {
    const result = await BookService.ScanBooks("");
    books.value = result;
  } catch (err) {
    console.error("扫描失败", err);
  } finally {
    loading.value = false;
  }
};

const selectBook = (book) => {
  emit('select', book);
};
</script>

<style scoped>
/* --- 书架样式 --- */
.library-view { 
  height: 100%; 
  overflow-y: auto; 
  padding: 30px; 
  background: var(--bg-color);
}

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

.book-cover {
  width: 80px;
  height: 100px;
  background: #f0f2f5;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 15px;
  font-size: 32px;
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

.empty-msg {
  text-align: center;
  color: var(--text-secondary);
  margin-top: 100px;
}

.empty-icon { 
  font-size: 48px; 
  margin-bottom: 10px; 
  opacity: 0.5; 
}

.hint { 
  font-size: 12px; 
  opacity: 0.7; 
}

.loading-state {
  text-align: center;
  color: var(--text-secondary);
  margin-top: 100px;
}
</style>
