<template>
  <div class="library-view">
    <div class="folder-bar">
      <div class="folder-path" :title="scanDir || '未选择文件夹'">
        {{ scanDir || '未选择文件夹' }}
      </div>
      <button class="folder-btn" @click="chooseDir">选择文件夹</button>
    </div>

    <div v-if="loading" class="loading-state">正在扫描书籍...</div>
    
    <div v-else-if="books.length === 0" class="empty-msg">
      <div class="empty-icon">📚</div>
      <p>暂无书籍</p>
      <p class="hint">在{{ scanDir }} 文件夹找不到PDF文件喵>_< </p>
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
const scanDir = ref("");

const emit = defineEmits(['select']);

onMounted(async () => {
  scanDir.value = await BookService.GetScanDir();
  console.log("扫描目录:", scanDir.value);
  await loadLibrary();
});

const chooseDir = async () => {
  try {
    scanDir.value = await BookService.ChooseDir();
    if (scanDir.value) {
      console.log("选择的文件夹:", scanDir.value);
      await loadLibrary(scanDir.value);
    }
  } catch (err) {
    console.error("选择文件夹失败", err);
  }
};

const loadLibrary = async (dir) => {
  loading.value = true;
  try {
    const result = await BookService.ScanBooks(dir);
    books.value = result;
    console.log("扫描结果:", books.value);
  } catch (err) {
    console.error("扫描失败", err);
  } finally {
    loading.value = false;
  }
};

const selectBook = (book) => {
  emit('select', book);
};

defineExpose({
  chooseDir
});
</script>

<style scoped>
/* --- 书架样式 --- */
.library-view { 
  height: 100%; 
  min-height: 0;
  overflow-y: auto; 
  padding: 24px 30px 30px; 
  background: var(--bg-color);
  box-sizing: border-box;
}

.folder-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 20px;
  padding: 12px 16px;
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.78);
  border: 1px solid rgba(0, 0, 0, 0.06);
  box-shadow: var(--shadow-sm);
  backdrop-filter: blur(10px);
}

.folder-path {
  flex: 1;
  min-width: 0;
  font-size: 13px;
  color: var(--text-secondary);
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.folder-btn {
  flex-shrink: 0;
  min-width: 96px;
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: none;
  height: 34px;
  padding: 0 14px;
  border-radius: 8px;
  background: var(--accent-color);
  color: white;
  font-size: 13px;
  line-height: 1;
  white-space: nowrap;
  word-break: keep-all;
  cursor: pointer;
  transition: transform 0.15s ease, opacity 0.15s ease, box-shadow 0.15s ease;
  box-shadow: 0 4px 10px rgba(0, 122, 204, 0.18);
}

.folder-btn:hover {
  transform: translateY(-1px);
  opacity: 0.95;
}

.folder-btn:active {
  transform: translateY(0);
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
