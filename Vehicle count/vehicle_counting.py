import cv2
from ultralytics import YOLO, solutions
import os
import grpc
import threading
from concurrent import futures
import json
import logging
import numpy as np
from runtime_interface import runtimeIntent_pb2, runtimeIntent_pb2_grpc
import matplotlib

matplotlib.use('Agg')
os.environ["QT_QPA_PLATFORM"] = "offscreen"
os.environ["DISPLAY"] = ""

# 强制使用CPU
os.environ['CUDA_VISIBLE_DEVICES'] = '-1'
os.environ['KMP_DUPLICATE_LIB_OK'] = 'True'
os.environ['OMP_NUM_THREADS'] = '1'
os.environ['MKL_NUM_THREADS'] = '1'

# 全局状态变量
life_state = -1  # 程序生命周期状态: -1=停止, 0=暂停, 1=运行
frame_index = 0  # 当前处理的帧号
total_in = 0  # 累计进入车辆数
total_out = 0  # 累计离开车辆数
restore_data = None  # 恢复时需要的数据
model = None
capture = None
video_writer = None
w, h, fps = 0, 0, 0
VIDEO_PATH = os.path.join(os.path.dirname(os.path.realpath(__file__)), "video", "test_traffic.mp4")


# gRPC服务实现
class RuntimeService(runtimeIntent_pb2_grpc.RuntimeIntentServicer):
    def init(self, request, context):
        logging.info(f"{'-' * 10} {self.__class__.__name__} --init()--start {'-' * 10}")
        #
        logging.info(f"{'-' * 10} {self.__class__.__name__} --init()--end {'-' * 10}")
        return runtimeIntent_pb2.Result(msg='init finished', stateCode=1)

    def start(self, request, context):
        logging.info(f"{'-' * 10} {self.__class__.__name__} --start()--start {'-' * 10}")
        global life_state
        life_state = 0
        logging.info(f"{'-' * 10} {self.__class__.__name__} --start()--end {'-' * 10}")
        return runtimeIntent_pb2.Result(msg='start finished', stateCode=1)

    def store(self, request, context):
        logging.info(f"{'-' * 10} {self.__class__.__name__} --store()--start {'-' * 10}")
        global frame_index, total_in, total_out

        state_data = {
            "frame_index": frame_index,
            "total_in": total_in,
            "total_out": total_out
        }

        logging.info(f'Stored state: {state_data}')
        logging.info(f"{'-' * 10} {self.__class__.__name__} --store()--end {'-' * 10}")
        return runtimeIntent_pb2.Result(
            msg='store finished',
            stateCode=1,
            data=json.dumps(state_data)
        )

    def restore(self, request, context):
        logging.info(f"{'-' * 10} {self.__class__.__name__} --restore()--start {'-' * 10}")
        global life_state, restore_data

        try:
            if not request.data:
                raise ValueError("No data provided in restore request")

            data_value = request.data[0].data
            logging.debug(f"Received restore data: {data_value}")

            state_data = json.loads(data_value)
            restore_data = {
                "frame_index": int(state_data["frame_index"]),
                "total_in": int(state_data["total_in"]),
                "total_out": int(state_data["total_out"])
            }
            # 定位到恢复帧
            capture.set(cv2.CAP_PROP_POS_FRAMES, frame_index)
            logging.info(f'恢复处理完成: {restore_data}')
            life_state = 0
            return runtimeIntent_pb2.Result(msg='restore finished', stateCode=1)

        except Exception as e:
            logging.error(f'Restore error: {str(e)}')
            return runtimeIntent_pb2.Result(msg=f'restore failed: {str(e)}', stateCode=0)

        finally:
            logging.info(f"{'-' * 10} {self.__class__.__name__} --restore()--end {'-' * 10}")



def serve(port=5123):
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10), options=[
        ('grpc.max_send_message_length', 100 * 1024 * 1024),
        ('grpc.max_receive_message_length', 100 * 1024 * 1024)])
    runtimeIntent_pb2_grpc.add_RuntimeIntentServicer_to_server(RuntimeService(), server)
    server.add_insecure_port(f'localhost:{port}')
    server.start()
    server.wait_for_termination()


def gRPC_server_start():
    port = os.environ.get("PORT_FOR_RPC", "5123")
    logging.info(f"RPC Port: {port}")
    if port:
        port = int(port)
        grpc_thread = threading.Thread(target=serve, daemon=True, args=(port,))
        grpc_thread.start()


