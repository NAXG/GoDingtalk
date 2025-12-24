# GoDingtalk v2.1.0 发布说明

## 🎉 代码质量优化版本

这是 GoDingtalk 的代码质量优化版本，主要聚焦于代码安全性、测试覆盖和配置管理的改进。

## ✨ 新增功能

- **配置文件集成**: `config.json` 现在完全集成到主程序，启动时自动加载
- **单元测试覆盖**: 新增 21 个单元测试，覆盖核心功能模块
  - M3u8Downloader 工具函数测试（11个）
  - 下载器配置测试（6个）
  - 配置管理测试（4个）

## 🔧 代码质量提升

### 安全性改进
- **移除 unsafe.Pointer**: 完全移除所有不安全的类型转换，改用标准库函数
  - `tool.go`: 使用 `fmt.Sprintf` 替代 unsafe 转换
  - `downloader.go`: 使用 `string([]byte)` 标准转换
  - `bar.go`: 使用安全的字符串转换
- **类型安全**: 所有类型转换都使用 Go 推荐的安全方式

### Bug 修复
- **修复 SetMovieName 逻辑错误**:
  - 修复前：条件判断后依然执行添加后缀的代码
  - 修复后：正确使用 if-else 分支
- **配置文件功能**: 修复配置文件定义但未使用的问题
  - 现在程序启动时会自动加载 `config.json`
  - 命令行参数优先级高于配置文件
  - 不存在配置文件时使用默认值

### 代码优化
- **简化时间戳生成**: `getUnixTimeAndToByte()` 使用标准库 `fmt.Sprintf` 替代手动转换
- **提高可维护性**: 代码更符合 Go 语言最佳实践
- **改进错误处理**: 统一错误处理模式

## 📦 测试覆盖

```bash
# 测试结果
✓ 21/21 tests passed
✓ 覆盖率包括：
  - URL 处理和解析
  - 文件操作（创建、合并、删除）
  - 下载器配置管理
  - 配置文件读写
  - Cookie 验证
```

## 🔄 向后兼容

- 完全兼容 v2.0.0 的所有功能
- API 接口保持不变
- 配置文件格式兼容

## 📋 变更详情

### 修改的文件
- `go.mod`: Go 版本声明（与系统版本匹配）
- `M3u8Downloader/downloader.go`: 修复 SetMovieName 逻辑，移除 unsafe
- `M3u8Downloader/tool.go`: 简化代码，移除 unsafe
- `M3u8Downloader/bar.go`: 安全的字符串转换
- `main.go`: 集成配置文件加载功能

### 新增的文件
- `M3u8Downloader/tool_test.go`: 工具函数测试
- `M3u8Downloader/downloader_test.go`: 下载器测试
- `main_test.go`: 主程序测试

## 📥 下载和安装

### 下载对应平台的二进制文件：

- **Windows**: `GoDingtalk_v2.1.0_windows_amd64.exe`
- **macOS (Apple Silicon)**: `GoDingtalk_v2.1.0_darwin_arm64`
- **macOS (Intel)**: `GoDingtalk_v2.1.0_darwin_amd64`
- **Linux (AMD64)**: `GoDingtalk_v2.1.0_linux_amd64`
- **Linux (ARM64)**: `GoDingtalk_v2.1.0_linux_arm64`

### 安装步骤：

1. 下载对应平台的二进制文件
2. macOS/Linux 用户需要添加执行权限：
   ```bash
   chmod +x GoDingtalk_v2.1.0_*
   ```
3. 运行程序：
   ```bash
   ./GoDingtalk_v2.1.0_darwin_arm64 -url="your_url_here"
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

# 使用配置文件（可选）
cp config.example.json config.json
# 编辑 config.json 设置默认参数
./GoDingtalk -url="..."

# 指定线程数和保存目录
./GoDingtalk -url="..." -thread=20 -saveDir="downloads/"
```

## 🆕 配置文件使用

创建 `config.json` 文件可以设置默认参数：

```json
{
  "thread_count": 10,
  "save_directory": "video/",
  "cookies_file": "cookies.json",
  "chrome_timeout": 20,
  "http_timeout": 30
}
```

命令行参数会覆盖配置文件中的设置。

## 📖 完整文档

详细使用文档请查看 [README.md](https://github.com/NAXG/GoDingtalk/blob/master/README.md)

## 🙏 致谢

感谢所有使用和反馈的用户！

## 📄 许可证

MIT License

---

**从 v2.0.0 升级**: 直接替换二进制文件即可，无需其他配置更改。

**完整变更日志**: 查看项目 commit 历史
