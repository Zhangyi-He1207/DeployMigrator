package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	ty "hit.edu/framework/pkg/apimachinery/types"
	metav1 "hit.edu/framework/pkg/apis/meta"
	"hit.edu/framework/pkg/nodelet/task/controller"

	"os"
	"time"

	"hit.edu/framework/pkg/apimachinery/watch"
	apis "hit.edu/framework/pkg/apis/cores"
	meta "hit.edu/framework/pkg/apis/meta"
	"hit.edu/framework/pkg/client-go/clients/typed/core"
	"hit.edu/framework/pkg/client-go/util/manager"
	"hit.edu/framework/pkg/component-base/logs"
	"hit.edu/framework/pkg/nodelet/events"
	fileManager "hit.edu/framework/pkg/nodelet/registry"
	"hit.edu/framework/pkg/nodelet/task/group"
	"hit.edu/framework/pkg/nodelet/task/types"
	cross_core "hit.edu/framework/test/etcd_sync/active/clients/typed/core"
)

type GroupHandler struct {
	// group managers
	groupManager group.Manager

	// group workers, 实际部署任务
	groupWorkers group.GroupWorkers

	groupQueues *group.GroupQueues
	// client -go
	//groupClient   core.GroupInterface
	//actionClient  core.ActionInterface
	//runtimeClient core.RuntimeInterface
	clientsManager *manager.Manager
	//
	// eventRecorder 记录事件
	//recorder    recorder.EventRecorder
	eventClient core.EventInterface
	fileManager *fileManager.FileManager

	stopCh chan struct{}
	// 跨域
	groupTarget   map[string]cross_core.GroupInterface
	actionTarget  map[string]cross_core.ActionInterface
	runtimeTarget map[string]cross_core.RuntimeInterface
}

func NewGroupHandler(groupManager group.Manager, groupWorkers group.GroupWorkers, groupQueues *group.GroupQueues, clientsManager *manager.Manager,
	groupTarget map[string]cross_core.GroupInterface, actionTarget map[string]cross_core.ActionInterface,
	runtimeTarget map[string]cross_core.RuntimeInterface, fileManager *fileManager.FileManager) *GroupHandler {

	return &GroupHandler{
		groupManager: groupManager,
		groupWorkers: groupWorkers,
		groupQueues:  groupQueues,
		//groupClient:   groupClient,
		//actionClient:  actionClient,
		//runtimeClient: runtimeClient,
		clientsManager: clientsManager,
		//recorder:       recorder,
		//eventClient:   eventClient,
		fileManager:   fileManager,
		stopCh:        make(chan struct{}),
		groupTarget:   groupTarget,
		actionTarget:  actionTarget,
		runtimeTarget: runtimeTarget,
	}
}

type Handler interface {
	HandleGroupAdds(groups []*apis.Group)
	HandleGroupUpdates(groups []*apis.Group)
	HandleGroupDeletes(groups []*apis.Group)
}

// 开启监听上层指令
func (gh *GroupHandler) Loop(ctx context.Context, updateCh <-chan types.GroupUpdate) {
	const base = 100 * time.Millisecond //

	logs.Info("GroupHandler component start")
	//duration := base
	for {
		// 檢查Context是否已取消
		select {
		case <-ctx.Done():
			logs.Info("Context canceled, exiting GroupHandler loop")
			return
		case <-time.After(base):
			gh.LoopIteration(ctx, updateCh) //是否采用协程，取决于该函数是否要与Loop方法并行执行
		}
		//time.Sleep(duration)
		//// TODO: 二进制指数退避
	}
}

