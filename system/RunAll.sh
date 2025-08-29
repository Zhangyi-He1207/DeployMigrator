#!/bin/bash
BIN_DIR="./bin"
SCHEDULER_PATH="$BIN_DIR/scheduler"
NODELET_PATH="$BIN_DIR/nodelet"
# 删除上一次运行留下来的日志文件
rm -f nohup.out

# 函数用于检查进程是否存在
check_process_running() {
    local process_name="$1"
    if pgrep -f "$process_name" > /dev/null; then
        return 0
    else
        return 1
    fi
}

# 创建tmux会话并运行etcd
tmux new-session -d -s etcd "cd ./bin/etcd-v3.5.17-linux-amd64 && ./etcd"
echo "etcd服务已启动 [会话: etcd]"

# 创建tmux会话并运行apiserver
tmux new-session -d -s apiserver "cd ./bin && ./apiserver --etcd-servers=127.0.0.1:2379"
echo "API服务器已启动 [会话: apiserver]"

# 创建tmux会话并运行registry
tmux new-session -d -s registry "cd ./bin && ./registry"
echo "注册服务已启动 [会话: registry]"


# 等待服务初始化
echo "等待服务初始化(5秒)..."
sleep 5

# 提交数据并运行测试
# 判断 scheduler 文件是否存在
if [ -f "$SCHEDULER_PATH" ]; then
  if check_process_running "$SCHEDULER_PATH"; then
      echo "The scheduler is already running. Restarting scheduler ..."
      killall scheduler
      sleep 1
  fi
  # 使用 nohup 将 scheduler 放到后台运行，并将输出重定向到 scheduler_log.log 文件
  nohup "$SCHEDULER_PATH" --framework-conf=./frameworkConf.yaml > scheduler_log.log 2>&1 & 
  echo "Started scheduler and redirected output to scheduler_log.log."
else
    echo "The scheduler file at $SCHEDULER_PATH does not exist."
    exit 1
fi

# 休眠 2 秒
sleep 2

# 判断 nodelet 文件是否存在
if [ -f "$NODELET_PATH" ]; then
  if check_process_running "$NODELET_PATH"; then
      echo "The nodelet is already running. Restarting nodelet ..."
      killall nodelet
      sleep 1
  fi
  # 将 nodelet 放到前台运行
  echo "Starting nodelet in the foreground ..."
  nohup "$NODELET_PATH" --framework-conf=./frameworkConf.yaml &
else
    echo "The nodelet file at $NODELET_PATH does not exist."
    exit 1
fi

echo "数据提交和测试脚本已执行"

# 显示会话状态
echo ""
echo "所有服务已启动！您可以使用以下命令连接tmux会话："
echo "-----------------------------------------------"
echo "tmux attach -t etcd        # 连接etcd服务"
echo "tmux attach -t apiserver   # 连接API服务器"
echo "tmux attach -t registry    # 连接文件仓库服务"
echo "-----------------------------------------------"
echo "要查看所有会话列表，请执行: tmu