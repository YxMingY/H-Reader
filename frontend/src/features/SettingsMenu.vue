<template>
  <Teleport to="body">
    <Transition name="settings-fade">
      <div v-if="open" class="settings-layer" @click.self="close">
        <div class="settings-panel" role="dialog" aria-modal="true" aria-label="设置菜单">
          <div class="settings-header">
            <div>
              <h3 class="settings-title">设置</h3>
              <p class="settings-subtitle">当前仅支持后端 API_KEY，后续会继续扩展。</p>
            </div>
            <button class="settings-close" type="button" @click="close" aria-label="关闭设置">×</button>
          </div>

          <div class="settings-body">
            <!-- API Key -->
            <label class="settings-label" for="api-key-input">API_KEY</label>
            <input
              id="api-key-input"
              v-model="draftKey"
              class="settings-input"
              type="text"
              spellcheck="false"
              autocomplete="off"
              placeholder="输入后端 API_KEY"
              @keyup.enter="submit"
            />
            <p class="settings-hint">保存后会写入本地 config.json，并在下次启动时用于初始化后端。</p>

            <!-- 模型提供商 -->
            <label class="settings-label" for="provider-select" style="margin-top: 18px;">模型提供商</label>
            <select
              id="provider-select"
              v-model="draftProvider"
              class="settings-select"
              @change="onProviderChange"
            >
              <option value="aliyun">阿里云通义千问</option>
              <option value="deepseek">DeepSeek</option>
              <option value="glm">智谱 GLM</option>
            </select>

            <!-- 具体模型 -->
            <label class="settings-label" for="model-select">具体模型</label>
            <select
              id="model-select"
              v-model="draftModel"
              class="settings-select"
            >
              <option v-for="model in availableModels" :key="model.value" :value="model.value">
                {{ model.label }}
                <span v-if="model.multimodal" class="multimodal-badge">🖼️</span>
              </option>
            </select>
            <p class="settings-hint">选择您要使用的 AI 模型。<span v-if="selectedModel?.multimodal">该模型支持图片理解。</span></p>
          </div>

          <div class="settings-actions">
            <button class="settings-button secondary" type="button" @click="close">取消</button>
            <button class="settings-button primary" type="button" @click="submit">保存</button>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup>
import { ref, watch, computed } from 'vue';

const props = defineProps({
  open: Boolean,
  apiKey: {
    type: String,
    default: '',
  },
  provider: {
    type: String,
    default: 'aliyun',
  },
  model: {
    type: String,
    default: 'qwen3-omni-flash',
  },
});

const emit = defineEmits(['update:open', 'save']);

const draftKey = ref('');
const draftProvider = ref('aliyun');
const draftModel = ref('qwen3-omni-flash');

// 各提供商的可用模型列表
const modelOptions = {
  aliyun: [
    { value: 'qwen3-omni-flash', label: '通义千问 Omni Flash (多模态)', multimodal: true },
    { value: 'qwen3.5-omni-plus', label: '通义千问 3.5 Omni Plus (多模态)', multimodal: true },
    { value: 'qwen-turbo', label: '通义千问 Turbo' },
    { value: 'qwen-plus', label: '通义千问 Plus' },
    { value: 'qwen-max', label: '通义千问 Max' },
  ],
  deepseek: [
    { value: 'deepseek-chat', label: 'DeepSeek Chat' },
    { value: 'deepseek-coder', label: 'DeepSeek Coder' },
  ],
  glm: [
    { value: 'glm-4', label: 'GLM-4 (多模态)', multimodal: true },
    { value: 'glm-4-plus', label: 'GLM-4 Plus (多模态)', multimodal: true },
    { value: 'glm-3-turbo', label: 'GLM-3 Turbo' },
  ],
};

// 当前提供商的可用模型
const availableModels = computed(() => {
  return modelOptions[draftProvider.value] || [];
});

// 当前选中的模型对象
const selectedModel = computed(() => {
  return availableModels.value.find(m => m.value === draftModel.value);
});

