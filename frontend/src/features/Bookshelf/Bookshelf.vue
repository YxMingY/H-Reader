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

      <div v-if="loading" class="loading-state">正在扫描书籍…</div>

      <div v-else class="library-layout">
        <nav class="folder-nav" aria-label="书架目录">
          <button type="button" :aria-current="!activeFolder ? 'true' : undefined"
            @click="activeFolder = ''">全部 PDF</button>
          <div v-for="folder in visibleFolders" :key="folder.path" class="folder-row"
            :style="{ paddingInlineStart: (folder.parts.length - 1) * 12 + 'px' }">
            <button v-if="folder.hasChildren" type="button" class="folder-toggle"
              :aria-expanded="expandedFolders.has(folder.path)" :aria-label="folder.name + '的子目录'"
              @click="toggleFolder(folder.path)">{{ expandedFolders.has(folder.path) ? '▾' : '▸' }}</button>
            <span v-else class="folder-toggle"></span>
            <button type="button" :title="folder.path"
              :aria-current="activeFolder === folder.path ? 'true' : undefined"
              @click="activeFolder = folder.path">{{ folder.name }}</button>
          </div>
        </nav>

        <section class="library-results" aria-label="书架结果">
          <div class="library-tools">
            <input v-model="query" type="search" aria-label="搜索 PDF 文件名" placeholder="搜索 PDF 文件名">
            <label><input v-model="searchAll" type="checkbox"> 搜索全书架</label>
            <button type="button" :aria-pressed="listView" @click="listView = !listView">列表视图</button>
          </div>
          <p class="location" aria-live="polite">
            {{ searchingEverywhere ? '全部 PDF' : activeFolder || '全部 PDF' }} · {{ visibleBooks.length }} 份
          </p>
          <BookGrid v-if="visibleBooks.length" :books="visibleBooks" :cover-cache="previewCache" :list-view="listView" @select="selectBook" />
          <p v-else class="empty-msg">
            {{ searchTerm ? '没有匹配的 PDF' : books.length ? '此目录下没有 PDF，可展开查看子目录' : '扫描目录中没有 PDF' }}
          </p>
        </section>
      </div>
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
import { computed, ref, shallowRef, watch } from 'vue';
import { useLibrary } from './composables';
import { BookGrid } from './components';
import OnboardingGuide from './OnboardingGuide.vue';

// 使用图书库管理 composable
const { books, loading, scanDir, chooseDir, loadLibrary } = useLibrary();

const previewCache = shallowRef(new Map());
watch(books, () => {
  previewCache.value = new Map();
});

const activeFolder = ref('');
const expandedFolders = ref(new Set());
const query = ref('');
const searchAll = ref(false);
const listView = ref(false);
const searchTerm = computed(() => query.value.trim().toLowerCase());
const searchingEverywhere = computed(() => Boolean(searchTerm.value && searchAll.value));

watch(scanDir, () => {
  activeFolder.value = '';
  expandedFolders.value = new Set();
  query.value = '';
  searchAll.value = false;
});

const libraryBooks = computed(() => {
  // Use the native path separator so Unix filenames can contain backslashes.
  const separator = books.value[0]?.path.startsWith('/') ? '/' : '\\';
  const normalizedRoot = separator === '/' ? scanDir.value : scanDir.value.replaceAll('/', separator);
  const root = normalizedRoot.endsWith(separator) ? normalizedRoot : normalizedRoot + separator;
  return books.value.map(book => {
    const pathParts = book.path.split(separator);
    const filename = pathParts.at(-1);
    const parts = book.path.startsWith(root) ? book.path.slice(root.length).split(separator).slice(0, -1) : [];
    return { book, parts, folder: parts.join('/'), filename };
  });
});

const folders = computed(() => {
  const entries = new Map();
  for (const { parts } of libraryBooks.value) {
    parts.forEach((name, index) => {
      const ancestors = parts.slice(0, index);
      const path = parts.slice(0, index + 1).join('/');
      entries.set(path, { name, path, parts: parts.slice(0, index + 1), ancestors, hasChildren: index < parts.length - 1 || entries.get(path)?.hasChildren });
    });
  }
  return [...entries.values()];
});

const visibleFolders = computed(() => folders.value.filter(folder =>
  folder.ancestors.every((_, index) => expandedFolders.value.has(folder.parts.slice(0, index + 1).join('/')))
));

function toggleFolder(path) {
  const expanded = expandedFolders.value;
  if (expanded.has(path)) expanded.delete(path);
  else expanded.add(path);
}

const visibleBooks = computed(() => libraryBooks.value.filter(({ folder, filename }) => {
  const inScope = !activeFolder.value || searchingEverywhere.value || folder === activeFolder.value ||
    (searchTerm.value && folder.startsWith(activeFolder.value + '/'));
  return inScope && filename.toLowerCase().includes(searchTerm.value);
}));

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
 * 
 * @param {string} status - 'has-key' 表示有 API Key，空字符串表示没有
 */
const checkOnboarding = (status) => {
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

.library-layout { display: grid; grid-template-columns: minmax(130px, 23%) minmax(0, 1fr); gap: 20px; }
.folder-nav { border-inline-end: 1px solid #ddd; padding-inline-end: 12px; }
.folder-row { display: flex; align-items: center; }
.folder-nav button, .library-tools button {
  border: 1px solid transparent;
  border-radius: 6px;
  background: transparent;
  color: var(--text-primary);
  text-align: start;
  cursor: pointer;
  padding: 6px;
}
.folder-row button:not(.folder-toggle) { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.folder-toggle { width: 24px; flex: 0 0 24px; }
.folder-nav button:hover, .library-tools button:hover,
.folder-nav [aria-current="true"], .library-tools [aria-pressed="true"] {
  border-color: var(--accent-color);
  color: var(--accent-color);
}
.library-tools { display: flex; flex-wrap: wrap; align-items: center; gap: 10px; }
.library-tools > input { min-width: 120px; flex: 1; padding: 8px; border: 1px solid #aaa; border-radius: 6px; }
.library-tools label, .location { font-size: 13px; color: var(--text-secondary); }
.location { overflow-wrap: anywhere; }
@media (max-width: 560px) {
  .library-layout { grid-template-columns: minmax(0, 1fr); }
  .folder-nav { border-inline-end: none; max-height: 130px; overflow-y: auto; }
}

.empty-msg {
  text-align: center;
  color: var(--text-secondary);
  margin-top: 48px;
}

.loading-state {
  text-align: center;
  color: var(--text-secondary);
  margin-top: 100px;
}
</style>
