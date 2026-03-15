package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"strconv"
	"time"

	"GoDingtalk/M3u8Downloader"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// Version 程序版本号，通过 -ldflags "-X main.Version=vX.X.X" 注入
var Version = "dev"

// 全局HTTP客户端，在 main 中根据配置初始化
var httpClient *http.Client


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

// ffmpeg 把ts转换mp4
func ffmpeg(ts, tempDir, saveDir string) error {
	fmt.Println("正在转换ts为mp4...")
	tsPath := filepath.Join(tempDir, ts+".ts")
	mp4Path := filepath.Join(saveDir, ts+".mp4")

	cmd := exec.Command("ffmpeg", "-i", tsPath, "-c:v", "copy", "-c:a", "copy", "-f", "mp4", "-y", mp4Path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("FFmpeg转换失败: %v\n输出: %s\n", err, string(output))
		return fmt.Errorf("ffmpeg conversion failed: %w", err)
	}
	fmt.Println(ts + ".mp4 转换完成")

	return nil
}

// generateUniqueTempDir 生成唯一的临时目录名称
func generateUniqueTempDir(saveDir string) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const randomLength = 6
	
	// 初始化随机数生成器
	rand.Seed(time.Now().UnixNano())
	
	// 最大尝试次数，避免无限循环
	maxAttempts := 10
	
	for attempt := 0; attempt < maxAttempts; attempt++ {
		// 生成随机字符串
		randomStr := make([]byte, randomLength)
		for i := range randomStr {
			randomStr[i] = charset[rand.Intn(len(charset))]
		}
		
		// 构建目录名称：.videoTemp + 日期 + _ + 随机字符串
		dateStr := time.Now().Format("20060102")
		tempDirName := fmt.Sprintf(".videoTemp%s_%s", dateStr, string(randomStr))
		tempDir := filepath.Join(saveDir, tempDirName)
		
		// 检查目录是否已存在
		if _, err := os.Stat(tempDir); os.IsNotExist(err) {
			// 目录不存在，创建它
			if err := os.MkdirAll(tempDir, 0755); err != nil {
				return "", fmt.Errorf("创建临时目录失败: %w", err)
			}
			return tempDir, nil
		}
		
		// 目录已存在，等待一小段时间后重试
		time.Sleep(100 * time.Millisecond)
	}
	
	return "", fmt.Errorf("无法生成唯一的临时目录名称，尝试次数超过 %d 次", maxAttempts)
}

// startChrome 函数启动Chrome浏览器，访问钉钉登录页面，获取并保存Cookies到本地文件。
func startChrome(config *Config) error {
	fmt.Println("正在启动Chrome获取Cookies...")

	// 抑制 chromedp 的日志输出
	log.SetOutput(io.Discard)
	defer log.SetOutput(os.Stderr)

	opts := append(
		// 跳过前3个选项以禁用 headless 模式，让浏览器可见
		chromedp.DefaultExecAllocatorOptions[3:],
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("excludeSwitches", "enable-automation"),
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
	)

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

	if err := os.WriteFile(config.CookiesFile, jsonCookies, 0600); err != nil {
		return fmt.Errorf("保存 Cookies 文件失败: %w", err)
	}

	fmt.Println("Cookies保存成功")
	return nil
}

