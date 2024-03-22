package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
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
func ffmpeg(ts string) {
	fmt.Println("正在转换ts为mp4...")
	cmd := exec.Command("ffmpeg", "-i", "video/"+ts+".ts", "-c:v", "copy", "-c:a", "copy", "-f", "mp4", "-y", ts+".mp4")
	err := cmd.Run()
	if err != nil {
		return
	}
	fmt.Println(ts + ".mp4 转换完成")
	fmt.Println("正在删除ts文件...")
	os.Remove("video/" + ts + ".ts")
	fmt.Println("删除完成")
}

// startChrome 函数启动Chrome浏览器，访问钉钉登录页面，获取并保存Cookies到本地文件。
func startChrome() {
	fmt.Println("正在启动Chrome获取Cookies...")
	opts := append(
		// select all the elements after the third element
		chromedp.DefaultExecAllocatorOptions[3:],
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
	chromedp.Run(ctx,
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
					break
				}
				time.Sleep(2 * time.Second)
			}
			return nil
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			// 到达此处，说明已经跳转到了指定的URL
			siteCookies, _ = network.GetCookies().Do(ctx)
			for _, cookie := range siteCookies {
				fmt.Printf("Cookie: %s=%s\n", cookie.Name, cookie.Value)
			}
			return nil
		}),
	)

	// 保存cookies到文件
	cookies := make(map[string]string)
	for _, cookie := range siteCookies {
		cookies[cookie.Name] = cookie.Value
	}
	jsonCookies, _ := json.Marshal(cookies)
	os.WriteFile("cookies.json", jsonCookies, 0644)
}

// M3u8Down 函数用于下载直播回放视频
// title：直播标题
// playbackUrl：直播回放链接
func M3u8Down(title, playbackUrl string, Thread int) {
	m3u8 := M3u8Downloader.NewDownloader()
	m3u8.SetUrl(playbackUrl)
	m3u8.SetMovieName(title)
	m3u8.SetNumOfThread(Thread)
	m3u8.SetIfShowTheBar(true)
	if m3u8.DefaultDownload() {
		fmt.Println("下载成功")
		ffmpeg(title)
	}
}

// getLiveRoomPublicInfo 函数用于获取钉钉直播间的公开信息
// roomId：直播间ID
// liveUuid：直播UUID
func getLiveRoomPublicInfo(roomId, liveUuid string, Thread int) {
	// 构造URL
	urlStr := "https://lv.dingtalk.com/getOpenLiveInfo?roomId=" + roomId + "&liveUuid=" + liveUuid
	urlObj, _ := url.Parse(urlStr)

	// 创建请求
	req, _ := http.NewRequest("GET", urlObj.String(), nil)

	// 读取Cookies.json文件
	jsonCookies, _ := os.ReadFile("cookies.json")
	var cookies map[string]string
	_ = json.Unmarshal(jsonCookies, &cookies)

	// 添加Cookies到请求
	var cookieStr strings.Builder
	for name, value := range cookies {
		cookieStr.WriteString(fmt.Sprintf("%s=%s; ", name, value))
	}
	cookieHeader := cookieStr.String()
	CookiepcSession := cookies["LV_PC_SESSION"]
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
	resp, _ := client.Do(req)

	// 关闭响应
	defer resp.Body.Close()

	// 读取响应内容
	body, _ := io.ReadAll(resp.Body)

	var result map[string]interface{}
	json.Unmarshal(body, &result)

	title := result["openLiveDetailModel"].(map[string]interface{})["title"].(string)
	playbackUrl := result["openLiveDetailModel"].(map[string]interface{})["playbackUrl"].(string)

	fmt.Println("Title:", title)
	fmt.Println("PlaybackUrl:", playbackUrl)
	M3u8Down(title, playbackUrl, Thread)
}

// processURL 函数接收一个URL字符串作为参数，并解析出其中的roomId和liveUuid参数
// 然后调用getLiveRoomPublicInfo函数进行处理
// 如果URL解析出错或缺少roomId或liveUuid参数，则打印错误信息并返回
func processURL(urlStr string, Thread int) {
	// 解析 URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		fmt.Println("解析 URL 时出错:", err)
		return
	}
	// 提取查询参数中的 roomId 和 liveUuid
	queryParams := parsedURL.Query()
	roomId := queryParams.Get("roomId")
	liveUuid := queryParams.Get("liveUuid")
	if roomId == "" || liveUuid == "" {
		fmt.Println("URL 中缺少 roomId 或 liveUuid 参数，退出...")
		return
	}

	// 调用函数
	// 假设你有一个处理这些信息的函数
	getLiveRoomPublicInfo(roomId, liveUuid, Thread)
}

// processURLFromFile 从文件中读取URL进行处理
// 参数：
//
//	filePath：需要读取的URL文件路径
//
// 返回值：无
func processURLFromFile(filePath string, Thread int) {
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Println("打开文件时出错:", err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		urlStr := scanner.Text()   // 假设每行包含一个URL
		processURL(urlStr, Thread) // 调用之前定义的 processURL 函数处理每个URL
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("读取文件时出错:", err)
	}
}

// main 函数是程序的入口点
func main() {
	fmt.Println("  _______   _______   _______   _______   _______   _______ ")
	fmt.Println(" |       | |       | |   _   | |   _   | |   _   | |   _   |")
	fmt.Println(" |   Go  | |  Ding | |  | |  | |  | |  | |  | |  | |  | |  |")
	fmt.Println(" |       | |  talk | |  |_|  | |  |_|  | |  |_|  | |  |_|  |")
	fmt.Println(" |_______| |_______| |_______| |_______| |_______| |_______|")

	// 示例：使用示例的roomId和liveUuid
	urlFlag := flag.String("url", "", "需要下载的回放URL，格式为 -url \"https://n.dingtalk.com/dingding/live-room/index.html?roomId=XXXX&liveUuid=XXXX\"")
	urlFile := flag.String("urlFile", "", "包含需要下载的回放URL的文件路径，格式为 -urlFile \"/path/to/file\"")
	Thread := flag.Int("thread", 10, "下载线程数")

	// 解析命令行参数
	flag.Parse()

	if *urlFlag != "" {
		startChrome()
		processURL(*urlFlag, *Thread)
	} else if *urlFile != "" {
		startChrome() // 如果这个调用对于处理文件路径不是必需的，可以移动到processURLFromFile函数内部
		processURLFromFile(*urlFile, *Thread)
	} else {
		fmt.Println("未提供 URL 或 URL 文件路径，退出...")
		return
	}
}