// 针对不同的指令，对接受到的任务执行不同的操作（开启任务、更新任务、删除任务）
func (gh *GroupHandler) LoopIteration(ctx context.Context, updateCh <-chan types.GroupUpdate) {
	// TODO: Watch Group信息
	for {
		select {
		case <-ctx.Done():
			logs.Error("Context canceled, exiting loop iteration")
			return
		case u, ok := <-updateCh:
			if !ok {
				logs.Error("Update channel is closed, can‘t get group")
				return
			}
			switch u.Op {
			case types.ADD:
				logs.Debug("Add new group")
				gh.HandleGroupAdd(u.Group)
			case types.KILL:
				logs.Debug("Delete group")
				gh.HandleGroupKill(u.Group)
			case types.Stop:
				logs.Debug("Stop group")
				gh.HandleGroupStop(u.Group)
			case types.Restore:
				logs.Debug("Restore group")
				gh.HandleGroupRestore(u.Group)
			case types.UPDATE:
				logs.Debug("Update group")
				gh.HandleGroupUpdate(u.Group)
			default:
				logs.Error("Unhandled default case")
			}
		}
	}
}

// 处理Group启动指令  主要内容：检查当前节点是否能执行group，以及检查当前节点是否接收过当前group（得依据groupID）
func (gh *GroupHandler) HandleGroupAdd(gr *apis.Group) {
	logs.Infof("Start HandleGroupAdd,group:%v", gr.Name)
	// TODO: 对Pod按照优先级排序（目前先按照创建时间） ---目前是处理发过来的单个Group，所以无法做排序工作
	start := time.Now()
	// TODO: 检查任务是否可以在当前节点上运行, 如果不能，则拒绝Group的部署
	//  不能部署的情况包括
	//  1、Group已经在本地部署（是否包括副本）？
	//  2、没有可以执行的资源
	//  ......
	// TODO 这里有一个问题：就是如果节点上部署不了这个group，那么久会导致一直Handler一直接收这个group，解决方法：检查后立即放入
	//_, err := gh.groupManager.GetGroupByID(gr.Status.GroupID) // 使用groupID查，因为groupID是唯一分配的;也可以使用GroupName来查，因为GroupName也是唯一分配的
	_, err := gh.groupManager.GetGroupByName(gr.Name)
	if err == nil { //err等于nil说明在group_manager当中能找到group信息
		// 1、说明group已经存
		logs.Infof("Group:%s is already put into Deployer", gr.Spec.Name)
		return
	}
	//如果说部署器本地没有改groupID信息的话，就存入group信息
	gh.groupManager.AddGroup(gr)

	// 2、检查资源是否足够并满足部署条件
	if isCanDeploy := gh.checkResource(gr); !isCanDeploy {
		// 如果无法部署，拒绝改Group的部署并通知调度器
		logs.Errorf("Group:%s cannot be deployed,err:%v", gr.Name, err)
		// TODO 这里得直接提交给调度器，告知group无法部署
		return
	}

	// TODO：3、fileDownload协程，只执行一次，如果没有才下载。目录依据group.Spec.Name来创建，不带后缀
	// 4-30 runtime第一次被准备启动，在检查依赖等之前，创建runtime专属的文件目录，并且下载其Data[]里面填入的所有文件===暂时只考虑到单个文件
	// 后续还需要考虑到这些目录的删除。例如在运行完成之后，生成的结果要么直接上传到etcd，要么直接上传到文件仓库。在这些操作完成之后，考虑删除这些已完成的任务的文件夹
	// 1、尝试创建group专属的目录，需要先检查前置的目录apis.FileFolder = "../tmp/data"目录是否存在，然后再开始创建group专属目录
	homeDir, err := os.UserHomeDir()
	if err != nil {
		logs.Errorf("获取用户主目录失败: %v", err)
	}
	fileFolder := filepath.Join(homeDir, "tmp", "data")
	groupdir := fileFolder + "/" + gr.Name
	//groupdir := apis.FileFolder + "/" + gr.Name
	logs.Infof("File Path:%v", groupdir)
	if _, err := os.Stat(groupdir); os.IsNotExist(err) {
		// 目录不存在，创建目录
		logs.Infof("handler create directory :%v", groupdir)
		err := os.Mkdir(groupdir, os.ModePerm) // 权限
		if err != nil {
			logs.Errorf("monitor创建目录时发生错误: %v\n", err)
		}
		logs.Tracef("monitor创建目录完成,runtime:%v, namaspace:%v", gr.Name, gr.Namespace)
	} else if err != nil {
		// 其他错误
		logs.Errorf("检查目录时发生未知错误: %v\n", err)
		return
	}
	// TODO检查完目录之后，准备使用fileManager下载文件，并更新文件下载状态。使用协程下载
	for _, actionReference := range gr.Status.Actions {
		action, err := gh.clientsManager.GetAction(actionReference.Name, actionReference.Namespace)
		if err != nil {
			logs.Errorf("Etcd get action error-4:%v", err)
		}
		for _, runtimeReference := range action.Status.Runtimes {
			runtime, err := gh.clientsManager.GetRuntime(runtimeReference.Name, runtimeReference.Namespace)
			patchRuntime, err := json.Marshal(map[string]interface{}{
				"status": map[string]interface{}{
					"directory": groupdir, // 设置工作目录
				},
			})
			_, err = gh.clientsManager.PatchRuntime(runtime.Name, runtime.Namespace, patchRuntime)
			if err != nil {
				logs.Errorf("Patch runtime error-8:%v", err)
			}

			if err != nil {
				logs.Errorf("Get runtime error-3:%v", err)
			}
			logs.Tracef("runtime Data[]:%v", runtime.Spec.Data)
			for _, filedata := range runtime.Spec.Data { // 这里需要考虑到runtime的Data[]里面填入的所有文件
				// 检查DownloadStatus[]是否存在
				// FIXME: 5-15测试发现有bug，fileManager针对的是文件名，那么多个runtime使用到同名文件的时候会出错，导致文件不再被下载---已解决，增加runtime的信息保证不重复
				// 修改建议：1、当一个文件下载完成之后，立刻删除filaManager里面的记录，保证能再次下载（这里会不会有同步的问题？感觉会有）
				// 2、fileManager对文件下载的记录增加针对runtime的记录，保证每个文件都与runtime联系，这样就不会导致不同的runtime下载直接相互冲突了
				fileKey := runtime.Name + "-" + filedata.Name
				if _, ok := gh.fileManager.DownloadStatus[fileKey]; !ok { // 说明没有下载过
					gh.fileManager.DownloadStatus[fileKey] = fileManager.NotDownloaded
					logs.Tracef("file not downloaded filedata.Name:%v,filedata.Path:%v", fileKey, groupdir)
				}
				if gh.fileManager.DownloadStatus[fileKey] == fileManager.Downloaded { // 说明已经下载过了
					logs.Tracef("file already downloaded filedata.Name:%v,filedata.Path:%v", fileKey, groupdir)
					continue
				} else if gh.fileManager.DownloadStatus[fileKey] == fileManager.Downloading { // 说明正在下载
					logs.Tracef("file is downloading filedata.Name:%v,filedata.Path:%v", fileKey, groupdir)
					continue
				}
				if gh.fileManager.DownloadStatus[fileKey] != fileManager.Downloading && gh.fileManager.DownloadStatus[fileKey] != fileManager.Downloaded { // 说明没有下载过
					gh.fileManager.DownloadStatus[fileKey] = fileManager.Downloading
					logs.Infof("Now start downloading filedata.Name:%v,filedata.Path:%v", filedata.Name, groupdir)
					if strings.Contains(filedata.Name, ".") { //暂时考虑这个简单的办法，因为文件仓库里的文件不一定在本机上，所以不清楚这个文件是文件还是文件夹
						// 进行文件的下载
						go gh.fileManager.DownloadFile(filedata.Name, groupdir)
					} else { // 进行文件夹的下载
						// go gh.fileManager.DownloadFolder(filedata.Name, groupdir)
						// 阻塞下载
						logs.Infof("Download Folder:%v, groupName:%v, actionName:%v, runtimeName:%v", filedata.Name, gr.Name, action.Name, runtime.Name)
						gh.fileManager.DownloadFolder(filedata.Name, groupdir)
					}
				}
			}
		}

	}

	// 使用协程检查是否有副本并部署
	go func() {
		// 4、判断group是否需要部署副本，如果需要，在此处往域或者跨域的etcd当中添加副本
		var copiesInDomain, copiesInOtherDomain int32 = 0, 0
		// 安全处理逻辑
		// 情况1：用户未传参时 Replicas == nil
		if gr.Spec.Replicas == nil {
			logs.Info("Replicas未配置，使用默认值[0,0]")
		} else if len(gr.Spec.Replicas) < 2 {
			logs.Warn("Replicas长度不足，使用前N个值并用0补全",
				"输入值", gr.Spec.Replicas,
				"有效长度", len(gr.Spec.Replicas))
			// 安全取值（避免越界）
			if len(gr.Spec.Replicas) >= 1 {
				copiesInDomain = gr.Spec.Replicas[0]
			}
			// 第二个值保持默认0
		} else { // 情况3：正常情况
			copiesInDomain = gr.Spec.Replicas[0]
			copiesInOtherDomain = gr.Spec.Replicas[1]
		}
		if gr.Spec.HasReplca == true {
			copiesInDomain = 1
			logs.Info("Replicas未配置，使用默认值[1,0]")
		}

		if copiesInDomain > 0 { //如果传进任务的时候该属性没有赋值的话，初始化是为0的
			// 为了适配迁移 ,如果有多个副本要求的话，需要部署多个副本
			for i := 0; i < int(copiesInDomain); i++ {
				// 复制创建一个全新的副本group信息（注意Succeed的Phase不用修改，DeployCheck和Running状态需要修改），另外还需要将副本的groupStatus改为Starting
				//groupCopyName := "Reason-Copy"                                               // TODO 这里之后改成随机生成即可源group.Name + 一串随机字符
				groupCopy := controller.NewGroupInfoCopy(gr, true, "") // 第二个参数为true，表示的是提前写入etcd
				logs.Infof("Create groupCopy:%v", groupCopy.Name)
				// 新增操作--5.20--将Group写入到Task当中
				gh.AddGroupCopyToTaskStatus(groupCopy)
				// 遍历action和Runtime，依次创建
				for _, actionReference := range gr.Status.Actions {
					action, err := gh.clientsManager.GetAction(actionReference.Name, actionReference.Namespace)
					if err != nil {
						logs.Errorf("Get action %s failed: %v", actionReference.Name, err)
					}
					actionCopy := controller.NewActionInfoCopy(action)
					actionClient := gh.clientsManager.GetActionClient(actionCopy.Namespace)
					_, err = actionClient.Client.Create(context.TODO(), actionCopy, metav1.CreateOptions{}) // 因为是创建同一个域内的Action副本，所以说副本的namespace和源任务相同，直接用源action的namespace
					if err != nil {
						logs.Errorf("Create copy action %s in local failed: %v", actionReference.Name, err)
					}
					logs.Infof("Create actionCopy:%v", actionCopy.Name)
					for _, runtimeReference := range action.Status.Runtimes {
						runtime, err := gh.clientsManager.GetRuntime(runtimeReference.Name, runtimeReference.Namespace)
						if err != nil {
							logs.Errorf("Get runtime %s failed: %v", runtimeReference.Name, err)
						}
						runtimeCopy := controller.NewRuntimeInfoCopy(runtime, false)
						runtimeClient := gh.clientsManager.GetRuntimeClient(runtimeCopy.Namespace)
						_, err = runtimeClient.Client.Create(context.TODO(), runtimeCopy, metav1.CreateOptions{}) // 因为是创建同一个域内的Runtime副本，所以说副本的namespace和源任务相同，直接用源runtime的namespace
						if err != nil {
							logs.Errorf("Create copy runtime %s in local failed: %v", runtimeReference.Name, err)
						}
						logs.Infof("Create runtimeCopy:%v", runtimeCopy.Name)
					}
				}
				// 将副本group信息写入到etcd当中，目前还只适配本域内迁移
				logs.Trace("group:%v===================", groupCopy.Name)
				groupClient := gh.clientsManager.GetGroupClient(gr.Namespace)
				_, err = groupClient.Client.Create(context.TODO(), groupCopy, metav1.CreateOptions{}) // 因为是创建同一个域内的Group副本，所以说副本的namespace和源任务相同，直接用源group的namespace
				if err != nil {
					logs.Errorf("Create group:%s err: %v", groupCopy.Name, err)
				}

				// TODO 这里需要将副本信息写入到源任务的Copyinfo当中
				patchGroup, err := json.Marshal(map[string]interface{}{
					"spec": map[string]interface{}{
						"copy_info": map[string]string{groupCopy.Name: "local"}, //value值不同
					},
				})
				if err != nil {
					logs.Errorf("Json Marshal failed, err:%v", err)
				}
				patchResult, err := groupClient.Client.Patch(context.TODO(), gr.Name, ty.StrategicMergePatchType, patchGroup, metav1.PatchOptions{})
				if err != nil {
					logs.Errorf("Patch group error:%v", err)
				}
				logs.Infof("Source CopyInfo:[value:%v]", patchResult.Spec.CopyInfo[groupCopy.Name])
			}
		}
		if copiesInOtherDomain > 0 { // 说明有副本需要部署在其他域---后续可能要添加要求：部署在其他哪个域
			logs.Info("==================================================================copiesInOtherDomain")
			// 为了适配迁移 ,如果有多个副本要求的话，需要部署多个副本
			for i := 0; i < int(copiesInOtherDomain); i++ {
				// 复制创建一个全新的副本group信息（注意Succeed的Phase不用修改，DeployCheck和Running状态需要修改），另外还需要将副本的groupStatus改为Starting
				//groupCopyName := "Reason-Copy"           】                                    // TODO 这里之后改成随机生成即可源group.Name + 一串随机字符
				// 适配天数环境
				nodeName := "EdgeNode1"
				groupCopy := controller.NewGroupInfoCopy(gr, true, nodeName) // 第二个参数为true，表示的是提前写入etcd
				// 新增操作--5.20--将Group写入到Task当中
				gh.AddGroupCopyToTaskStatus(groupCopy)
				// 遍历action和Runtime，依次创建
				for _, actionReference := range gr.Status.Actions {
					action, err := gh.clientsManager.GetAction(actionReference.Name, actionReference.Namespace)
					if err != nil {
						logs.Errorf("Get action %s failed: %v", actionReference.Name, err)
					}
					actionCopy := controller.NewActionInfoCopy(action)
					actionClient := gh.clientsManager.GetActionClient(actionCopy.Namespace)
					_, err = actionClient.Client.Create(context.TODO(), actionCopy, metav1.CreateOptions{}) // 因为是创建同一个域内的Action副本，所以说副本的namespace和源任务相同，直接用源action的namespace
					if err != nil {
						logs.Errorf("Create copy action %s in local failed: %v", actionReference.Name, err)
					}
					logs.Infof("Create actionCopy:%v", actionCopy.Name)
					for _, runtimeReference := range action.Status.Runtimes {
						runtime, err := gh.clientsManager.GetRuntime(runtimeReference.Name, runtimeReference.Namespace)
						if err != nil {
							logs.Errorf("Get runtime %s failed: %v", runtimeReference.Name, err)
						}
						runtimeCopy := controller.NewRuntimeInfoCopy(runtime, true) // 适配天数环境
						runtimeClient := gh.clientsManager.GetRuntimeClient(runtimeCopy.Namespace)
						_, err = runtimeClient.Client.Create(context.TODO(), runtimeCopy, metav1.CreateOptions{}) // 因为是创建同一个域内的Runtime副本，所以说副本的namespace和源任务相同，直接用源runtime的namespace
						if err != nil {
							logs.Errorf("Create copy runtime %s in local failed: %v", runtimeReference.Name, err)
						}
						logs.Infof("Create runtimeCopy:%v", runtimeCopy.Name)
					}
				}
				// 将副本group信息写入到etcd当中，目前还只适配本域内迁移
				logs.Infof("group:%v===================", groupCopy.Name)
				groupClient := gh.clientsManager.GetGroupClient(gr.Namespace)
				_, err = groupClient.Client.Create(context.TODO(), groupCopy, metav1.CreateOptions{}) // 因为是创建同一个域内的Group副本，所以说副本的namespace和源任务相同，直接用源group的namespace
				if err != nil {
					logs.Errorf("Create group:%s err: %v", groupCopy.Name, err)
				}

				// TODO 这里需要将副本信息写入到源任务的Copyinfo当中
				patchGroup, err := json.Marshal(map[string]interface{}{
					"spec": map[string]interface{}{
						"copy_info": map[string]string{groupCopy.Name: "local"}, //value值不同
					},
				})
				if err != nil {
					logs.Errorf("Json Marshal failed, err:%v", err)
				}
				patchResult, err := groupClient.Client.Patch(context.TODO(), gr.Name, ty.StrategicMergePatchType, patchGroup, metav1.PatchOptions{})
				if err != nil {
					logs.Errorf("Patch group error:%v", err)
				}
				logs.Infof("Source CopyInfo:[value:%v]", patchResult.Spec.CopyInfo[groupCopy.Name])
			}

			//for i := 0; i < int(copiesInOtherDomain); i++ {
			//	// TODO （需要和调度器确认）发送一个事件通知调度器去选择一个域（不能为本域），事件里面放group信息--我已经生成好副本group了，调度器直接把这个group放到别的域即可
			//	// 复制创建一个全新的副本group信息（注意Succeed的Phase不用修改，DeployCheck和Running状态需要修改），另外还需要将副本的groupStatus改为Starting
			//	//groupCopyName := "Reason-Copy" // TODO 这里之后改成随机生成即可源group.Name + 一串随机字符
			//	groupCopy := controller.NewGroupInfoCopy(gr, true, "") // 第二个参数为true，表示的是提前写入etcd
			//	// 遍历action和Runtime，依次创建
			//	for _, actionReference := range gr.Status.Actions {
			//		action, err := gh.clientsManager.GetAction(actionReference.Name, actionReference.Namespace) // 从本域获得Action
			//		if err != nil {
			//			logs.Errorf("Get action %s failed: %v", actionReference.Name, err)
			//		}
			//		actionCopy := controller.NewActionInfoCopy(action)
			//		actionTarget, ok := gh.actionTarget["broker"] //-=-=-=-= 这里应该先根据nodeName查到clusterID，然后再使用这个ClusterID   TODO 目前跨域还没有适配指定namespace创建
			//		if !ok {
			//			logs.Info("[actionTarget]键 'broker' 不存在==========================================")
			//		}
			//		_, err = actionTarget.Create(context.TODO(), actionCopy, metav1.CreateOptions{})
			//		if err != nil {
			//			logs.Errorf("Create copy action %s in other domainfailed: %v", actionReference.Name, err)
			//		}
			//		logs.Infof("Create actionCopy:%v", actionCopy.Name)
			//		for _, runtimeReference := range action.Status.Runtimes {
			//			runtime, err := gh.clientsManager.GetRuntime(runtimeReference.Name, runtimeReference.Namespace)
			//			if err != nil {
			//				logs.Errorf("Get runtime %s failed: %v", runtimeReference.Name, err)
			//			}
			//			runtimeCopy := controller.NewRuntimeInfoCopy(runtime, true) // 区别，这里是true
			//			runtimeTarget, ok := gh.runtimeTarget["broker"]             //-=-=-=-= 这里应该先根据nodeName查到clusterID，然后再使用这个ClusterID
			//			if !ok {
			//				logs.Info("[runtimeTarget]键 'broker' 不存在==========================================")
			//			}
			//			_, err = runtimeTarget.Create(context.TODO(), runtimeCopy, metav1.CreateOptions{})
			//			if err != nil {
			//				logs.Errorf("Create copy runtime in other domain %s failed: %v", actionReference.Name, err)
			//			}
			//			logs.Infof("Create runtimeCopy:%v", runtimeCopy.Name)
			//		}
			//	}
			//	// 暂时做成，往跨域的etcd里写入数据
			//	groupTarget, ok := gh.groupTarget["broker"] // -=-=-=-=- TODO：这里还得加逻辑，就是有这个域的连接，才能填入这个key
			//	if !ok {
			//		logs.Info("[groupTarget]键 'broker' 不存在==========================================")
			//	}
			//	_, err = groupTarget.Create(context.TODO(), groupCopy, metav1.CreateOptions{})
			//	if err != nil {
			//		logs.Errorf("Create cross-domain group error:%v", err)
			//	}
			//	// 将跨域的连接写入到源任务的copyInfo当中
			//	patchGroup, err := json.Marshal(map[string]interface{}{
			//		"spec": map[string]interface{}{
			//			"copy_info": map[string]string{groupCopy.Name: "broker"},
			//		},
			//	})
			//	if err != nil {
			//		logs.Errorf("Json Marshal failed, err:%v", err)
			//	}
			//	_, err = gh.clientsManager.PatchGroup(gr.Name, gr.Namespace, patchGroup)
			//	if err != nil {
			//		logs.Errorf("Patch group error:%v", err)
			//	}
			//
			//	// TODO 这里需要监听调度器调度完成的事件，然后将副本信息填入到源group的copyInfo当中，先开启监听再发送事件给调度器
			//	//go gh.CheckEventForSchedulerResult(gr, groupCopyName)
			//	//gh.recorder.Event(groupCopy, apis.EventTypeNormal, events.SelectOtherDomain, fmt.Sprintf("Need Scheduler to choose the domain to cross"))
			//}
			//// TODO 这里得让调度器那边发送一个事件给我，我在这监听
		}
	}()
	// 任务满足条件，提交给GroupWorkers
	gh.groupWorkers.UpdateGroup(
		&group.UpdateGroupOptions{
			Group:      gr,
			StartTime:  start,
			UpdateType: group.GroupCreate,
		},
	)
	// TODO: 监控任务执行状态的组件
	// TODO: Probe Manager
}