// M3u8Down 函数用于下载直播回放视频
// title：直播标题
// playbackUrl：直播回放链接
// saveDir: 保存目录
// Thread：线程数
// tempDir: 临时目录（可选，如果为空则创建新的临时目录）
func M3u8Down(title, playbackUrl, saveDir string, Thread int, tempDir string) error {
	// 如果未提供临时目录，则创建新的临时文件夹
	var err error
	if tempDir == "" {
		tempDir, err = generateUniqueTempDir(saveDir)
		if err != nil {
			return fmt.Errorf("创建临时文件夹失败: %w", err)
		}
		fmt.Printf("临时文件夹创建成功: %s\n", tempDir)
	}

	m3u8 := M3u8Downloader.NewDownloader()
	m3u8.SetUrl(playbackUrl)
	m3u8.SetMovieName(title)
	m3u8.SetNumOfThread(Thread)
	m3u8.SetIfShowTheBar(true)
	m3u8.SetSaveDirectory(tempDir)

	if !m3u8.DefaultDownload() {
		// 下载失败时清理临时文件夹
		os.RemoveAll(tempDir)
		return fmt.Errorf("下载失败")
	}
	fmt.Println("下载成功")

	if err := ffmpeg(title, tempDir, saveDir); err != nil {
		// 转换失败时清理临时文件夹
		os.RemoveAll(tempDir)
		return fmt.Errorf("视频转换失败: %w", err)
	}
	
	// 转换成功后清理临时文件夹
	if err := os.RemoveAll(tempDir); err != nil {
		fmt.Printf("警告: 删除临时文件夹失败: %v\n", err)
	} else {
		fmt.Println("临时文件夹清理完成")
	}
	
	return nil
}

