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

// Config holds the settings stored in the user's config.json.
type Config struct {
	DefaultFolder string       `json:"default_folder"`
	API           *APISettings `json:"api,omitempty"`
}

var (
	globalConfig *Config
	configMutex  sync.Mutex
	dataMutex    sync.Mutex
)

// GetConfig loads and caches the application configuration.
func GetConfig() *Config {
	configMutex.Lock()
	defer configMutex.Unlock()

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

// InvalidateConfigCache clears the cached configuration for tests.
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

// LoadConfig reads an independent configuration from disk.
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

// SaveToFile replaces the configuration with a private file in the same directory.
func (c *Config) SaveToFile() error {
	dataMutex.Lock()
	defer dataMutex.Unlock()
	return c.saveToFileLocked()
}

func (c *Config) saveToFileLocked() error {
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

func (c *Config) ReadDefaultFolder() string {
	dataMutex.Lock()
	defer dataMutex.Unlock()
	return c.DefaultFolder
}

func (c *Config) SetDefaultFolder(path string) error {
	dataMutex.Lock()
	defer dataMutex.Unlock()
	next := *c
	next.DefaultFolder = path
	if err := next.saveToFileLocked(); err != nil {
		return err
	}
	c.DefaultFolder = path
	return nil
}
