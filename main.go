package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"GoDingtalk/M3u8Downloader"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// isWSLEnvironment 检测是否运行在WSL环境中
func isWSLEnvironment() bool {
	// 检测WSL特有的文件
	wslInteropFile := "/proc/sys/fs/binfmt_misc/WSLInterop"
	if _, err := os.Stat(wslInteropFile); err == nil {
		return true
	}

	// 检测WSL版本文件
	wslVersionFile := "/proc/version"
	if content, err := os.ReadFile(wslVersionFile); err == nil {
		if strings.Contains(strings.ToLower(string(content)), "microsoft") {
			return true
		}
	}

	return false
}

// convertWindowsPathToWSL 将Windows路径转换为WSL路径
func convertWindowsPathToWSL(windowsPath string) string {
	// 移除Windows路径中的转义反斜杠
	cleanPath := strings.ReplaceAll(windowsPath, "\\", "/")

	// 匹配Windows驱动器路径（如 C:/Users/...）
	if len(cleanPath) >= 2 && cleanPath[1] == ':' {
		driveLetter := strings.ToLower(string(cleanPath[0]))
		// 转换为WSL路径格式：/mnt/c/Users/...
		wslPath := "/mnt/" + driveLetter + cleanPath[2:]
		return wslPath
	}

	return cleanPath
}

// convertWSLPathToWindows 将WSL路径转换为Windows路径
func convertWSLPathToWindows(wslPath string) string {
	// 匹配WSL路径格式（如 /mnt/c/Users/...）
	if strings.HasPrefix(wslPath, "/mnt/") && len(wslPath) > 5 {
		driveLetter := string(wslPath[5])
		// 转换为Windows路径格式：C:/Users/...
		windowsPath := strings.ToUpper(driveLetter) + ":" + wslPath[6:]
		// 将正斜杠转换为反斜杠
		windowsPath = strings.ReplaceAll(windowsPath, "/", "\\")
		return windowsPath
	}

	return wslPath
}

func isWindowsPath(path string) bool {
	return strings.Contains(path, "\\") || (len(path) >= 2 && path[1] == ':')
}

func isWSLPath(path string) bool {
	return strings.HasPrefix(path, "/mnt/") && len(path) > len("/mnt/x/")
}

func normalizePathForTarget(path string, goos string, inWSL bool) string {
	switch goos {
	case "linux":
		if inWSL && isWindowsPath(path) {
			return convertWindowsPathToWSL(path)
		}
	case "windows":
		if isWSLPath(path) {
			return convertWSLPathToWindows(path)
		}
	}
	return path
}

func normalizePathForRuntime(path string) string {
	return normalizePathForTarget(path, runtime.GOOS, isWSLEnvironment())
}

// sanitizeInvalidChars 匹配文件名中的非法字符（/ \ : * ? " < > |）。
// 预编译为包级变量全局复用，避免每次调用 sanitizeFileName 都重新编译正则。
var sanitizeInvalidChars = regexp.MustCompile(`[\\/:*?"<>|]`)

// sanitizeFileName 清理文件名中的非法字符，替换为下划线
func sanitizeFileName(fileName string) string {
	// 经过测试，只有以下字符会影响文件路径创建：/ \ : * ? " < > |
	// 空格和反引号不会影响，所以不需要清理
	sanitized := sanitizeInvalidChars.ReplaceAllString(fileName, "_")

	// 如果清理后为空字符串，使用默认名称
	if sanitized == "" {
		sanitized = "unnamed_video"
	}

	return sanitized
}

// videoTimeLayout 文件名中附加的直播发布时间格式
const videoTimeLayout = "20060102_150405"

// publishedAtFields 直播发布时间字段（毫秒时间戳），startTime 为开播时间，endTime 为结束时间
var publishedAtFields = []string{"startTime", "endTime"}

// videoName 由标题和发布时间组成保存用的文件名，避免同名视频互相覆盖
func videoName(title string, publishedAt time.Time) string {
	if publishedAt.IsZero() {
		return sanitizeFileName(title)
	}
	return sanitizeFileName(title + "_" + publishedAt.Format(videoTimeLayout))
}

