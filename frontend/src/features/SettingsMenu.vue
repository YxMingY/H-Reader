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
import { ref, watch } from 'vue';

const props = defineProps({
  open: Boolean,
  apiKey: {
    type: String,
    default: '',
  },
});

const emit = defineEmits(['update:open', 'save']);

const draftKey = ref('');

watch(
  () => props.open,
  (isOpen) => {
    if (isOpen) {
      draftKey.value = props.apiKey || '';
    }
  }
);

const close = () => {
  emit('update:open', false);
};

const submit = () => {
  emit('save', draftKey.value.trim());
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