// TODO 目前这块的功能还需要讨论，其实group的信息修改，是否调度器就可以修改，就不用让部署器去修改了，有待商榷
func (gh *GroupHandler) HandleGroupUpdate(gr *apis.Group) {
	// 覆盖Manager中对应的group信息
	gh.groupManager.UpdateGroup(gr)
	// TODO 同时修改etcd当中的group信息
}

func (gh *GroupHandler) HandleGroupKill(gr *apis.Group) {
	start := time.Now()
	logs.Infof("Start HandleGroupKill")
	// 遍历所有的Group,创建Group
	// 向 GroupWorkers 提交任务组的删除请求-hzy
	gh.groupWorkers.UpdateGroup(&group.UpdateGroupOptions{
		Group:      gr,
		StartTime:  start,
		UpdateType: group.GroupKill,
	})
	//gh.groupManager.DeleteGroup(gr)
}
func (gh *GroupHandler) HandleGroupStop(gr *apis.Group) {
	start := time.Now()
	logs.Infof("Start HandleGroupStop")
	// 遍历所有的Group,创建Group
	// 向 GroupWorkers 提交任务组的删除请求-hzy
	gh.groupWorkers.UpdateGroup(&group.UpdateGroupOptions{
		Group:      gr,
		StartTime:  start,
		UpdateType: group.GroupStop,
	})
	//gh.groupManager.DeleteGroup(gr)
}

