package main

import (
	"encoding/json"
	"os"
)

type Config struct {
	API_KEY string `json:"api_key"`
}

func NewConfig() *Config {
	cfg, err := LoadConfig()
	if err != nil {
		return nil
	}
	return cfg
}

func (c *Config) GetConfigFilePath() string {
	return "config.json"
}

func LoadConfig() (*Config, error) {
	// 尝试从文件加载配置
	config := &Config{}
	err := config.LoadFromFile()
	if err != nil {
		return nil, err
	}
	return config, nil
}

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

func (c *Config) SaveToFile() error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.GetConfigFilePath(), data, 0644)
}
