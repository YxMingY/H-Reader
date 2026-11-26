<template>
  <div class="onboarding-guide">
    <div class="guide-content">
      <!-- Logo 和标题 -->
      <div class="guide-header">
        <div class="logo">📚</div>
        <h1>欢迎使用 H-Reader</h1>
        <p class="subtitle">智能 PDF 阅读器与 AI 助手</p>
      </div>

      <!-- 步骤指引 -->
      <div class="steps-container">
        <!-- 步骤 1: 选择文件夹 -->
        <div class="step-card" :class="{ completed: hasDefaultFolder, active: !hasDefaultFolder }">
          <div class="step-number">1</div>
          <div class="step-content">
            <h3>选择书籍文件夹</h3>
            <p v-if="!hasDefaultFolder" class="step-desc">
              选择包含 PDF 文件的文件夹作为默认书库
            </p>
            <p v-else class="step-desc success">
              ✓ 已选择文件夹：{{ folderPath }}
            </p>
            <button 
              v-if="!hasDefaultFolder"
              class="step-btn primary" 
              @click="chooseFolder"
            >
              选择文件夹
            </button>
            <button 
              v-else
              class="step-btn secondary" 
              @click="chooseFolder"
            >
              更改文件夹
            </button>
          </div>
        </div>

        <!-- 步骤 2: 配置 AI 模型 -->
        <div class="step-card">
          <div class="step-number">2</div>
          <div class="step-content">
            <h3>配置 AI 模型（可选）</h3>
            <p class="step-desc">
              如需使用 AI 对话，可在设置中添加你自己的 OpenAI 兼容服务，填写 API 地址、API Key 和模型。也可以稍后配置。
            </p>
            <p v-if="hasApiKey" class="step-desc success">✓ 已保存 API Key</p>
            <button class="step-btn secondary" @click="openSettings">
              打开设置
            </button>
          </div>
        </div>

        <!-- 步骤 3: 开始使用 -->
        <div class="step-card" :class="{ completed: isReady, active: isReady }">
          <div class="step-number">3</div>
          <div class="step-content">
            <h3>开始阅读</h3>
            <p v-if="!isReady" class="step-desc">
              选择书籍文件夹后即可进入书架
            </p>
            <p v-else class="step-desc success">
              ✓ 一切就绪，享受阅读吧！
            </p>
            <button 
              v-if="isReady"
              class="step-btn success" 
              @click="startUsing"
            >
              进入书架 →
            </button>
          </div>
        </div>
      </div>

      <div v-if="isReady" class="tip-box info">
        <span class="tip-icon">💡</span>
        <div class="tip-text">
          <strong>小贴士：</strong>你可以在右上角的设置中管理 AI 供应商和模型
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
/**
 * OnboardingGuide - 首次启动使用指南组件
 * 
 * 功能说明：
 * - 提供可选的 AI 模型设置入口
 * - 引导用户选择默认文件夹
 * - 检测配置状态并显示相应提示
 */

import { computed } from 'vue';
import { BookService } from '../../../bindings/hreader/services/book';

const props = defineProps({
  /** API Key 是否已设置 */
  hasApiKey: {
    type: Boolean,
    required: true
  },
  /** 默认文件夹路径 */
  defaultFolder: {
    type: String,
    default: ''
  }
});

const emit = defineEmits(['settings-opened', 'folder-chosen', 'ready']);

/** 是否有默认文件夹 */
const hasDefaultFolder = computed(() => {
  return props.defaultFolder && props.defaultFolder.length > 0;
});

/** 文件夹显示路径（截断过长路径） */
const folderPath = computed(() => {
  if (!props.defaultFolder) return '';
  const path = props.defaultFolder;
  if (path.length > 50) {
    return '...' + path.slice(-47);
  }
  return path;
});

/** 是否可以开始使用（只需选择文件夹即可） */
const isReady = computed(() => {
  return hasDefaultFolder.value;
});

/**
 * 打开设置面板
 */
const openSettings = () => {
  emit('settings-opened');
};

/**
 * 选择文件夹
 */
