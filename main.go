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
	"strings"
	"time"

	"GoDingtalk/M3u8Downloader"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// ffmpeg 把ts转换mp4
func ffmpeg(ts, saveDir string) error {
	fmt.Println("正在转换ts为mp4...")
	tsPath := saveDir + ts + ".ts"
	mp4Path := ts + ".mp4"

	cmd := exec.Command("ffmpeg", "-i", tsPath, "-c:v", "copy", "-c:a", "copy", "-f", "mp4", "-y", mp4Path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("FFmpeg转换失败: %v\n输出: %s\n", err, string(output))
		return fmt.Errorf("ffmpeg conversion failed: %w", err)
	}
	fmt.Println(ts + ".mp4 转换完成")

	fmt.Println("正在删除ts文件...")
	if err := os.Remove(tsPath); err != nil {
		fmt.Printf("警告: 删除ts文件失败: %v\n", err)
		// 不返回错误，因为mp4已经生成成功
	} else {
		fmt.Println("删除完成")
	}
	return nil
}

// startChrome 函数启动Chrome浏览器，访问钉钉登录页面，获取并保存Cookies到本地文件。
func startChrome() error {
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

	// 设置超时时间，确保扫码后能够及时跳转
	ctx, cancel = context.WithTimeout(ctx, 20*time.Minute)
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
				return fmt.Errorf("failed to get cookies: %w", err)
			}
			return nil
		}),
	)

	if err != nil {
		return fmt.Errorf("chrome automation failed: %w", err)
	}

	// 保存cookies到文件
	cookies := make(map[string]string)
	for _, cookie := range siteCookies {
		cookies[cookie.Name] = cookie.Value
	}
	jsonCookies, err := json.Marshal(cookies)
	if err != nil {
		return fmt.Errorf("failed to marshal cookies: %w", err)
	}

	if err := os.WriteFile("cookies.json", jsonCookies, 0600); err != nil {
		return fmt.Errorf("failed to save cookies: %w", err)
	}

	fmt.Println("Cookies保存成功")
	return nil
}

// M3u8Down 函数用于下载直播回放视频
// title：直播标题
// playbackUrl：直播回放链接
// saveDir: 保存目录
func M3u8Down(title, playbackUrl, saveDir string, Thread int) error {
	m3u8 := M3u8Downloader.NewDownloader()
	m3u8.SetUrl(playbackUrl)
	m3u8.SetMovieName(title)
	m3u8.SetNumOfThread(Thread)
	m3u8.SetIfShowTheBar(true)
	m3u8.SetSaveDirectory(saveDir)

	if !m3u8.DefaultDownload() {
		return fmt.Errorf("下载失败")
	}
	fmt.Println("下载成功")

	if err := ffmpeg(title, saveDir); err != nil {
		return fmt.Errorf("视频转换失败: %w", err)
	}
	return nil
}

// getLiveRoomPublicInfo 函数用于获取钉钉直播间的公开信息
// roomId：直播间ID
// liveUuid：直播UUID
func getLiveRoomPublicInfo(roomId, liveUuid, saveDir string, Thread int) error {
	// 构造URL
	urlStr := "https://lv.dingtalk.com/getOpenLiveInfo?roomId=" + roomId + "&liveUuid=" + liveUuid
	urlObj, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("failed to parse URL: %w", err)
	}

	// 创建请求
	req, err := http.NewRequest("GET", urlObj.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// 读取Cookies.json文件
	jsonCookies, err := os.ReadFile("cookies.json")
	if err != nil {
		return fmt.Errorf("failed to read cookies file: %w", err)
	}

	var cookies map[string]string
	if err := json.Unmarshal(jsonCookies, &cookies); err != nil {
		return fmt.Errorf("failed to unmarshal cookies: %w", err)
	}

	// 添加Cookies到请求
	var cookieStr strings.Builder
	for name, value := range cookies {
		cookieStr.WriteString(fmt.Sprintf("%s=%s; ", name, value))
	}
	cookieHeader := cookieStr.String()
	CookiepcSession, ok := cookies["LV_PC_SESSION"]
	if !ok {
		return fmt.Errorf("LV_PC_SESSION cookie not found")
	}

	// 设置请求头
	req.Header.Set("Host", "lv.dingtalk.com")
	req.Header.Set("Cookie", cookieHeader)
	req.Header.Set("Cookie", "PC_SESSION="+CookiepcSession)
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

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应内容
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// 安全地获取嵌套字段
	openLiveDetailModel, ok := result["openLiveDetailModel"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid response format: openLiveDetailModel not found")
	}

	title, ok := openLiveDetailModel["title"].(string)
	if !ok {
		return fmt.Errorf("invalid response format: title not found")
	}

	playbackUrl, ok := openLiveDetailModel["playbackUrl"].(string)
	if !ok {
		return fmt.Errorf("invalid response format: playbackUrl not found")
	}

	fmt.Println("Title:", title)
	fmt.Println("PlaybackUrl:", playbackUrl)

	return M3u8Down(title, playbackUrl, saveDir, Thread)
}