watch(
  () => props.open,
  (isOpen) => {
    if (isOpen) {
      draftKey.value = props.apiKey || '';
      draftProvider.value = props.provider || 'aliyun';
      draftModel.value = props.model || 'qwen3-omni-flash';
    }
  }
);

/**
 * 当提供商改变时，自动选择该提供商的第一个模型
 */
const onProviderChange = () => {
  const models = modelOptions[draftProvider.value];
  if (models && models.length > 0) {
    draftModel.value = models[0].value;
  }
};

const close = () => {
  emit('update:open', false);
};

const submit = () => {
  emit('save', {
    apiKey: draftKey.value.trim(),
    provider: draftProvider.value,
    model: draftModel.value,
  });
};
</script>

<style scoped>
.settings-layer {
  position: fixed;
  inset: 0;
  z-index: 1000;
  background: rgba(13, 18, 28, 0.22);
  backdrop-filter: blur(2px);
}

.settings-panel {
  position: fixed;
  top: 72px;
  right: 20px;
  width: min(360px, calc(100vw - 32px));
  border-radius: 16px;
  border: 1px solid rgba(0, 0, 0, 0.08);
  background: rgba(255, 255, 255, 0.96);
  box-shadow: 0 16px 48px rgba(0, 0, 0, 0.18);
  padding: 18px;
  color: var(--text-primary);
}

.settings-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.settings-title {
  margin: 0;
  font-size: 16px;
  line-height: 1.2;
}

.settings-subtitle {
  margin: 6px 0 0;
  font-size: 12px;
  line-height: 1.5;
  color: var(--text-secondary);
}

.settings-close {
  width: 30px;
  height: 30px;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  font-size: 22px;
  line-height: 1;
}

.settings-close:hover {
  background: rgba(0, 0, 0, 0.05);
  color: var(--text-primary);
}

.settings-body {
  margin-top: 18px;
}

.settings-label {
  display: block;
  margin-bottom: 8px;
  font-size: 12px;
  font-weight: 600;
  color: var(--text-secondary);
}

.settings-input {
  width: 100%;
  box-sizing: border-box;
  border: 1px solid var(--border-color);
  border-radius: 10px;
  background: #fff;
  padding: 11px 12px;
  font-size: 14px;
  color: var(--text-primary);
  outline: none;
}

.settings-input:focus {
  border-color: var(--accent-color);
  box-shadow: 0 0 0 3px rgba(0, 122, 204, 0.12);
}

.settings-select {
  width: 100%;
  box-sizing: border-box;
  border: 1px solid var(--border-color);
  border-radius: 10px;
  background: #fff;
  padding: 11px 12px;
  font-size: 14px;
  color: var(--text-primary);
  outline: none;
  cursor: pointer;
  appearance: none;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='12' viewBox='0 0 12 12'%3E%3Cpath fill='%23666' d='M6 8L1 3h10z'/%3E%3C/svg%3E");
  background-repeat: no-repeat;
  background-position: right 12px center;
  padding-right: 36px;
}

.settings-select:focus {
  border-color: var(--accent-color);
  box-shadow: 0 0 0 3px rgba(0, 122, 204, 0.12);
}

.multimodal-badge {
  margin-left: 6px;
  font-size: 14px;
}

.settings-hint {
  margin: 10px 0 0;
  font-size: 12px;
  line-height: 1.5;
  color: var(--text-secondary);
}

.settings-actions {
  margin-top: 18px;
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

.settings-button {
  min-width: 78px;
  height: 34px;
  border: none;
  border-radius: 10px;
  padding: 0 14px;
  font-size: 13px;
  cursor: pointer;
}

.settings-button.secondary {
  background: #f0f2f5;
  color: var(--text-primary);
}

.settings-button.primary {
  background: var(--accent-color);
  color: #fff;
}

.settings-button:hover {
  opacity: 0.95;
}

.settings-fade-enter-active,
.settings-fade-leave-active {
  transition: opacity 0.14s ease;
}

.settings-fade-enter-from,
.settings-fade-leave-to {
  opacity: 0;
}
</style>
