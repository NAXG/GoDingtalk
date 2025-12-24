#!/bin/bash

# 项目名称
PROJECT_NAME="GoDingtalk"
VERSION="${1:-v1.0.0}"

# 创建构建目录
BUILD_DIR="build"
rm -rf ${BUILD_DIR}
mkdir -p ${BUILD_DIR}

echo "Building ${PROJECT_NAME} ${VERSION}..."
echo "=================================="

# 编译 Windows AMD64
echo "Building for Windows AMD64..."
GOOS=windows GOARCH=amd64 go build -ldflags "-w -s" -o ${BUILD_DIR}/${PROJECT_NAME}_${VERSION}_windows_amd64.exe

# 编译 macOS ARM64 (Apple Silicon)
echo "Building for macOS ARM64 (Apple Silicon)..."
GOOS=darwin GOARCH=arm64 go build -ldflags "-w -s" -o ${BUILD_DIR}/${PROJECT_NAME}_${VERSION}_darwin_arm64

# 编译 macOS AMD64 (Intel)
echo "Building for macOS AMD64 (Intel)..."
GOOS=darwin GOARCH=amd64 go build -ldflags "-w -s" -o ${BUILD_DIR}/${PROJECT_NAME}_${VERSION}_darwin_amd64

# 编译 Linux AMD64
echo "Building for Linux AMD64..."
GOOS=linux GOARCH=amd64 go build -ldflags "-w -s" -o ${BUILD_DIR}/${PROJECT_NAME}_${VERSION}_linux_amd64

# 编译 Linux ARM64
echo "Building for Linux ARM64..."
GOOS=linux GOARCH=arm64 go build -ldflags "-w -s" -o ${BUILD_DIR}/${PROJECT_NAME}_${VERSION}_linux_arm64

echo ""
echo "Build completed successfully!"
echo "=================================="
echo "Binaries are in the '${BUILD_DIR}' directory:"
ls -lh ${BUILD_DIR}/
