#!/bin/bash
set -e

# 配置信息
ENDPOINTS="127.0.0.1:2377"

echo "=== 开始直接删除操作 ==="
echo "注意: 此脚本不会创建备份，直接执行删除命令!"

# 直接执行删除操作
echo "删除 runtimes..."
./etcdctl --endpoints="${ENDPOINTS}" del /registry/resources/runtimes --prefix

echo "删除 actions..."
./etcdctl --endpoints="${ENDPOINTS}" del /registry/resources/actions --prefix

echo "删除 tasks..."
./etcdctl --endpoints="${ENDPOINTS}" del /registry/resources/tasks --prefix

echo "删除 groups..."
./etcdctl --endpoints="${ENDPOINTS}" del /registry/resources/groups --prefix

echo "删除 Events..."
./etcdctl --endpoints="${ENDPOINTS}" del /registry/resources/Events --prefix

echo "=== 删除操作完成 ==="
~                            
