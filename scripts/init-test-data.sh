#!/bin/bash

# 测试数据初始化脚本
# 用于快速创建测试所需的基础数据

set -e

BASE_URL="${BASE_URL:-http://localhost:8080}"
BROWSER_ORIGIN="${BROWSER_ORIGIN:-http://localhost:8001}"
ADMIN_USER="${ADMIN_USER:-admin}"
ADMIN_PASS="${ADMIN_PASS:-123456}"
COOKIE_JAR=$(mktemp)
trap 'rm -f "$COOKIE_JAR"' EXIT

echo "=== mss-boot-admin 测试数据初始化 ==="
echo "服务地址: $BASE_URL"
echo ""

# 建立 V6 HttpOnly 会话；脚本后续与浏览器使用同一 cookie + CSRF 合同。
echo "1. 建立 V6 浏览器会话..."
LOGIN_RESPONSE=$(curl -sS -c "$COOKIE_JAR" -X POST "$BASE_URL/admin/api/user/session/login" \
	-H "Origin: $BROWSER_ORIGIN" \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"$ADMIN_USER\",\"password\":\"$ADMIN_PASS\",\"type\":\"account\"}")

LOGIN_CODE=$(echo "$LOGIN_RESPONSE" | jq -r '.code // empty')
CSRF_TOKEN=$(awk '$6 == "mss_csrf" { print $7 }' "$COOKIE_JAR" | tail -n 1)

if [ "$LOGIN_CODE" != "200" ] || [ -z "$CSRF_TOKEN" ]; then
  echo "❌ 登录失败"
  echo "$LOGIN_RESPONSE" | jq .
  exit 1
fi

echo "✅ 登录成功"
echo ""

# 创建测试部门
echo "2. 创建测试部门..."
DEPT_RESPONSE=$(curl -s -X POST "$BASE_URL/admin/api/departments" \
	-b "$COOKIE_JAR" -H "Origin: $BROWSER_ORIGIN" -H "X-CSRF-Token: $CSRF_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"测试部门","status":"enabled","remark":"用于自动化测试"}')

echo "✅ 部门创建完成"
echo ""

# 创建测试岗位
echo "3. 创建测试岗位..."
POST_RESPONSE=$(curl -s -X POST "$BASE_URL/admin/api/posts" \
	-b "$COOKIE_JAR" -H "Origin: $BROWSER_ORIGIN" -H "X-CSRF-Token: $CSRF_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"测试岗位","status":"enabled","remark":"用于自动化测试"}')

echo "✅ 岗位创建完成"
echo ""

# 创建测试角色
echo "4. 创建测试角色..."
ROLE_RESPONSE=$(curl -s -X POST "$BASE_URL/admin/api/roles" \
	-b "$COOKIE_JAR" -H "Origin: $BROWSER_ORIGIN" -H "X-CSRF-Token: $CSRF_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"测试角色","keyword":"test_role","status":"enabled","remark":"用于自动化测试"}')

echo "✅ 角色创建完成"
echo ""

# 创建测试用户
echo "5. 创建测试用户..."
USER_RESPONSE=$(curl -s -X POST "$BASE_URL/admin/api/users" \
	-b "$COOKIE_JAR" -H "Origin: $BROWSER_ORIGIN" -H "X-CSRF-Token: $CSRF_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","email":"test@example.com","password":"Test1234","status":"enabled","remark":"用于自动化测试"}')

echo "✅ 用户创建完成"
echo ""

# 验证数据
echo "6. 验证创建的数据..."
USER_COUNT=$(curl -sS -b "$COOKIE_JAR" "$BASE_URL/admin/api/users" | jq '.data | length')
ROLE_COUNT=$(curl -sS -b "$COOKIE_JAR" "$BASE_URL/admin/api/roles" | jq '.data | length')

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
echo "  1. 启动默认前端: cd web/antd-v6 && corepack pnpm@10.34.5 start:dev"
echo "  2. 访问: http://localhost:8001"
echo "  3. 使用测试账号登录验证"
