package main

import (
	"encoding/json"
	"os"
)

// Config 配置文件结构
type Config struct {
	// 下载线程数
	ThreadCount int `json:"thread_count"`
	// 视频保存目录
	SaveDirectory string `json:"save_directory"`
	// Cookies文件路径
	CookiesFile string `json:"cookies_file"`
	// Chrome超时时间（分钟）
	ChromeTimeout int `json:"chrome_timeout"`
	// HTTP超时时间（秒）
	HTTPTimeout int `json:"http_timeout"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		ThreadCount:   10,
		SaveDirectory: "video/",
		CookiesFile:   "cookies.json",
		ChromeTimeout: 20,
		HTTPTimeout:   30,
	}
}

// LoadConfig 从文件加载配置，如果文件不存在则创建默认配置文件并返回默认配置
func LoadConfig(path string) (*Config, error) {
	// 检查文件是否存在
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// 文件不存在，创建默认配置文件
		config := DefaultConfig()
		if saveErr := SaveConfig(path, config); saveErr != nil {
			// 保存失败不影响程序运行，只是提示
			return config, nil
		}
		return config, nil
	}

	// 读取配置文件
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// 解析配置
	config := DefaultConfig()
	if err := json.Unmarshal(data, config); err != nil {
		return nil, err
	}

	return config, nil
}

// SaveConfig 保存配置到文件
func SaveConfig(path string, config *Config) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