func (gh *GroupHandler) HandleGroupRestore(gr *apis.Group) {
	start := time.Now()
	logs.Infof("Start HandleGroupRestore")
	//// 遍历所有的Group,创建Group
	//// 向 GroupWorkers 提交任务组的删除请求-hzy
	gh.groupWorkers.UpdateGroup(&group.UpdateGroupOptions{
		Group:      gr,
		StartTime:  start,
		UpdateType: group.GroupRestore,
	})
	////gh.groupManager.DeleteGroup(gr)
}

// TODO 检查本地资源是否可以启动该Group
func (gh *GroupHandler) checkResource(g *apis.Group) bool {
	// 首先检查一下这个group的状态是否为ReadyToDeploy
	logs.Infof("CheckResource：g.Status.Phase:%v", g.Status.Phase)
	if g.Status.Phase != apis.ReadyToDeploy {
		return false
	}
	//检查当前节点资源是否满足

	//检查当前节点是否满足Group的条件

	return true
}

// TODO 监听事件：当调度器调决定将group放置在哪个域上的时候，这时候需要往本域的etcd发送一个事件，这样的话我这里如果监听到这个事件，将将副本所在域的连接信息写入到源任务的copyInfo当中
func (gh *GroupHandler) CheckEventForSchedulerResult(gr *apis.Group, copyGroupName string) {
	nowtime := time.Now()
	fieldSelector := fmt.Sprintf("reason=%v", events.ScheduledToOtherDomain)
	watchOptions := meta.ListOptions{
		FieldSelector: fieldSelector,
	}
	watcher, err := gh.clientsManager.GetEventClient(gr.Namespace).Client.Watch(context.TODO(), watchOptions)
	if err != nil {
		logs.Errorf("Watch group error:%v", err)
	}
	defer watcher.Stop() // 确保 watcher 被停止
	watchChan := watcher.ResultChan()
	for {
		select {
		case event, ok := <-watchChan:
			if !ok {
				logs.Infof("watchChan closed")
				return
			}
			// 打印事件类型和对象的相关信息
			logs.Tracef("接收到事件类型: %v\n", event.Type)
			switch event.Type {
			case watch.Added:
				logs.Infof("资源被添加: ", event.Object)
				newEvent := event.Object.(*apis.Event)
				if newEvent.InvolvedObject.Name == copyGroupName && newEvent.EventTime.Time.After(nowtime) { //前者晚于后者返回true
					message := newEvent.Message // Message当中，调度器告诉我们，迁移到哪个域了
					// 将跨域的连接写入到源的copyInfo当中
					patchGroup, err := json.Marshal(map[string]interface{}{
						"spec": map[string]interface{}{
							"copy_info": map[string]string{copyGroupName: message},
						},
					})
					if err != nil {
						logs.Errorf("Json Marshal failed, err:%v", err)
					}
					_, err = gh.clientsManager.PatchGroup(gr.Name, gr.Namespace, patchGroup)
					if err != nil {
						logs.Errorf("Patch group error:%v", err)
					}
					return
				}
			default:
				logs.Infof("未识别的事件类型: ", event.Type)
			}
		}
	}
}

// 将Task下面的status添加入副本Group信息
func (gh *GroupHandler) AddGroupCopyToTaskStatus(gr *apis.Group) {
	logs.Infof("====================AddGroupCopyToTaskStatus")
	// 首先找到Group所属的Task（副本Group和原先的Group，目前本域迁移的话，所属的Group没有改动）
	taskName := gr.Status.Belong.Name
	taskNamespace := gr.Status.Belong.Namespace
	task, err2 := gh.clientsManager.GetTask(taskName, taskNamespace)
	if err2 != nil {
		logs.Errorf("Get task error:%v", err2)
	}
	taskStatusGroups := task.Status.Groups
	taskStatusGroups[gr.Spec.Name+"-copy"] = apis.ObjectReference{
		Name:      gr.Name,
		Namespace: gr.Namespace,
		Kind:      gr.Kind,
	}
	patchTask, err := json.Marshal(map[string]interface{}{
		"status": map[string]interface{}{
			"groups": &taskStatusGroups,
		},
	})
	_, err = gh.clientsManager.PatchTask(taskName, taskNamespace, patchTask)
	if err != nil {
		logs.Errorf("Patch task error:%v", err2)
	}

}
