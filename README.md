# 钉钉回放视频下载工具

这个工具旨在帮助用户方便地下载钉钉上的回放视频，便于离线观看和保存重要会议或课程内容。

## 特性

- 快速下载钉钉回放视频
- 用户友好的命令行界面
- 支持批量下载

## 前提条件

在使用本工具前，请确保你的系统中已安装以下软件：

- Go
- ffmpeg
- Google Chrome

## 安装

首先，克隆本项目到本地：

```bash
git clone https://github.com/NAXG/GoDingtalk.git
cd GoDingtalk
./build.sh
```

## 使用方式

获取钉钉分享回放链接-复制链接-打开命令行窗口-执行以下命令：
![0.jpg](img%2F0.jpg)
```bash
./GoDingtalk_darwin_arm64 -url="https://ndingtalk.com/xxx"
```
![1.png](img%2F1.png)

## 免责声明

本工具仅供学习和研究目的，请勿用于非法用途。用户因使用本工具产生的一切后果，本项目开发者不承担任何责任。

## 参考

https://gitee.com/edmund-shelby/m3u8-downloader

本工具代码都是baidu comate生成的，本人稍加润色。

https://comate.baidu.com/?inviteCode=6gfx8i2y

![shareImg.jpeg](img%2FshareImg.jpeg)
