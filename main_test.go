package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"GoDingtalk/M3u8Downloader"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type fakeDownloader struct {
	url             string
	movieName       string
	threadNum       int
	showBar         bool
	saveDir         string
	defaultDownload bool
}

func (f *fakeDownloader) DefaultDownload() bool { return f.defaultDownload }
func (f *fakeDownloader) ParseM3u8FileEncrypted(link string) (*M3u8Downloader.Result, error) {
	return nil, nil
}
func (f *fakeDownloader) Download() error                                         { return nil }
func (f *fakeDownloader) SetUrl(url string)                                       { f.url = url }
func (f *fakeDownloader) SetIfShowTheBar(ifShow bool)                             { f.showBar = ifShow }
func (f *fakeDownloader) SetNumOfThread(num int)                                  { f.threadNum = num }
func (f *fakeDownloader) SetMovieName(videoName string)                           { f.movieName = videoName }
func (f *fakeDownloader) SetSaveDirectory(targetDir string)                       { f.saveDir = targetDir }
func (f *fakeDownloader) SetDownloadModel(model M3u8Downloader.DownloadModelType) {}
func (f *fakeDownloader) MergeFile() error                                        { return nil }
func (f *fakeDownloader) MergeFileInDir(path string, saveName string) error       { return nil }

func jsonHTTPResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func writeCookiesFile(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "cookies.json")
	if err := os.WriteFile(path, []byte(`{"LV_PC_SESSION":"session-token","foo":"bar"}`), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

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

	if !strings.HasSuffix(config.CookiesFile, "cookies.json") {
		t.Errorf("Default CookiesFile = %s, should end with cookies.json", config.CookiesFile)
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

func TestLoadConfigInvalidJSON(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "config_invalid_*.json")
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.WriteString(`{"thread_count":`); err != nil {
		t.Fatal(err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadConfig(tmpPath); err == nil {
		t.Fatal("LoadConfig() error = nil, want non-nil for invalid JSON")
	}
}

func TestLoadConfigPartialUsesDefaults(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "config_partial_*.json")
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.WriteString(`{"thread_count":12}`); err != nil {
		t.Fatal(err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatal(err)
	}

	config, err := LoadConfig(tmpPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if config.ThreadCount != 12 {
		t.Fatalf("LoadConfig() ThreadCount = %d, want 12", config.ThreadCount)
	}
	if config.SaveDirectory != "video/" {
		t.Fatalf("LoadConfig() SaveDirectory = %q, want %q", config.SaveDirectory, "video/")
	}
	if config.ChromeTimeout != 20 || config.HTTPTimeout != 30 {
		t.Fatalf("LoadConfig() defaults not preserved: %+v", config)
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
	// 创建临时目录用于测试
	tmpDir, err := os.MkdirTemp("", "cookies_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cookiesFile := tmpDir + "/cookies.json"

	// 测试不存在的 cookies 文件
	if checkCookiesValid(cookiesFile) {
		t.Errorf("checkCookiesValid() = true for non-existent file, want false")
	}

	// 测试有效的 cookies 文件
	validCookies := `{"LV_PC_SESSION": "test_session_value"}`
	if err := os.WriteFile(cookiesFile, []byte(validCookies), 0600); err != nil {
		t.Fatal(err)
	}

	if !checkCookiesValid(cookiesFile) {
		t.Errorf("checkCookiesValid() = false for valid cookies, want true")
	}

	// 测试无效的 cookies 文件（缺少 LV_PC_SESSION）
	invalidCookies := `{"OTHER_COOKIE": "value"}`
	if err := os.WriteFile(cookiesFile, []byte(invalidCookies), 0600); err != nil {
		t.Fatal(err)
	}

	if checkCookiesValid(cookiesFile) {
		t.Errorf("checkCookiesValid() = true for invalid cookies, want false")
	}
}

func TestSanitizeFileName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "keeps valid chars", input: "lesson 01", expected: "lesson 01"},
		{name: "replaces invalid chars", input: `a/b\c:d*e?f"g<h>i|j`, expected: "a_b_c_d_e_f_g_h_i_j"},
		{name: "empty fallback", input: "", expected: "unnamed_video"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeFileName(tt.input); got != tt.expected {
				t.Fatalf("sanitizeFileName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestCreateVideoListFile(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		fileName string
		fileExt  string
		expected string
	}{
		{name: "txt", fileName: "video.txt", fileExt: ".txt", expected: ""},
		{name: "m3u", fileName: "video.m3u", fileExt: ".m3u", expected: "#EXTM3U\n"},
		{name: "m3u8", fileName: "video.m3u8", fileExt: ".m3u8", expected: "#EXTM3U\n"},
		{name: "dpl", fileName: "video.dpl", fileExt: ".dpl", expected: "DAUMPLAYLIST\nplayname=\ntopindex=0\nsaveplaypos=0\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(tmpDir, tt.fileName)
			if err := createVideoListFile(path, tt.fileExt); err != nil {
				t.Fatalf("createVideoListFile() error = %v", err)
			}

			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile() error = %v", err)
			}

			if got := string(data); got != tt.expected {
				t.Fatalf("createVideoListFile() wrote %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestCreateVideoListFileReturnsErrorForMissingParent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "video.m3u8")

	if err := createVideoListFile(path, ".m3u8"); err == nil {
		t.Fatal("createVideoListFile() error = nil, want non-nil")
	}
}

func TestAppendTitleToVideoListFile(t *testing.T) {
	tmpDir := t.TempDir()
	listDir := filepath.Join(tmpDir, "lists")
	saveDir := filepath.Join(tmpDir, "videos")

	if err := os.MkdirAll(listDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(saveDir, 0755); err != nil {
		t.Fatal(err)
	}

	listPath := filepath.Join(listDir, "video.m3u8")
	if err := createVideoListFile(listPath, ".m3u8"); err != nil {
		t.Fatalf("createVideoListFile() error = %v", err)
	}

	if err := appendTitleToVideoListFile(listPath, "lesson", ".m3u8", 1, saveDir); err != nil {
		t.Fatalf("appendTitleToVideoListFile() error = %v", err)
	}

	data, err := os.ReadFile(listPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	expected := "#EXTM3U\n../videos/lesson.mp4\n"
	if got := string(data); got != expected {
		t.Fatalf("appendTitleToVideoListFile() wrote %q, want %q", got, expected)
	}
}

func TestAppendTitleToVideoListFileFormats(t *testing.T) {
	tmpDir := t.TempDir()
	listDir := filepath.Join(tmpDir, "lists")
	saveDir := filepath.Join(tmpDir, "videos")

	if err := os.MkdirAll(listDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(saveDir, 0755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		fileName string
		fileExt  string
		want     string
	}{
		{name: "txt", fileName: "video.txt", fileExt: ".txt", want: "lesson\n"},
		{name: "dpl", fileName: "video.dpl", fileExt: ".dpl", want: "3*file*../videos/lesson.mp4\n"},
		{name: "fallback", fileName: "video.unknown", fileExt: ".unknown", want: "lesson\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listPath := filepath.Join(listDir, tt.fileName)
			if err := os.WriteFile(listPath, nil, 0644); err != nil {
				t.Fatal(err)
			}

			if err := appendTitleToVideoListFile(listPath, "lesson", tt.fileExt, 3, saveDir); err != nil {
				t.Fatalf("appendTitleToVideoListFile() error = %v", err)
			}

			data, err := os.ReadFile(listPath)
			if err != nil {
				t.Fatal(err)
			}
			if got := string(data); got != tt.want {
				t.Fatalf("appendTitleToVideoListFile() wrote %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGenerateUniqueTempDirAndCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	tempDir, err := generateUniqueTempDir(tmpDir)
	if err != nil {
		t.Fatalf("generateUniqueTempDir() error = %v", err)
	}

	info, err := os.Stat(tempDir)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if !info.IsDir() {
		t.Fatal("generateUniqueTempDir() did not create a directory")
	}

	cleanupTempDir(tempDir)
	if _, err := os.Stat(tempDir); !os.IsNotExist(err) {
		t.Fatalf("cleanupTempDir() did not remove %s", tempDir)
	}
}

func TestPathHelpers(t *testing.T) {
	if got := convertWindowsPathToWSL(`C:\Users\demo\file.txt`); got != "/mnt/c/Users/demo/file.txt" {
		t.Fatalf("convertWindowsPathToWSL() = %q", got)
	}
	if got := convertWSLPathToWindows("/mnt/d/work/file.txt"); got != `D:\work\file.txt` {
		t.Fatalf("convertWSLPathToWindows() = %q", got)
	}
	if !isWSLPath("/mnt/e/data/cookies.json") {
		t.Fatal("isWSLPath() = false, want true")
	}
	if isWSLPath("/tmp/cookies.json") {
		t.Fatal("isWSLPath() = true, want false")
	}
	if got := normalizePathForTarget(`E:\data\cookies.json`, "linux", true); got != "/mnt/e/data/cookies.json" {
		t.Fatalf("normalizePathForTarget() = %q", got)
	}
	if got := normalizePathForTarget("/mnt/d/work/file.txt", "windows", false); got != `D:\work\file.txt` {
		t.Fatalf("normalizePathForTarget() = %q", got)
	}
	if got := normalizePathForTarget("/tmp/file.txt", "linux", false); got != "/tmp/file.txt" {
		t.Fatalf("normalizePathForTarget() unexpectedly rewrote %q", got)
	}
}

func TestInitHTTPClient(t *testing.T) {
	originalHTTPClient := httpClient
	t.Cleanup(func() {
		httpClient = originalHTTPClient
	})

	initHTTPClient(42)

	if httpClient == nil {
		t.Fatal("initHTTPClient() left httpClient nil")
	}
	if httpClient.Timeout != 42*time.Second {
		t.Fatalf("httpClient.Timeout = %v, want %v", httpClient.Timeout, 42*time.Second)
	}
	if httpClient.Transport == nil {
		t.Fatal("initHTTPClient() left Transport nil")
	}
}

func TestFFmpegError(t *testing.T) {
	oldPath := os.Getenv("PATH")
	if err := os.Setenv("PATH", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Setenv("PATH", oldPath)
	})

	tempDir := t.TempDir()
	saveDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tempDir, "video.ts"), []byte("dummy"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := ffmpeg("video", tempDir, saveDir); err == nil {
		t.Fatal("ffmpeg() error = nil, want non-nil")
	}
}

func TestM3u8Down(t *testing.T) {
	originalNewDownloader := newDownloader
	originalFFmpeg := ffmpegFunc
	t.Cleanup(func() {
		newDownloader = originalNewDownloader
		ffmpegFunc = originalFFmpeg
	})

	t.Run("requires temp dir", func(t *testing.T) {
		if err := M3u8Down("title", "https://example.com/video.m3u8", t.TempDir(), 2, ""); err == nil {
			t.Fatal("M3u8Down() error = nil, want non-nil")
		}
	})

	t.Run("success", func(t *testing.T) {
		fd := &fakeDownloader{defaultDownload: true}
		newDownloader = func() M3u8Downloader.M3u8Downloader { return fd }

		var ffmpegArgs []string
		ffmpegFunc = func(ts, tempDir, saveDir string) error {
			ffmpegArgs = []string{ts, tempDir, saveDir}
			return nil
		}

		tempDir := t.TempDir()
		saveDir := t.TempDir()
		err := M3u8Down(`bad:/title`, "https://example.com/video.m3u8", saveDir, 4, tempDir)
		if err != nil {
			t.Fatalf("M3u8Down() error = %v", err)
		}
		if fd.url != "https://example.com/video.m3u8" {
			t.Fatalf("SetUrl got %q", fd.url)
		}
		if fd.movieName != "bad__title" {
			t.Fatalf("SetMovieName got %q", fd.movieName)
		}
		if fd.threadNum != 4 || !fd.showBar || fd.saveDir != tempDir {
			t.Fatalf("downloader config not propagated: %+v", fd)
		}
		if strings.Join(ffmpegArgs, "|") != strings.Join([]string{`bad:/title`, tempDir, saveDir}, "|") {
			t.Fatalf("ffmpeg args = %v", ffmpegArgs)
		}
	})

	t.Run("download failure", func(t *testing.T) {
		newDownloader = func() M3u8Downloader.M3u8Downloader { return &fakeDownloader{defaultDownload: false} }
		ffmpegCalled := false
		ffmpegFunc = func(ts, tempDir, saveDir string) error {
			ffmpegCalled = true
			return nil
		}

		err := M3u8Down("title", "https://example.com/video.m3u8", t.TempDir(), 1, t.TempDir())
		if err == nil {
			t.Fatal("M3u8Down() error = nil, want non-nil")
		}
		if ffmpegCalled {
			t.Fatal("ffmpeg should not be called after download failure")
		}
	})
}

func TestGetLiveRoomPublicInfo(t *testing.T) {
	originalHTTPClient := httpClient
	originalNewDownloader := newDownloader
	originalFFmpeg := ffmpegFunc
	originalTempDirFactory := tempDirFactory
	originalTempDirCleanup := tempDirCleanup
	t.Cleanup(func() {
		httpClient = originalHTTPClient
		newDownloader = originalNewDownloader
		ffmpegFunc = originalFFmpeg
		tempDirFactory = originalTempDirFactory
		tempDirCleanup = originalTempDirCleanup
	})

	tmpDir := t.TempDir()
	cookiesFile := writeCookiesFile(t, tmpDir)
	config := &Config{CookiesFile: cookiesFile}

	t.Run("success", func(t *testing.T) {
		httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Header.Get("Cookie") == "" {
				t.Fatal("expected Cookie header to be set")
			}
			return jsonHTTPResponse(`{"openLiveDetailModel":{"title":"demo:title","playbackUrl":"https://example.com/media.m3u8"}}`), nil
		})}

		fd := &fakeDownloader{defaultDownload: true}
		newDownloader = func() M3u8Downloader.M3u8Downloader { return fd }
		ffmpegFunc = func(ts, tempDir, saveDir string) error { return nil }

		var createdTempDir string
		tempDirFactory = func(saveDir string) (string, error) {
			dir, err := os.MkdirTemp(saveDir, "room-*")
			if err == nil {
				createdTempDir = dir
			}
			return dir, err
		}
		tempDirCleanup = cleanupTempDir

		title, err := getLiveRoomPublicInfo("room", "uuid", tmpDir, 2, config)
		if err != nil {
			t.Fatalf("getLiveRoomPublicInfo() error = %v", err)
		}
		if title != "demo:title" {
			t.Fatalf("title = %q, want %q", title, "demo:title")
		}
		if _, err := os.Stat(createdTempDir); !os.IsNotExist(err) {
			t.Fatalf("temp dir %q should be removed", createdTempDir)
		}
	})

	t.Run("empty playback url", func(t *testing.T) {
		httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return jsonHTTPResponse(`{"openLiveDetailModel":{"title":"demo","playbackUrl":""}}`), nil
		})}

		if _, err := getLiveRoomPublicInfo("room", "uuid", tmpDir, 2, config); err == nil {
			t.Fatal("getLiveRoomPublicInfo() error = nil, want non-nil")
		}
	})

	t.Run("missing LV_PC_SESSION", func(t *testing.T) {
		badCookies := filepath.Join(tmpDir, "missing-session.json")
		if err := os.WriteFile(badCookies, []byte(`{"foo":"bar"}`), 0600); err != nil {
			t.Fatal(err)
		}

		if _, err := getLiveRoomPublicInfo("room", "uuid", tmpDir, 2, &Config{CookiesFile: badCookies}); err == nil {
			t.Fatal("getLiveRoomPublicInfo() error = nil, want non-nil")
		}
	})

	t.Run("invalid response json", func(t *testing.T) {
		httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return jsonHTTPResponse(`not-json`), nil
		})}

		if _, err := getLiveRoomPublicInfo("room", "uuid", tmpDir, 2, config); err == nil {
			t.Fatal("getLiveRoomPublicInfo() error = nil, want non-nil")
		}
	})

	t.Run("missing detail model", func(t *testing.T) {
		httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return jsonHTTPResponse(`{"other":"value"}`), nil
		})}

		if _, err := getLiveRoomPublicInfo("room", "uuid", tmpDir, 2, config); err == nil {
			t.Fatal("getLiveRoomPublicInfo() error = nil, want non-nil")
		}
	})
}

func TestProcessURL(t *testing.T) {
	originalHTTPClient := httpClient
	originalNewDownloader := newDownloader
	originalFFmpeg := ffmpegFunc
	originalTempDirFactory := tempDirFactory
	originalTempDirCleanup := tempDirCleanup
	t.Cleanup(func() {
		httpClient = originalHTTPClient
		newDownloader = originalNewDownloader
		ffmpegFunc = originalFFmpeg
		tempDirFactory = originalTempDirFactory
		tempDirCleanup = originalTempDirCleanup
	})

	tmpDir := t.TempDir()
	saveDir := filepath.Join(tmpDir, "videos")
	if err := os.MkdirAll(saveDir, 0755); err != nil {
		t.Fatal(err)
	}
	config := &Config{CookiesFile: writeCookiesFile(t, tmpDir)}
	httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return jsonHTTPResponse(`{"openLiveDetailModel":{"title":"lesson:1","playbackUrl":"https://example.com/media.m3u8"}}`), nil
	})}
	newDownloader = func() M3u8Downloader.M3u8Downloader { return &fakeDownloader{defaultDownload: true} }
	ffmpegFunc = func(ts, tempDir, saveDir string) error { return nil }
	tempDirFactory = func(saveDir string) (string, error) { return os.MkdirTemp(saveDir, "proc-*") }
	tempDirCleanup = func(tempDir string) { _ = os.RemoveAll(tempDir) }

	t.Run("success appends title", func(t *testing.T) {
		listPath := filepath.Join(tmpDir, "videos.m3u8")
		if err := createVideoListFile(listPath, ".m3u8"); err != nil {
			t.Fatal(err)
		}

		title, err := processURL("https://example.com/?roomId=1&liveUuid=2", saveDir, 3, config, listPath, ".m3u8", 1)
		if err != nil {
			t.Fatalf("processURL() error = %v", err)
		}
		if title != "lesson:1" {
			t.Fatalf("title = %q", title)
		}

		data, err := os.ReadFile(listPath)
		if err != nil {
			t.Fatal(err)
		}
		if got := string(data); got != "#EXTM3U\nvideos/lesson_1.mp4\n" {
			t.Fatalf("video list = %q", got)
		}
	})

	t.Run("missing query params", func(t *testing.T) {
		if _, err := processURL("https://example.com/?roomId=1", saveDir, 3, config, "", "", 1); err == nil {
			t.Fatal("processURL() error = nil, want non-nil")
		}
	})
}

