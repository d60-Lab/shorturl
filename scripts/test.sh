#!/bin/bash

# 测试脚本

API_URL="http://localhost:8080"

echo "=== Fuxi API 测试 ==="
echo ""

# 1. 健康检查
echo "1. 健康检查"
curl -s "$API_URL/health" | python3 -m json.tool
echo ""

# 2. 创建短URL
echo "2. 创建短URL"
SHORT_CODE=$(curl -s -X POST "$API_URL/api/shorten" \
  -H "Content-Type: application/json" \
  -d '{"long_url":"https://www.example.com/very/long/url/test"}' | \
  python3 -c "import sys, json; print(json.load(sys.stdin)['short_code'])")

echo "生成的短代码: $SHORT_CODE"
echo ""

# 3. 测试重定向
echo "3. 测试重定向"
curl -I "$API_URL/$SHORT_CODE" 2>&1 | head -n 10
echo ""

# 4. 获取统计信息
echo "4. 统计信息"
curl -s "$API_URL/api/stats" | python3 -m json.tool
echo ""

# 5. 创建多个短URL
echo "5. 批量创建测试"
for i in {1..5}; do
  curl -s -X POST "$API_URL/api/shorten" \
    -H "Content-Type: application/json" \
    -d "{\"long_url\":\"https://test.com/page$i\"}" | \
    python3 -c "import sys, json; r=json.load(sys.stdin); print(f\"  {i}. {r['short_code']} -> {r['long_url']}\")"
done
echo ""

# 6. 最终统计
echo "6. 最终统计"
curl -s "$API_URL/api/stats" | python3 -m json.tool
echo ""

echo "✓ 测试完成"
