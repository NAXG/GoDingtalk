package M3u8Downloader

import (
	"testing"
)

func TestSetMovieName(t *testing.T) {
	downloader := NewDownloader().(*m3u8downloader)

	tests := []struct {
		input    string
		expected string
	}{
		{"test", "test.ts"},
		{"test.ts", "test.ts"},
		{"my_video", "my_video.ts"},
		{"my_video.ts", "my_video.ts"},
	}

	for _, tt := range tests {
		downloader.SetMovieName(tt.input)
		if downloader.config.VideoName != tt.expected {
			t.Errorf("SetMovieName(%s): got %s, want %s", tt.input, downloader.config.VideoName, tt.expected)
		}
	}
}

func TestSetNumOfThread(t *testing.T) {
	downloader := NewDownloader().(*m3u8downloader)

	tests := []int{1, 5, 10, 20, 50}

	for _, threads := range tests {
		downloader.SetNumOfThread(threads)
		if downloader.config.NumOfThreads != threads {
			t.Errorf("SetNumOfThread(%d): got %d", threads, downloader.config.NumOfThreads)
		}
		if len(downloader.buffer) != threads {
			t.Errorf("SetNumOfThread(%d): buffer length = %d, want %d", threads, len(downloader.buffer), threads)
		}
	}
}

func TestSetSaveDirectory(t *testing.T) {
	downloader := NewDownloader().(*m3u8downloader)

	tests := []struct {
		input    string
		expected string
	}{
		{"video", "video/"},
		{"video/", "video/"},
		{"/tmp/downloads", "/tmp/downloads/"},
		{"/tmp/downloads/", "/tmp/downloads/"},
	}

	for _, tt := range tests {
		downloader.SetSaveDirectory(tt.input)
		if downloader.config.SaveDirectory != tt.expected {
			t.Errorf("SetSaveDirectory(%s): got %s, want %s", tt.input, downloader.config.SaveDirectory, tt.expected)
		}
	}
}

func TestSetIfShowTheBar(t *testing.T) {
	downloader := NewDownloader().(*m3u8downloader)

	downloader.SetIfShowTheBar(true)
	if !downloader.config.ifShowBar {
		t.Errorf("SetIfShowTheBar(true) failed")
	}

	downloader.SetIfShowTheBar(false)
	if downloader.config.ifShowBar {
		t.Errorf("SetIfShowTheBar(false) failed")
	}
}

func TestSetDownloadModel(t *testing.T) {
	downloader := NewDownloader().(*m3u8downloader)

	// 测试有效的下载模式
	downloader.SetDownloadModel(SaveAsTsFileAndMergeModel)
	if downloader.config.DownloadModel != SaveAsTsFileAndMergeModel {
		t.Errorf("SetDownloadModel(SaveAsTsFileAndMergeModel) failed")
	}

	// 测试无效的下载模式，应该回退到默认模式
	downloader.SetDownloadModel(999)
	if downloader.config.DownloadModel != SaveAsTsFileAndMergeModel {
		t.Errorf("SetDownloadModel(invalid) should default to SaveAsTsFileAndMergeModel")
	}
}

func TestNewDownloader(t *testing.T) {
	downloader := NewDownloader()

	if downloader == nil {
		t.Fatal("NewDownloader() returned nil")
	}

	md, ok := downloader.(*m3u8downloader)
	if !ok {
		t.Fatal("NewDownloader() did not return *m3u8downloader")
	}

	// 验证默认配置
	if md.config.NumOfThreads != defaultNumberOfThread {
		t.Errorf("Default NumOfThreads = %d, want %d", md.config.NumOfThreads, defaultNumberOfThread)
	}

	if md.config.SaveDirectory != defaultSaveDirectory {
		t.Errorf("Default SaveDirectory = %s, want %s", md.config.SaveDirectory, defaultSaveDirectory)
	}

	if md.config.DownloadModel != SaveAsTsFileAndMergeModel {
		t.Errorf("Default DownloadModel = %d, want %d", md.config.DownloadModel, SaveAsTsFileAndMergeModel)
	}
}
