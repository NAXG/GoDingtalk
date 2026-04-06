# 钉钉回放视频下载工具

下载钉钉直播回放视频，支持多线程下载、加密视频解密、批量下载，自动转换为 MP4 格式。

## 前提条件

- **FFmpeg** (用于视频格式转换)
- **Google Chrome** (用于自动登录获取 Cookies)

## 安装

### 直接下载

从 [GitHub Releases](https://github.com/NAXG/GoDingtalk/releases) 下载对应平台的可执行文件。

### 从源码构建

```bash
git clone https://github.com/NAXG/GoDingtalk.git
cd GoDingtalk
go build -o GoDingtalk .
```

## 使用方式

### 下载单个视频

```bash
./GoDingtalk -url="https://n.dingtalk.com/dingding/live-room/index.html?roomId=XXXX&liveUuid=XXXX"
```

### 指定 Chrome 路径

如果系统安装了多个 Chrome 版本，或 Chrome 不在标准路径：

```bash
# macOS
./GoDingtalk -url="..." -chromePath="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"

# Windows
./GoDingtalk.exe -url="..." -chromePath="C:\Program Files\Google\Chrome\Application\chrome.exe"

# WSL (指定 Windows 版 Chrome)
./GoDingtalk -url="..." -chromePath="/mnt/c/Program Files/Google/Chrome/Application/chrome.exe"
```

### 批量下载

创建文本文件（如 `urls.txt`），每行一个 URL（`#` 开头的行会被忽略）：

```bash
./GoDingtalk -urlFile="urls.txt"
```

### 全部参数

```bash
./GoDingtalk -h
```

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `-url` | 回放 URL | |
| `-urlFile` | 包含回放 URL 的文件路径 | |
| `-thread` | 下载线程数 | 10 |
| `-saveDir` | 视频保存目录 | video/ |
| `-videoList` | 视频列表文件路径，记录已下载的视频标题 | |
| `-config` | 配置文件路径 | |
| `-login` | 强制重新登录获取 Cookies | |
| `-cookies` | Cookies 文件路径 | |
| `-httpTimeout` | HTTP 超时时间（秒） | 30 |
| `-chromeTimeout` | Chrome 登录超时时间（分钟） | 20 |
| `-chromePath` | Chrome/Chromium 可执行文件路径 | 自动查找 |
| `-version` | 显示版本号 | |

### 配置文件

首次运行时会在可执行文件同级目录的 `.goDingtalkConfig/` 下自动生成 `config.json`：

```json
{
  "thread_count": 10,
  "save_directory": "video/",
  "cookies_file": ".goDingtalkConfig/cookies.json",
  "chrome_path": "",
  "chrome_timeout": 20,
  "http_timeout": 30
}
```

命令行参数优先级高于配置文件。

## 免责声明

本工具仅供学习和研究目的。下载的视频仅供个人学习使用，请勿传播或用于商业用途。

## 参考

- M3U8 下载器: https://gitee.com/edmund-shelby/m3u8-downloader

## 许可证

MIT License
