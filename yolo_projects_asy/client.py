import grpc
import json
import logging
import argparse
from runtime_interface import runtimeIntent_pb2, runtimeIntent_pb2_grpc

# 日志配置
LOG_FORMAT = "%(asctime)s - %(levelname)s - %(message)s"
logging.basicConfig(level=logging.INFO, format=LOG_FORMAT)
logger = logging.getLogger("gRPC Client")


class InferenceClient:
    def __init__(self, host, port):
        self.channel = grpc.insecure_channel(f"{host}:{port}")
        self.stub = runtimeIntent_pb2_grpc.RuntimeIntentStub(self.channel)
        logger.info(f"连接到服务端 {host}:{port}")

    def call_init(self):
        """调用init方法"""
        logger.info("调用 init 方法...")
        try:
            # 使用 InitIntent 而不是 Empty
            request = runtimeIntent_pb2.InitIntent(app="yolo-task")
            response = self.stub.init(request)
            logger.info(f"init 调用完成: {response.msg}, stateCode={response.stateCode}")
            return response
        except grpc.RpcError as e:
            logger.error(f"调用 init 方法失败: {e.details()}")
            return None

    def call_start(self):
        """调用start方法"""
        logger.info("调用 start 方法...")
        try:
            # 使用 StartIntent 而不是 Empty
            request = runtimeIntent_pb2.StartIntent(app="yolo-task")
            response = self.stub.start(request)
            logger.info(f"start 调用完成: {response.msg}, stateCode={response.stateCode}")
            return response
        except grpc.RpcError as e:
            logger.error(f"调用 start 方法失败: {e.details()}")
            return None

    def call_store(self):
        """调用store方法获取状态"""
        logger.info("调用 store 方法...")
        try:
            # 使用 StoreIntent 而不是 Empty
            request = runtimeIntent_pb2.StoreIntent(app="yolo-task")
            response = self.stub.store(request)

            if response.stateCode == 1 and response.data:
                # 解析返回的状态数据
                try:
                    state = json.loads(response.data)
                    logger.info(f"store 调用完成: {response.data}, state={state}")
                    return state
                except json.JSONDecodeError:
                    logger.info(f"store 调用完成，但返回消息不是JSON: {response.data}")
                    return response.data
            else:
                logger.warning(f"store 调用返回无状态数据: {response.data}")
                return None
        except grpc.RpcError as e:
            logger.error(f"调用 store 方法失败: {e.details()}")
            return None

    def call_restore(self, state_data):
        """调用restore方法恢复状态"""
        logger.info(f"调用 restore 方法, state_data={state_data}")
        try:
            # 准备要恢复的状态数据
            if isinstance(state_data, dict):
                state_str = json.dumps(state_data)
            else:
                state_str = str(state_data)

            # 创建 Data 对象
            data = runtimeIntent_pb2.Data()
            data.name = "restoreData" 
            data.type = runtimeIntent_pb2.DataType.DATA
            data.data = state_str

            # 创建 RestoreIntent 请求
            request = runtimeIntent_pb2.RestoreIntent(
                app="yolo-task",
                data=[data]
            )

            response = self.stub.restore(request)
            logger.info(f"restore 调用完成: {response.msg}, stateCode={response.stateCode}")
            return response
        except grpc.RpcError as e:
            logger.error(f"调用 restore 方法失败: {e.details()}")
            return None

    def call_stop(self):
        """调用stop方法"""
        logger.info("调用 stop 方法...")
        try:
            # 使用 StopIntent 而不是 Empty
            request = runtimeIntent_pb2.StopIntent(app="yolo-task")
            response = self.stub.stop(request)
            logger.info(f"stop 调用完成: {response.msg}, stateCode={response.stateCode}")
            return response
        except grpc.RpcError as e:
            logger.error(f"调用 stop 方法失败: {e.details()}")
            return None


def main():
    # 解析命令行参数
    parser = argparse.ArgumentParser(description='gRPC Inference Client')
    parser.add_argument('--host', type=str, default='localhost', help='gRPC服务主机')
    parser.add_argument('--port', type=int, default=5123, help='gRPC服务端口')
    parser.add_argument('--command', type=str, required=True,
                        choices=['init', 'start', 'store', 'restore', 'stop'],
                        help='要执行的操作')
    parser.add_argument('--state-data', type=str, help='恢复状态的JSON数据')
    args = parser.parse_args()

    # 创建客户端
    client = InferenceClient(args.host, args.port)

    # 执行请求的命令
    if args.command == 'init':
        client.call_init()

    elif args.command == 'start':
        client.call_start()

    elif args.command == 'store':
        state = client.call_store()
        if state:
            logger.info(f"当前状态: {json.dumps(state, indent=2)}")

    elif args.command == 'restore':
        if args.state_data:
            try:
                # 解析状态数据
                state_data = json.loads(args.state_data)
                client.call_restore(state_data)
            except json.JSONDecodeError:
                logger.error("无法解析JSON状态数据")
        else:
            logger.error("restore操作需要 --state-data 参数")

    elif args.command == 'stop':
        client.call_stop()


if __name__ == '__main__':
    main()
