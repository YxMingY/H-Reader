<template>
  <Teleport to="body">
    <Transition name="settings-fade">
      <div v-if="open" class="settings-layer" @click.self="close">
        <div class="settings-panel" role="dialog" aria-modal="true" aria-label="设置">
          <div class="settings-header">
            <h3 class="settings-title">设置</h3>
            <button class="settings-close" type="button" :disabled="saving" @click="close" aria-label="关闭设置">×</button>
          </div>

          <p v-if="loading" class="settings-hint" role="status">正在读取设置…</p>
          <p v-if="error && !editing" class="settings-error" role="alert">{{ error }}</p>

          <template v-if="!loading">
            <section class="settings-section">
              <h4>供应商配置</h4>
              <ul v-if="settings.profiles.length" class="profile-list">
                <li v-for="profile in settings.profiles" :key="profile.id" class="profile-row">
                  <div>
                    <strong>{{ profile.name }}</strong>
                    <small>{{ providers[profile.provider] || profile.provider }} · {{ profile.models.length }} 个模型 · {{ profile.hasKey ? '已设置 Key' : '未设置 Key' }}</small>
                  </div>
                  <button class="settings-button secondary" type="button" :disabled="saving" @click="editProfile(profile)" :aria-label="`编辑 ${profile.name}`">编辑</button>
                </li>
              </ul>
              <button v-if="!editing" class="settings-button secondary" type="button" :disabled="saving" @click="editProfile()">新增供应商</button>

              <form v-if="editing" class="profile-editor" @submit.prevent="submitProfile">
                <label class="settings-label" for="profile-name">配置名称</label>
                <input id="profile-name" v-model.trim="draft.name" class="settings-input" :disabled="saving" required />

                <label class="settings-label" for="profile-provider">供应商</label>
                <select id="profile-provider" v-model="draft.provider" class="settings-select" :disabled="saving" @change="changeProvider">
                  <option v-if="!providers[draft.provider]" :value="draft.provider">{{ draft.provider }}</option>
                  <option v-for="(label, provider) in providers" :key="provider" :value="provider" :disabled="providerUnavailable(provider)">{{ label }}</option>
                </select>

                <template v-if="draft.provider === 'custom'">
                  <label class="settings-label" for="profile-base-url">API 地址</label>
                  <input id="profile-base-url" v-model.trim="draft.baseUrl" class="settings-input" type="url" placeholder="https://example.com/v1" :disabled="saving" required />
                  <p class="settings-hint">支持 OpenAI 兼容的 Chat Completions 接口。</p>
                </template>

                <label class="settings-label" for="profile-api-key">API Key</label>
                <input
                  id="profile-api-key"
                  v-model.trim="draft.apiKey"
                  class="settings-input"
                  type="password"
                  autocomplete="off"
                  spellcheck="false"
                  :placeholder="canKeepKey ? '留空保留已保存的 Key' : '输入 API Key'"
                  :required="!canKeepKey"
                  :disabled="saving"
                />
                <label class="settings-label" for="new-model-name">模型列表</label>
                <ul v-if="draft.models.length" class="profile-list">
                  <li v-for="model in draft.models" :key="model" class="profile-row">
                    <small>{{ model }}</small>
                    <button type="button" class="settings-button secondary" :disabled="saving" :aria-label="`删除模型 ${model}`" @click="removeModel(model)">删除</button>
                  </li>
                </ul>
                <div class="model-input-row">
                  <input id="new-model-name" v-model.trim="modelName" class="settings-input" placeholder="输入模型名称" spellcheck="false" :disabled="saving" @keydown.enter.prevent="addModel" />
                  <button type="button" class="settings-button secondary" :disabled="saving || !canAddModel" @click="addModel">添加</button>
                </div>
                <select v-if="availablePresets.length" class="settings-select preset-select" aria-label="添加预置模型" :disabled="saving" @change="addPreset">
                  <option value="">添加预置模型</option>
                  <option v-for="model in availablePresets" :key="model.value" :value="model.value">{{ model.label }}</option>
                </select>
                <p v-if="clearedRoutes.length" class="settings-error" role="status">保存后，{{ clearedRoutes.join('、') }}需要重新选择模型。</p>
                <p v-if="error" class="settings-error" role="alert">{{ error }}</p>
                <div class="settings-actions">
                  <button class="settings-button secondary" type="button" :disabled="saving" @click="dismissEditor">取消编辑</button>
                  <button class="settings-button primary" type="submit" :disabled="saving">保存配置</button>
                </div>
              </form>
            </section>

            <section class="settings-section">
              <h4>对话使用的配置</h4>
              <div v-for="route in routes" :key="route.id" class="routing-entry">
                <label class="settings-label" :for="`${route.id}-api-profile`">{{ route.label }}供应商</label>
                <select :id="`${route.id}-api-profile`" v-model="choices[route.id].profileId" class="settings-select" :disabled="saving || editing" @change="choices[route.id].model = ''">
                  <option value="">未选择供应商</option>
                  <option v-for="profile in selectableProfiles" :key="profile.id" :value="profile.id">{{ profile.name }}</option>
                </select>
                <label class="settings-label" :for="`${route.id}-api-model`">{{ route.label }}模型</label>
                <select :id="`${route.id}-api-model`" v-model="choices[route.id].model" class="settings-select" :disabled="saving || editing || !choices[route.id].profileId">
                  <option value="">未选择模型</option>
                  <option v-for="model in modelsForRoute(route.id)" :key="model" :value="model">{{ modelLabel(route.id, model) }}</option>
                </select>
                <p v-if="route.id === 'sidebar'" class="settings-hint">使用截图提问时，请选择支持图片输入的模型。</p>
              </div>
              <div v-if="!editing" class="settings-actions">
                <button class="settings-button secondary" type="button" :disabled="saving" @click="close">关闭</button>
                <button class="settings-button primary" type="button" :disabled="saving || !selectionChanged" @click="submitSelections">保存选择</button>
              </div>
            </section>
          </template>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup>
