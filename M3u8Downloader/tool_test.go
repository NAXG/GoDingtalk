package M3u8Downloader

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestProcessNum(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0000.ts"},
		{5, "0005.ts"},
		{23, "0023.ts"},
		{456, "0456.ts"},
		{9999, "9999.ts"},
	}

	for _, tt := range tests {
		result := string(processNum(tt.input))
		if result != tt.expected {
			t.Errorf("processNum(%d) = %s; want %s", tt.input, result, tt.expected)
		}
	}
}

func TestResolveURL(t *testing.T) {
	baseURL, _ := url.Parse("https://example.com/path/to/file.m3u8")

	tests := []struct {
		path     string
		expected string
	}{
		{"https://other.com/file.ts", "https://other.com/file.ts"},
		{"http://other.com/file.ts", "http://other.com/file.ts"},
		{"/absolute/path.ts", "https://example.com/absolute/path.ts"},
		{"relative/path.ts", "https://example.com/path/to/relative/path.ts"},
	}

	for _, tt := range tests {
		result := ResolveURL(baseURL, tt.path)
		if result != tt.expected {
			t.Errorf("ResolveURL(%s) = %s; want %s", tt.path, result, tt.expected)
		}
	}
}

func TestCheckAndCreatDirectory(t *testing.T) {
	// 创建临时目录
	tmpDir := filepath.Join(os.TempDir(), "godingtalk_test")
	defer os.RemoveAll(tmpDir)

	// 测试创建新目录
	err := CheckAndCreatDirectory(tmpDir)
	if err != nil {
		t.Errorf("CheckAndCreatDirectory() error = %v", err)
	}

	// 验证目录是否存在
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		t.Errorf("Directory was not created")
	}

	// 测试目录已存在的情况
	err = CheckAndCreatDirectory(tmpDir)
	if err != nil {
		t.Errorf("CheckAndCreatDirectory() on existing dir error = %v", err)
	}
}

func TestPathExists(t *testing.T) {
	// 创建临时文件
	tmpFile, err := os.CreateTemp("", "godingtalk_test_*.txt")
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// 测试文件存在
	exists, err := PathExists(tmpPath)
	if err != nil {
		t.Errorf("PathExists() error = %v", err)
	}
	if !exists {
		t.Errorf("PathExists() = false; want true")
	}

	// 测试文件不存在
	os.Remove(tmpPath)
	exists, err = PathExists(tmpPath)
	if err != nil {
		t.Errorf("PathExists() error = %v", err)
	}
	if exists {
		t.Errorf("PathExists() = true; want false")
	}
}

func TestGetUnixTimeAndToByte(t *testing.T) {
	result := getUnixTimeAndToByte()

	// 验证结果是非空字符串
	if result == "" {
		t.Errorf("getUnixTimeAndToByte() returned empty string")
	}

	// 验证结果是数字字符串
	if len(result) < 10 {
		t.Errorf("getUnixTimeAndToByte() = %s; expected at least 10 digits", result)
	}
}
