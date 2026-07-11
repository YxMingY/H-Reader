// Package chat - 聊天服务主入口
//
// service.go 包含 ChatService 的主结构体和初始化逻辑。
//
// 文件结构说明：
//   - types.go:            数据结构和常量定义
//   - store.go:            文件 I/O 和索引管理
//   - session_manager.go:  会话 CRUD 操作
//   - message_handler.go:  消息发送和流式传输
//   - config_manager.go:   API Key 和模型配置管理
//   - global_session.go:   全局会话（遗留接口）
//   - service.go:          主结构体和初始化（本文件）
package chat

import (
	"hreader/llmkit"
	"hreader/services/config"
	"sync"
)

/*
 * 模型配置说明：
 *
 * 模型信息和 API Key 保存在后端的 Client 对象中。
 * 当用户在前端保存新的模型配置时：
 *   1. 调用 SaveModelConfig() 更新配置
 *   2. 后端替换内部的 Client 实例
 *   3. 下次发送消息时使用新的 Client 生成 Conversation
 *
 * 这意味着更换模型配置后会自动应用到后续的对话中，无需手动重建会话。
 */

// ChatService 聊天服务
//
// 字段说明：
//   - mu:           互斥锁，保护所有字段的并发访问
//   - config:       应用配置（从 services/config 包获取的单例缓存）
//   - client:       LLM API 客户端（通过 API_KEY 初始化，用于创建对话）
//   - conversation: 全局会话对象（仅用于非作用域的旧版消息发送，不持久化）
type ChatService struct {
	mu           sync.Mutex
	config       *config.Config
	client       *llmkit.Client
	conversation *llmkit.TracedConversation
}

// NewChatService 创建并初始化一个 ChatService 实例
//
// 启动时尝试从配置文件中加载已保存的 API_KEY 和模型配置，并创建 LLM 客户端。
// 如果 API_KEY 无效或未配置，client 保持 nil，需要通过 SaveAPIKey() 后续设置。
//
// 返回：
//   - 初始化后的 ChatService 实例
func NewChatService() *ChatService {
	config := config.GetConfig()
	cs := &ChatService{
		config:       config,
		client:       nil,
		conversation: nil,
	}

	// 尝试使用已保存的 API_KEY 和模型配置初始化 client
	if config.API_KEY != "" {
		provider := config.Provider
		if provider == "" {
			provider = string(defaultProvider)
		}

		model := config.Model
		if model == "" {
			model = defaultModel
		}

		// 创建 LLM 客户端，如果失败则保持 client 为 nil
		if client, err := llmkit.NewClient(llmkit.Config{
			Provider: llmkit.ProviderType(provider),
			APIKey:   config.API_KEY,
			Model:    model,
		}); err == nil {
			cs.client = client
		}
	}

	return cs
}
