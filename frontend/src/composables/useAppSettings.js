import { ref } from 'vue';

/**
 * useAppSettings - 应用设置管理 Composable
 * 
 * 功能：
 * - 加载和保存各提供商的 API Keys
 * - 加载和保存模型配置
 * - 管理设置面板开关状态
 * 
 * @param {Object} options - 配置选项
 * @param {Ref} options.bookshelfRef - Bookshelf 组件引用（用于控制引导界面）
 * @param {Object} options.chatService - ChatService 实例（由调用方传入）
 * @returns {Object} 设置管理方法和状态
 */
export function useAppSettings({ bookshelfRef, chatService }) {
  const apiKeys = ref({
    aliyun: '',
    deepseek: '',
    glm: '',
  });
  const modelProvider = ref('aliyun');
  const modelName = ref('qwen3-omni-flash');
  const settingsOpen = ref(false);

  /**
   * 从后端加载所有提供商的 API Keys
   */
  const loadApiKeys = async () => {
    try {
      const keys = await chatService.GetAllAPIKeys();
      // 合并到现有对象，保留未设置的 provider
      apiKeys.value = {
        aliyun: keys.aliyun || '',
        deepseek: keys.deepseek || '',
        glm: keys.glm || '',
      };
    } catch (err) {
      console.error('加载 API Keys 失败:', err);
    }
  };

  /**
   * 从后端加载模型配置
   */
  const loadModelConfig = async () => {
    try {
      const config = await chatService.GetModelConfig();
      if (config.provider) {
        modelProvider.value = config.provider;
      }
      if (config.model) {
        modelName.value = config.model;
      }
    } catch (err) {
      console.error('加载模型配置失败:', err);
    }
  };

  /**
   * 切换设置面板显示状态
   * - 打开时自动加载所有 API Keys
   */
  const toggleSettings = async () => {
    if (!settingsOpen.value) {
      await loadApiKeys();
    }
    settingsOpen.value = !settingsOpen.value;
  };

  /**
   * 保存设置到后端
   * @param {Object} settings - 设置对象
   * @param {Object} settings.apiKeys - 各提供商的 API Keys 映射
   * @param {string} settings.provider - 当前选中的模型提供商
   * @param {string} settings.model - 模型名称
   */
  const saveSettings = async (settings) => {
    try {
      // 保存当前 provider 的 API Key
      const currentProviderKey = settings.apiKeys[settings.provider];
      if (currentProviderKey && currentProviderKey.trim()) {
        await chatService.SaveAPIKey(settings.provider, currentProviderKey.trim());
      }
      
      // 保存模型配置
      await chatService.SaveModelConfig({
        provider: settings.provider,
        model: settings.model,
      });
      
      // 更新本地状态
      apiKeys.value = {
        ...apiKeys.value,
        ...settings.apiKeys,
      };
      modelProvider.value = settings.provider;
      modelName.value = settings.model;
      settingsOpen.value = false; // 保存成功后自动关闭设置面板
      
      // API Key 保存后，检查是否需要显示/隐藏引导界面
      if (bookshelfRef.value) {
        const hasAnyKey = Object.values(apiKeys.value).some(key => key && key.trim());
        bookshelfRef.value.checkOnboarding(hasAnyKey ? 'has-key' : '');
      }
    } catch (err) {
      alert('保存设置失败：' + err.message);
    }
  };

  return {
    apiKeys,
    modelProvider,
    modelName,
    settingsOpen,
    loadApiKeys,
    loadModelConfig,
    toggleSettings,
    saveSettings,
  };
}
