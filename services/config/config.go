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
// macOS 和 Windows 使用用户配置目录，其他平台使用应用工作目录。
//
// 配置格式：
//
//	{
//	  "api_key": "sk-xxxxx",
//	  "default_folder": "C:/Users/xxx/Documents/Papers",
//	  "provider": "aliyun",
//	  "model": "qwen-turbo"
//	}
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// Config 应用配置结构体
type Config struct {
	APIKey        string            `json:"api_key"`        // LLM服务API密钥（已废弃，保留用于向后兼容）
	APIKeys       map[string]string `json:"api_keys"`       // 各提供商的API密钥映射 {provider: api_key}
	DefaultFolder string            `json:"default_folder"` // 默认PDF书籍文件夹路径
	Provider      string            `json:"provider"`       // 模型提供商：aliyun, deepseek, glm
	Model         string            `json:"model"`          // 具体模型名称
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
		log.Printf("load config: %v", err)
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

const configFileName = "config.json"

func configFilePath() (string, error) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		return filepath.Abs(configFileName)
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate user configuration directory: %w", err)
	}
	if !filepath.IsAbs(dir) {
		return "", fmt.Errorf("user configuration directory must be absolute: %q", dir)
	}
	return filepath.Join(dir, "hreader", configFileName), nil
}

func legacyConfigPaths() []string {
	var paths []string
	if cwd, err := os.Getwd(); err == nil {
		paths = append(paths, filepath.Join(cwd, configFileName))
	}
	if executable, err := os.Executable(); err == nil {
		dir := filepath.Dir(executable)
		paths = append(paths, filepath.Join(dir, configFileName))
		contents := filepath.Dir(dir)
		bundle := filepath.Dir(contents)
		if runtime.GOOS == "darwin" && filepath.Ext(bundle) == ".app" &&
			filepath.Base(dir) == "MacOS" && filepath.Base(contents) == "Contents" {
			paths = append(paths, filepath.Join(filepath.Dir(bundle), configFileName))
		}
	}
	return paths
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

// LoadFromFile loads the current configuration or imports an older configuration
// when the destination file is missing.
func (c *Config) LoadFromFile() error {
	path, err := configFilePath()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
			for _, legacyPath := range legacyConfigPaths() {
				if legacyPath == path {
					continue
				}
				data, err := os.ReadFile(legacyPath)
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				var legacy Config
				if err == nil {
					_, err = legacy.decode(data)
				}
				if err != nil {
					log.Printf("load legacy config %q: %v", legacyPath, err)
					continue
				}
				*c = legacy
				if err := c.SaveToFile(); err != nil {
					log.Printf("migrate legacy config %q: %v", legacyPath, err)
				}
				return nil
			}
		}
		return c.SaveToFile()
	}
	if err != nil {
		return err
	}
	upgraded, err := c.decode(data)
	if upgraded {
		if saveErr := c.SaveToFile(); saveErr != nil {
			log.Print(saveErr)
		}
	}
	return err
}

func (c *Config) decode(data []byte) (bool, error) {
	if err := json.Unmarshal(data, c); err != nil {
		return false, err
	}
	if c.APIKeys == nil {
		c.APIKeys = make(map[string]string)
	}
	if c.APIKey != "" && len(c.APIKeys) == 0 {
		c.APIKeys["aliyun"] = c.APIKey
		c.APIKey = ""
		return true, nil
	}
	return false, nil
}

// SaveToFile replaces the configuration with a private file in the same directory.
func (c *Config) SaveToFile() error {
	path, err := configFilePath()
	if err != nil {
		return err
	}
	if err := c.save(path); err != nil {
		return fmt.Errorf("save config %q: %w", path, err)
	}
	return nil
}

func (c *Config) save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, ".config-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	_, writeErr := file.Write(data)
	if err := errors.Join(writeErr, file.Close()); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