def preload_resources():
    global model, w, h, fps, video_writer

    # 预加载模型
    try:
        model_path = os.path.join(os.path.dirname(os.path.realpath(__file__)), 'model/yolov8n.pt')
        model = YOLO(model_path)
        logging.info("模型加载成功")
    except Exception as e:
        logging.error(f"模型加载失败: {str(e)}")
        return False

    # 预加载视频元数据
    temp_capture = cv2.VideoCapture(VIDEO_PATH)
    if not temp_capture.isOpened():
        logging.error(f"无法打开视频文件: {VIDEO_PATH}")
        return False

    w = int(temp_capture.get(cv2.CAP_PROP_FRAME_WIDTH))
    h = int(temp_capture.get(cv2.CAP_PROP_FRAME_HEIGHT))
    fps = int(temp_capture.get(cv2.CAP_PROP_FPS))

    # 模型预热
    try:
        dummy_img = np.random.randint(0, 255, (640, 640, 3), dtype=np.uint8)
        model.predict(dummy_img, verbose=False)
        logging.info("模型预热完成")
    except Exception as e:
        logging.error(f"模型预热失败: {str(e)}")
        logging.warning("跳过模型预热")

    temp_capture.release()

    # 预创建视频写入器
    video_writer = cv2.VideoWriter("result3.mp4", cv2.VideoWriter_fourcc(*"mp4v"), fps, (w, h))

    logging.info("资源预加载完成")
    return True


def initialize_counter():
    # 定义计数线
    line_points = [(0, int(h / 2)), (int(w), int(h / 2))]

    # 初始化对象计数器
    return solutions.ObjectCounter(
        view_img=False,
        reg_pts=line_points,
        names=model.names,
        line_thickness=2,
    )


def process_frame(frame, counter):
    global frame_index, total_in, total_out

    # 更新帧计数器
    frame_index += 1

    # 执行目标跟踪
    tracks = model.track(frame, persist=True, show=False)

    # 更新计数并获取结果图像
    result_frame = counter.start_counting(frame, tracks)

    # 更新全局计数
    total_in = counter.in_counts
    total_out = counter.out_counts

    # 获取当前帧的计数
    current_in = 0
    if hasattr(tracks[0], 'boxes') and hasattr(tracks[0].boxes, 'id'):
        current_ids = tracks[0].boxes.id.int().cpu().tolist()
        current_in = len(current_ids)

    # 打印当前帧信息
    logging.info(
        f"帧号: {frame_index}, 当前帧检测车辆: {current_in}, 累计进入: {total_in}, 累计离开: {total_out}")

    return result_frame


def main_processing_loop():
    global frame_index, total_in, total_out, restore_data
    logging.info("资源初始化完成，等待启动命令...")

    # 等待start或restore被调用
    while life_state < 0:
        continue
    counter = initialize_counter()

    # 处理恢复数据（如果有）
    if restore_data:
        frame_index = restore_data["frame_index"]
        total_in = restore_data["total_in"]
        total_out = restore_data["total_out"]
        counter.in_counts = total_in
        counter.out_counts = total_out

        # # 定位到恢复帧
        logging.info(f"1已恢复到帧 {frame_index}, 累计进入: {total_in}, 累计离开: {total_out}")
        # capture.set(cv2.CAP_PROP_POS_FRAMES, frame_index)


    # 主处理循环
    while life_state >= 0 and capture.isOpened():
        try:
            # 读取视频帧
            success, frame = capture.read()
            if not success:
                logging.info("视频读取完成")
                break

            # 处理当前帧
            result_frame = process_frame(frame, counter)

            # 写入视频帧
            video_writer.write(result_frame)

        except Exception as e:
            logging.error(f"处理帧 {frame_index} 时出错: {str(e)}")
            continue

    # 释放资源
    capture.release()
    video_writer.release()
    cv2.destroyAllWindows()


if __name__ == '__main__':
    LOG_FORMAT = "%(asctime)s - %(message)s"
    logging.basicConfig(level=logging.INFO, format=LOG_FORMAT)

    # 预加载资源
    if not preload_resources():
        logging.error("资源预加载失败，程序退出")
        exit(1)

    # 开启gRPC服务端
    gRPC_server_start()

    # 初始化视频资源
    logging.info('初始化视频资源...')
    capture = cv2.VideoCapture(VIDEO_PATH)
    if not capture.isOpened():
        logging.error(f"无法打开视频文件: {VIDEO_PATH}")
        exit(1)

    # 启动主处理循环
    main_processing_loop()