// processURL 函数接收一个URL字符串作为参数，并解析出其中的roomId和liveUuid参数
// 然后调用getLiveRoomPublicInfo函数进行处理
// 如果URL解析出错或缺少roomId或liveUuid参数，则打印错误信息并返回
func processURL(urlStr, saveDir string, Thread int) error {
	// 解析 URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("解析 URL 时出错: %w", err)
	}

	// 提取查询参数中的 roomId 和 liveUuid
	queryParams := parsedURL.Query()
	roomId := queryParams.Get("roomId")
	liveUuid := queryParams.Get("liveUuid")
	if roomId == "" || liveUuid == "" {
		return fmt.Errorf("URL 中缺少 roomId 或 liveUuid 参数")
	}

	return getLiveRoomPublicInfo(roomId, liveUuid, saveDir, Thread)
}

// processURLFromFile 从文件中读取URL进行处理
func processURLFromFile(filePath, saveDir string, Thread int) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("打开文件时出错: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	var errors []error

	for scanner.Scan() {
		lineNum++
		urlStr := strings.TrimSpace(scanner.Text())
		if urlStr == "" || strings.HasPrefix(urlStr, "#") {
			continue // 跳过空行和注释
		}

		fmt.Printf("\n[%d] 处理 URL: %s\n", lineNum, urlStr)
		if err := processURL(urlStr, saveDir, Thread); err != nil {
			errMsg := fmt.Errorf("第 %d 行处理失败: %w", lineNum, err)
			fmt.Println(errMsg)
			errors = append(errors, errMsg)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("读取文件时出错: %w", err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("批量处理完成，%d 个 URL 处理失败", len(errors))
	}

	return nil
}

// checkCookiesValid 检查cookies文件是否存在且有效
func checkCookiesValid() bool {
	// 检查文件是否存在
	if _, err := os.Stat("cookies.json"); os.IsNotExist(err) {
		return false
	}

	// 尝试读取和解析
	jsonCookies, err := os.ReadFile("cookies.json")
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

	// 加载配置文件
	config, err := LoadConfig("config.json")
	if err != nil {
		fmt.Printf("警告: 加载配置文件失败: %v，使用默认配置\n", err)
		config = DefaultConfig()
	}

	// 命令行参数
	urlFlag := flag.String("url", "", "需要下载的回放URL，格式为 -url \"https://n.dingtalk.com/dingding/live-room/index.html?roomId=XXXX&liveUuid=XXXX\"")
	urlFile := flag.String("urlFile", "", "包含需要下载的回放URL的文件路径，格式为 -urlFile \"/path/to/file\"")
	Thread := flag.Int("thread", config.ThreadCount, "下载线程数")
	saveDir := flag.String("saveDir", config.SaveDirectory, "视频保存目录")

	flag.Parse()

	// 参数验证
	if *urlFlag == "" && *urlFile == "" {
		fmt.Println("错误: 未提供 URL 或 URL 文件路径")
		flag.Usage()
		os.Exit(1)
	}

	if *Thread <= 0 || *Thread > 100 {
		fmt.Println("错误: 线程数必须在 1-100 之间")
		os.Exit(1)
	}

	// 确保保存目录以斜杠结尾
	if !strings.HasSuffix(*saveDir, "/") {
		*saveDir += "/"
	}

	// 检查cookies是否有效，无效则重新登录
	if !checkCookiesValid() {
		fmt.Println("Cookies无效或不存在，需要重新登录...")
		if err := startChrome(); err != nil {
			fmt.Printf("错误: 获取Cookies失败: %v\n", err)
			os.Exit(1)
		}
	} else {
		fmt.Println("使用现有Cookies...")
	}

	// 处理URL
	if *urlFlag != "" {
		err = processURL(*urlFlag, *saveDir, *Thread)
	} else if *urlFile != "" {
		err = processURLFromFile(*urlFile, *saveDir, *Thread)
	}

	if err != nil {
		fmt.Printf("\n错误: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n所有任务完成！")
}
