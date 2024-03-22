#!/bin/bash

# 项目名称
PROJECT_NAME="GoDingtalk"



# 编译 Windows 版本
echo "Building for Windows..."
GOOS=windows GOARCH=amd64 go build -ldflags "-w -s" -o ${PROJECT_NAME}_windows_amd64.exe

# 编译 macOS 版本 (针对 ARM 架构)
echo "Building for macOS (ARM)..."
GOOS=darwin GOARCH=arm64 go build -ldflags "-w -s" -o ${PROJECT_NAME}_darwin_arm64

echo "Build completed."