// extractPublishedAt 从直播详情中解析发布时间，接口未返回时返回零值
func extractPublishedAt(detail map[string]interface{}) time.Time {
	for _, field := range publishedAtFields {
		if ms, ok := detail[field].(float64); ok && ms > 0 {
			return time.UnixMilli(int64(ms))
		}
	}
	return time.Time{}
}

// Version 程序版本号，通过 -ldflags "-X main.Version=vX.X.X" 注入
var Version = "dev"

// dingtalkUserAgent 请求钉钉接口使用的 User-Agent
const dingtalkUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"

// 全局HTTP客户端，在 main 中根据配置初始化
var httpClient *http.Client

var (
	newDownloader  = M3u8Downloader.NewDownloader
	ffmpegFunc     = ffmpeg
	tempDirFactory = generateUniqueTempDir
	tempDirCleanup = cleanupTempDir
	executablePath = os.Executable
)

// initHTTPClient 初始化全局HTTP客户端
func initHTTPClient(timeout int) {
	httpClient = &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}
}

// findFFmpeg 定位 ffmpeg 可执行文件，返回可直接用于 exec.Command 的完整路径。
// 查找顺序：
//  1. PATH（仅当 LookPath 无错误时才算找到；Go 1.19+ 在 Windows 上通过当前目录
//     隐式解析时会返回 exec.ErrDot，这种情况不能直接使用）
//  2. 程序自身所在目录（Windows 用户常把 ffmpeg.exe 放在程序旁边）
func findFFmpeg() (string, error) {
	if path, err := exec.LookPath("ffmpeg"); err == nil {
		return path, nil
	}

	exePath, err := executablePath()
	if err != nil {
		return "", fmt.Errorf("无法定位程序所在目录: %w", err)
	}
	names := []string{"ffmpeg"}
	if runtime.GOOS == "windows" {
		names = []string{"ffmpeg.exe", "ffmpeg"}
	}
	for _, name := range names {
		path := filepath.Join(filepath.Dir(exePath), name)
		if info, statErr := os.Stat(path); statErr == nil && !info.IsDir() {
			return path, nil
		}
	}

	return "", fmt.Errorf("未找到 ffmpeg，请安装 FFmpeg 并加入 PATH，或将 ffmpeg 可执行文件放到程序所在目录（用于视频格式转换）")
}

// ffmpeg 把ts转换mp4
func ffmpeg(name, tempDir, saveDir string) error {
	fmt.Println("正在转换ts为mp4...")

	// 清理文件名中的非法字符
	sanitizedVideoName := sanitizeFileName(name)
	tsPath := filepath.Join(tempDir, sanitizedVideoName+".ts")
	mp4Path := filepath.Join(saveDir, sanitizedVideoName+".mp4")

	ffmpegPath, err := findFFmpeg()
	if err != nil {
		return err
	}
	cmd := exec.Command(ffmpegPath, "-i", tsPath, "-c:v", "copy", "-c:a", "copy", "-f", "mp4", "-y", mp4Path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("FFmpeg转换失败: %v\n输出: %s\n", err, string(output))
		return fmt.Errorf("ffmpeg conversion failed: %w", err)
	}

	// 使用清理后的文件名输出日志
	fmt.Println(sanitizedVideoName + ".mp4 转换完成")

	return nil
}

// dingtalkHeaders 构造请求钉钉回放接口所需的请求头
func dingtalkHeaders(cookie string) http.Header {
	h := http.Header{}
	h.Set("Host", "lv.dingtalk.com")
	h.Set("Cookie", cookie)
	h.Set("Sec-Ch-Ua", `"Chromium";v="122", "Not(A:Brand";v="24", "Google Chrome";v="122"`)
	h.Set("Sec-Ch-Ua-Mobile", "?0")
	h.Set("Sec-Ch-Ua-Platform", "macOS")
	h.Set("Dnt", "1")
	h.Set("Upgrade-Insecure-Requests", "1")
	h.Set("User-Agent", dingtalkUserAgent)
	h.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	h.Set("Sec-Fetch-Site", "none")
	h.Set("Sec-Fetch-Mode", "navigate")
	h.Set("Sec-Fetch-User", "?1")
	h.Set("Sec-Fetch-Dest", "document")
	h.Set("Accept-Language", "zh-CN,zh;q=0.9")
	return h
}

