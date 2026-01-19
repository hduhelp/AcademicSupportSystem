#!/bin/bash

# 脚本出错时立即退出
set -e

# 项目根目录
ROOT_DIR=$(dirname "$0")/..
cd "$ROOT_DIR"

# 检查后端可执行文件是否存在
if [ ! -f "build/server" ]; then
  echo "❌ Backend executable not found! Please run ./scripts/build.sh first."
  exit 1
fi

# 检查前端静态文件是否存在
if [ ! -d "build/static" ] || [ -z "$(ls -A build/static)" ]; then
  echo "❌ Frontend assets not found! Please run ./scripts/build.sh first."
  exit 1
fi

echo "🚀 Starting server in screen session 'academic-server'..."
# 启动后端服务，它将同时提供 API 和静态文件
screen -dmS academic-server ./build/server server

echo "✅ Server started!"
echo "   - 查看日志: screen -r academic-server"
echo "   - 退出查看: Ctrl+A 然后按 D"
echo "   - 停止服务: screen -S academic-server -X quit"
