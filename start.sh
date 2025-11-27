#!/bin/bash

# 消息接收系统启动脚本

echo "正在启动消息接收系统..."

# 检查可执行文件是否存在
if [ ! -f "miemie" ]; then
    echo "错误：找不到可执行文件 'miemie'"
    echo "请先运行 'go build -o miemie main.go' 编译项目"
    exit 1
fi

# 创建数据目录
mkdir -p data

# 设置默认端口（如果未设置）
export PORT=${PORT:-8080}
export DATABASE_PATH=${DATABASE_PATH:-./data/messages.db}

echo "服务配置："
echo "  - 端口: $PORT"
echo "  - 数据库: $DATABASE_PATH"
echo ""
echo "服务启动后，您可以访问："
echo "  - 测试页面: http://localhost:$PORT/test.html"
echo "  - 健康检查: http://localhost:$PORT/health"
echo "  - API接口: http://localhost:$PORT/api/v3/"
echo ""

# 启动服务
./miemie