// checkFFmpegAvailable 检查 ffmpeg 是否可用（PATH 或程序所在目录）
func checkFFmpegAvailable() error {
	_, err := findFFmpeg()
	return err
}

// generateUniqueTempDir 生成唯一的临时目录名称
func generateUniqueTempDir(saveDir string) (string, error) {
	pattern := fmt.Sprintf(".videoTemp%s_*", time.Now().Format("20060102"))
	tempDir, err := os.MkdirTemp(saveDir, pattern)
	if err != nil {
		return "", fmt.Errorf("创建临时目录失败: %w", err)
	}
	return tempDir, nil
}

func cleanupTempDir(tempDir string) {
	if err := os.RemoveAll(tempDir); err != nil {
		fmt.Printf("警告: 删除临时文件夹失败: %v\n", err)
		return
	}
	fmt.Println("临时文件夹清理完成")
}

// startChrome 函数启动Chrome浏览器，访问钉钉登录页面，获取并保存Cookies到本地文件。
func startChrome(config *Config) error {
	fmt.Println("正在启动Chrome获取Cookies...")

	// 抑制 chromedp 的日志输出（保存当前输出目标，结束时恢复，避免污染全局 log 状态）
	prevLogOut := log.Writer()
	log.SetOutput(io.Discard)
	defer log.SetOutput(prevLogOut)

	opts := append(
		// 跳过前3个选项以禁用 headless 模式，让浏览器可见
		chromedp.DefaultExecAllocatorOptions[3:],
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("excludeSwitches", "enable-automation"),
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
	)

	// 如果指定了 Chrome 路径，使用它
	if config.ChromePath != "" {
		normalizedPath := normalizePathForRuntime(config.ChromePath)
		if normalizedPath != config.ChromePath {
			fmt.Printf("Chrome路径已转换: %s → %s\n", config.ChromePath, normalizedPath)
		}
		opts = append(opts, chromedp.ExecPath(normalizedPath))
		fmt.Printf("使用指定的Chrome路径: %s\n", normalizedPath)
	} else {
		fmt.Println("使用系统自动查找的Chrome")
	}

	var siteCookies []*network.Cookie
	parentCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(parentCtx)
	defer cancel()

	// 使用配置文件中的超时时间
	ctx, cancel = context.WithTimeout(ctx, time.Duration(config.ChromeTimeout)*time.Minute)
	defer cancel()

	// 访问钉钉登录页面
	H5url := "https://h5.dingtalk.com"
	Lurl := "https://login.dingtalk.com/oauth2/challenge.htm?client_id=dingavo6at488jbofmjs&response_type=code&scope=openid&redirect_uri=https%3A%2F%2Flv.dingtalk.com%2Fsso%2Flogin%3Fcontinue%3Dhttps%253A%252F%252Fh5.dingtalk.com%252Fgroup-live-share%252Findex.htm%253Ftype%253D2%2523%252F"

	fmt.Println("请在浏览器中完成登录...")
	err := chromedp.Run(ctx,
		network.Enable(), // 启用网络事件
		chromedp.Navigate(Lurl),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var currentURL string
			for {
				if err := chromedp.Evaluate(`window.location.href`, &currentURL).Do(ctx); err != nil {
					return err
				}

				if strings.Contains(currentURL, H5url) {
					fmt.Println("登录成功，正在获取Cookies...")
					break
				}
				time.Sleep(2 * time.Second)
			}
			return nil
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			// 到达此处，说明已经跳转到了指定的URL
			var err error
			siteCookies, err = network.GetCookies().Do(ctx)
			if err != nil {
				return fmt.Errorf("获取 Cookies 失败: %w", err)
			}
			return nil
		}),
	)

	if err != nil {
		return fmt.Errorf("Chrome 自动化操作失败: %w", err)
	}

	// 保存cookies到文件
	cookies := make(map[string]string)
	for _, cookie := range siteCookies {
		cookies[cookie.Name] = cookie.Value
	}
	jsonCookies, err := json.Marshal(cookies)
	if err != nil {
		return fmt.Errorf("序列化 Cookies 失败: %w", err)
	}

	// 将获取到的 Cookies 保存到配置文件指定的文件中
	// config.CookiesFile 存储了 Cookies 文件的完整路径
	// 文件权限设置为 0600，确保只有当前用户可读写
	fmt.Printf("Cookies 保存到: %s\n", config.CookiesFile)
	if err := os.WriteFile(config.CookiesFile, jsonCookies, 0600); err != nil {
		return fmt.Errorf("保存 Cookies 文件失败: %w", err)
	}

	fmt.Println("Cookies保存成功")
	return nil
}

