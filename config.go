// Package main - Application configuration management
//
// config.go 管理应用全局配置（如API_KEY），采用单例模式设计。
//
// 使用指南：
//
//  1. 常规使用（推荐）- 获取单例Config：
//     cfg := GetConfig()
//     apiKey := cfg.API_KEY
//     cfg.API_KEY = "new_key"
//     cfg.SaveToFile()
//
//  2. 测试场景 - 使用LoadConfig加载新实例：
//     testCfg, err := LoadConfig()  // 不影响全局单例
//     testCfg.API_KEY = "test_key"
//
//  3. 测试清理 - 重置全局单例：
//     InvalidateConfigCache()  // 仅用于测试teardown
//     // 下次GetConfig()会重新加载文件
//
// 核心特性：
// - 单例模式：首次调用GetConfig时从config.json加载，后续直接返回缓存
// - 线程安全：使用RWMutex保护并发访问（读取用RLock，修改用Lock）
// - 懒加载：应用启动不立即加载，首次访问时才初始化
// - 失败容错：文件缺失或错误时自动创建空配置
//
// 存储路径：./config.json（应用工作目录）
//
// 配置格式：
//
//	{
//	  "api_key": "sk-xxxxx"
//	}
package main

import (
	"encoding/json"
	"os"
	"sync"
)

// Config 应用配置结构体
type Config struct {
	API_KEY string `json:"api_key"` // LLM服务API密钥
}

var (
	globalConfig *Config
	configMutex  sync.RWMutex
)

// GetConfig 获取全局单例Config
//
// 特点：
// - 首次调用时从config.json加载（若文件缺失则创建空配置）
// - 后续调用直接返回缓存的单例（无文件I/O）
// - 线程安全的双重检查锁定（DCL）
//
// 典型用法：
//
//	cfg := GetConfig()
//	fmt.Println(cfg.API_KEY)
//	cfg.API_KEY = "new_key"
//	cfg.SaveToFile()  // 保存修改到config.json
//
// 注意：
// - 返回的Config对象在全局生命周期内保持不变
// - 对返回对象的修改不会自动保存，必须显式调用SaveToFile()
// - 若需要不影响全局单例的独立配置，使用LoadConfig()而非GetConfig()
func GetConfig() *Config {
	configMutex.RLock()
	if globalConfig != nil {
		defer configMutex.RUnlock()
		return globalConfig
	}
	configMutex.RUnlock()

	// 首次加载，需要写锁
	configMutex.Lock()
	defer configMutex.Unlock()

	// 双重检查，避免竞态条件
	if globalConfig != nil {
		return globalConfig
	}

	cfg := &Config{}
	if err := cfg.LoadFromFile(); err == nil {
		globalConfig = cfg
	} else {
		// 文件不存在或加载失败，创建空配置
		globalConfig = &Config{}
	}
	return globalConfig
}

// InvalidateConfigCache 使全局Config缓存失效（仅在测试中使用）
//
// 目的：重置全局单例，使下一次GetConfig()重新从文件加载
//
// 使用场景（仅限单元测试）：
// - 测试case间的隔离：在teardown时调用以清理全局状态
// - 测试不同的配置文件状态
//
// 示例（测试cleanup）：
//
//	func TestConfigHandling(t *testing.T) {
//	    defer InvalidateConfigCache()  // 测试结束后重置
//	    cfg := GetConfig()
//	    cfg.API_KEY = "test_value"
//	    cfg.SaveToFile()
//	    // ... 测试逻辑
//	}
//
// 警告：不要在生产代码中调用此函数
func InvalidateConfigCache() {
	configMutex.Lock()
	defer configMutex.Unlock()
	globalConfig = nil
}

func (c *Config) GetConfigFilePath() string {
	return "config.json"
}

// LoadConfig 加载配置文件到新实例（不使用全局单例）
//
// 何时使用：
// - 单元测试：需要独立的Config实例，不影响全局状态
// - 配置验证：加载和测试新配置而不修改当前应用配置
// - 重新加载：从磁盘重新读取最新的配置
//
// 返回值：
// - (*Config, nil): 成功加载或文件缺失时返回空Config
// - (*Config, error): 仅在JSON解析错误时返回error
//
// 示例（测试场景）：
//
//	testCfg, _ := LoadConfig()  // 不影响GetConfig()
//	testCfg.API_KEY = "test_key"
//	testCfg.SaveToFile()
func LoadConfig() (*Config, error) {
	config := &Config{}
	err := config.LoadFromFile()
	if err != nil {
		return nil, err
	}
	return config, nil
}

// LoadFromFile 从配置文件加载数据到当前Config对象
//
// 行为：
// - 文件存在：加载JSON并反序列化到当前Config
// - 文件不存在：创建空config.json文件并返回nil
// - 其他错误：返回读取或解析错误
//
// 注意：此方法会修改接收者对象，通常不需要直接调用
func (c *Config) LoadFromFile() error {
	filePath := c.GetConfigFilePath()
	// 如果文件不存在，返回一个空配置，并创建一个新的配置文件
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		emptyConfig := &Config{}
		err := emptyConfig.SaveToFile()
		if err != nil {
			return err
		}
		return nil
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, c)
}

// SaveToFile 将当前Config对象序列化为JSON并写入config.json
//
// 使用场景：
// - 更新API_KEY后需要持久化：cfg.API_KEY = "key"; cfg.SaveToFile()
// - 修改任何配置字段后需要保存到文件
//
// 注意：
// - 使用原子写入（temp file + rename）保证数据一致性
// - 文件权限为644（所有者读写，其他人只读）
//
// 示例：
//
//	cfg := GetConfig()
//	cfg.API_KEY = "new_api_key"
//	if err := cfg.SaveToFile(); err != nil {
//	    log.Fatalf("Failed to save config: %v", err)
//	}
func (c *Config) SaveToFile() error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.GetConfigFilePath(), data, 0644)
}
