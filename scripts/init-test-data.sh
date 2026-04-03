#!/bin/bash

# 测试数据初始化脚本
# 用于快速创建测试所需的基础数据

set -e

BASE_URL="${BASE_URL:-http://localhost:8080}"
ADMIN_USER="${ADMIN_USER:-admin}"
ADMIN_PASS="${ADMIN_PASS:-123456}"

echo "=== mss-boot-admin 测试数据初始化 ==="
echo "服务地址: $BASE_URL"
echo ""

# 登录获取 token
echo "1. 登录获取 token..."
LOGIN_RESPONSE=$(curl -s -X POST "$BASE_URL/admin/api/user/login" \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"$ADMIN_USER\",\"password\":\"$ADMIN_PASS\",\"type\":\"account\"}")

TOKEN=$(echo "$LOGIN_RESPONSE" | jq -r '.token // empty')

if [ -z "$TOKEN" ]; then
  echo "❌ 登录失败"
  echo "$LOGIN_RESPONSE" | jq .
  exit 1
fi

echo "✅ 登录成功"
echo ""

# 创建测试部门
echo "2. 创建测试部门..."
DEPT_RESPONSE=$(curl -s -X POST "$BASE_URL/admin/api/departments" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"测试部门","status":"enabled","remark":"用于自动化测试"}')

echo "✅ 部门创建完成"
echo ""

# 创建测试岗位
echo "3. 创建测试岗位..."
POST_RESPONSE=$(curl -s -X POST "$BASE_URL/admin/api/posts" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"测试岗位","status":"enabled","remark":"用于自动化测试"}')

echo "✅ 岗位创建完成"
echo ""

# 创建测试角色
echo "4. 创建测试角色..."
ROLE_RESPONSE=$(curl -s -X POST "$BASE_URL/admin/api/roles" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"测试角色","keyword":"test_role","status":"enabled","remark":"用于自动化测试"}')

echo "✅ 角色创建完成"
echo ""

# 创建测试用户
echo "5. 创建测试用户..."
USER_RESPONSE=$(curl -s -X POST "$BASE_URL/admin/api/users" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","email":"test@example.com","password":"Test1234","status":"enabled","remark":"用于自动化测试"}')

echo "✅ 用户创建完成"
echo ""

# 验证数据
echo "6. 验证创建的数据..."
USER_COUNT=$(curl -s -H "Authorization: Bearer $TOKEN" "$BASE_URL/admin/api/users" | jq '.data | length')
ROLE_COUNT=$(curl -s -H "Authorization: Bearer $TOKEN" "$BASE_URL/admin/api/roles" | jq '.data | length')

echo "用户总数: $USER_COUNT"
echo "角色总数: $ROLE_COUNT"
echo ""

echo "=== 测试数据初始化完成 ==="
echo ""
echo "测试账号："
echo "  管理员: admin / 123456"
echo "  测试用户: testuser / Test1234"
echo ""
echo "下一步："
echo "  1. 启动前端: cd mss-boot-admin-antd && pnpm dev"
echo "  2. 访问: http://localhost:8000"
echo "  3. 使用测试账号登录验证"