// M3u8Down 函数用于下载直播回放视频
// name：保存用的文件名（标题 + 发布时间，不含扩展名）
// playbackUrl：直播回放链接
// saveDir: 保存目录
// Thread：线程数
// tempDir: 临时目录（必须提供，不能为空）
// 注意：临时目录的创建和清理由调用方负责
func M3u8Down(name, playbackUrl, saveDir string, Thread int, tempDir string) error {
	// 临时目录必须由调用方提供
	if tempDir == "" {
		return fmt.Errorf("临时目录不能为空")
	}

	m3u8 := newDownloader()
	m3u8.SetUrl(playbackUrl)

	// 清理文件名中的非法字符，避免文件路径创建失败
	sanitizedVideoName := sanitizeFileName(name)
	m3u8.SetMovieName(sanitizedVideoName)

	m3u8.SetNumOfThread(Thread)
	m3u8.SetIfShowTheBar(true)
	m3u8.SetSaveDirectory(tempDir)

	if !m3u8.DefaultDownload() {
		return fmt.Errorf("下载失败")
	}
	fmt.Println("下载成功")

	if err := ffmpegFunc(name, tempDir, saveDir); err != nil {
		return fmt.Errorf("视频转换失败: %w", err)
	}

	return nil
}

// getLiveRoomPublicInfo 函数用于获取钉钉直播间的公开信息
// roomId：直播间ID
// liveUuid：直播UUID
// 返回值中的 publishedAt 为直播发布时间，接口未提供时为零值
func getLiveRoomPublicInfo(roomId, liveUuid, saveDir string, Thread int, config *Config) (string, time.Time, error) {
	// 构造URL
	urlStr := "https://lv.dingtalk.com/getOpenLiveInfo?roomId=" + roomId + "&liveUuid=" + liveUuid
	urlObj, err := url.Parse(urlStr)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("URL 解析失败: %w", err)
	}

	// 创建请求
	req, err := http.NewRequest("GET", urlObj.String(), nil)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("创建请求失败: %w", err)
	}

	// 读取Cookies文件
	jsonCookies, err := os.ReadFile(config.CookiesFile)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("读取 Cookies 文件失败: %w", err)
	}

	var cookies map[string]string
	if err := json.Unmarshal(jsonCookies, &cookies); err != nil {
		return "", time.Time{}, fmt.Errorf("解析 Cookies 失败: %w", err)
	}

	// 添加Cookies到请求
	var cookieStr strings.Builder
	for name, value := range cookies {
		cookieStr.WriteString(fmt.Sprintf("%s=%s; ", name, value))
	}
	// 确保 PC_SESSION 使用 LV_PC_SESSION 的值
	CookiepcSession, ok := cookies["LV_PC_SESSION"]
	if !ok {
		return "", time.Time{}, fmt.Errorf("未找到 LV_PC_SESSION Cookie，请重新登录")
	}
	cookieStr.WriteString(fmt.Sprintf("PC_SESSION=%s", CookiepcSession))
	cookieHeader := cookieStr.String()

	// 设置请求头（集中维护，便于钉钉接口变更时统一调整）
	req.Header = dingtalkHeaders(cookieHeader)

	// 发送请求（使用全局 HTTP 客户端）
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 检查 HTTP 状态码，避免把错误页（如 401/302 跳转）当作正常 JSON 解析
	if resp.StatusCode != http.StatusOK {
		return "", time.Time{}, fmt.Errorf("钉钉接口请求失败(HTTP %d)，登录可能已过期，请使用 -login 参数重新登录", resp.StatusCode)
	}

	// 读取响应内容
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("读取响应内容失败: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", time.Time{}, fmt.Errorf("解析响应 JSON 失败: %w", err)
	}

	// 钉钉接口在登录失效等场景下返回 {"code":"1003"} 形式的错误，优先检查
	if code, ok := result["code"].(string); ok && code != "" && code != "0" {
		return "", time.Time{}, fmt.Errorf("钉钉接口返回错误(code=%s)，登录可能已过期，请使用 -login 参数重新登录", code)
	}

	// 安全地获取嵌套字段
	openLiveDetailModel, ok := result["openLiveDetailModel"].(map[string]interface{})
	if !ok {
		return "", time.Time{}, fmt.Errorf("响应格式错误: 未找到 openLiveDetailModel 字段（登录可能已过期，请使用 -login 参数重新登录）")
	}

	title, ok := openLiveDetailModel["title"].(string)
	if !ok {
		return "", time.Time{}, fmt.Errorf("响应格式错误: 未找到 title 字段（登录可能已过期，请使用 -login 参数重新登录）")
	}

	playbackUrl, ok := openLiveDetailModel["playbackUrl"].(string)
	if !ok {
		return "", time.Time{}, fmt.Errorf("响应格式错误: 未找到 playbackUrl 字段（登录可能已过期，请使用 -login 参数重新登录）")
	}

	publishedAt := extractPublishedAt(openLiveDetailModel)

	fmt.Println("标题:", title)
	fmt.Println("回放地址:", playbackUrl)
	if !publishedAt.IsZero() {
		fmt.Println("发布时间:", publishedAt.Format("2006-01-02 15:04:05"))
	}

	// 检查回放地址是否为空
	if playbackUrl == "" {
		return "", publishedAt, fmt.Errorf("回放地址为空，可能直播尚未结束或回放不可用")
	}

	tempDir, err := tempDirFactory(saveDir)
	if err != nil {
		return title, publishedAt, err
	}
	fmt.Printf("临时文件夹创建成功: %s\n", tempDir)
	defer tempDirCleanup(tempDir)

	name := videoName(title, publishedAt)
	fmt.Println("保存文件名:", name)

	if err := M3u8Down(name, playbackUrl, saveDir, Thread, tempDir); err != nil {
		return title, publishedAt, err
	}

	return title, publishedAt, nil
}

