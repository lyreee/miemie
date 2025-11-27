#!/bin/bash

# 消息接收系统编译脚本

echo "正在编译消息接收系统..."

# 设置Go环境变量
export GOROOT=/usr/local/go
export GOMODCACHE=/tmp/go-mod-cache
export GOCACHE=/tmp/go-build-cache

# 清理旧的编译产物
if [ -f "miemie" ]; then
    echo "清理旧的可执行文件..."
    rm -f miemie
fi

# 下载依赖
echo "下载依赖包..."
go mod tidy

# 编译项目
echo "编译项目..."
go build -o miemie main.go

# 检查编译结果
if [ $? -eq 0 ]; then
    echo ""
    echo "✅ 编译成功！"
    echo ""
    echo "生成的可执行文件："
    ls -lh miemie
    echo ""
    echo "运行服务："
    echo "  ./start.sh"
    echo ""
    echo "或者直接运行："
    echo "  ./miemie"
    echo ""
else
    echo "❌ 编译失败！"
    exit 1
fi