const chooseFolder = async () => {
  try {
    const selectedDir = await BookService.ChooseDir();
    if (selectedDir) {
      emit('folder-chosen', selectedDir);
    }
  } catch (err) {
    console.error('选择文件夹失败:', err);
    alert('选择文件夹失败: ' + err.message);
  }
};

/**
 * 开始使用（关闭引导界面）
 */
const startUsing = () => {
  emit('ready');
};
</script>

<style scoped>
/* --- 使用指南样式 --- */
.onboarding-guide {
  height: 100%;
  min-height: 0;
  /* 移除 overflow-y: auto，使用父容器的滚动 */
  padding: 40px 30px;
  background: var(--bg-color);
  display: flex;
  align-items: flex-start; /* 改为顶部对齐，避免内容被裁剪 */
  justify-content: center;
}

.guide-content {
  max-width: 680px;
  width: 100%;
}

/* --- 头部区域 --- */
.guide-header {
  text-align: center;
  margin-bottom: 48px;
}

.logo {
  font-size: 64px;
  margin-bottom: 16px;
  opacity: 0.9;
}

.guide-header h1 {
  font-size: 32px;
  font-weight: 700;
  color: var(--text-primary);
  margin: 0 0 8px 0;
}

.subtitle {
  font-size: 16px;
  color: var(--text-secondary);
  margin: 0;
}

/* --- 步骤卡片 --- */
.steps-container {
  display: flex;
  flex-direction: column;
  gap: 20px;
  margin-bottom: 32px;
}

.step-card {
  display: flex;
  gap: 20px;
  padding: 24px;
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.9);
  border: 2px solid var(--border-color);
  transition: all 0.3s ease;
}

.step-card.active {
  border-color: var(--accent-color);
  box-shadow: 0 4px 12px rgba(0, 122, 204, 0.15);
  transform: translateY(-2px);
}

.step-card.completed {
  border-color: #52c41a;
  background: rgba(82, 196, 26, 0.05);
}

.step-number {
  flex-shrink: 0;
  width: 40px;
  height: 40px;
  border-radius: 50%;
  background: var(--accent-color);
  color: white;
  font-size: 18px;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
}

.step-card.completed .step-number {
  background: #52c41a;
}

.step-content {
  flex: 1;
  min-width: 0;
}

.step-content h3 {
  font-size: 18px;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0 0 8px 0;
}

.step-desc {
  font-size: 14px;
  color: var(--text-secondary);
  margin: 0 0 16px 0;
  line-height: 1.6;
}

.step-desc.success {
  color: #52c41a;
  font-weight: 500;
}

/* --- 按钮样式 --- */
.step-btn {
  padding: 10px 20px;
  border: none;
  border-radius: 8px;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
}

.step-btn.primary {
  background: var(--accent-color);
  color: white;
  box-shadow: 0 2px 8px rgba(0, 122, 204, 0.2);
}

.step-btn.primary:hover:not(:disabled) {
  background: #0066b3;
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(0, 122, 204, 0.3);
}

.step-btn.primary:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.step-btn.secondary {
  background: #f0f2f5;
  color: var(--text-primary);
  border: 1px solid var(--border-color);
}

.step-btn.secondary:hover {
  background: #e6e8eb;
  border-color: #ccc;
}

.step-btn.success {
  background: linear-gradient(135deg, #52c41a, #389e0d);
  color: white;
  box-shadow: 0 2px 8px rgba(82, 196, 26, 0.3);
}

.step-btn.success:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(82, 196, 26, 0.4);
}

/* --- 提示框 --- */
.tip-box {
  display: flex;
  gap: 12px;
  padding: 16px 20px;
  border-radius: 8px;
  margin-top: 24px;
}

.tip-box.warning {
  background: rgba(255, 193, 7, 0.1);
  border: 1px solid rgba(255, 193, 7, 0.3);
}

.tip-box.info {
  background: rgba(0, 122, 204, 0.08);
  border: 1px solid rgba(0, 122, 204, 0.2);
}

.tip-icon {
  font-size: 20px;
  flex-shrink: 0;
}

.tip-text {
  flex: 1;
  font-size: 14px;
  color: var(--text-primary);
  line-height: 1.6;
}

.tip-text strong {
  font-weight: 600;
}
</style>