// extractParamsFromURL 从多种格式的钉钉直播URL中提取 roomId 和 liveUuid
// 支持的格式：
// 1. 查询参数: ?roomId=XXX&liveUuid=XXX
// 2. Hash路由: #/live?roomId=XXX&liveUuid=XXX 或 #/room/XXX/live/XXX
// 3. 路径参数: /live-room/XXX/XXX
// 4. 只有liveUuid: ?liveUuid=XXX (某些API只需要liveUuid)
func extractParamsFromURL(urlStr string) (roomId, liveUuid string, err error) {
	if strings.TrimSpace(urlStr) == "" {
		return "", "", fmt.Errorf("URL 不能为空")
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", "", fmt.Errorf("解析 URL 时出错: %w", err)
	}

	// 1. 首先尝试从查询参数提取
	queryParams := parsedURL.Query()
	roomId = queryParams.Get("roomId")
	liveUuid = queryParams.Get("liveUuid")

	// 2. 如果查询参数中没有，尝试从 hash 片段提取
	if roomId == "" || liveUuid == "" {
		hash := parsedURL.Fragment
		if hash != "" {
			// Hash 可能包含查询参数，如 #/union?roomId=XXX&liveUuid=XXX
			// 需要找到 ? 后面的部分
			hashQueryIndex := strings.Index(hash, "?")
			if hashQueryIndex != -1 {
				hashQuery := hash[hashQueryIndex+1:]
				hashURL, err := url.Parse("http://dummy.com/?" + hashQuery)
				if err == nil {
					hashParams := hashURL.Query()
					if roomId == "" {
						roomId = hashParams.Get("roomId")
					}
					if liveUuid == "" {
						liveUuid = hashParams.Get("liveUuid")
					}
				}
			}

			// 尝试从 hash 路径中提取，如 #/room/12345/live/abc-def
			if roomId == "" || liveUuid == "" {
				hashParts := strings.Split(hash, "/")
				for i, part := range hashParts {
					lowerPart := strings.ToLower(part)
					if lowerPart == "room" && i+1 < len(hashParts) && roomId == "" {
						roomId = hashParts[i+1]
					}
					if lowerPart == "live" && i+1 < len(hashParts) && liveUuid == "" {
						liveUuid = hashParts[i+1]
					}
				}
			}
		}
	}

	// 3. 尝试从路径中提取，如 /live-room/12345/abc-def
	if roomId == "" || liveUuid == "" {
		pathParts := strings.Split(parsedURL.Path, "/")
		for i, part := range pathParts {
			lowerPart := strings.ToLower(part)
			if (lowerPart == "live-room" || lowerPart == "liveroom") && i+2 < len(pathParts) {
				if roomId == "" {
					roomId = pathParts[i+1]
				}
				if liveUuid == "" {
					liveUuid = pathParts[i+2]
				}
			}
		}
	}

	// 4. 清理参数（移除可能的额外字符）
	roomId = strings.TrimSpace(roomId)
	liveUuid = strings.TrimSpace(liveUuid)

	// 移除 URL 编码
	roomId, _ = url.QueryUnescape(roomId)
	liveUuid, _ = url.QueryUnescape(liveUuid)

	return roomId, liveUuid, nil
}