import { computed, reactive, ref, watch } from 'vue';

const props = defineProps({
  open: Boolean,
  settings: { type: Object, required: true },
  loading: Boolean,
  saving: Boolean,
  error: String,
  saveProfile: { type: Function, required: true },
});

const emit = defineEmits(['update:open', 'save-selections']);

const providers = {
  aliyun: '阿里云百炼',
  deepseek: 'DeepSeek',
  glm: '智谱 GLM',
  openai: 'OpenAI',
  moonshot: 'Moonshot',
  custom: '自定义供应商',
};

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

const routes = [
  { id: 'sidebar', label: '右侧栏对话' },
  { id: 'inline', label: '原地问' },
];
const draft = ref(null);
const modelName = ref('');
const choices = reactive({});
const editing = computed(() => draft.value !== null);
const selectableProfiles = computed(() => props.settings.profiles.filter(profile => profile.hasKey));
const canAddModel = computed(() => modelName.value !== '' && !draft.value?.models.includes(modelName.value));
const availablePresets = computed(() => (modelOptions[draft.value?.provider] || [])
  .filter(model => !draft.value.models.includes(model.value)));
const selectionChanged = computed(() => routes.some(({ id }) =>
  !sameSelection(choices[id], props.settings.selections[id])
));
const clearedRoutes = computed(() => routes.filter(({ id }) => {
  const current = props.settings.selections[id];
  return draft.value?.id && current.profileId === draft.value.id &&
    current.model && !draft.value.models.includes(current.model);
}).map(route => route.label));
const canKeepKey = computed(() => props.settings.profiles.some(profile =>
  profile.id === draft.value?.id && profile.provider === draft.value.provider &&
  profile.baseUrl === draft.value.baseUrl && profile.hasKey
));

