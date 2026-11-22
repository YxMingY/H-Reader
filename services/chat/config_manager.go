// Package chat - 配置管理
//
// config_manager.go 负责 API Key 和模型配置的管理。
package chat

import (
	"fmt"
	"hreader/llmkit"
	"strings"
)

// ============================================================================
// API Key 管理
// ============================================================================

// GetAPIKey 从缓存配置中读取指定提供商的 API 密钥
//
// 无需重新加载文件，直接使用内存中的单例缓存。
// 如果配置未初始化或该提供商没有配置 key，返回空字符串。
//
// 参数：
//   - provider: 提供商名称（aliyun, deepseek, glm）
//
// 返回：
//   - 当前配置的 API_KEY
func (c *ChatService) GetAPIKey(provider string) string {
	if c.config == nil || c.config.APIKeys == nil {
		return ""
	}
	return c.config.APIKeys[provider]
}

// GetAllAPIKeys 获取所有提供商的 API Keys
//
// 返回：
//   - map[string]string{provider: api_key}
func (c *ChatService) GetAllAPIKeys() map[string]string {
	if c.config == nil || c.config.APIKeys == nil {
		return make(map[string]string)
	}
	// 返回副本，避免外部修改
	keys := make(map[string]string)
	for k, v := range c.config.APIKeys {
		keys[k] = v
	}
	return keys
}

// SaveAPIKey 保存指定提供商的 API 密钥到配置文件
//
// 流程：
//  1. 验证新 API_KEY 是否有效（创建临时 client 测试连接）
//  2. 保存到配置文件（同时更新内存缓存）
//  3. 如果当前 provider 匹配，替换当前 client 对象
//  4. 重置 conversation（会在下次 ensureConversationLocked 时重建）
//
// 参数：
//   - provider: 提供商名称（aliyun, deepseek, glm）
//   - apiKey: 新的 API 密钥
//
// 返回：
//   - 错误信息（如果 API_KEY 无效或文件写入失败）
//
// 注意：
//   - 如果 API_KEY 无效，保存失败且不会修改现有 client
//   - 保存成功后，后续对话会自动使用新的 API_KEY
func (c *ChatService) SaveAPIKey(provider string, apiKey string) error {
	cleanKey := strings.TrimSpace(apiKey)
	if cleanKey == "" {
		return fmt.Errorf("api key cannot be empty")
	}

	if provider == "" {
		return fmt.Errorf("provider cannot be empty")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// 先验证 API_KEY 是否有效（创建临时 client）
	model := c.config.Model
	if model == "" {
		model = defaultModel
	}

	newClient, err := llmkit.NewClient(llmkit.Config{
		Provider: llmkit.ProviderType(provider),
		APIKey:   cleanKey,
		Model:    model,
	})
	if err != nil {
		return fmt.Errorf("invalid api key: %w", err)
	}

	// 确保 APIKeys map 已初始化
	if c.config.APIKeys == nil {
		c.config.APIKeys = make(map[string]string)
	}

	// 保存到配置文件（同时更新内存缓存）
	previousKey, hadKey := c.config.APIKeys[provider]
	c.config.APIKeys[provider] = cleanKey
	if err := c.config.SaveToFile(); err != nil {
		if hadKey {
			c.config.APIKeys[provider] = previousKey
		} else {
			delete(c.config.APIKeys, provider)
		}
		return err
	}

	// 如果当前 provider 匹配，更新 client 并重置 conversation

	if c.config.Provider == provider {
		c.client = newClient
		c.conversation = nil
	}

	return nil
}

// ============================================================================
// 模型配置管理
// ============================================================================

// GetModelConfig 获取模型配置（提供商和具体模型）
//
// 返回：
//   - map[string]string{"provider": 提供商, "model": 模型名称}
//   - 如果配置未初始化，返回默认值（aliyun, qwen3.8-flash）
func (c *ChatService) GetModelConfig() map[string]string {
	if c.config == nil {
		return map[string]string{
			"provider": "aliyun",
			"model":    defaultModel,
		}
	}

	provider := c.config.Provider
	if provider == "" {
		provider = "aliyun"
	}

	model := c.config.Model
	if model == "" {
		model = defaultModel
	}

	return map[string]string{
		"provider": provider,
		"model":    model,
	}
}

// SaveModelConfig 保存模型配置到配置文件
//
// 参数：
//   - config: map[string]string{"provider": 提供商, "model": 模型名称}
//
// 流程：
//  1. 更新内存中的配置
//  2. 保存到文件
//  3. 使用所选提供商的 API Key 更新 client
//  4. 重置 conversation（下次对话时重建）
//
// 返回：
//   - 错误信息（如果文件写入失败）
//
// 注意：
//   - 更换模型后，现有会话的 LLMState 仍然有效
//   - 新消息会使用新的模型生成回复
func (c *ChatService) SaveModelConfig(config map[string]string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	provider := config["provider"]
	model := config["model"]

	var client *llmkit.Client
	if apiKey := c.config.APIKeys[provider]; apiKey != "" {
		var err error
		client, err = llmkit.NewClient(llmkit.Config{
			Provider: llmkit.ProviderType(provider),
			APIKey:   apiKey,
			Model:    model,
		})
		if err != nil {
			return err
		}
	}

	previousProvider, previousModel := c.config.Provider, c.config.Model
	c.config.Provider, c.config.Model = provider, model
	if err := c.config.SaveToFile(); err != nil {
		c.config.Provider, c.config.Model = previousProvider, previousModel
		return err
	}

	c.client = client
	c.conversation = nil

	return nil
}
