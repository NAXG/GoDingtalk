package main

import (
	"os"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config == nil {
		t.Fatal("DefaultConfig() returned nil")
	}

	if config.ThreadCount != 10 {
		t.Errorf("Default ThreadCount = %d, want 10", config.ThreadCount)
	}

	if config.SaveDirectory != "video/" {
		t.Errorf("Default SaveDirectory = %s, want video/", config.SaveDirectory)
	}

	if config.CookiesFile != "cookies.json" {
		t.Errorf("Default CookiesFile = %s, want cookies.json", config.CookiesFile)
	}

	if config.ChromeTimeout != 20 {
		t.Errorf("Default ChromeTimeout = %d, want 20", config.ChromeTimeout)
	}

	if config.HTTPTimeout != 30 {
		t.Errorf("Default HTTPTimeout = %d, want 30", config.HTTPTimeout)
	}
}

func TestLoadConfig(t *testing.T) {
	// 测试加载不存在的配置文件，应该返回默认配置
	config, err := LoadConfig("non_existent_config.json")
	if err != nil {
		t.Errorf("LoadConfig() error = %v", err)
	}

	if config.ThreadCount != 10 {
		t.Errorf("LoadConfig() for non-existent file should return default config")
	}

	// 创建临时配置文件
	tmpFile, err := os.CreateTemp("", "config_test_*.json")
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// 写入测试配置
	testConfig := `{
  "thread_count": 15,
  "save_directory": "downloads/",
  "cookies_file": "test_cookies.json",
  "chrome_timeout": 30,
  "http_timeout": 60
}`
	if _, err := tmpFile.WriteString(testConfig); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	// 测试加载有效的配置文件
	config, err = LoadConfig(tmpPath)
	if err != nil {
		t.Errorf("LoadConfig() error = %v", err)
	}

	if config.ThreadCount != 15 {
		t.Errorf("LoadConfig() ThreadCount = %d, want 15", config.ThreadCount)
	}

	if config.SaveDirectory != "downloads/" {
		t.Errorf("LoadConfig() SaveDirectory = %s, want downloads/", config.SaveDirectory)
	}

	if config.CookiesFile != "test_cookies.json" {
		t.Errorf("LoadConfig() CookiesFile = %s, want test_cookies.json", config.CookiesFile)
	}

	if config.ChromeTimeout != 30 {
		t.Errorf("LoadConfig() ChromeTimeout = %d, want 30", config.ChromeTimeout)
	}

	if config.HTTPTimeout != 60 {
		t.Errorf("LoadConfig() HTTPTimeout = %d, want 60", config.HTTPTimeout)
	}
}

func TestSaveConfig(t *testing.T) {
	// 创建临时文件路径
	tmpFile, err := os.CreateTemp("", "config_save_test_*.json")
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	os.Remove(tmpPath) // 删除临时文件，让 SaveConfig 创建它
	defer os.Remove(tmpPath)

	// 创建测试配置
	testConfig := &Config{
		ThreadCount:   25,
		SaveDirectory: "test_videos/",
		CookiesFile:   "test.json",
		ChromeTimeout: 40,
		HTTPTimeout:   120,
	}

	// 保存配置
	err = SaveConfig(tmpPath, testConfig)
	if err != nil {
		t.Errorf("SaveConfig() error = %v", err)
	}

	// 加载并验证保存的配置
	loadedConfig, err := LoadConfig(tmpPath)
	if err != nil {
		t.Errorf("LoadConfig() after SaveConfig() error = %v", err)
	}

	if loadedConfig.ThreadCount != testConfig.ThreadCount {
		t.Errorf("Saved ThreadCount = %d, want %d", loadedConfig.ThreadCount, testConfig.ThreadCount)
	}

	if loadedConfig.SaveDirectory != testConfig.SaveDirectory {
		t.Errorf("Saved SaveDirectory = %s, want %s", loadedConfig.SaveDirectory, testConfig.SaveDirectory)
	}

	if loadedConfig.CookiesFile != testConfig.CookiesFile {
		t.Errorf("Saved CookiesFile = %s, want %s", loadedConfig.CookiesFile, testConfig.CookiesFile)
	}
}

func TestCheckCookiesValid(t *testing.T) {
	// 保存当前的 cookies.json（如果存在）
	var originalCookies []byte
	var cookiesExisted bool
	if data, err := os.ReadFile("cookies.json"); err == nil {
		originalCookies = data
		cookiesExisted = true
	}
	defer func() {
		if cookiesExisted {
			os.WriteFile("cookies.json", originalCookies, 0600)
		} else {
			os.Remove("cookies.json")
		}
	}()

	// 测试不存在的 cookies 文件
	os.Remove("cookies.json")
	if checkCookiesValid() {
		t.Errorf("checkCookiesValid() = true for non-existent file, want false")
	}

	// 测试有效的 cookies 文件
	validCookies := `{"LV_PC_SESSION": "test_session_value"}`
	if err := os.WriteFile("cookies.json", []byte(validCookies), 0600); err != nil {
		t.Fatal(err)
	}

	if !checkCookiesValid() {
		t.Errorf("checkCookiesValid() = false for valid cookies, want true")
	}

	// 测试无效的 cookies 文件（缺少 LV_PC_SESSION）
	invalidCookies := `{"OTHER_COOKIE": "value"}`
	if err := os.WriteFile("cookies.json", []byte(invalidCookies), 0600); err != nil {
		t.Fatal(err)
	}

	if checkCookiesValid() {
		t.Errorf("checkCookiesValid() = true for invalid cookies, want false")
	}
}