function sameSelection(a, b) {
  return a.profileId === b.profileId && a.model === b.model;
}

watch(() => props.open, (open) => {
  if (!open) dismissEditor();
  for (const { id } of routes) choices[id] = { ...props.settings.selections[id] };
}, { immediate: true });

watch(() => props.settings.selections, (next, previous) => {
  for (const { id } of routes) {
    if (!sameSelection(next[id], previous[id])) choices[id] = { ...next[id] };
  }
});

function profileForRoute(id) {
  return selectableProfiles.value.find(profile => profile.id === choices[id].profileId);
}

function modelsForRoute(id) {
  return profileForRoute(id)?.models || [];
}

function modelLabel(id, name) {
  const options = modelOptions[profileForRoute(id)?.provider] || [];
  const preset = options.find(model => model.value === name);
  return preset?.multimodal ? `${name} · 支持图片` : name;
}

function providerUnavailable(provider) {
  return provider !== 'custom' &&
    props.settings.profiles.some(profile => profile.provider === provider && profile.id !== draft.value?.id);
}

function changeProvider() {
  draft.value.baseUrl = '';
  draft.value.apiKey = '';
  draft.value.models = (modelOptions[draft.value.provider] || []).map(model => model.value);
  modelName.value = '';
}

function editProfile(profile) {
  modelName.value = '';
  if (profile) {
    draft.value = { ...profile, models: [...profile.models], apiKey: '' };
    return;
  }
  const provider = Object.keys(providers).find(value => !providerUnavailable(value));
  draft.value = { id: '', name: '', provider, apiKey: '', baseUrl: '', models: [] };
  changeProvider();
}

function addModel() {
  if (!canAddModel.value || props.saving) return;
  draft.value.models.push(modelName.value);
  modelName.value = '';
}

function addPreset(event) {
  if (event.target.value) draft.value.models.push(event.target.value);
  event.target.value = '';
}

function removeModel(model) {
  draft.value.models = draft.value.models.filter(name => name !== model);
}

function dismissEditor() {
  draft.value = null;
  modelName.value = '';
}

function close() {
  if (!props.saving) {
    dismissEditor();
    emit('update:open', false);
  }
}

async function submitProfile() {
  if (props.saving) return;
  addModel();
  const { id, name, provider, baseUrl, models, apiKey } = draft.value;
  if (await props.saveProfile({ id, name, provider, baseUrl, models, apiKey })) dismissEditor();
}

function submitSelections() {
  emit('save-selections', { sidebar: { ...choices.sidebar }, inline: { ...choices.inline } });
}
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
  width: min(420px, calc(100vw - 32px));
  box-sizing: border-box;
  max-height: calc(100dvh - 88px);
  overflow: auto;
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

.settings-section, .profile-editor {
  margin: 16px 0 0;
  padding: 14px 0 0;
  border: 0;
  border-top: 1px solid var(--border-color);
}

.settings-section h4 {
  margin: 0 0 12px;
  font-size: 14px;
}

.model-input-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.preset-select {
  margin-top: 8px;
}

.routing-entry + .routing-entry {
  margin-top: 16px;
}

.profile-list {
  display: grid;
  gap: 8px;
  padding: 0;
  margin: 0 0 10px;
  list-style: none;
}

.profile-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  border: 1px solid var(--border-color);
  border-radius: 10px;
  padding: 8px 10px;
}

.profile-row strong, .profile-row small {
  display: block;
  overflow-wrap: anywhere;
}

.profile-row strong {
  font-size: 13px;
}

.profile-row small {
  margin-top: 2px;
  color: var(--text-secondary);
  font-size: 12px;
}

.settings-error {
  color: #a22626;
  font-size: 13px;
  overflow-wrap: anywhere;
}

.settings-label:not(:first-child) {
  margin-top: 12px;
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

.settings-button:disabled {
  opacity: 0.55;
  cursor: default;
}

.settings-button:not(:disabled):hover {
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
