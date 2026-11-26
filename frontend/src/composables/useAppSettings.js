import { ref } from 'vue';

export function useAppSettings({ bookshelfRef, chatService }) {
  const apiSettings = ref({ profiles: [], selections: { sidebar: { profileId: '', model: '' }, inline: { profileId: '', model: '' } } });
  const settingsOpen = ref(false);
  const settingsLoading = ref(false);
  const settingsSaving = ref(false);
  const settingsError = ref('');

  const applySettings = (settings) => {
    apiSettings.value = settings;
    const hasKey = settings.profiles.some(profile => profile.hasKey);
    bookshelfRef.value?.checkOnboarding(hasKey ? 'has-key' : '');
  };

  const loadApiSettings = async () => {
    settingsLoading.value = true;
    settingsError.value = '';
    try {
      applySettings(await chatService.GetAPISettings());
    } catch (err) {
      settingsError.value = '读取 API 配置失败：' + (err?.message || String(err));
    } finally {
      settingsLoading.value = false;
    }
  };

  const toggleSettings = () => {
    if (settingsSaving.value) return;
    settingsOpen.value = !settingsOpen.value;
    if (settingsOpen.value && !settingsLoading.value) void loadApiSettings();
  };

  const save = async (operation) => {
    settingsSaving.value = true;
    settingsError.value = '';
    try {
      applySettings(await operation());
      return true;
    } catch (err) {
      settingsError.value = '保存失败：' + (err?.message || String(err));
      return false;
    } finally {
      settingsSaving.value = false;
    }
  };

  const saveApiProfile = (profile) => save(() => chatService.SaveAPIProfile(profile));

  const saveApiSelections = async (selections) => {
    if (await save(() => chatService.SaveAPISelections(selections))) {
      settingsOpen.value = false;
    }
  };

  return {
    apiSettings,
    settingsOpen,
    settingsLoading,
    settingsSaving,
    settingsError,
    loadApiSettings,
    toggleSettings,
    saveApiProfile,
    saveApiSelections,
  };
}
