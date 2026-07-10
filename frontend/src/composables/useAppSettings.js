import { ref } from 'vue';

/**
 * useAppSettings - 应用设置管理 Composable
 * 
 * 功能：
 * - 加载和保存 API Key
 * - 加载和保存模型配置
 * - 管理设置面板开关状态
 * 
 * @param {Object} options - 配置选项
 * @param {Ref} options.bookshelfRef - Bookshelf 组件引用（用于控制引导界面）
 * @param {Object} options.chatService - ChatService 实例（由调用方传入）
 * @returns {Object} 设置管理方法和状态
 */
export function useAppSettings({ bookshelfRef, chatService }) {
  const apiKey = ref('');
  const modelProvider = ref('aliyun');
  const modelName = ref('qwen3-omni-flash');
  const settingsOpen = ref(false);

  /**
   * 从后端加载 API Key
   */
  const loadApiKey = async () => {
    try {
      apiKey.value = await chatService.GetAPIKey();
    } catch (err) {
      console.error('加载 API Key 失败:', err);
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
   * - 打开时自动加载 API Key
   */
  const toggleSettings = async () => {
    if (!settingsOpen.value) {
      await loadApiKey();
    }
    settingsOpen.value = !settingsOpen.value;
  };

  /**
   * 保存设置到后端
   * @param {Object} settings - 设置对象
   * @param {string} settings.apiKey - API Key
   * @param {string} settings.provider - 模型提供商
   * @param {string} settings.model - 模型名称
   */
  const saveSettings = async (settings) => {
    try {
      await chatService.SaveAPIKey(settings.apiKey);
      await chatService.SaveModelConfig({
        provider: settings.provider,
        model: settings.model,
      });
      
      apiKey.value = settings.apiKey;
      modelProvider.value = settings.provider;
      modelName.value = settings.model;
      settingsOpen.value = false; // 保存成功后自动关闭设置面板
      
      // API Key 保存后，检查是否需要显示/隐藏引导界面
      if (bookshelfRef.value) {
        bookshelfRef.value.checkOnboarding(settings.apiKey);
      }
    } catch (err) {
      alert('保存设置失败：' + err.message);
    }
  };

  return {
    apiKey,
    modelProvider,
    modelName,
    settingsOpen,
    loadApiKey,
    loadModelConfig,
    toggleSettings,
    saveSettings,
  };
}
