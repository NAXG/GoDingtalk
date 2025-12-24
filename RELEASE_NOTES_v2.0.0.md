# GoDingtalk v2.0.0 发布说明

## 🎉 重大版本更新

这是 GoDingtalk 的重大优化版本，在稳定性、性能和用户体验方面进行了全面改进。

## ✨ 新增功能

- **配置文件支持**: 新增 `config.json` 配置文件支持，可自定义下载参数
- **智能 Cookie 管理**: 自动检查 Cookie 有效性，避免重复登录
- **自定义保存目录**: 新增 `-saveDir` 参数，可指定视频保存位置
- **批量下载增强**: 支持 URL 文件中的空行和注释（# 开头）
- **友好的用户提示**: 更详细的错误信息和进度提示

## 🔧 稳定性提升

- **完善错误处理**: 所有函数都正确返回和处理错误，不再忽略异常
- **并发安全**: 使用 `sync.Mutex` 保护共享变量，修复数据竞争问题
- **类型安全**: 所有类型断言都进行了安全检查，避免 panic
- **Chrome 兼容性**: 更新 chromedp 到 v0.14.2，兼容最新 Chrome 浏览器
- **日志优化**: 抑制 Chrome DevTools Protocol 非致命错误日志

## ⚡ 性能优化

- **HTTP 连接池复用**: 全局 HTTP 客户端复用，显著提升网络性能
- **流式文件处理**: 使用 `io.Copy` 进行流式文件合并，大幅减少内存占用
- **并发控制优化**: 改进下载线程管理机制

## 💄 用户体验改进

- **专业错误信息**: 移除所有不当用语，使用专业的错误描述
- **参数验证**: 线程数范围检查（1-100），防止无效输入
- **批量下载统计**: 显示处理进度和失败任务统计
- **登录提示优化**: 清晰的登录状态提示

## 🔒 安全性增强

- **Cookie 文件权限**: 从 0644 改为 0600，提升安全性
- **输入验证**: 对所有用户输入进行验证和清理

## 📝 代码质量

- **完善文档**: 重写 README，添加详细使用说明和常见问题
- **变更日志**: 新增 CHANGELOG.md 记录所有改进
- **Git 配置**: 添加 .gitignore 文件，规范仓库管理

## 🐛 修复问题

- 修复并发访问 `errCount` 导致的数据竞争
- 修复 FFmpeg 路径硬编码问题
- 修复类型断言可能导致的 panic
- 修复批量下载时重复登录问题
- 修复文件合并时内存占用过高
- 修复 Chrome 浏览器不显示的问题

## 📦 支持平台

- Windows (AMD64)
- macOS (Intel 和 Apple Silicon)
- Linux (AMD64 和 ARM64)

## 📥 下载和安装

### 下载对应平台的二进制文件：

- **Windows**: `GoDingtalk_v2.0.0_windows_amd64.exe`
- **macOS (Apple Silicon)**: `GoDingtalk_v2.0.0_darwin_arm64`
- **macOS (Intel)**: `GoDingtalk_v2.0.0_darwin_amd64`
- **Linux (AMD64)**: `GoDingtalk_v2.0.0_linux_amd64`
- **Linux (ARM64)**: `GoDingtalk_v2.0.0_linux_arm64`

### 安装步骤：

1. 下载对应平台的二进制文件
2. 解压（如果有压缩）
3. macOS/Linux 用户需要添加执行权限：
   ```bash
   chmod +x GoDingtalk_v2.0.0_*
   ```
4. 运行程序：
   ```bash
   ./GoDingtalk_v2.0.0_darwin_arm64 -url="your_url_here"
   ```

## 📋 前提条件

使用前请确保已安装：
- **FFmpeg**: 用于视频格式转换
- **Google Chrome**: 用于自动登录获取 cookies

## 🚀 快速开始

```bash
# 下载单个视频
./GoDingtalk -url="https://n.dingtalk.com/dingding/live-room/index.html?roomId=XXXX&liveUuid=XXXX"

# 批量下载
./GoDingtalk -urlFile="urls.txt"

# 指定线程数和保存目录
./GoDingtalk -url="..." -thread=20 -saveDir="downloads/"
```

## 📖 完整文档

详细使用文档请查看 [README.md](https://github.com/NAXG/GoDingtalk/blob/master/README.md)

## 🙏 致谢

感谢所有使用和反馈的用户！

## 📄 许可证

MIT License

---

**完整变更日志**: [CHANGELOG.md](https://github.com/NAXG/GoDingtalk/blob/master/CHANGELOG.md)
