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

    <!-- 使用 BookGrid 组件展示图书列表 -->
    <BookGrid v-else :books="books" @select="selectBook" />
  </div>
</template>

<script setup>
/**
 * Bookshelf.vue - 图书书架组件
 * 
 * 展示 PDF 图书列表，支持：
 * - 选择扫描目录
 * - 显示图书卡片
 * - 双击打开图书
 */
import { useLibrary } from './composables';
import { BookGrid } from './components';

// 使用图书库管理 composable
const { books, loading, scanDir, chooseDir } = useLibrary();

const emit = defineEmits(['select']);

/**
 * 选择图书并触发事件
 * @param {Object} book - 选中的图书记录
 */
const selectBook = (book) => {
  emit('select', book);
};

defineExpose({
  chooseDir  // 暴露选择目录方法供外部调用
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

.hint { 
  font-size: 12px; 
  opacity: 0.7; 
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

.loading-state {
  text-align: center;
  color: var(--text-secondary);
  margin-top: 100px;
}
</style>