func TestProcessURLFromFile(t *testing.T) {
	originalHTTPClient := httpClient
	originalNewDownloader := newDownloader
	originalFFmpeg := ffmpegFunc
	originalTempDirFactory := tempDirFactory
	originalTempDirCleanup := tempDirCleanup
	t.Cleanup(func() {
		httpClient = originalHTTPClient
		newDownloader = originalNewDownloader
		ffmpegFunc = originalFFmpeg
		tempDirFactory = originalTempDirFactory
		tempDirCleanup = originalTempDirCleanup
	})

	tmpDir := t.TempDir()
	saveDir := filepath.Join(tmpDir, "videos")
	if err := os.MkdirAll(saveDir, 0755); err != nil {
		t.Fatal(err)
	}
	config := &Config{CookiesFile: writeCookiesFile(t, tmpDir)}
	newDownloader = func() M3u8Downloader.M3u8Downloader { return &fakeDownloader{defaultDownload: true} }
	ffmpegFunc = func(ts, tempDir, saveDir string) error { return nil }
	tempDirFactory = func(saveDir string) (string, error) { return os.MkdirTemp(saveDir, "batch-*") }
	tempDirCleanup = func(tempDir string) { _ = os.RemoveAll(tempDir) }

	httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		roomID := req.URL.Query().Get("roomId")
		return jsonHTTPResponse(fmt.Sprintf(`{"openLiveDetailModel":{"title":"title-%s","playbackUrl":"https://example.com/%s.m3u8"}}`, roomID, roomID)), nil
	})}

	t.Run("success", func(t *testing.T) {
		urlFile := filepath.Join(tmpDir, "urls.txt")
		content := strings.Join([]string{
			"# comment",
			"",
			"https://example.com/?roomId=one&liveUuid=u1",
			"https://example.com/?roomId=two&liveUuid=u2",
		}, "\n")
		if err := os.WriteFile(urlFile, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		titles, err := processURLFromFile(urlFile, saveDir, 2, config, "", "")
		if err != nil {
			t.Fatalf("processURLFromFile() error = %v", err)
		}
		if got := strings.Join(titles, ","); got != "title-one,title-two" {
			t.Fatalf("titles = %q", got)
		}
	})

	t.Run("partial failure", func(t *testing.T) {
		urlFile := filepath.Join(tmpDir, "urls-invalid.txt")
		content := strings.Join([]string{
			"https://example.com/?roomId=one&liveUuid=u1",
			"https://example.com/?roomId=two",
		}, "\n")
		if err := os.WriteFile(urlFile, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		titles, err := processURLFromFile(urlFile, saveDir, 2, config, "", "")
		if err == nil {
			t.Fatal("processURLFromFile() error = nil, want non-nil")
		}
		if len(titles) != 1 || titles[0] != "title-one" {
			t.Fatalf("titles = %v, want [title-one]", titles)
		}
	})
}

// ==================== Issue #6: URL Parsing Tests ====================

func TestExtractParamsFromURL(t *testing.T) {
	tests := []struct {
		name         string
		urlStr       string
		wantRoomId   string
		wantLiveUuid string
		wantErr      bool
	}{
		{
			name:         "标准查询参数格式",
			urlStr:       "https://n.dingtalk.com/dingding/live-room/index.html?roomId=12345&liveUuid=abc-def-123",
			wantRoomId:   "12345",
			wantLiveUuid: "abc-def-123",
			wantErr:      false,
		},
		{
			name:         "只有liveUuid的查询参数",
			urlStr:       "https://h5.dingtalk.com/group-live-share/index.htm?type=2&liveUuid=abc-def-123",
			wantRoomId:   "",
			wantLiveUuid: "abc-def-123",
			wantErr:      false,
		},
		{
			name:         "Hash路由带查询参数",
			urlStr:       "https://n.dingtalk.com/dingding/live-room/index.html?roomId=12345#/live?liveUuid=abc-def-123",
			wantRoomId:   "12345",
			wantLiveUuid: "abc-def-123",
			wantErr:      false,
		},
		{
			name:         "Hash路由路径格式",
			urlStr:       "https://n.dingtalk.com/app/live/index.html#/room/12345/live/abc-def-123",
			wantRoomId:   "12345",
			wantLiveUuid: "abc-def-123",
			wantErr:      false,
		},
		{
			name:         "路径参数格式",
			urlStr:       "https://n.dingtalk.com/dingding/live-room/12345/abc-def-123",
			wantRoomId:   "12345",
			wantLiveUuid: "abc-def-123",
			wantErr:      false,
		},
		{
			name:         "复杂Hash路由",
			urlStr:       "https://h5.dingtalk.com/group-live-share/index.htm?type=2&liveFromType=6&liveUuid=abc-def#/union",
			wantRoomId:   "",
			wantLiveUuid: "abc-def",
			wantErr:      false,
		},
		{
			name:         "URL编码的参数",
			urlStr:       "https://n.dingtalk.com/dingding/live-room/index.html?roomId=123%2045&liveUuid=abc%20def",
			wantRoomId:   "123 45",
			wantLiveUuid: "abc def",
			wantErr:      false,
		},
		{
			name:         "空URL",
			urlStr:       "",
			wantRoomId:   "",
			wantLiveUuid: "",
			wantErr:      true,
		},
		{
			name:         "无效URL",
			urlStr:       "://invalid-url",
			wantRoomId:   "",
			wantLiveUuid: "",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRoomId, gotLiveUuid, err := extractParamsFromURL(tt.urlStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractParamsFromURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotRoomId != tt.wantRoomId {
				t.Errorf("extractParamsFromURL() gotRoomId = %v, want %v", gotRoomId, tt.wantRoomId)
			}
			if gotLiveUuid != tt.wantLiveUuid {
				t.Errorf("extractParamsFromURL() gotLiveUuid = %v, want %v", gotLiveUuid, tt.wantLiveUuid)
			}
		})
	}
}

func TestExtractParamsFromURL_RealWorldCases(t *testing.T) {
	// 测试真实世界的URL格式（来自issue报告）
	tests := []struct {
		name         string
		urlStr       string
		wantRoomId   string
		wantLiveUuid string
	}{
		{
			name:         "Issue #6 可能的URL格式1 - 标准格式",
			urlStr:       "https://n.dingtalk.com/dingding/live-room/index.html?roomId=123456789&liveUuid=abcdef-123456",
			wantRoomId:   "123456789",
			wantLiveUuid: "abcdef-123456",
		},
		{
			name:         "Issue #6 可能的URL格式2 - 带hash",
			urlStr:       "https://h5.dingtalk.com/group-live-share/index.htm?liveUuid=test-uuid-123#/union",
			wantRoomId:   "",
			wantLiveUuid: "test-uuid-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRoomId, gotLiveUuid, err := extractParamsFromURL(tt.urlStr)
			if err != nil {
				t.Errorf("extractParamsFromURL() unexpected error = %v", err)
				return
			}
			if gotRoomId != tt.wantRoomId {
				t.Errorf("extractParamsFromURL() gotRoomId = %v, want %v", gotRoomId, tt.wantRoomId)
			}
			if gotLiveUuid != tt.wantLiveUuid {
				t.Errorf("extractParamsFromURL() gotLiveUuid = %v, want %v", gotLiveUuid, tt.wantLiveUuid)
			}
		})
	}
}
