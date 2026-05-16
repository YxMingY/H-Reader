<template>
  <div class="library-view">
    <!-- 首次启动引导界面 -->
    <OnboardingGuide
      v-if="showOnboarding"
      :has-api-key="hasApiKey"
      :default-folder="scanDir"
      @settings-opened="openSettings"
      @folder-chosen="handleFolderChosen"
      @ready="hideOnboarding"
    />

    <!-- 正常书架界面 -->
    <template v-else>
      <!-- API Key 未设置提示 -->
      <div v-if="!hasApiKey" class="api-key-warning">
        <span class="warning-icon">⚠️</span>
        <div class="warning-content">
          <strong>API Key 未配置</strong>
          <span>AI 对话功能暂时不可用。</span>
          <button class="btn-link" @click="openSettings">立即设置</button>
        </div>
      </div>

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
    </template>
  </div>
</template>

<script setup>
/**
 * Bookshelf.vue - 图书书架组件
 * 
 * 展示 PDF 图书列表，支持：
 * - 首次启动引导（设置 API Key 和选择文件夹）
 * - 选择扫描目录
 * - 显示图书卡片
 * - 双击打开图书
 */
import { ref } from 'vue';
import { useLibrary } from './composables';
import { BookGrid } from './components';
import OnboardingGuide from './OnboardingGuide.vue';

// 使用图书库管理 composable
const { books, loading, scanDir, chooseDir, loadLibrary } = useLibrary();

const props = defineProps({
  /** API Key 是否已设置 */
  hasApiKey: {
    type: Boolean,
    default: false
  }
});

const emit = defineEmits(['select', 'settings-opened']);

// 是否显示引导界面（首次启动且未完成配置时显示）
const showOnboarding = ref(false);

/**
 * 检查是否需要显示引导界面
 * 
 * 策略：
 * - 只有当文件夹也未设置时，才显示完整引导界面
 * - 如果只缺少 API Key，显示书架但给出提示（允许使用阅读功能）
 */
const checkOnboarding = (apiKey) => {
  // 只有当文件夹也未设置时，才显示引导界面
  const needsOnboarding = !scanDir.value;
  showOnboarding.value = needsOnboarding;
};

/**
 * 隐藏引导界面
 */
const hideOnboarding = () => {
  showOnboarding.value = false;
};

/**
 * 打开设置面板
 */
const openSettings = () => {
  emit('settings-opened');
};

/**
 * 处理文件夹选择完成
 */
const handleFolderChosen = (folderPath) => {
  // 更新 scanDir（确保 OnboardingGuide 能实时显示）
  scanDir.value = folderPath;
  // 刷新图书列表
  loadLibrary(folderPath);
};

/**
 * 选择图书并触发事件
 * @param {Object} book - 选中的图书记录
 */
const selectBook = (book) => {
  emit('select', book);
};

// 暴露方法供外部调用
defineExpose({
  chooseDir,
  checkOnboarding,  // 暴露检查引导状态的方法
  loadLibrary       // 暴露加载图书的方法
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

/* --- API Key 未设置提示条 --- */
.api-key-warning {
  display: flex;
  gap: 12px;
  padding: 14px 18px;
  margin-bottom: 16px;
  border-radius: 10px;
  background: rgba(255, 193, 7, 0.12);
  border: 1px solid rgba(255, 193, 7, 0.3);
  align-items: flex-start;
}

.warning-icon {
  font-size: 20px;
  flex-shrink: 0;
  line-height: 1.5;
}

.warning-content {
  flex: 1;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  color: var(--text-primary);
  line-height: 1.5;
}

.warning-content strong {
  font-weight: 600;
  color: #d48806;
}

.btn-link {
  padding: 4px 12px;
  border: none;
  background: transparent;
  color: var(--accent-color);
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  text-decoration: underline;
  transition: opacity 0.2s;
}

.btn-link:hover {
  opacity: 0.8;
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
