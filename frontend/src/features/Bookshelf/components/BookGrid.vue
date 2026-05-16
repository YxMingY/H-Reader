<template>
  <div class="book-grid">
    <div 
      v-for="book in books" 
      :key="book.id" 
      class="book-card"
      @dblclick="handleSelect(book)"
    >
      <div class="book-cover">
        <div class="icon">📄</div>
      </div>
      <div class="book-meta">
        <div class="title" :title="book.title">{{ book.title }}</div>
      </div>
    </div>
  </div>
</template>

<script setup>
/**
 * BookGrid.vue - 图书网格展示组件
 * 
 * 以网格形式展示图书列表，支持双击选择
 */

defineProps({
  books: {
    type: Array,
    required: true,
    default: () => []
  }
});

const emit = defineEmits(['select']);

/**
 * 处理图书选择
 * @param {Object} book - 选中的图书记录
 */
const handleSelect = (book) => {
  emit('select', book);
};
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
</style>
