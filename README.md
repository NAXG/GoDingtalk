# 钉钉回放视频下载工具

这个工具旨在帮助用户方便地下载钉钉上的回放视频，便于离线观看和保存重要会议或课程内容。

## 特性

- 快速下载钉钉回放视频
- 多线程并发下载，提升下载速度
- 支持加密视频自动解密
- 自动合并视频片段并转换为 MP4 格式
- 用户友好的命令行界面和进度条显示
- 支持批量下载多个视频
- Cookie 自动管理，避免重复登录
- 支持配置文件自定义设置

## 前提条件

在使用本工具前，请确保你的系统中已安装以下软件：

- **Go** (1.20 或更高版本)
- **FFmpeg** (用于视频格式转换)
- **Google Chrome** (用于自动登录获取 cookies)

### 安装依赖

**macOS:**
```bash
brew install go ffmpeg
```

**Ubuntu/Debian:**
```bash
sudo apt-get update
sudo apt-get install golang-go ffmpeg
```

**Windows:**
- 从 [Go 官网](https://golang.org/dl/) 下载安装 Go
- 从 [FFmpeg 官网](https://ffmpeg.org/download.html) 下载安装 FFmpeg

## 安装

首先，克隆本项目到本地：

```bash
git clone https://github.com/NAXG/GoDingtalk.git
cd GoDingtalk
```

### 方式一：使用构建脚本

```bash
./build.sh
```

### 方式二：手动构建

```bash
go mod download
go build -o GoDingtalk .
```

## 使用方式

### 基本使用

1. **获取钉钉分享回放链接**
   - 在钉钉中找到要下载的回放
   - 点击分享，复制链接

2. **下载单个视频**

```bash
./GoDingtalk -url="https://n.dingtalk.com/dingding/live-room/index.html?roomId=XXXX&liveUuid=XXXX"
```

3. **批量下载**

创建一个文本文件（如 `urls.txt`），每行一个 URL：

```
https://n.dingtalk.com/dingding/live-room/index.html?roomId=XXXX&liveUuid=XXXX
https://n.dingtalk.com/dingding/live-room/index.html?roomId=YYYY&liveUuid=YYYY
# 以 # 开头的行会被忽略
```

然后运行：

```bash
./GoDingtalk -urlFile="urls.txt"
```

### 高级选项

```bash
# 指定下载线程数（默认 10）
./GoDingtalk -url="..." -thread=20

# 指定视频保存目录（默认 video/）
./GoDingtalk -url="..." -saveDir="downloads/"

# 组合使用
./GoDingtalk -urlFile="urls.txt" -thread=15 -saveDir="my_videos/"
```

### 配置文件

首次运行时，程序会使用默认配置。你可以创建 `config.json` 文件来自定义配置：

```json
{
  "thread_count": 10,
  "save_directory": "video/",
  "cookies_file": "cookies.json",
  "chrome_timeout": 20,
  "http_timeout": 30
}
```

配置说明：
- `thread_count`: 下载线程数（1-100）
- `save_directory`: 视频保存目录
- `cookies_file`: Cookies 文件路径
- `chrome_timeout`: Chrome 登录超时时间（分钟）
- `http_timeout`: HTTP 请求超时时间（秒）

## 工作流程

1. **首次运行**：程序会启动 Chrome 浏览器，要求用户登录钉钉
2. **Cookie 保存**：登录成功后，程序会保存 cookies 到本地文件
3. **后续使用**：程序会检查 cookies 是否有效，避免重复登录
4. **视频下载**：
   - 获取视频回放地址
   - 解析 M3U8 播放列表
   - 多线程下载视频片段
   - 自动解密（如果视频加密）
   - 合并片段为单个 TS 文件
   - 使用 FFmpeg 转换为 MP4 格式
   - 清理临时文件

## 优化改进

本项目相比原版进行了以下优化：

### 稳定性提升
- ✅ 完善的错误处理机制，所有错误都会被正确捕获和报告
- ✅ 并发安全保护，使用互斥锁防止数据竞争
- ✅ 安全的类型断言，避免 panic
- ✅ Cookie 有效性检查，避免重复登录
- ✅ 更新 chromedp 到最新版本，兼容最新 Chrome 浏览器
- ✅ 抑制 Chrome DevTools Protocol 非致命错误日志

### 性能优化
- ✅ HTTP 客户端连接池复用，提升网络性能
- ✅ 流式文件合并，减少内存占用
- ✅ 优化的并发控制

### 用户体验
- ✅ 清晰的错误信息提示
- ✅ 批量下载支持空行和注释
- ✅ 参数验证和友好的使用提示
- ✅ 配置文件支持

### 代码质量
- ✅ 专业的错误信息（移除不当用语）
- ✅ 更安全的文件权限设置（cookies 文件 0600）
- ✅ 代码注释和文档完善

## 常见问题

**Q: 下载失败怎么办？**
A: 检查网络连接，确认 URL 是否正确，查看错误提示信息。如果 cookies 过期，程序会自动提示重新登录。

**Q: 可以同时下载多个视频吗？**
A: 可以使用 `-urlFile` 参数批量下载。

**Q: 视频质量如何？**
A: 下载的是钉钉提供的回放原始质量。

**Q: 下载速度慢怎么办？**
A: 可以通过 `-thread` 参数增加并发线程数，但建议不超过 20。

**Q: 支持哪些操作系统？**
A: 支持 macOS、Linux 和 Windows。

## 注意事项

- 请确保有足够的磁盘空间存储视频文件
- 下载线程数不宜过高，建议 10-20 之间
- 首次使用需要登录钉钉账号
- 请勿用于商业用途或侵犯他人版权

## 免责声明

本工具仅供学习和研究目的，请勿用于非法用途。用户因使用本工具产生的一切后果，本项目开发者不承担任何责任。

下载的视频仅供个人学习使用，请勿传播或用于商业用途。

## 参考

- M3U8 下载器: https://gitee.com/edmund-shelby/m3u8-downloader

## 贡献

欢迎提交 Issue 和 Pull Request！

## 许可证

MIT License