// getLiveRoomPublicInfo 函数用于获取钉钉直播间的公开信息
// roomId：直播间ID
// liveUuid：直播UUID
func getLiveRoomPublicInfo(roomId, liveUuid, saveDir string, Thread int, config *Config, tempDir string) (string, error) {
	// 构造URL
	urlStr := "https://lv.dingtalk.com/getOpenLiveInfo?roomId=" + roomId + "&liveUuid=" + liveUuid
	urlObj, err := url.Parse(urlStr)
	if err != nil {
		return "", fmt.Errorf("URL 解析失败: %w", err)
	}

	// 创建请求
	req, err := http.NewRequest("GET", urlObj.String(), nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	// 读取Cookies文件
	jsonCookies, err := os.ReadFile(config.CookiesFile)
	if err != nil {
		return "", fmt.Errorf("读取 Cookies 文件失败: %w", err)
	}

	var cookies map[string]string
	if err := json.Unmarshal(jsonCookies, &cookies); err != nil {
		return "", fmt.Errorf("解析 Cookies 失败: %w", err)
	}

	// 添加Cookies到请求
	var cookieStr strings.Builder
	for name, value := range cookies {
		cookieStr.WriteString(fmt.Sprintf("%s=%s; ", name, value))
	}
	// 确保 PC_SESSION 使用 LV_PC_SESSION 的值
	CookiepcSession, ok := cookies["LV_PC_SESSION"]
	if !ok {
		return "", fmt.Errorf("未找到 LV_PC_SESSION Cookie，请重新登录")
	}
	cookieStr.WriteString(fmt.Sprintf("PC_SESSION=%s", CookiepcSession))
	cookieHeader := cookieStr.String()

	// 设置请求头
	req.Header.Set("Host", "lv.dingtalk.com")
	req.Header.Set("Cookie", cookieHeader)
	req.Header.Set("Sec-Ch-Ua", `"Chromium";v="122", "Not(A:Brand";v="24", "Google Chrome";v="122"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", "macOS")
	req.Header.Set("Dnt", "1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")

	// 发送请求（使用全局 HTTP 客户端）
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应内容
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应内容失败: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析响应 JSON 失败: %w", err)
	}

	// 安全地获取嵌套字段
	openLiveDetailModel, ok := result["openLiveDetailModel"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("响应格式错误: 未找到 openLiveDetailModel 字段")
	}

	title, ok := openLiveDetailModel["title"].(string)
	if !ok {
		return "", fmt.Errorf("响应格式错误: 未找到 title 字段")
	}

	playbackUrl, ok := openLiveDetailModel["playbackUrl"].(string)
	if !ok {
		return "", fmt.Errorf("响应格式错误: 未找到 playbackUrl 字段")
	}

	fmt.Println("标题:", title)
	fmt.Println("回放地址:", playbackUrl)

	if err := M3u8Down(title, playbackUrl, saveDir, Thread, tempDir); err != nil {
		return title, err
	}
	
	return title, nil
}

// processURL 函数接收一个URL字符串作为参数，并解析出其中的roomId和liveUuid参数
// 然后调用getLiveRoomPublicInfo函数进行处理
// 如果URL解析出错或缺少roomId或liveUuid参数，则打印错误信息并返回
func processURL(urlStr, saveDir string, Thread int, config *Config, videoListFile string, tempDir, fileExt string, lineNum int) (string, error) {
	// 解析 URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", fmt.Errorf("解析 URL 时出错: %w", err)
	}

	// 提取查询参数中的 roomId 和 liveUuid
	queryParams := parsedURL.Query()
	roomId := queryParams.Get("roomId")
	liveUuid := queryParams.Get("liveUuid")
	if roomId == "" || liveUuid == "" {
		return "", fmt.Errorf("URL 中缺少 roomId 或 liveUuid 参数")
	}

	title, err := getLiveRoomPublicInfo(roomId, liveUuid, saveDir, Thread, config, tempDir)
	
	// 下载完成后立即追加标题到视频列表文件
	if err == nil && videoListFile != "" && title != "" {
		if appendErr := appendTitleToVideoListFile(videoListFile, title, fileExt, lineNum); appendErr != nil {
			fmt.Printf("警告: 追加标题到视频列表文件失败: %v\n", appendErr)
		} else {
			fmt.Printf("标题已添加到视频列表文件: %s\n", title)
		}
	}
	
	return title, err
}

// processURLFromFile 从文件中读取URL进行处理
func processURLFromFile(filePath, saveDir string, Thread int, config *Config, videoListFile, fileExt string) ([]string, error) {
	// 批量下载时只创建一个临时文件夹
	tempDir, err := generateUniqueTempDir(saveDir)
	if err != nil {
		return nil, fmt.Errorf("创建临时文件夹失败: %w", err)
	}
	fmt.Printf("批量下载临时文件夹创建成功: %s\n", tempDir)
	
	// 确保在函数结束时清理临时文件夹
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			fmt.Printf("警告: 删除临时文件夹失败: %v\n", err)
		} else {
			fmt.Println("批量下载临时文件夹清理完成")
		}
	}()
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("打开文件时出错: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	var errors []error
	var titles []string

	for scanner.Scan() {
		lineNum++
		urlStr := strings.TrimSpace(scanner.Text())
		if urlStr == "" || strings.HasPrefix(urlStr, "#") {
			continue // 跳过空行和注释
		}

		fmt.Printf("\n[%d] 处理 URL: %s\n", lineNum, urlStr)
		title, err := processURL(urlStr, saveDir, Thread, config, videoListFile, tempDir, fileExt, lineNum)
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
	if *cookiesFile != "" {
		config.CookiesFile = *cookiesFile
	}
	
	// 初始化全局 HTTP 客户端
	initHTTPClient(config.HTTPTimeout)

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
		_, err = processURL(*urlFlag, *saveDir, *Thread, config, *videoListFile, "", fileExt, 1)
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
	fileHeader := ""
	fmt.Printf("检测到%s文件，正在创建视频列表\n", fileExt)
	switch fileExt {
		case ".txt":
			fileHeader = ""
		case ".m3u":
			fileHeader = "#EXTM3U\n"
		case ".m3u8":
			fileHeader = "#EXTM3U8\n"
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
	defer file.Close()

	return nil
}

// appendTitleToVideoListFile 向视频列表文件追加标题
func appendTitleToVideoListFile(filePath, title, fileExt string, lineNum int) error {
	// 以追加模式打开文件
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	// 写入标题
	// 根据文件扩展名添加标题格式
	switch fileExt {
		case ".txt":
			_, err = file.WriteString(title + "\n")
		case ".m3u":
			_, err = file.WriteString("." + string(filepath.Separator) + title + ".mp4\n")
		case ".m3u8":
			_, err = file.WriteString("." + string(filepath.Separator) + title + ".mp4\n")
		case ".dpl":
			_, err = file.WriteString(strconv.Itoa(lineNum) + "*file*" + title + ".mp4\n")
		default:
			_, err = file.WriteString(title + "\n")
	}
	if err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	return nil
}
