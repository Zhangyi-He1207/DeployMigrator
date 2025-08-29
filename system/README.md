1、前置工作
安装Anaconda虚拟环境，安装应用所需的依赖

这里我们以我们当前测试的任务为例（yolo_projects_asy/yolo-asy-running.py），安装了conda虚拟环境。

2、系统启动

首先将当前文件夹拷贝至A、B节点，其中在A、B节点使用不同的framework.yaml。

其中A节点framework.yaml：

```
EtcdPort : 2379
ApiServerAddr : "http://127.0.0.1:10000" 
FileRegistryAddr : "http://127.0.0.1:8919"
NodeName : "EdgeNode1"
ClusterCategory : "Edge"
LocalClusterID : "Edge1"
IsMasterNode : true
```

B节点framework.yaml：

```
EtcdPort : 2379
ApiServerAddr : "http://172.150.0.24:10000" #ip needs to be changed to the ip address of the master node (EdgeNode1)
FileRegistryAddr : "http://172.150.0.24:8919" #ip needs to be changed to the ip address of the master node (EdgeNode1)
NodeName : "EdgeNode2"
ClusterCategory : "Edge"
LocalClusterID : "Edge1"
IsMasterNode : true
```

需要修改B节点framwork.yaml当中的ApiServerAddr、FileRegistryAddr的ip，为A节点的ip

然后分别执行./RunAll.sh，便可以在A、B节点启动项目。

3、确认应用描述文件

我们提供application.json文件，可以用于输入用户需要提交的应用

```
{
	"data": [ {"name": "yolo_projects_asy" }],
	"type": "command",
	"command": ["python" ],
	"args": ["yolo_projects_asy/yolo-asy-running.py"],
	"enable_control": true,
	"conditions": {
	  "formulas": [
		{
		  "condition_type": "ProgramDependency",
		  "from": "yolo_projects_asy/requirements.txt"
		}
	  ]
    },
	"hasReplca": true
}
```

可以修改自己的提交文件目录以及脚本名称，例如替换yolo_projects_asy、yolo-asy-running.py。

4、提交任务

这里我们提供了一个提交任务的脚本，用来提交可以提供迁移的任务，需要传入预先准备好的文件内容yolo_projects_asy，指定其绝对路径或者相对路径

```
./submit_task ../yolo_projects_asy
```

该脚本首先会将应用所需的文件夹(../yolo_projects_asy)上传至文件仓库，然后部署任务后，首先会从文件仓库下载应用所需的文件，然后启动应用。

当原任务、副本任务启动后，可以通过回车来模拟发送“迁移事件”，之后A节点的应用就会迁移到B节点去了



---

1、Work in advance
Install the Anaconda virtual environment and the dependencies for your application

Here we'll take our current test task (yolo_projects_asy/yolo-asy-running.py) and install the conda virtual environment.

2、The system starts

Start by copying the current folder to nodes A and B, where different framely.yaml are used.

Where A node framework.yaml:

```
EtcdPort : 2379
ApiServerAddr : "http://127.0.0.1:10000" 
FileRegistryAddr : "http://127.0.0.1:8919"
NodeName : "EdgeNode1"
ClusterCategory : "Edge"
LocalClusterID : "Edge1"
IsMasterNode : true
```

Where B node framework.yaml:

```
EtcdPort : 2379
ApiServerAddr : "http://172.150.0.24:10000" #ip needs to be changed to the ip address of the master node (EdgeNode1)
FileRegistryAddr : "http://172.150.0.24:8919" #ip needs to be changed to the ip address of the master node (EdgeNode1)
NodeName : "EdgeNode2"
ClusterCategory : "Edge"
LocalClusterID : "Edge1"
IsMasterNode : true
```

We need to change the ip of ApiServerAddr and FileRegistryAddr in framwork.yaml of node B to the ip of node A

Then run./RunAll.sh to start the project on node A and node B.

3、Confirm the application profile

We provide the application.json file, which can be used to enter the application that the user wants to submit

```
{
	"data": [ {"name": "yolo_projects_asy" }],
	"type": "command",
	"command": ["python" ],
	"args": ["yolo_projects_asy/yolo-asy-running.py"],
	"enable_control": true,
	"conditions": {
	  "formulas": [
		{
		  "condition_type": "ProgramDependency",
		  "from": "yolo_projects_asy/requirements.txt"
		}
	  ]
    },
	"hasReplca": true
}
```

You can change your commit directory and script name, such as yolo_projects_asy or yolo-asy-running.py.

4、Submit the task

Here we provide a script for submitting jobs that can provide migration.To submit a job, we pass in the preprepared yolo_projects_asy file and specify its absolute or relative path

```
./submit_task ../yolo_projects_asy
```

The script starts by adding the folders (.. /yolo_projects_asy) to the file repository, and then when the job is deployed, the files needed for the application are first downloaded from the repository and the application is launched.

When the original task and the replica task are started, we can simulate sending A "migration event" by pressing return, and then the application of node A will migrate to node B