// processURL 函数接收一个URL字符串作为参数，并解析出其中的roomId和liveUuid参数
// 然后调用getLiveRoomPublicInfo函数进行处理
// 如果URL解析出错或缺少roomId或liveUuid参数，则打印错误信息并返回
func processURL(urlStr, saveDir string, Thread int, config *Config, videoListFile, fileExt string, processedCount int) (string, time.Time, error) {
	roomId, liveUuid, err := extractParamsFromURL(urlStr)
	if err != nil {
		return "", time.Time{}, err
	}

	if roomId == "" || liveUuid == "" {
		return "", time.Time{}, fmt.Errorf("URL 中缺少 roomId 或 liveUuid 参数，请确保 URL 格式正确。支持的格式包括：\n" +
			"1. 查询参数格式: ?roomId=XXX&liveUuid=XXX\n" +
			"2. Hash路由格式: #/live?roomId=XXX&liveUuid=XXX\n" +
			"3. 路径参数格式: /live-room/XXX/XXX")
	}

	title, publishedAt, err := getLiveRoomPublicInfo(roomId, liveUuid, saveDir, Thread, config)

	// 下载完成后立即追加标题到视频列表文件
	if err == nil && videoListFile != "" && title != "" {
		name := videoName(title, publishedAt)
		if appendErr := appendTitleToVideoListFile(videoListFile, name, fileExt, processedCount, saveDir); appendErr != nil {
			fmt.Printf("警告: 追加标题到视频列表文件失败: %v\n", appendErr)
		} else {
			fmt.Printf("标题已添加到视频列表文件: %s (原始标题: %s)\n", name, title)
		}
	}

	return title, publishedAt, err
}

