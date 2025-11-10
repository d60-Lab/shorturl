#!/bin/bash

# Fuxi 短URL系统 - 完整演示

set -e

echo "========================================"
echo "  Fuxi 短URL系统 - 完整演示"
echo "  基于李智慧《高并发架构实战课》"
echo "========================================"
echo ""

# 清理旧数据
echo "📦 步骤1: 清理旧数据..."
rm -rf data/*.dat data/*.db
mkdir -p data
echo "✓ 清理完成"
echo ""

# 生成短URL
echo "🔧 步骤2: 生成10万条短URL..."
go run cmd/generator/main.go -count 100000
echo ""

# 检查文件
echo "📊 步骤3: 检查生成的文件..."
ls -lh data/
echo ""

# 查看前10个短URL
echo "🔍 步骤4: 查看示例短URL..."
head -c 60 data/shorturls.dat | fold -w 6
echo ""
echo ""

# 启动API服务器（后台）
echo "🚀 步骤5: 启动API服务器..."
go run cmd/api/main.go > /tmp/fuxi.log 2>&1 &
API_PID=$!
echo "✓ 服务器已启动 (PID: $API_PID)"

# 等待服务器启动
echo "⏳ 等待服务器就绪..."
sleep 3
echo ""

# 测试API
echo "🧪 步骤6: 测试API接口..."
echo ""

echo "6.1 健康检查"
curl -s http://localhost:8080/health | python3 -m json.tool
echo ""

echo "6.2 创建短URL #1"
RESPONSE1=$(curl -s -X POST http://localhost:8080/api/shorten \
  -H "Content-Type: application/json" \
  -d '{"long_url":"https://github.com/golang/go"}')
CODE1=$(echo $RESPONSE1 | python3 -c "import sys, json; print(json.load(sys.stdin)['short_code'])")
echo $RESPONSE1 | python3 -m json.tool
echo ""

echo "6.3 创建短URL #2"
RESPONSE2=$(curl -s -X POST http://localhost:8080/api/shorten \
  -H "Content-Type: application/json" \
  -d '{"long_url":"https://www.example.com/very/long/url/for/testing"}')
CODE2=$(echo $RESPONSE2 | python3 -c "import sys, json; print(json.load(sys.stdin)['short_code'])")
echo $RESPONSE2 | python3 -m json.tool
echo ""

echo "6.4 测试重定向 (短URL: $CODE1)"
curl -I http://localhost:8080/$CODE1 2>&1 | grep -E "HTTP|Location"
echo ""

echo "6.5 再次访问相同短URL（测试缓存）"
curl -I http://localhost:8080/$CODE1 2>&1 | grep -E "HTTP|Location"
echo ""

echo "6.6 查看统计信息"
curl -s http://localhost:8080/api/stats | python3 -m json.tool
echo ""

# 批量创建测试
echo "📈 步骤7: 批量创建测试（50个）..."
START_TIME=$(date +%s)
for i in {1..50}; do
  curl -s -X POST http://localhost:8080/api/shorten \
    -H "Content-Type: application/json" \
    -d "{\"long_url\":\"https://test.com/page$i\"}" > /dev/null
done
END_TIME=$(date +%s)
ELAPSED=$((END_TIME - START_TIME))
echo "✓ 完成50个请求，耗时: ${ELAPSED}秒"
echo ""

# 最终统计
echo "📊 步骤8: 最终统计..."
curl -s http://localhost:8080/api/stats | python3 -m json.tool
echo ""

# 清理
echo "🧹 步骤9: 停止服务器..."
kill $API_PID 2>/dev/null || true
echo "✓ 服务器已停止"
echo ""

echo "========================================"
echo "  ✅ 演示完成！"
echo "========================================"
echo ""
echo "项目文件："
echo "  - README.md        项目说明"
echo "  - TECH_BLOG.md     技术博客"
echo "  - QUICKSTART.md    快速开始"
echo "  - PROJECT_SUMMARY.md 项目总结"
echo ""
echo "常用命令："
echo "  make generate      生成短URL数据"
echo "  make run           启动API服务"
echo "  make test          运行测试"
echo "  make stress        压力测试"
echo "  make help          查看帮助"
echo ""
