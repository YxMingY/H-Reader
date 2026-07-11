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

// GetAPIKey 从缓存配置中读取 API 密钥
//
// 无需重新加载文件，直接使用内存中的单例缓存。
// 如果配置未初始化，返回空字符串。
//
// 返回：
//   - 当前配置的 API_KEY
func (c *ChatService) GetAPIKey() string {
	if c.config == nil {
		return ""
	}
	return c.config.API_KEY
}

// SaveAPIKey 保存新的 API 密钥到配置文件
//
// 流程：
//  1. 验证新 API_KEY 是否有效（创建临时 client 测试连接）
//  2. 保存到配置文件（同时更新内存缓存）
//  3. 替换当前 client 对象
//  4. 重置 conversation（会在下次 ensureConversationLocked 时重建）
//
// 参数：
//   - apiKey: 新的 API 密钥
//
// 返回：
//   - 错误信息（如果 API_KEY 无效或文件写入失败）
//
// 注意：
//   - 如果 API_KEY 无效，保存失败且不会修改现有 client
//   - 保存成功后，后续对话会自动使用新的 API_KEY
func (c *ChatService) SaveAPIKey(apiKey string) error {
	cleanKey := strings.TrimSpace(apiKey)
	if cleanKey == "" {
		return fmt.Errorf("api key cannot be empty")
	}

	// 先验证 API_KEY 是否有效（创建临时 client）
	provider := c.config.Provider
	if provider == "" {
		provider = string(defaultProvider)
	}

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

	// 保存到配置文件（同时更新内存缓存）
	c.config.API_KEY = cleanKey
	if err := c.config.SaveToFile(); err != nil {
		return err
	}

	// 更新 client 并重置 conversation
	c.mu.Lock()
	defer c.mu.Unlock()

	c.client = newClient
	c.conversation = nil

	return nil
}

// ============================================================================
// 模型配置管理
// ============================================================================

// GetModelConfig 获取模型配置（提供商和具体模型）
//
// 返回：
//   - map[string]string{"provider": 提供商, "model": 模型名称}
//   - 如果配置未初始化，返回默认值（aliyun, qwen3-omni-flash）
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
//  3. 如果 client 已初始化，重新创建以使用新配置
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

	// 更新配置
	c.config.Provider = provider
	c.config.Model = model

	// 保存到文件
	if err := c.config.SaveToFile(); err != nil {
		return err
	}

	// 如果 client 已初始化，需要重新创建以使用新配置
	if c.client != nil && c.config.API_KEY != "" {
		newClient, err := llmkit.NewClient(llmkit.Config{
			Provider: llmkit.ProviderType(provider),
			APIKey:   c.config.API_KEY,
			Model:    model,
		})
		if err == nil {
			c.client = newClient
			c.conversation = nil // 重置会话
		}
	}

	return nil
}