// processURLFromFile 从文件中读取URL进行处理
func processURLFromFile(filePath, saveDir string, Thread int, config *Config, videoListFile, fileExt string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("打开文件时出错: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	processedCount := 0 // 实际处理的URL计数器
	var errors []error
	var titles []string

	for scanner.Scan() {
		lineNum++
		urlStr := strings.TrimSpace(scanner.Text())
		if urlStr == "" || strings.HasPrefix(urlStr, "#") {
			continue // 跳过空行和注释
		}

		processedCount++ // 只有实际处理的URL才递增
		fmt.Printf("\n[%d] 处理 URL: %s\n", processedCount, urlStr)
		title, _, err := processURL(urlStr, saveDir, Thread, config, videoListFile, fileExt, processedCount)
		if err != nil {
			errMsg := fmt.Errorf("第 %d 行处理失败: %w", lineNum, err)
			fmt.Println(errMsg)
			errors = append(errors, errMsg)
		} else {
			titles = append(titles, title)
			// 标题已经在 processURL 函数中追加到视频列表文件，这里不需要重复追加
		}
	}

	if err := scanner.Err(); err != nil {
		return titles, fmt.Errorf("读取文件时出错: %w", err)
	}

	if len(errors) > 0 {
		return titles, fmt.Errorf("批量处理完成，%d 个 URL 处理失败", len(errors))
	}

	return titles, nil
}

// checkCookiesValid 检查cookies文件是否存在且有效
func checkCookiesValid(cookiesFile string) bool {
	// 检查文件是否存在
	if _, err := os.Stat(cookiesFile); os.IsNotExist(err) {
		return false
	}

	// 尝试读取和解析
	jsonCookies, err := os.ReadFile(cookiesFile)
	if err != nil {
		return false
	}

	var cookies map[string]string
	if err := json.Unmarshal(jsonCookies, &cookies); err != nil {
		return false
	}

	// 检查关键cookie是否存在
	if _, ok := cookies["LV_PC_SESSION"]; !ok {
		return false
	}

	return true
}

// main 函数是程序的入口点
func main() {
	fmt.Println("  _______   _______   _______   _______   _______   _______ ")
	fmt.Println(" |       | |       | |   _   | |   _   | |   _   | |   _   |")
	fmt.Println(" |   Go  | |  Ding | |  | |  | |  | |  | |  | |  | |  | |  |")
	fmt.Println(" |       | |  talk | |  |_|  | |  |_|  | |  |_|  | |  |_|  |")
	fmt.Println(" |_______| |_______| |_______| |_______| |_______| |_______|")

	// 判断系统类型
	fmt.Printf("当前系统:")
	switch runtime.GOOS {
	case "windows":
		fmt.Println("Windows")
	case "linux":
		fmt.Println("Linux")
	case "darwin":
		fmt.Println("macOS")
	default:
		fmt.Println("Others")
	}

	// 命令行参数
	versionFlag := flag.Bool("version", false, "显示版本号")
	configFile := flag.String("config", "", "配置文件路径")
	loginFlag := flag.Bool("login", false, "强制重新登录获取Cookies")
	urlFlag := flag.String("url", "", "需要下载的回放URL，格式为 -url \"https://n.dingtalk.com/dingding/live-room/index.html?roomId=XXXX&liveUuid=XXXX\"")
	urlFile := flag.String("urlFile", "", "包含需要下载的回放URL的文件路径，格式为 -urlFile \"/path/to/file\"")
	Thread := flag.Int("thread", 0, "下载线程数 (默认: 10)")
	saveDir := flag.String("saveDir", "", "视频保存目录 (默认: video/)")
	videoListFile := flag.String("videoList", "", "视频列表文件路径，格式为 -videoList \"/path/to/video_list.txt\"")
	httpTimeout := flag.Int("httpTimeout", 0, "HTTP超时时间，单位秒 (默认: 30)")
	chromeTimeout := flag.Int("chromeTimeout", 0, "Chrome登录超时时间，单位分钟 (默认: 20)")
	chromePath := flag.String("chromePath", "", "Chrome可执行文件路径，用于指定特定的Chrome/Chromium位置")
	cookiesFile := flag.String("cookies", "", "Cookies文件路径")

	flag.Parse()

	// 显示版本号
	if *versionFlag {
		fmt.Printf("GoDingtalk %s\n", Version)
		os.Exit(0)
	}

	// 加载配置文件
	config, err := LoadConfig(*configFile)
	if err != nil {
		fmt.Printf("警告: 加载配置文件失败: %v，使用默认配置\n", err)
		config = DefaultConfig()
	}

	// 命令行参数覆盖配置文件
	if *Thread <= 0 {
		*Thread = config.ThreadCount
	}
	if *saveDir == "" {
		*saveDir = config.SaveDirectory
	}
	if *httpTimeout > 0 {
		config.HTTPTimeout = *httpTimeout
	}
	if *chromeTimeout > 0 {
		config.ChromeTimeout = *chromeTimeout
	}
	if *chromePath != "" {
		config.ChromePath = *chromePath
	}
	if *cookiesFile != "" {
		config.CookiesFile = *cookiesFile
	}

	normalizedCookies := normalizePathForRuntime(config.CookiesFile)
	if normalizedCookies != config.CookiesFile {
		fmt.Printf("Cookies文件路径已转换: %s → %s\n", config.CookiesFile, normalizedCookies)
		config.CookiesFile = normalizedCookies
	}

	normalizedSaveDir := normalizePathForRuntime(*saveDir)
	if normalizedSaveDir != *saveDir {
		fmt.Printf("保存目录路径已转换: %s → %s\n", *saveDir, normalizedSaveDir)
		*saveDir = normalizedSaveDir
	}

	// 初始化全局 HTTP 客户端，并注入到下载器子包，
	// 使 -httpTimeout 配置对实际的 ts 分片下载同样生效
	initHTTPClient(config.HTTPTimeout)
	M3u8Downloader.SetHTTPClient(httpClient)

	// 参数验证
	if *urlFlag == "" && *urlFile == "" && !*loginFlag {
		fmt.Println("错误: 未提供 URL 或 URL 文件路径")
		flag.Usage()
		os.Exit(1)
	}

	if *Thread <= 0 || *Thread > 100 {
		fmt.Println("错误: 线程数必须在 1-100 之间")
		os.Exit(1)
	}

	// 规范化保存目录路径
	*saveDir = filepath.Clean(*saveDir) + string(filepath.Separator)

	// 检查cookies是否有效，无效则重新登录
	if *loginFlag || !checkCookiesValid(config.CookiesFile) {
		if *loginFlag {
			fmt.Println("强制重新登录...")
		} else {
			fmt.Println("Cookies无效或不存在，需要重新登录...")
		}
		if err := startChrome(config); err != nil {
			fmt.Printf("错误: 获取Cookies失败: %v\n", err)
			os.Exit(1)
		}
	} else {
		fmt.Println("使用现有Cookies...")
	}

	// 仅登录模式：只登录不下载
	if *urlFlag == "" && *urlFile == "" {
		fmt.Println("\n登录完成！")
		os.Exit(0)
	}

	// 下载流程依赖 ffmpeg 做格式转换，提前检查是否可用
	if err := checkFFmpegAvailable(); err != nil {
		fmt.Printf("错误: %v\n", err)
		os.Exit(1)
	}

	// 创建视频列表文件（在下载前创建）
	var fileExt string
	if *videoListFile != "" {
		fileExt = strings.ToLower(filepath.Ext(*videoListFile))
		if err := createVideoListFile(*videoListFile, fileExt); err != nil {
			fmt.Printf("\n警告: 创建视频列表文件失败: %v\n", err)
		} else {
			fmt.Printf("视频列表文件已创建: %s (格式: %s)\n", *videoListFile, fileExt)
		}
	}

	// 处理URL
	if *urlFlag != "" {
		_, _, err = processURL(*urlFlag, *saveDir, *Thread, config, *videoListFile, fileExt, 1)
	} else if *urlFile != "" {
		_, err = processURLFromFile(*urlFile, *saveDir, *Thread, config, *videoListFile, fileExt)
	}

	if err != nil {
		fmt.Printf("\n错误: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n所有任务完成！")
}

// createVideoListFile 创建视频列表文件（在下载前创建文件）
func createVideoListFile(filePath string, fileExt string) error {
	// 创建或清空文件
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer file.Close()

	fileHeader := ""
	fmt.Printf("检测到%s文件，正在创建视频列表\n", fileExt)
	switch fileExt {
	case ".txt":
		fileHeader = ""
	case ".m3u", ".m3u8":
		fileHeader = "#EXTM3U\n"
	case ".dpl":
		fileHeader = "DAUMPLAYLIST\nplayname=\ntopindex=0\nsaveplaypos=0\n"
	default:
		fileHeader = ""
		fmt.Printf("警告：未知的文件扩展名%s，将按.txt文件格式进行处理\n", fileExt)
	}

	_, err = file.WriteString(fileHeader)
	if err != nil {
		return fmt.Errorf("写入文件头失败: %w", err)
	}

	return nil
}

// appendTitleToVideoListFile 向视频列表文件追加视频文件名
func appendTitleToVideoListFile(filePath, name, fileExt string, processedCount int, saveDir string) error {
	// 以追加模式打开文件
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	videoPath := filepath.Join(saveDir, name+".mp4")
	relPath, err := filepath.Rel(filepath.Dir(filePath), videoPath)
	if err != nil {
		return fmt.Errorf("计算相对路径失败: %w", err)
	}

	switch fileExt {
	case ".txt":
		_, err = file.WriteString(name + "\n")
	case ".m3u", ".m3u8":
		_, err = file.WriteString(filepath.ToSlash(relPath) + "\n")
	case ".dpl":
		_, err = file.WriteString(strconv.Itoa(processedCount) + "*file*" + relPath + "\n")
	default:
		_, err = file.WriteString(name + "\n")
	}
	if err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	return nil
}
