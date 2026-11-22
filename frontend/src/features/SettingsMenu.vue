<template>
  <Teleport to="body">
    <Transition name="settings-fade">
      <div v-if="open" class="settings-layer" @click.self="close">
        <div class="settings-panel" role="dialog" aria-modal="true" aria-label="设置菜单">
          <div class="settings-header">
            <div>
              <h3 class="settings-title">设置</h3>
              <p class="settings-subtitle">当前仅部分模型，后续会继续扩展。</p>
            </div>
            <button class="settings-close" type="button" @click="close" aria-label="关闭设置">×</button>
          </div>

          <div class="settings-body">
            <!-- API Key -->
            <label class="settings-label" for="api-key-input">API_KEY（{{ getProviderLabel(draftProvider) }}）</label>
            <input
              id="api-key-input"
              :value="currentDraftKey"
              @input="updateCurrentDraftKey($event.target.value)"
              class="settings-input"
              type="text"
              spellcheck="false"
              autocomplete="off"
              placeholder="输入后端 API_KEY"
              @keyup.enter="submit"
            />
            <p class="settings-hint">保存后会写入本地 config.json，每个提供商可以有独立的 API Key。</p>

            <!-- 模型提供商 -->
            <label class="settings-label" for="provider-select" style="margin-top: 18px;">模型提供商</label>
            <select
              id="provider-select"
              v-model="draftProvider"
              class="settings-select"
              @change="onProviderChange"
            >
              <option value="aliyun">阿里云百炼</option>
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
              <option value="__custom__">✏️ 自定义模型...</option>
            </select>
            
            <!-- 自定义模型输入框 -->
            <div v-if="draftModel === '__custom__'" class="custom-model-input">
              <input
                v-model="customModelName"
                class="settings-input"
                type="text"
                spellcheck="false"
                autocomplete="off"
                placeholder="输入自定义模型名称"
                @keyup.enter="submit"
              />
              <p class="settings-hint">输入您要使用的自定义模型名称。</p>
            </div>
            
            <p v-else class="settings-hint">选择您要使用的 AI 模型。<span v-if="selectedModel?.multimodal">该模型支持图片理解。</span></p>
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
  apiKeys: {
    type: Object,
    default: () => ({
      aliyun: '',
      deepseek: '',
      glm: '',
    }),
  },
  provider: {
    type: String,
    default: 'aliyun',
  },
  model: {
    type: String,
    default: 'qwen3.8-flash',
  },
});

const emit = defineEmits(['update:open', 'save']);

const draftApiKeys = ref({
  aliyun: '',
  deepseek: '',
  glm: '',
});
const draftProvider = ref('aliyun');
const draftModel = ref('qwen3.8-flash');
const customModelName = ref('');

// 各提供商的可用模型列表
const modelOptions = {
  aliyun: [
    { value: 'qwen3.8-flash', label: 'Qwen 3.8 Flash', multimodal: true },
    { value: 'qwen4-max', label: 'Qwen 4 Max', multimodal: true },
    { value: 'qwen3.8-max-0902', label: 'Qwen 3.8 Max (0902)' },
    { value: 'qwen3.7-plus', label: 'Qwen 3.7 Plus', multimodal: true },
    { value: 'kimi-k3.1-pro', label: 'Kimi K3.1 Pro' },
    { value: 'deepseek-v4.1-flash-1017', label: 'DeepSeek V4.1 Flash (1017)' },
    { value: 'deepseek-v4.1-flash', label: 'DeepSeek V4.1 Flash' },
    { value: 'glm-5.3', label: 'GLM-5.3' },
  ],
  deepseek: [
    { value: 'deepseek-flash', label: 'DeepSeek V4.1 Flash', multimodal: true },
    { value: 'deepseek-v4-pro', label: 'DeepSeek V4 Pro' },
  ],
  glm: [
    { value: 'glm-5.3-flash', label: 'GLM-5.3 Flash', multimodal: true },
    { value: 'glm-5.3-flashx', label: 'GLM-5.3 FlashX', multimodal: true },
    { value: 'glm-5.3', label: 'GLM-5.3' },
    { value: 'glm-5.2', label: 'GLM-5.2' },
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
      // 加载所有提供商的 API Keys
      draftApiKeys.value = {
        aliyun: props.apiKeys?.aliyun || '',
        deepseek: props.apiKeys?.deepseek || '',
        glm: props.apiKeys?.glm || '',
      };
      draftProvider.value = props.provider || 'aliyun';
      draftModel.value = props.model || 'qwen3.8-flash';
      // 检查当前模型是否在预设列表中，如果不在则设为自定义
      const isInList = availableModels.value.some(m => m.value === draftModel.value);
      if (!isInList && draftModel.value) {
        customModelName.value = draftModel.value;
        draftModel.value = '__custom__';
      } else {
        customModelName.value = '';
      }
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
    customModelName.value = '';
  }
};

/**
 * 获取当前 provider 对应的 draft key
 */
const currentDraftKey = computed(() => {
  return draftApiKeys.value[draftProvider.value] || '';
});

/**
 * 更新当前 provider 的 draft key
 */
const updateCurrentDraftKey = (value) => {
  draftApiKeys.value[draftProvider.value] = value;
};

/**
 * 获取提供商的中文标签
 */
const getProviderLabel = (provider) => {
  const labels = {
    aliyun: '阿里云',
    deepseek: 'DeepSeek',
    glm: '智谱 GLM',
  };
  return labels[provider] || provider;
};

const close = () => {
  emit('update:open', false);
};

const submit = () => {
  // 如果是自定义模型，使用输入的模型名称
  const finalModel = draftModel.value === '__custom__' ? customModelName.value.trim() : draftModel.value;
  
  if (draftModel.value === '__custom__' && !finalModel) {
    alert('请输入自定义模型名称');
    return;
  }
  
  emit('save', {
    apiKeys: { ...draftApiKeys.value },
    provider: draftProvider.value,
    model: finalModel,
  });
  // 保存后自动关闭设置菜单
  close();
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

.custom-model-input {
  margin-top: 10px;
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
